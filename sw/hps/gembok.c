#include "gembok.h"

#include <string.h>

#define POLY_BYTES 384u

static uint32_t rd(gembok_dev *dev, uint32_t offset)
{
    return dev->io.read32(dev->io.ctx, offset);
}

static void wr(gembok_dev *dev, uint32_t offset, uint32_t value)
{
    dev->io.write32(dev->io.ctx, offset, value);
}

static void mem_put(gembok_dev *dev, uint32_t addr, const uint8_t *src, size_t len)
{
    for (size_t i = 0; i < len; i++)
        wr(dev, GEMBOK_MEM_BASE + 4u * (addr + (uint32_t)i), src[i]);
}

static void mem_get(gembok_dev *dev, uint32_t addr, uint8_t *dst, size_t len)
{
    for (size_t i = 0; i < len; i++)
        dst[i] = (uint8_t)(rd(dev, GEMBOK_MEM_BASE + 4u * (addr + (uint32_t)i)) & 0xFFu);
}

static gembok_err wait_idle(gembok_dev *dev)
{
    for (uint32_t polls = 0; polls < dev->poll_limit; polls++) {
        uint32_t st = rd(dev, GEMBOK_REG_STATUS);

        if ((st & GEMBOK_ST_BOOTED) && !(st & GEMBOK_ST_BUSY))
            return GEMBOK_OK;
    }
    return GEMBOK_DRV_BUSY;
}

static void discard_stale_status(gembok_dev *dev)
{
    (void)rd(dev, GEMBOK_REG_STATUS);
}

static gembok_err execute(gembok_dev *dev, uint32_t cmd)
{
    uint64_t limit = gembok_command_poll_limit(dev, cmd);

    wr(dev, GEMBOK_REG_CTRL, cmd);
    discard_stale_status(dev);
    for (uint64_t polls = 0; polls < limit; polls++) {
        uint32_t st = rd(dev, GEMBOK_REG_STATUS);

        if ((st & GEMBOK_ST_DONE) && !(st & GEMBOK_ST_BUSY)) {
            dev->last_status = st;
            dev->last_cycles = rd(dev, GEMBOK_REG_CYCLES);
            return (gembok_err)((st >> GEMBOK_ST_ERR_SHIFT) & GEMBOK_ST_ERR_MASK);
        }
    }
    return GEMBOK_DRV_TIMEOUT;
}

static gembok_err begin(gembok_dev *dev, unsigned k)
{
    gembok_err e = wait_idle(dev);

    if (e != GEMBOK_OK)
        return e;
    wr(dev, GEMBOK_REG_PARAM, k);
    return GEMBOK_OK;
}

static uint32_t known_caps(gembok_dev *dev)
{
    if (dev->caps == 0)
        dev->caps = rd(dev, GEMBOK_REG_CAPS);
    return dev->caps;
}

static uint64_t window_cycles(unsigned win_log2)
{
    return (uint64_t)1 << win_log2;
}

static uint64_t puf_cycle_bound(unsigned oscillators, unsigned votes, unsigned win_log2)
{
    uint64_t pairs = oscillators > 0 ? oscillators - 1u : 0;
    uint64_t per_vote = window_cycles(win_log2) + GEMBOK_PUF_VOTE_SLACK;

    return pairs * ((uint64_t)votes * per_vote + GEMBOK_PUF_PAIR_SLACK) + GEMBOK_PUF_FIXED_SLACK;
}

static void load_public_key_part(gembok_dev *dev, unsigned k, const uint8_t *dk)
{
    size_t dk_pke_len = POLY_BYTES * k;
    size_t ek_len = gembok_ek_len(k);

    mem_put(dev, GEMBOK_MEM_EK, dk + dk_pke_len, ek_len);
    mem_put(dev, GEMBOK_MEM_H, dk + dk_pke_len + ek_len, GEMBOK_HASH_LEN);
}

int gembok_k_valid(unsigned k)
{
    return k == 2 || k == 3 || k == 4;
}

size_t gembok_ek_len(unsigned k)
{
    return gembok_k_valid(k) ? POLY_BYTES * k + 32u : 0;
}

size_t gembok_dk_len(unsigned k)
{
    return gembok_k_valid(k) ? 2u * POLY_BYTES * k + 96u : 0;
}

size_t gembok_ct_len(unsigned k)
{
    unsigned du = (k == 4) ? 11 : 10;
    unsigned dv = (k == 4) ? 5 : 4;

    return gembok_k_valid(k) ? 32u * (du * k + dv) : 0;
}

int gembok_err_is_chip(gembok_err e)
{
    return (int)e >= 1 && (int)e <= (int)GEMBOK_CHIP_LAST;
}

const char *gembok_strerror(gembok_err e)
{
    switch ((int)e) {
    case GEMBOK_OK:
        return "berhasil";
    case GEMBOK_CHIP_BAD_EK:
        return "chip: BAD_EK, kunci enkapsulasi gagal cek modulus";
    case GEMBOK_CHIP_BAD_DK:
        return "chip: BAD_DK, kunci dekapsulasi gagal cek hash";
    case GEMBOK_CHIP_BAD_HELPER:
        return "chip: BAD_HELPER, data bantu PUF tidak cocok dengan kunci yang dipulihkan";
    case GEMBOK_CHIP_BAD_PARAM:
        return "chip: BAD_PARAM, k bukan 2, 3, atau 4";
    case GEMBOK_CHIP_NO_KEY:
        return "chip: NO_KEY, kunci PUF belum siap (jalankan PUF_ENROLL atau PUF_RECON dulu)";
    case GEMBOK_CHIP_RATE:
        return "chip: RATE, pembatas laju masih aktif";
    case GEMBOK_CHIP_BAD_CMD:
        return "chip: BAD_CMD, perintah tidak dikenal atau tidak tersedia di bangunan ini";
    case GEMBOK_CHIP_PUF_FAIL:
        return "chip: PUF_FAIL, PUF tidak memberi tepat 256 bit";
    case GEMBOK_DRV_ARG:
        return "driver: argumen tidak sah";
    case GEMBOK_DRV_K:
        return "driver: k harus 2, 3, atau 4";
    case GEMBOK_DRV_LEN:
        return "driver: panjang data tidak sesuai dengan tingkat ML-KEM yang dipilih";
    case GEMBOK_DRV_ID:
        return "driver: register ID tidak berisi GEMB";
    case GEMBOK_DRV_VERSION:
        return "driver: versi IP tidak didukung";
    case GEMBOK_DRV_BUSY:
        return "driver: chip tetap sibuk atau belum selesai boot";
    case GEMBOK_DRV_TIMEOUT:
        return "driver: batas waktu habis saat menunggu chip";
    case GEMBOK_DRV_CAPS:
        return "driver: CAPS menyebut jumlah osilator di luar 2 sampai 1025";
    default:
        break;
    }
    return gembok_err_is_chip(e) ? "chip: kode galat tidak dikenal" : "driver: galat tidak dikenal";
}

gembok_err gembok_init(gembok_dev *dev, const gembok_io *io)
{
    if (dev == NULL || io == NULL || io->read32 == NULL || io->write32 == NULL)
        return GEMBOK_DRV_ARG;
    dev->io = *io;
    dev->poll_limit = GEMBOK_POLL_LIMIT_DEFAULT;
    dev->last_status = 0;
    dev->last_cycles = 0;
    dev->caps = 0;
    return GEMBOK_OK;
}

void gembok_set_poll_limit(gembok_dev *dev, uint32_t polls)
{
    dev->poll_limit = polls != 0 ? polls : GEMBOK_POLL_LIMIT_DEFAULT;
}

uint64_t gembok_command_poll_limit(gembok_dev *dev, uint32_t cmd)
{
    gembok_caps caps;
    uint64_t normal;
    uint64_t needed = 0;

    if (dev == NULL)
        return 0;
    normal = dev->poll_limit;

    if (cmd == GEMBOK_CMD_PUF_ENROLL || cmd == GEMBOK_CMD_PUF_RECON) {
        gembok_decode_caps(known_caps(dev), &caps);
        needed = 2u * caps.puf_cycle_bound;
    } else if (cmd == GEMBOK_CMD_PUF_MEASURE) {
        gembok_decode_caps(known_caps(dev), &caps);
        needed = 2u * (window_cycles(caps.win_log2) + GEMBOK_PUF_MEASURE_SLACK);
    }
    return needed > normal ? needed : normal;
}

gembok_err gembok_probe(gembok_dev *dev, uint32_t *version)
{
    uint32_t v;

    if (dev == NULL)
        return GEMBOK_DRV_ARG;
    if (rd(dev, GEMBOK_REG_ID) != GEMBOK_ID_VALUE)
        return GEMBOK_DRV_ID;
    v = rd(dev, GEMBOK_REG_VERSION);
    if (version != NULL)
        *version = v;
    if ((v >> 16) != GEMBOK_VERSION_MAJOR)
        return GEMBOK_DRV_VERSION;
    dev->caps = rd(dev, GEMBOK_REG_CAPS);
    return GEMBOK_OK;
}

gembok_err gembok_wait_ready(gembok_dev *dev)
{
    if (dev == NULL)
        return GEMBOK_DRV_ARG;
    return wait_idle(dev);
}

uint32_t gembok_reg_read(gembok_dev *dev, uint32_t offset)
{
    return rd(dev, offset);
}

uint32_t gembok_status(gembok_dev *dev)
{
    return rd(dev, GEMBOK_REG_STATUS);
}

void gembok_decode_caps(uint32_t raw, gembok_caps *caps)
{
    unsigned oscillators = (raw >> 16) & 0xFFFu;

    caps->raw = raw;
    caps->puf_mode = raw & 0x3u;
    caps->debug = (raw >> 2) & 0x1u;
    caps->win_log2 = (raw >> 3) & 0x1Fu;
    caps->votes = (raw >> 8) & 0xFFu;
    caps->oscillators = oscillators;
    caps->helper_mask_len = (oscillators >= GEMBOK_N_RO_MIN && oscillators <= GEMBOK_N_RO_MAX)
                            ? (oscillators + 6u) / 8u : 0;
    caps->puf_cycle_bound = puf_cycle_bound(oscillators, caps->votes, caps->win_log2);
}

void gembok_get_caps(gembok_dev *dev, gembok_caps *caps)
{
    dev->caps = rd(dev, GEMBOK_REG_CAPS);
    gembok_decode_caps(dev->caps, caps);
}

size_t gembok_helper_mask_len(gembok_dev *dev)
{
    gembok_caps caps;

    if (dev == NULL)
        return 0;
    gembok_decode_caps(known_caps(dev), &caps);
    return caps.helper_mask_len;
}

uint32_t gembok_cycles(gembok_dev *dev)
{
    return rd(dev, GEMBOK_REG_CYCLES);
}

uint32_t gembok_last_cycles(const gembok_dev *dev)
{
    return dev->last_cycles;
}

uint32_t gembok_cooldown(gembok_dev *dev)
{
    return rd(dev, GEMBOK_REG_COOLDOWN);
}

uint32_t gembok_proofs(gembok_dev *dev)
{
    return rd(dev, GEMBOK_REG_PROOFS);
}

gembok_err gembok_cooldown_wait(gembok_dev *dev)
{
    uint32_t left;
    uint64_t budget;

    if (dev == NULL)
        return GEMBOK_DRV_ARG;
    left = rd(dev, GEMBOK_REG_COOLDOWN);
    budget = (uint64_t)left + GEMBOK_COOLDOWN_SLACK;
    while (left != 0) {
        if (budget == 0)
            return GEMBOK_DRV_TIMEOUT;
        budget--;
        left = rd(dev, GEMBOK_REG_COOLDOWN);
    }
    return GEMBOK_OK;
}

uint16_t gembok_get_thresh(gembok_dev *dev)
{
    return (uint16_t)(rd(dev, GEMBOK_REG_PUF_THRESH) & 0xFFFFu);
}

gembok_err gembok_set_thresh(gembok_dev *dev, uint16_t thresh)
{
    gembok_err e;

    if (dev == NULL)
        return GEMBOK_DRV_ARG;
    e = wait_idle(dev);
    if (e != GEMBOK_OK)
        return e;
    wr(dev, GEMBOK_REG_PUF_THRESH, thresh);
    return GEMBOK_OK;
}

gembok_err gembok_command(gembok_dev *dev, uint32_t cmd)
{
    gembok_err e;

    if (dev == NULL || cmd > 15u)
        return GEMBOK_DRV_ARG;
    e = wait_idle(dev);
    if (e != GEMBOK_OK)
        return e;
    return execute(dev, cmd);
}

gembok_err gembok_mem_write(gembok_dev *dev, uint32_t addr, const uint8_t *data, size_t len)
{
    gembok_err e;

    if (dev == NULL || (data == NULL && len != 0))
        return GEMBOK_DRV_ARG;
    if (addr > GEMBOK_MEM_SIZE || len > GEMBOK_MEM_SIZE - addr)
        return GEMBOK_DRV_LEN;
    e = wait_idle(dev);
    if (e != GEMBOK_OK)
        return e;
    mem_put(dev, addr, data, len);
    return GEMBOK_OK;
}

gembok_err gembok_mem_read(gembok_dev *dev, uint32_t addr, uint8_t *data, size_t len)
{
    gembok_err e;

    if (dev == NULL || (data == NULL && len != 0))
        return GEMBOK_DRV_ARG;
    if (addr > GEMBOK_MEM_SIZE || len > GEMBOK_MEM_SIZE - addr)
        return GEMBOK_DRV_LEN;
    e = wait_idle(dev);
    if (e != GEMBOK_OK)
        return e;
    mem_get(dev, addr, data, len);
    return GEMBOK_OK;
}

gembok_err gembok_keygen(gembok_dev *dev, unsigned k,
                         const uint8_t d[GEMBOK_SEED_LEN], const uint8_t z[GEMBOK_SEED_LEN],
                         uint8_t *ek, size_t ek_len, uint8_t *dk, size_t dk_len)
{
    gembok_err e;
    size_t dk_pke_len;

    if (dev == NULL || d == NULL || z == NULL || ek == NULL || dk == NULL)
        return GEMBOK_DRV_ARG;
    if (!gembok_k_valid(k))
        return GEMBOK_DRV_K;
    if (ek_len != gembok_ek_len(k) || dk_len != gembok_dk_len(k))
        return GEMBOK_DRV_LEN;
    e = begin(dev, k);
    if (e != GEMBOK_OK)
        return e;
    mem_put(dev, GEMBOK_MEM_D, d, GEMBOK_SEED_LEN);
    mem_put(dev, GEMBOK_MEM_Z, z, GEMBOK_SEED_LEN);
    e = execute(dev, GEMBOK_CMD_KEYGEN);
    if (e != GEMBOK_OK)
        return e;
    dk_pke_len = POLY_BYTES * k;
    mem_get(dev, GEMBOK_MEM_EK, ek, ek_len);
    mem_get(dev, GEMBOK_MEM_DK, dk, dk_pke_len);
    memcpy(dk + dk_pke_len, ek, ek_len);
    mem_get(dev, GEMBOK_MEM_H, dk + dk_pke_len + ek_len, GEMBOK_HASH_LEN);
    memcpy(dk + dk_pke_len + ek_len + GEMBOK_HASH_LEN, z, GEMBOK_SEED_LEN);
    return GEMBOK_OK;
}

gembok_err gembok_encaps(gembok_dev *dev, unsigned k,
                         const uint8_t *ek, size_t ek_len, const uint8_t m[GEMBOK_SEED_LEN],
                         uint8_t *ct, size_t ct_len, uint8_t shared[GEMBOK_SHARED_LEN])
{
    gembok_err e;

    if (dev == NULL || ek == NULL || m == NULL || ct == NULL || shared == NULL)
        return GEMBOK_DRV_ARG;
    if (!gembok_k_valid(k))
        return GEMBOK_DRV_K;
    if (ek_len != gembok_ek_len(k) || ct_len != gembok_ct_len(k))
        return GEMBOK_DRV_LEN;
    e = begin(dev, k);
    if (e != GEMBOK_OK)
        return e;
    mem_put(dev, GEMBOK_MEM_EK, ek, ek_len);
    mem_put(dev, GEMBOK_MEM_M, m, GEMBOK_SEED_LEN);
    e = execute(dev, GEMBOK_CMD_ENCAPS);
    if (e != GEMBOK_OK)
        return e;
    mem_get(dev, GEMBOK_MEM_K, shared, GEMBOK_SHARED_LEN);
    mem_get(dev, GEMBOK_MEM_CT, ct, ct_len);
    return GEMBOK_OK;
}

gembok_err gembok_decaps(gembok_dev *dev, unsigned k,
                         const uint8_t *dk, size_t dk_len, const uint8_t *ct, size_t ct_len,
                         uint8_t shared[GEMBOK_SHARED_LEN])
{
    gembok_err e;
    size_t dk_pke_len;

    if (dev == NULL || dk == NULL || ct == NULL || shared == NULL)
        return GEMBOK_DRV_ARG;
    if (!gembok_k_valid(k))
        return GEMBOK_DRV_K;
    if (dk_len != gembok_dk_len(k) || ct_len != gembok_ct_len(k))
        return GEMBOK_DRV_LEN;
    e = begin(dev, k);
    if (e != GEMBOK_OK)
        return e;
    dk_pke_len = POLY_BYTES * k;
    mem_put(dev, GEMBOK_MEM_DK, dk, dk_pke_len);
    load_public_key_part(dev, k, dk);
    mem_put(dev, GEMBOK_MEM_Z, dk + dk_len - GEMBOK_SEED_LEN, GEMBOK_SEED_LEN);
    mem_put(dev, GEMBOK_MEM_CT, ct, ct_len);
    e = execute(dev, GEMBOK_CMD_DECAPS);
    if (e != GEMBOK_OK)
        return e;
    mem_get(dev, GEMBOK_MEM_K, shared, GEMBOK_SHARED_LEN);
    return GEMBOK_OK;
}

gembok_err gembok_check_ek(gembok_dev *dev, unsigned k, const uint8_t *ek, size_t ek_len)
{
    gembok_err e;

    if (dev == NULL || ek == NULL)
        return GEMBOK_DRV_ARG;
    if (!gembok_k_valid(k))
        return GEMBOK_DRV_K;
    if (ek_len != gembok_ek_len(k))
        return GEMBOK_DRV_LEN;
    e = begin(dev, k);
    if (e != GEMBOK_OK)
        return e;
    mem_put(dev, GEMBOK_MEM_EK, ek, ek_len);
    return execute(dev, GEMBOK_CMD_CHECK_EK);
}

gembok_err gembok_check_dk(gembok_dev *dev, unsigned k, const uint8_t *dk, size_t dk_len)
{
    gembok_err e;

    if (dev == NULL || dk == NULL)
        return GEMBOK_DRV_ARG;
    if (!gembok_k_valid(k))
        return GEMBOK_DRV_K;
    if (dk_len != gembok_dk_len(k))
        return GEMBOK_DRV_LEN;
    e = begin(dev, k);
    if (e != GEMBOK_OK)
        return e;
    load_public_key_part(dev, k, dk);
    return execute(dev, GEMBOK_CMD_CHECK_DK);
}

static gembok_err check_mask_len(gembok_dev *dev, size_t mask_len)
{
    size_t want = gembok_helper_mask_len(dev);

    if (want == 0)
        return GEMBOK_DRV_CAPS;
    return mask_len == want ? GEMBOK_OK : GEMBOK_DRV_LEN;
}

gembok_err gembok_puf_enroll(gembok_dev *dev, uint8_t *mask, size_t mask_len,
                             uint8_t chk[GEMBOK_HELPER_CHK_LEN])
{
    gembok_err e;

    if (dev == NULL || mask == NULL || chk == NULL)
        return GEMBOK_DRV_ARG;
    e = check_mask_len(dev, mask_len);
    if (e != GEMBOK_OK)
        return e;
    e = wait_idle(dev);
    if (e != GEMBOK_OK)
        return e;
    e = execute(dev, GEMBOK_CMD_PUF_ENROLL);
    if (e != GEMBOK_OK)
        return e;
    mem_get(dev, GEMBOK_MEM_HELPER_MASK, mask, mask_len);
    mem_get(dev, GEMBOK_MEM_HELPER_CHK, chk, GEMBOK_HELPER_CHK_LEN);
    return GEMBOK_OK;
}

gembok_err gembok_puf_recon(gembok_dev *dev, const uint8_t *mask, size_t mask_len,
                            const uint8_t chk[GEMBOK_HELPER_CHK_LEN])
{
    gembok_err e;

    if (dev == NULL || mask == NULL || chk == NULL)
        return GEMBOK_DRV_ARG;
    e = check_mask_len(dev, mask_len);
    if (e != GEMBOK_OK)
        return e;
    e = wait_idle(dev);
    if (e != GEMBOK_OK)
        return e;
    mem_put(dev, GEMBOK_MEM_HELPER_MASK, mask, mask_len);
    mem_put(dev, GEMBOK_MEM_HELPER_CHK, chk, GEMBOK_HELPER_CHK_LEN);
    return execute(dev, GEMBOK_CMD_PUF_RECON);
}

gembok_err gembok_puf_measure(gembok_dev *dev, unsigned pair,
                              uint32_t *count_c, uint32_t *count_c1)
{
    gembok_caps caps;
    gembok_err e;

    if (dev == NULL || count_c == NULL || count_c1 == NULL || pair > GEMBOK_PUF_IDX_MAX)
        return GEMBOK_DRV_ARG;
    gembok_decode_caps(known_caps(dev), &caps);
    if (caps.helper_mask_len == 0)
        return GEMBOK_DRV_CAPS;
    if (pair + 1u >= caps.oscillators)
        return GEMBOK_DRV_ARG;
    e = wait_idle(dev);
    if (e != GEMBOK_OK)
        return e;
    wr(dev, GEMBOK_REG_PUF_IDX, pair);
    e = execute(dev, GEMBOK_CMD_PUF_MEASURE);
    if (e != GEMBOK_OK)
        return e;
    *count_c = rd(dev, GEMBOK_REG_PUF_DBG) & GEMBOK_PUF_COUNT_MASK;
    *count_c1 = rd(dev, GEMBOK_REG_PUF_DBG1) & GEMBOK_PUF_COUNT_MASK;
    return GEMBOK_OK;
}

gembok_err gembok_enroll(gembok_dev *dev, unsigned k, uint8_t *ek, size_t ek_len)
{
    gembok_err e;

    if (dev == NULL || ek == NULL)
        return GEMBOK_DRV_ARG;
    if (!gembok_k_valid(k))
        return GEMBOK_DRV_K;
    if (ek_len != gembok_ek_len(k))
        return GEMBOK_DRV_LEN;
    e = begin(dev, k);
    if (e != GEMBOK_OK)
        return e;
    e = execute(dev, GEMBOK_CMD_ENROLL);
    if (e != GEMBOK_OK)
        return e;
    mem_get(dev, GEMBOK_MEM_EK, ek, ek_len);
    return GEMBOK_OK;
}

gembok_err gembok_prove(gembok_dev *dev, unsigned k,
                        const uint8_t *ct, size_t ct_len,
                        const uint8_t *ctx, size_t ctx_len,
                        uint8_t tag[GEMBOK_TAG_LEN])
{
    gembok_err e;

    if (dev == NULL || ct == NULL || tag == NULL || (ctx == NULL && ctx_len != 0))
        return GEMBOK_DRV_ARG;
    if (!gembok_k_valid(k))
        return GEMBOK_DRV_K;
    if (ct_len != gembok_ct_len(k) || ctx_len > GEMBOK_CTX_MAX)
        return GEMBOK_DRV_LEN;
    e = begin(dev, k);
    if (e != GEMBOK_OK)
        return e;
    wr(dev, GEMBOK_REG_CTXLEN, (uint32_t)ctx_len);
    mem_put(dev, GEMBOK_MEM_CTX, ctx, ctx_len);
    mem_put(dev, GEMBOK_MEM_CT, ct, ct_len);
    e = execute(dev, GEMBOK_CMD_PROVE);
    if (e != GEMBOK_OK)
        return e;
    mem_get(dev, GEMBOK_MEM_TAG, tag, GEMBOK_TAG_LEN);
    return GEMBOK_OK;
}

gembok_err gembok_wipe(gembok_dev *dev)
{
    gembok_err e;

    if (dev == NULL)
        return GEMBOK_DRV_ARG;
    e = wait_idle(dev);
    if (e != GEMBOK_OK)
        return e;
    return execute(dev, GEMBOK_CMD_WIPE);
}
