#include <inttypes.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "gembok.h"
#include "gembok_io.h"
#include "sha3.h"

#define LABEL_SEED "GEMBOK-v1/benih"
#define LABEL_CHECK "GEMBOK-v1/cek"
#define DEV_KEY "!dev-gembok-gembok-gembok-gembok"
#define PUF_KEY_LEN 32u
#define PUF_KEY_BITS 256u
#define SIM_CHIP_SEED 1u
#define PUF_MODE_SIM 1u
#define PUF_MODE_DEV 2u
#define SECRET_SLOTS_LEN 192u
#define MOCK_REGS 14u
#define SHORT_POLL_LIMIT 8u
#define SCRATCH_VALUE 0x11u
#define LATE_VALUE 0x22u

#define CHECK(cond) check((cond) ? 1 : 0, #cond, __LINE__)
#define CHECK_ERR(expr, want) check_err((expr), (want), #expr, __LINE__)

typedef struct {
    uint32_t reg[MOCK_REGS];
    unsigned long reads;
    unsigned long writes;
} mock_chip;

typedef struct {
    gembok_io inner;
    unsigned long reads;
    unsigned long writes;
} spy;

static const char *const MODEL_EK_DIGEST[3] = {
    "7f4faed478d6eb0c47f7c6eb664d2fc2255becd0b9adc288d48244e15bf3516c",
    "ffb7bda7f4438aca71b7eba6fd0c9f0d077c959a87b50ee658debb4dbde34e8a",
    "f5c87fd6b330f95070c469a50b2853af7d0f0a288ded211bd74e6d9dd1ce1954"
};

static unsigned checks;
static unsigned failures;

static void check(int ok, const char *what, int line)
{
    checks++;
    if (!ok) {
        failures++;
        printf("GAGAL baris %d: %s\n", line, what);
    }
}

static void check_err(gembok_err got, gembok_err want, const char *what, int line)
{
    checks++;
    if (got != want) {
        failures++;
        printf("GAGAL baris %d: %s\n  hasil    0x%X (%s)\n  harapan  0x%X (%s)\n", line, what,
               (unsigned)got, gembok_strerror(got), (unsigned)want, gembok_strerror(want));
    }
}

static uint32_t mock_read32(void *ctx, uint32_t offset)
{
    mock_chip *m = ctx;

    m->reads++;
    return offset / 4u < MOCK_REGS ? m->reg[offset / 4u] : 0;
}

static void mock_write32(void *ctx, uint32_t offset, uint32_t value)
{
    mock_chip *m = ctx;

    (void)offset;
    (void)value;
    m->writes++;
}

static uint32_t spy_read32(void *ctx, uint32_t offset)
{
    spy *s = ctx;

    s->reads++;
    return s->inner.read32(s->inner.ctx, offset);
}

static void spy_write32(void *ctx, uint32_t offset, uint32_t value)
{
    spy *s = ctx;

    s->writes++;
    s->inner.write32(s->inner.ctx, offset, value);
}

static void fill(uint8_t *out, size_t len, const char *label, unsigned index)
{
    char text[64];
    int n = snprintf(text, sizeof text, "%s/%u", label, index);

    shake256(out, len, text, (size_t)n);
}

static int all_zero(const uint8_t *data, size_t len)
{
    uint8_t acc = 0;

    for (size_t i = 0; i < len; i++)
        acc |= data[i];
    return acc == 0;
}

static int digest_is(const uint8_t *data, size_t len, const char *want_hex)
{
    uint8_t digest[SHA3_256_LEN];
    char hex[2 * SHA3_256_LEN + 1];

    sha3_256(digest, data, len);
    for (size_t i = 0; i < sizeof digest; i++)
        snprintf(hex + 2 * i, 3, "%02x", digest[i]);
    return strcmp(hex, want_hex) == 0;
}

static void expected_tag(const uint8_t shared[GEMBOK_SHARED_LEN], const uint8_t *ctx,
                         size_t ctx_len, uint8_t tag[GEMBOK_TAG_LEN])
{
    sha3_256_ctx h;
    uint8_t len_byte = (uint8_t)ctx_len;

    sha3_256_init(&h);
    sha3_256_update(&h, GEMBOK_TAG_LABEL, strlen(GEMBOK_TAG_LABEL));
    sha3_256_update(&h, shared, GEMBOK_SHARED_LEN);
    sha3_256_update(&h, &len_byte, 1);
    sha3_256_update(&h, ctx, ctx_len);
    sha3_256_final(&h, tag);
}

static void expected_seeds(const uint8_t key[PUF_KEY_LEN], uint8_t d[GEMBOK_SEED_LEN],
                           uint8_t z[GEMBOK_SEED_LEN])
{
    uint8_t in[sizeof LABEL_SEED - 1 + PUF_KEY_LEN];
    uint8_t out[2 * GEMBOK_SEED_LEN];

    memcpy(in, LABEL_SEED, sizeof LABEL_SEED - 1);
    memcpy(in + sizeof LABEL_SEED - 1, key, PUF_KEY_LEN);
    shake256(out, sizeof out, in, sizeof in);
    memcpy(d, out, GEMBOK_SEED_LEN);
    memcpy(z, out + GEMBOK_SEED_LEN, GEMBOK_SEED_LEN);
}

static void expected_helper_check(const uint8_t key[PUF_KEY_LEN], const uint8_t *mask,
                                  size_t mask_len, uint8_t chk[GEMBOK_HELPER_CHK_LEN])
{
    uint8_t in[sizeof LABEL_CHECK - 1 + PUF_KEY_LEN + GEMBOK_HELPER_MASK_MAX];
    size_t label_len = sizeof LABEL_CHECK - 1;

    memcpy(in, LABEL_CHECK, label_len);
    memcpy(in + label_len, key, PUF_KEY_LEN);
    memcpy(in + label_len + PUF_KEY_LEN, mask, mask_len);
    shake256(chk, GEMBOK_HELPER_CHK_LEN, in, label_len + PUF_KEY_LEN + mask_len);
}

static uint64_t expected_puf_bound(unsigned oscillators, unsigned votes, unsigned win_log2)
{
    uint64_t per_vote = ((uint64_t)1 << win_log2) + 32u;

    return (uint64_t)(oscillators - 1u) * ((uint64_t)votes * per_vote + 16u) + 100000u;
}

static uint64_t larger(uint64_t x, uint64_t y)
{
    return x > y ? x : y;
}

static uint32_t model_ro_count(uint32_t seed, uint32_t index)
{
    uint32_t x = seed ^ (index * 0x9E3779B1u);

    x ^= x >> 16;
    x *= 0x85EBCA6Bu;
    x ^= x >> 13;
    x *= 0xC2B2AE35u;
    x ^= x >> 16;
    return 20000u + (x & 0x3FFu);
}

static unsigned model_puf_enroll(uint32_t seed, unsigned oscillators, unsigned thresh,
                                 uint8_t key[PUF_KEY_LEN], uint8_t mask[GEMBOK_HELPER_MASK_MAX])
{
    unsigned selected = 0;
    int skip = 0;

    memset(key, 0, PUF_KEY_LEN);
    memset(mask, 0, GEMBOK_HELPER_MASK_MAX);
    for (unsigned pair = 0; pair + 1 < oscillators; pair++) {
        long diff;

        if (skip || selected == PUF_KEY_BITS) {
            skip = 0;
            continue;
        }
        diff = (long)model_ro_count(seed, pair) - (long)model_ro_count(seed, pair + 1);
        if (labs(diff) < (long)thresh)
            continue;
        mask[pair >> 3] |= (uint8_t)(1u << (pair & 7u));
        if (diff > 0)
            key[selected >> 3] |= (uint8_t)(1u << (selected & 7u));
        selected++;
        skip = 1;
    }
    return selected;
}

static void test_mock_chip(void)
{
    mock_chip m;
    gembok_io io = {mock_read32, mock_write32, &m};
    gembok_io no_read = {NULL, mock_write32, &m};
    gembok_io no_write = {mock_read32, NULL, &m};
    gembok_dev dev;
    gembok_caps caps;
    uint8_t ek[GEMBOK_EK_MAX];
    uint8_t mask[GEMBOK_HELPER_MASK_MAX] = {0};
    uint8_t chk[GEMBOK_HELPER_CHK_LEN] = {0};
    uint32_t count_c = 0;
    uint32_t count_c1 = 0;
    uint32_t version = 0;

    memset(&m, 0, sizeof m);
    CHECK_ERR(gembok_init(NULL, &io), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_init(&dev, NULL), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_init(&dev, &no_read), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_init(&dev, &no_write), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_init(&dev, &io), GEMBOK_OK);
    CHECK(dev.poll_limit == GEMBOK_POLL_LIMIT_DEFAULT);

    m.reg[GEMBOK_REG_ID / 4] = 0x12345678u;
    CHECK_ERR(gembok_probe(&dev, &version), GEMBOK_DRV_ID);
    m.reg[GEMBOK_REG_ID / 4] = GEMBOK_ID_VALUE;
    m.reg[GEMBOK_REG_VERSION / 4] = 0x00020000u;
    CHECK_ERR(gembok_probe(&dev, &version), GEMBOK_DRV_VERSION);
    CHECK(version == 0x00020000u);
    m.reg[GEMBOK_REG_VERSION / 4] = 0x00010007u;
    CHECK_ERR(gembok_probe(&dev, &version), GEMBOK_OK);
    CHECK(version == 0x00010007u);
    CHECK_ERR(gembok_probe(&dev, NULL), GEMBOK_OK);

    gembok_set_poll_limit(&dev, 50);
    m.reads = 0;
    m.reg[GEMBOK_REG_STATUS / 4] = 0;
    CHECK_ERR(gembok_wait_ready(&dev), GEMBOK_DRV_BUSY);
    CHECK(m.reads == 50);

    m.writes = 0;
    m.reg[GEMBOK_REG_STATUS / 4] = GEMBOK_ST_BOOTED | GEMBOK_ST_BUSY;
    CHECK_ERR(gembok_enroll(&dev, 3, ek, gembok_ek_len(3)), GEMBOK_DRV_BUSY);
    CHECK_ERR(gembok_wipe(&dev), GEMBOK_DRV_BUSY);
    CHECK(m.writes == 0);

    m.reads = 0;
    m.reg[GEMBOK_REG_STATUS / 4] = GEMBOK_ST_BOOTED;
    CHECK_ERR(gembok_wipe(&dev), GEMBOK_DRV_TIMEOUT);
    CHECK(m.writes == 1);
    CHECK(m.reads == 52);

    m.reg[GEMBOK_REG_STATUS / 4] = GEMBOK_ST_BOOTED | GEMBOK_ST_DONE | GEMBOK_ST_ERROR | (9u << 4);
    m.reg[GEMBOK_REG_CYCLES / 4] = 4242;
    CHECK_ERR(gembok_wipe(&dev), (gembok_err)9);
    CHECK(gembok_err_is_chip((gembok_err)9));
    CHECK(gembok_last_cycles(&dev) == 4242);
    CHECK(gembok_err_is_chip(GEMBOK_CHIP_RATE));
    CHECK(!gembok_err_is_chip(GEMBOK_OK));
    CHECK(!gembok_err_is_chip(GEMBOK_DRV_TIMEOUT));

    m.reads = 0;
    m.reg[GEMBOK_REG_COOLDOWN / 4] = 300;
    CHECK_ERR(gembok_cooldown_wait(&dev), GEMBOK_DRV_TIMEOUT);
    CHECK(m.reads == 1 + 300 + GEMBOK_COOLDOWN_SLACK);
    m.reads = 0;
    m.reg[GEMBOK_REG_COOLDOWN / 4] = 0;
    CHECK_ERR(gembok_cooldown_wait(&dev), GEMBOK_OK);
    CHECK(m.reads == 1);

    m.writes = 0;
    m.reg[GEMBOK_REG_CAPS / 4] = 1026u << 16;
    gembok_get_caps(&dev, &caps);
    CHECK(caps.helper_mask_len == 0);
    CHECK(gembok_helper_mask_len(&dev) == 0);
    CHECK_ERR(gembok_puf_enroll(&dev, mask, GEMBOK_HELPER_MASK_MAX, chk), GEMBOK_DRV_CAPS);
    CHECK_ERR(gembok_puf_recon(&dev, mask, GEMBOK_HELPER_MASK_MAX, chk), GEMBOK_DRV_CAPS);
    CHECK_ERR(gembok_puf_measure(&dev, 0, &count_c, &count_c1), GEMBOK_DRV_CAPS);
    CHECK(m.writes == 0);

    m.reg[GEMBOK_REG_CAPS / 4] = (1025u << 16) | (5u << 8) | (4u << 3) | 4u | 1u;
    gembok_get_caps(&dev, &caps);
    CHECK(caps.oscillators == 1025 && caps.helper_mask_len == 128);
    CHECK(caps.debug == 1 && caps.puf_mode == 1 && caps.votes == 5);
    CHECK(gembok_helper_mask_len(&dev) == 128);
    CHECK_ERR(gembok_puf_enroll(&dev, mask, 96, chk), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_puf_recon(&dev, mask, 129, chk), GEMBOK_DRV_LEN);
    CHECK(m.writes == 0);
    m.reg[GEMBOK_REG_STATUS / 4] = GEMBOK_ST_BOOTED | GEMBOK_ST_DONE;
    m.reg[GEMBOK_REG_PUF_DBG / 4] = 0xFFF12345u;
    m.reg[GEMBOK_REG_PUF_DBG1 / 4] = 0x000ABCDEu;
    CHECK_ERR(gembok_puf_measure(&dev, GEMBOK_PUF_IDX_MAX + 1u, &count_c, &count_c1), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_puf_measure(&dev, 1023, &count_c, &count_c1), GEMBOK_OK);
    CHECK(count_c == 0x12345u && count_c1 == 0xABCDEu);

    m.reg[GEMBOK_REG_CAPS / 4] = 0x03000562u;
    gembok_get_caps(&dev, &caps);
    CHECK(caps.oscillators == 768 && caps.helper_mask_len == 96 && caps.puf_mode == 2);
    CHECK_ERR(gembok_puf_measure(&dev, 767, &count_c, &count_c1), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_puf_measure(&dev, 766, &count_c, &count_c1), GEMBOK_OK);
    gembok_decode_caps(769u << 16, &caps);
    CHECK(caps.helper_mask_len == 96);
    gembok_decode_caps(770u << 16, &caps);
    CHECK(caps.helper_mask_len == 97);
    gembok_decode_caps(2u << 16, &caps);
    CHECK(caps.helper_mask_len == 1);
    gembok_decode_caps(1u << 16, &caps);
    CHECK(caps.helper_mask_len == 0);

    gembok_set_poll_limit(&dev, 0);
    CHECK(dev.poll_limit == GEMBOK_POLL_LIMIT_DEFAULT);
}

static void test_mock_caps(void)
{
    mock_chip m;
    gembok_io io = {mock_read32, mock_write32, &m};
    gembok_dev dev;
    gembok_caps caps;
    uint8_t mask[GEMBOK_HELPER_MASK_MAX] = {0};
    uint8_t chk[GEMBOK_HELPER_CHK_LEN] = {0};

    gembok_decode_caps(0x03000562u, &caps);
    CHECK(caps.puf_mode == 2 && caps.debug == 0 && caps.win_log2 == 12);
    CHECK(caps.votes == 5 && caps.oscillators == 768 && caps.helper_mask_len == 96);
    CHECK(caps.puf_cycle_bound == 15943152u);
    CHECK(caps.puf_cycle_bound == expected_puf_bound(768, 5, 12));

    gembok_decode_caps((1025u << 16) | (15u << 8) | (16u << 3) | 4u, &caps);
    CHECK(caps.puf_mode == 0 && caps.debug == 1 && caps.win_log2 == 16);
    CHECK(caps.votes == 15 && caps.oscillators == 1025 && caps.helper_mask_len == 128);
    CHECK(caps.puf_cycle_bound == 1007240864u);

    gembok_decode_caps(0x0300051Du, &caps);
    CHECK(caps.puf_mode == 1 && caps.debug == 1 && caps.win_log2 == 3 && caps.votes == 5);
    CHECK(caps.puf_cycle_bound == 265672u);

    gembok_decode_caps((600u << 16) | (1u << 8) | (4u << 3), &caps);
    CHECK(caps.win_log2 == 4 && caps.votes == 1 && caps.oscillators == 600);
    CHECK(caps.helper_mask_len == 75 && caps.puf_cycle_bound == 138336u);

    gembok_decode_caps(0xF8u, &caps);
    CHECK(caps.win_log2 == 31 && caps.puf_mode == 0 && caps.debug == 0);

    memset(&m, 0, sizeof m);
    CHECK_ERR(gembok_init(&dev, &io), GEMBOK_OK);
    m.reg[GEMBOK_REG_CAPS / 4] = (1025u << 16) | (15u << 8) | (16u << 3) | 4u;
    gembok_get_caps(&dev, &caps);
    CHECK(gembok_command_poll_limit(&dev, GEMBOK_CMD_PUF_ENROLL) == 2014481728u);
    CHECK(gembok_command_poll_limit(&dev, GEMBOK_CMD_PUF_RECON) == 2014481728u);
    CHECK(gembok_command_poll_limit(&dev, GEMBOK_CMD_PUF_MEASURE) == GEMBOK_POLL_LIMIT_DEFAULT);
    CHECK(gembok_command_poll_limit(&dev, GEMBOK_CMD_PROVE) == GEMBOK_POLL_LIMIT_DEFAULT);
    gembok_set_poll_limit(&dev, 100);
    CHECK(gembok_command_poll_limit(&dev, GEMBOK_CMD_PUF_MEASURE) == 2u * (65536u + 2048u));
    CHECK(gembok_command_poll_limit(&dev, GEMBOK_CMD_KEYGEN) == 100);
    CHECK(gembok_command_poll_limit(NULL, GEMBOK_CMD_KEYGEN) == 0);

    m.reg[GEMBOK_REG_CAPS / 4] = 0x03000562u;
    gembok_get_caps(&dev, &caps);
    CHECK(gembok_command_poll_limit(&dev, GEMBOK_CMD_PUF_ENROLL) == 31886304u);
    gembok_set_poll_limit(&dev, 0);
    CHECK(gembok_command_poll_limit(&dev, GEMBOK_CMD_PUF_ENROLL) == GEMBOK_POLL_LIMIT_DEFAULT);
    CHECK(gembok_command_poll_limit(&dev, GEMBOK_CMD_PUF_ENROLL)
          == larger(GEMBOK_POLL_LIMIT_DEFAULT, 2u * expected_puf_bound(768, 5, 12)));

    m.reg[GEMBOK_REG_CAPS / 4] = 0x0300051Du;
    gembok_get_caps(&dev, &caps);
    gembok_set_poll_limit(&dev, 50);
    m.reg[GEMBOK_REG_STATUS / 4] = GEMBOK_ST_BOOTED;
    m.reads = 0;
    CHECK_ERR(gembok_puf_recon(&dev, mask, caps.helper_mask_len, chk), GEMBOK_DRV_TIMEOUT);
    CHECK(m.reads == 2u + 2u * 265672u);
    m.reads = 0;
    CHECK_ERR(gembok_wipe(&dev), GEMBOK_DRV_TIMEOUT);
    CHECK(m.reads == 52);
    m.reads = 0;
    CHECK_ERR(gembok_command(&dev, GEMBOK_CMD_PUF_ENROLL), GEMBOK_DRV_TIMEOUT);
    CHECK(m.reads == 2u + 2u * 265672u);
}

static void test_lengths(void)
{
    CHECK(gembok_ek_len(2) == 800 && gembok_ek_len(3) == 1184 && gembok_ek_len(4) == 1568);
    CHECK(gembok_dk_len(2) == 1632 && gembok_dk_len(3) == 2400 && gembok_dk_len(4) == 3168);
    CHECK(gembok_ct_len(2) == 768 && gembok_ct_len(3) == 1088 && gembok_ct_len(4) == 1568);
    CHECK(gembok_ek_len(1) == 0 && gembok_dk_len(5) == 0 && gembok_ct_len(0) == 0);
    CHECK(!gembok_k_valid(1) && gembok_k_valid(2) && gembok_k_valid(3) && gembok_k_valid(4));
    CHECK(!gembok_k_valid(5));
}

static void test_registers(gembok_dev *dev, const gembok_caps *caps)
{
    uint32_t version = 0;
    uint32_t st = gembok_status(dev);

    CHECK_ERR(gembok_probe(dev, &version), GEMBOK_OK);
    CHECK(version == 0x00010000u);
    CHECK(caps->oscillators >= GEMBOK_N_RO_MIN && caps->oscillators <= GEMBOK_N_RO_MAX);
    CHECK(caps->helper_mask_len == (caps->oscillators + 6u) / 8u);
    CHECK(gembok_helper_mask_len(dev) == caps->helper_mask_len);
    CHECK(caps->votes >= 1 && caps->votes <= 15 && caps->votes % 2u == 1);
    CHECK(caps->win_log2 >= 1 && caps->win_log2 <= 16);
    CHECK(caps->puf_cycle_bound == expected_puf_bound(caps->oscillators, caps->votes, caps->win_log2));
    CHECK(gembok_command_poll_limit(dev, GEMBOK_CMD_PUF_RECON)
          == larger(dev->poll_limit, 2u * caps->puf_cycle_bound));
    CHECK(caps->raw == gembok_reg_read(dev, GEMBOK_REG_CAPS));
    CHECK((st & GEMBOK_ST_BOOTED) != 0);
    CHECK((st & (GEMBOK_ST_BUSY | GEMBOK_ST_PUF_READY | GEMBOK_ST_COOLDOWN)) == 0);
    CHECK(gembok_cooldown(dev) == 0);
    CHECK(gembok_proofs(dev) == 0);
    CHECK(gembok_get_thresh(dev) == 64);
    CHECK_ERR(gembok_set_thresh(dev, 123), GEMBOK_OK);
    CHECK(gembok_get_thresh(dev) == 123);
    CHECK_ERR(gembok_set_thresh(dev, 64), GEMBOK_OK);
    CHECK(gembok_get_thresh(dev) == 64);
}

static void test_arguments(gembok_dev *dev, spy *bus, const gembok_caps *caps)
{
    static uint8_t ek[GEMBOK_EK_MAX];
    static uint8_t dk[GEMBOK_DK_MAX];
    static uint8_t ct[GEMBOK_CT_MAX];
    uint8_t seed[GEMBOK_SEED_LEN] = {0};
    uint8_t out[GEMBOK_SHARED_LEN];
    uint8_t mask[GEMBOK_HELPER_MASK_MAX] = {0};
    uint8_t chk[GEMBOK_HELPER_CHK_LEN] = {0};
    uint8_t ctx[GEMBOK_CTX_MAX + 1u] = {0};
    uint32_t count;
    size_t mask_len = caps->helper_mask_len;
    unsigned long reads = bus->reads;
    unsigned long writes = bus->writes;

    CHECK_ERR(gembok_keygen(dev, 1, seed, seed, ek, 800, dk, 1632), GEMBOK_DRV_K);
    CHECK_ERR(gembok_keygen(dev, 5, seed, seed, ek, 1568, dk, 3168), GEMBOK_DRV_K);
    CHECK_ERR(gembok_keygen(dev, 3, seed, seed, ek, 1183, dk, 2400), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_keygen(dev, 3, seed, seed, ek, 1184, dk, 2401), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_keygen(dev, 3, NULL, seed, ek, 1184, dk, 2400), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_keygen(dev, 3, seed, NULL, ek, 1184, dk, 2400), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_keygen(dev, 3, seed, seed, NULL, 1184, dk, 2400), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_keygen(NULL, 3, seed, seed, ek, 1184, dk, 2400), GEMBOK_DRV_ARG);

    CHECK_ERR(gembok_encaps(dev, 0, ek, 800, seed, ct, 768, out), GEMBOK_DRV_K);
    CHECK_ERR(gembok_encaps(dev, 2, ek, 1184, seed, ct, 768, out), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_encaps(dev, 2, ek, 800, seed, ct, 1088, out), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_encaps(dev, 2, ek, 800, NULL, ct, 768, out), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_encaps(dev, 2, ek, 800, seed, ct, 768, NULL), GEMBOK_DRV_ARG);

    CHECK_ERR(gembok_decaps(dev, 7, dk, 3168, ct, 1568, out), GEMBOK_DRV_K);
    CHECK_ERR(gembok_decaps(dev, 4, dk, 2400, ct, 1568, out), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_decaps(dev, 4, dk, 3168, ct, 1088, out), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_decaps(dev, 4, NULL, 3168, ct, 1568, out), GEMBOK_DRV_ARG);

    CHECK_ERR(gembok_check_ek(dev, 3, ek, 800), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_check_ek(dev, 6, ek, 1184), GEMBOK_DRV_K);
    CHECK_ERR(gembok_check_ek(dev, 3, NULL, 1184), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_check_dk(dev, 3, dk, 2399), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_check_dk(dev, 1, dk, 2400), GEMBOK_DRV_K);
    CHECK_ERR(gembok_check_dk(NULL, 3, dk, 2400), GEMBOK_DRV_ARG);

    CHECK_ERR(gembok_enroll(dev, 5, ek, 1568), GEMBOK_DRV_K);
    CHECK_ERR(gembok_enroll(dev, 4, ek, 1184), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_enroll(dev, 4, NULL, 1568), GEMBOK_DRV_ARG);

    CHECK_ERR(gembok_prove(dev, 1, ct, 768, ctx, 0, out), GEMBOK_DRV_K);
    CHECK_ERR(gembok_prove(dev, 3, ct, 768, ctx, 0, out), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_prove(dev, 3, ct, 1088, ctx, GEMBOK_CTX_MAX + 1u, out), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_prove(dev, 3, ct, 1088, NULL, 1, out), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_prove(dev, 3, ct, 1088, ctx, 0, NULL), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_prove(dev, 3, NULL, 1088, ctx, 0, out), GEMBOK_DRV_ARG);

    CHECK_ERR(gembok_puf_enroll(dev, NULL, mask_len, chk), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_puf_enroll(dev, mask, mask_len, NULL), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_puf_enroll(dev, mask, mask_len - 1u, chk), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_puf_enroll(dev, mask, mask_len + 1u, chk), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_puf_recon(dev, NULL, mask_len, chk), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_puf_recon(dev, mask, mask_len, NULL), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_puf_recon(dev, mask, 0, chk), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_puf_measure(dev, GEMBOK_PUF_IDX_MAX + 1u, &count, &count), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_puf_measure(dev, caps->oscillators - 1u, &count, &count), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_puf_measure(dev, 0, NULL, &count), GEMBOK_DRV_ARG);

    CHECK_ERR(gembok_command(dev, 16), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_command(NULL, GEMBOK_CMD_WIPE), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_mem_write(dev, GEMBOK_MEM_SIZE, seed, 1), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_mem_write(dev, GEMBOK_MEM_SIZE - 16u, seed, 17), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_mem_write(dev, 0, NULL, 4), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_mem_read(dev, GEMBOK_MEM_SIZE + 1u, out, 0), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_mem_read(dev, GEMBOK_MEM_SIZE - 16u, out, 17), GEMBOK_DRV_LEN);
    CHECK_ERR(gembok_mem_read(dev, 0, NULL, 4), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_wipe(NULL), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_wait_ready(NULL), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_set_thresh(NULL, 64), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_cooldown_wait(NULL), GEMBOK_DRV_ARG);
    CHECK_ERR(gembok_probe(NULL, NULL), GEMBOK_DRV_ARG);

    CHECK(bus->reads == reads);
    CHECK(bus->writes == writes);
}

static void test_commands_without_key(gembok_dev *dev, spy *bus, const gembok_caps *caps)
{
    static uint8_t ek[GEMBOK_EK_MAX];
    static uint8_t ct[GEMBOK_CT_MAX];
    uint8_t tag[GEMBOK_TAG_LEN];
    uint32_t count_c;
    uint32_t count_c1;

    CHECK_ERR(gembok_enroll(dev, 3, ek, gembok_ek_len(3)), GEMBOK_CHIP_NO_KEY);
    CHECK(gembok_last_cycles(dev) == 1);
    CHECK_ERR(gembok_prove(dev, 3, ct, gembok_ct_len(3), NULL, 0, tag), GEMBOK_CHIP_NO_KEY);
    CHECK_ERR(gembok_command(dev, 12), GEMBOK_CHIP_BAD_CMD);
    CHECK_ERR(gembok_command(dev, 0), GEMBOK_CHIP_BAD_CMD);
    if (!caps->debug || caps->puf_mode == PUF_MODE_DEV)
        CHECK_ERR(gembok_puf_measure(dev, 0, &count_c, &count_c1), GEMBOK_CHIP_BAD_CMD);

    spy_write32(bus, GEMBOK_REG_PARAM, 5);
    CHECK_ERR(gembok_command(dev, GEMBOK_CMD_KEYGEN), GEMBOK_CHIP_BAD_PARAM);
    spy_write32(bus, GEMBOK_REG_PARAM, 0);
    CHECK_ERR(gembok_command(dev, GEMBOK_CMD_ENROLL), GEMBOK_CHIP_BAD_PARAM);
    CHECK((gembok_status(dev) & GEMBOK_ST_ERROR) != 0);
    CHECK(gembok_proofs(dev) == 0);
}

static void test_bus_contract(gembok_dev *dev, spy *bus)
{
    static uint8_t ek[GEMBOK_EK_MAX];
    uint8_t scratch = SCRATCH_VALUE;
    uint32_t first;

    CHECK_ERR(gembok_enroll(dev, 3, ek, gembok_ek_len(3)), GEMBOK_CHIP_NO_KEY);
    memset(ek, 0, sizeof ek);
    CHECK_ERR(gembok_mem_write(dev, GEMBOK_MEM_EK, ek, gembok_ek_len(3)), GEMBOK_OK);
    spy_write32(bus, GEMBOK_REG_PARAM, 3);
    spy_write32(bus, GEMBOK_REG_CTRL, GEMBOK_CMD_CHECK_EK);
    first = spy_read32(bus, GEMBOK_REG_STATUS);
    CHECK((first & GEMBOK_ST_BUSY) != 0);
    CHECK((first & GEMBOK_ST_DONE) == 0);
    CHECK_ERR(gembok_wait_ready(dev), GEMBOK_OK);
    CHECK((gembok_status(dev) & (GEMBOK_ST_DONE | GEMBOK_ST_ERROR)) == GEMBOK_ST_DONE);

    CHECK_ERR(gembok_mem_write(dev, GEMBOK_MEM_TAG, &scratch, 1), GEMBOK_OK);
    spy_write32(bus, GEMBOK_REG_CTRL, GEMBOK_CMD_CHECK_EK);
    spy_write32(bus, GEMBOK_MEM_BASE + 4u * GEMBOK_MEM_TAG, LATE_VALUE);
    CHECK_ERR(gembok_wait_ready(dev), GEMBOK_OK);
    scratch = 0;
    CHECK_ERR(gembok_mem_read(dev, GEMBOK_MEM_TAG, &scratch, 1), GEMBOK_OK);
    CHECK(scratch == SCRATCH_VALUE);
}

static void test_timeout_recovery(gembok_dev *dev)
{
    static uint8_t ek[2][GEMBOK_EK_MAX];
    static uint8_t dk[2][GEMBOK_DK_MAX];
    uint8_t d[GEMBOK_SEED_LEN];
    uint8_t z[GEMBOK_SEED_LEN];
    size_t ek_len = gembok_ek_len(3);
    size_t dk_len = gembok_dk_len(3);

    fill(d, sizeof d, "batas-d", 0);
    fill(z, sizeof z, "batas-z", 0);
    gembok_set_poll_limit(dev, SHORT_POLL_LIMIT);
    CHECK_ERR(gembok_keygen(dev, 3, d, z, ek[0], ek_len, dk[0], dk_len), GEMBOK_DRV_TIMEOUT);
    CHECK((gembok_status(dev) & GEMBOK_ST_BUSY) != 0);
    CHECK_ERR(gembok_wait_ready(dev), GEMBOK_DRV_BUSY);
    CHECK_ERR(gembok_keygen(dev, 3, d, z, ek[0], ek_len, dk[0], dk_len), GEMBOK_DRV_BUSY);
    CHECK_ERR(gembok_set_thresh(dev, 99), GEMBOK_DRV_BUSY);
    gembok_set_poll_limit(dev, 0);
    CHECK_ERR(gembok_wait_ready(dev), GEMBOK_OK);
    CHECK_ERR(gembok_keygen(dev, 3, d, z, ek[0], ek_len, dk[0], dk_len), GEMBOK_OK);
    CHECK_ERR(gembok_keygen(dev, 3, d, z, ek[1], ek_len, dk[1], dk_len), GEMBOK_OK);
    CHECK(gembok_get_thresh(dev) == 64);
    CHECK(memcmp(ek[0], ek[1], ek_len) == 0);
    CHECK(memcmp(dk[0], dk[1], dk_len) == 0);
    CHECK(memcmp(dk[0] + dk_len - GEMBOK_SEED_LEN, z, sizeof z) == 0);
    CHECK(memcmp(dk[0] + 384u * 3u, ek[0], ek_len) == 0);
    CHECK(gembok_last_cycles(dev) > 1000);
    CHECK(gembok_last_cycles(dev) == gembok_cycles(dev));
}

static void test_open_mode(gembok_dev *dev, unsigned k)
{
    static uint8_t ek[GEMBOK_EK_MAX];
    static uint8_t dk[GEMBOK_DK_MAX];
    static uint8_t ct[GEMBOK_CT_MAX];
    static uint8_t bad[GEMBOK_DK_MAX];
    uint8_t d[GEMBOK_SEED_LEN];
    uint8_t z[GEMBOK_SEED_LEN];
    uint8_t m[GEMBOK_SEED_LEN];
    uint8_t shared_enc[GEMBOK_SHARED_LEN];
    uint8_t shared_dec[GEMBOK_SHARED_LEN];
    uint8_t shared_bad[GEMBOK_SHARED_LEN];
    size_t ek_len = gembok_ek_len(k);
    size_t dk_len = gembok_dk_len(k);
    size_t ct_len = gembok_ct_len(k);
    uint32_t cycles_valid;

    fill(d, sizeof d, "buka-d", k);
    fill(z, sizeof z, "buka-z", k);
    fill(m, sizeof m, "buka-m", k);
    CHECK_ERR(gembok_keygen(dev, k, d, z, ek, ek_len, dk, dk_len), GEMBOK_OK);
    CHECK_ERR(gembok_check_ek(dev, k, ek, ek_len), GEMBOK_OK);
    CHECK_ERR(gembok_check_dk(dev, k, dk, dk_len), GEMBOK_OK);
    CHECK_ERR(gembok_encaps(dev, k, ek, ek_len, m, ct, ct_len, shared_enc), GEMBOK_OK);
    CHECK_ERR(gembok_decaps(dev, k, dk, dk_len, ct, ct_len, shared_dec), GEMBOK_OK);
    CHECK(memcmp(shared_enc, shared_dec, sizeof shared_enc) == 0);
    cycles_valid = gembok_last_cycles(dev);

    ct[ct_len / 2] ^= 0x10;
    CHECK_ERR(gembok_decaps(dev, k, dk, dk_len, ct, ct_len, shared_bad), GEMBOK_OK);
    CHECK(memcmp(shared_enc, shared_bad, sizeof shared_enc) != 0);
    CHECK(gembok_last_cycles(dev) == cycles_valid);
    ct[ct_len / 2] ^= 0x10;

    memcpy(bad, ek, ek_len);
    bad[384] = 0xFF;
    bad[385] |= 0x0F;
    CHECK_ERR(gembok_check_ek(dev, k, bad, ek_len), GEMBOK_CHIP_BAD_EK);
    CHECK_ERR(gembok_check_ek(dev, k, ek, ek_len), GEMBOK_OK);
    CHECK_ERR(gembok_encaps(dev, k, bad, ek_len, m, ct, ct_len, shared_bad), GEMBOK_CHIP_BAD_EK);
    CHECK_ERR(gembok_encaps(dev, k, ek, ek_len, m, ct, ct_len, shared_bad), GEMBOK_OK);
    CHECK(memcmp(shared_enc, shared_bad, sizeof shared_enc) == 0);

    memcpy(bad, dk, dk_len);
    bad[dk_len - 2u * GEMBOK_SEED_LEN] ^= 0x01;
    CHECK_ERR(gembok_check_dk(dev, k, bad, dk_len), GEMBOK_CHIP_BAD_DK);
    CHECK_ERR(gembok_decaps(dev, k, bad, dk_len, ct, ct_len, shared_bad), GEMBOK_CHIP_BAD_DK);
    CHECK_ERR(gembok_check_dk(dev, k, dk, dk_len), GEMBOK_OK);
    memcpy(bad, dk, dk_len);
    bad[384u * k + 7u] ^= 0x80;
    CHECK_ERR(gembok_check_dk(dev, k, bad, dk_len), GEMBOK_CHIP_BAD_DK);
    CHECK_ERR(gembok_decaps(dev, k, dk, dk_len, ct, ct_len, shared_dec), GEMBOK_OK);
    CHECK(memcmp(shared_enc, shared_dec, sizeof shared_enc) == 0);
}

static void predict_puf(gembok_dev *dev, const gembok_caps *caps, uint8_t key[PUF_KEY_LEN],
                        uint8_t mask[GEMBOK_HELPER_MASK_MAX])
{
    if (caps->puf_mode == PUF_MODE_DEV) {
        memcpy(key, DEV_KEY, PUF_KEY_LEN);
        fill(mask, caps->helper_mask_len, "topeng", 0);
        CHECK_ERR(gembok_mem_write(dev, GEMBOK_MEM_HELPER_MASK, mask, caps->helper_mask_len),
                  GEMBOK_OK);
    } else {
        unsigned selected = model_puf_enroll(SIM_CHIP_SEED, caps->oscillators,
                                             gembok_get_thresh(dev), key, mask);

        CHECK(selected == PUF_KEY_BITS);
    }
}

static void test_rate_limiter(gembok_dev *dev)
{
    uint32_t proofs = gembok_proofs(dev);
    uint32_t left;

    CHECK((gembok_status(dev) & GEMBOK_ST_COOLDOWN) != 0);
    CHECK_ERR(gembok_command(dev, GEMBOK_CMD_PROVE), GEMBOK_CHIP_RATE);
    CHECK(gembok_proofs(dev) == proofs);
    left = gembok_cooldown(dev);
    CHECK(left > 0);
    CHECK_ERR(gembok_cooldown_wait(dev), GEMBOK_OK);
    CHECK(gembok_cooldown(dev) == 0);
    CHECK((gembok_status(dev) & GEMBOK_ST_COOLDOWN) == 0);
}

static void test_vault_level(gembok_dev *dev, const gembok_caps *caps, unsigned k,
                             const uint8_t key[PUF_KEY_LEN])
{
    static const size_t ctx_lens[] = {0, 1, 17, GEMBOK_CTX_MAX};
    static uint8_t ek[GEMBOK_EK_MAX];
    static uint8_t open_ek[GEMBOK_EK_MAX];
    static uint8_t open_dk[GEMBOK_DK_MAX];
    static uint8_t ct[GEMBOK_CT_MAX];
    uint8_t d[GEMBOK_SEED_LEN];
    uint8_t z[GEMBOK_SEED_LEN];
    uint8_t m[GEMBOK_SEED_LEN];
    uint8_t shared[GEMBOK_SHARED_LEN];
    uint8_t shared_dec[GEMBOK_SHARED_LEN];
    uint8_t ctx[GEMBOK_CTX_MAX];
    uint8_t tag[GEMBOK_TAG_LEN];
    uint8_t want[GEMBOK_TAG_LEN];
    uint8_t slots[SECRET_SLOTS_LEN];
    size_t ek_len = gembok_ek_len(k);
    size_t dk_len = gembok_dk_len(k);
    size_t ct_len = gembok_ct_len(k);
    uint32_t proofs = gembok_proofs(dev);

    expected_seeds(key, d, z);
    CHECK_ERR(gembok_keygen(dev, k, d, z, open_ek, ek_len, open_dk, dk_len), GEMBOK_OK);
    CHECK_ERR(gembok_enroll(dev, k, ek, ek_len), GEMBOK_OK);
    CHECK(memcmp(ek, open_ek, ek_len) == 0);
    if (caps->puf_mode == PUF_MODE_DEV)
        CHECK(digest_is(ek, ek_len, MODEL_EK_DIGEST[k - 2]));
    CHECK_ERR(gembok_mem_read(dev, GEMBOK_MEM_D, slots, sizeof slots), GEMBOK_OK);
    CHECK(all_zero(slots, sizeof slots));

    fill(m, sizeof m, "brankas-m", k);
    fill(ctx, sizeof ctx, "brankas-ctx", k);
    CHECK_ERR(gembok_encaps(dev, k, ek, ek_len, m, ct, ct_len, shared), GEMBOK_OK);
    CHECK_ERR(gembok_decaps(dev, k, open_dk, dk_len, ct, ct_len, shared_dec), GEMBOK_OK);
    CHECK(memcmp(shared, shared_dec, sizeof shared) == 0);

    for (size_t i = 0; i < sizeof ctx_lens / sizeof ctx_lens[0]; i++) {
        size_t ctx_len = ctx_lens[i];

        CHECK_ERR(gembok_cooldown_wait(dev), GEMBOK_OK);
        memset(tag, 0, sizeof tag);
        CHECK_ERR(gembok_prove(dev, k, ct, ct_len, ctx, ctx_len, tag), GEMBOK_OK);
        expected_tag(shared, ctx, ctx_len, want);
        CHECK(memcmp(tag, want, sizeof tag) == 0);
        CHECK_ERR(gembok_mem_read(dev, GEMBOK_MEM_D, slots, sizeof slots), GEMBOK_OK);
        CHECK(all_zero(slots, sizeof slots));
        test_rate_limiter(dev);
    }
    CHECK(gembok_proofs(dev) == proofs + sizeof ctx_lens / sizeof ctx_lens[0]);

    ct[3] ^= 0x04;
    CHECK_ERR(gembok_prove(dev, k, ct, ct_len, ctx, 1, tag), GEMBOK_OK);
    expected_tag(shared, ctx, 1, want);
    CHECK(memcmp(tag, want, sizeof tag) != 0);
    CHECK_ERR(gembok_cooldown_wait(dev), GEMBOK_OK);
}

static void test_vault(gembok_dev *dev, const gembok_caps *caps)
{
    static uint8_t memory[GEMBOK_MEM_SIZE];
    static uint8_t ek[2][GEMBOK_EK_MAX];
    uint8_t key[PUF_KEY_LEN];
    uint8_t want_mask[GEMBOK_HELPER_MASK_MAX];
    uint8_t want_chk[GEMBOK_HELPER_CHK_LEN];
    uint8_t mask[GEMBOK_HELPER_MASK_MAX];
    uint8_t chk[GEMBOK_HELPER_CHK_LEN];
    uint8_t bad_mask[GEMBOK_HELPER_MASK_MAX];
    uint8_t bad_chk[GEMBOK_HELPER_CHK_LEN];
    size_t mask_len = caps->helper_mask_len;
    size_t ek_len = gembok_ek_len(3);

    predict_puf(dev, caps, key, want_mask);
    expected_helper_check(key, want_mask, mask_len, want_chk);
    gembok_set_poll_limit(dev, SHORT_POLL_LIMIT);
    CHECK_ERR(gembok_puf_enroll(dev, mask, mask_len, chk), GEMBOK_OK);
    gembok_set_poll_limit(dev, 0);
    CHECK(gembok_last_cycles(dev) <= caps->puf_cycle_bound);
    CHECK((gembok_status(dev) & GEMBOK_ST_PUF_READY) != 0);
    CHECK(memcmp(mask, want_mask, mask_len) == 0);
    CHECK(memcmp(chk, want_chk, sizeof chk) == 0);

    for (unsigned k = 2; k <= 4; k++)
        test_vault_level(dev, caps, k, key);

    CHECK_ERR(gembok_enroll(dev, 3, ek[0], ek_len), GEMBOK_OK);
    CHECK_ERR(gembok_wipe(dev), GEMBOK_OK);
    CHECK((gembok_status(dev) & GEMBOK_ST_PUF_READY) == 0);
    CHECK_ERR(gembok_mem_read(dev, 0, memory, sizeof memory), GEMBOK_OK);
    CHECK(all_zero(memory, sizeof memory));
    CHECK_ERR(gembok_enroll(dev, 3, ek[1], ek_len), GEMBOK_CHIP_NO_KEY);

    memset(bad_mask, 0xFF, sizeof bad_mask);
    CHECK_ERR(gembok_mem_write(dev, GEMBOK_MEM_HELPER_MASK, bad_mask, sizeof bad_mask), GEMBOK_OK);
    gembok_set_poll_limit(dev, SHORT_POLL_LIMIT);
    CHECK_ERR(gembok_puf_recon(dev, mask, mask_len, chk), GEMBOK_OK);
    gembok_set_poll_limit(dev, 0);
    CHECK(gembok_last_cycles(dev) <= caps->puf_cycle_bound);
    CHECK((gembok_status(dev) & GEMBOK_ST_PUF_READY) != 0);
    CHECK_ERR(gembok_enroll(dev, 3, ek[1], ek_len), GEMBOK_OK);
    CHECK(memcmp(ek[0], ek[1], ek_len) == 0);

    memcpy(bad_chk, chk, sizeof chk);
    bad_chk[GEMBOK_HELPER_CHK_LEN - 1u] ^= 0x80;
    CHECK_ERR(gembok_puf_recon(dev, mask, mask_len, bad_chk), GEMBOK_CHIP_BAD_HELPER);
    CHECK((gembok_status(dev) & GEMBOK_ST_PUF_READY) == 0);
    CHECK_ERR(gembok_enroll(dev, 3, ek[1], ek_len), GEMBOK_CHIP_NO_KEY);

    if (caps->puf_mode == PUF_MODE_DEV) {
        memcpy(bad_mask, mask, mask_len);
        bad_mask[mask_len - 1u] ^= 0x01;
        CHECK_ERR(gembok_puf_recon(dev, bad_mask, mask_len, chk), GEMBOK_CHIP_BAD_HELPER);
        CHECK((gembok_status(dev) & GEMBOK_ST_PUF_READY) == 0);
    }

    CHECK_ERR(gembok_puf_recon(dev, mask, mask_len, chk), GEMBOK_OK);
    CHECK_ERR(gembok_enroll(dev, 3, ek[1], ek_len), GEMBOK_OK);
    CHECK(memcmp(ek[0], ek[1], ek_len) == 0);
}

static void test_puf_thresholds(gembok_dev *dev, const gembok_caps *caps)
{
    uint8_t key[PUF_KEY_LEN];
    uint8_t want_mask[GEMBOK_HELPER_MASK_MAX];
    uint8_t want_chk[GEMBOK_HELPER_CHK_LEN];
    uint8_t mask[GEMBOK_HELPER_MASK_MAX];
    uint8_t chk[GEMBOK_HELPER_CHK_LEN];
    uint8_t low_mask[GEMBOK_HELPER_MASK_MAX];
    size_t mask_len = caps->helper_mask_len;

    CHECK(model_puf_enroll(SIM_CHIP_SEED, caps->oscillators, 64, key, low_mask) == PUF_KEY_BITS);

    CHECK_ERR(gembok_set_thresh(dev, 900), GEMBOK_OK);
    CHECK(model_puf_enroll(SIM_CHIP_SEED, caps->oscillators, 900, key, want_mask) < PUF_KEY_BITS);
    CHECK_ERR(gembok_puf_enroll(dev, mask, mask_len, chk), GEMBOK_CHIP_PUF_FAIL);
    CHECK((gembok_status(dev) & GEMBOK_ST_PUF_READY) == 0);
    CHECK(gembok_last_cycles(dev) <= caps->puf_cycle_bound);

    CHECK_ERR(gembok_set_thresh(dev, 200), GEMBOK_OK);
    CHECK(model_puf_enroll(SIM_CHIP_SEED, caps->oscillators, 200, key, want_mask) == PUF_KEY_BITS);
    expected_helper_check(key, want_mask, mask_len, want_chk);
    CHECK_ERR(gembok_puf_enroll(dev, mask, mask_len, chk), GEMBOK_OK);
    CHECK(memcmp(mask, want_mask, mask_len) == 0);
    CHECK(memcmp(mask, low_mask, mask_len) != 0);
    CHECK(memcmp(chk, want_chk, sizeof chk) == 0);

    memset(mask, 0, mask_len);
    CHECK_ERR(gembok_puf_recon(dev, mask, mask_len, chk), GEMBOK_CHIP_PUF_FAIL);
    CHECK((gembok_status(dev) & GEMBOK_ST_PUF_READY) == 0);
    CHECK_ERR(gembok_puf_recon(dev, want_mask, mask_len, chk), GEMBOK_OK);
    CHECK_ERR(gembok_set_thresh(dev, 64), GEMBOK_OK);
}

static void measure_pair(gembok_dev *dev, unsigned pair)
{
    uint32_t count_c = 0;
    uint32_t count_c1 = 0;

    CHECK_ERR(gembok_puf_measure(dev, pair, &count_c, &count_c1), GEMBOK_OK);
    CHECK(count_c == model_ro_count(SIM_CHIP_SEED, pair));
    CHECK(count_c1 == model_ro_count(SIM_CHIP_SEED, pair + 1u));
}

static void test_puf_measure(gembok_dev *dev, const gembok_caps *caps)
{
    static const unsigned pairs[] = {0, 1, 2, 77, 500, 766};
    unsigned last = caps->oscillators - 2u;

    for (size_t i = 0; i < sizeof pairs / sizeof pairs[0]; i++) {
        if (pairs[i] <= last)
            measure_pair(dev, pairs[i]);
    }
    measure_pair(dev, last <= GEMBOK_PUF_IDX_MAX ? last : GEMBOK_PUF_IDX_MAX);
}

int main(void)
{
    spy bus = {{NULL, NULL, NULL}, 0, 0};
    gembok_io io = {spy_read32, spy_write32, &bus};
    gembok_dev dev;
    gembok_caps caps;
    char why[160];

    test_mock_chip();
    test_mock_caps();
    test_lengths();

    if (gembok_io_open(&bus.inner, GEMBOK_IO_BASE_DEFAULT, why, sizeof why) != 0) {
        fprintf(stderr, "test-driver: %s\n", why);
        return 2;
    }
    CHECK_ERR(gembok_init(&dev, &io), GEMBOK_OK);
    CHECK_ERR(gembok_wait_ready(&dev), GEMBOK_OK);
    gembok_get_caps(&dev, &caps);
    printf("Uji driver pada %s, mode PUF %u, %u osilator, %u suara, jendela 2^%u%s\n",
           gembok_io_name(), caps.puf_mode, caps.oscillators, caps.votes, caps.win_log2,
           caps.debug ? ", bangunan debug" : "");

    test_registers(&dev, &caps);
    test_arguments(&dev, &bus, &caps);
    test_commands_without_key(&dev, &bus, &caps);
    test_bus_contract(&dev, &bus);
    test_timeout_recovery(&dev);
    for (unsigned k = 2; k <= 4; k++)
        test_open_mode(&dev, k);
    test_vault(&dev, &caps);
    if (caps.puf_mode == PUF_MODE_SIM)
        test_puf_thresholds(&dev, &caps);
    if (caps.puf_mode == PUF_MODE_SIM && caps.debug)
        test_puf_measure(&dev, &caps);

    gembok_io_close(&bus.inner);
    printf("Driver: %u dari %u pemeriksaan lolos (%lu baca bus, %lu tulis bus)\n",
           checks - failures, checks, bus.reads, bus.writes);
    return failures == 0 ? 0 : 1;
}
