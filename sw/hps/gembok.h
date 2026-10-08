#ifndef GEMBOK_H
#define GEMBOK_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define GEMBOK_REG_ID 0x0000u
#define GEMBOK_REG_VERSION 0x0004u
#define GEMBOK_REG_CTRL 0x0008u
#define GEMBOK_REG_STATUS 0x000Cu
#define GEMBOK_REG_PARAM 0x0010u
#define GEMBOK_REG_CTXLEN 0x0014u
#define GEMBOK_REG_CYCLES 0x0018u
#define GEMBOK_REG_CAPS 0x001Cu
#define GEMBOK_REG_COOLDOWN 0x0020u
#define GEMBOK_REG_PROOFS 0x0024u
#define GEMBOK_REG_PUF_THRESH 0x0028u
#define GEMBOK_REG_PUF_IDX 0x002Cu
#define GEMBOK_REG_PUF_DBG 0x0030u
#define GEMBOK_REG_PUF_DBG1 0x0034u

#define GEMBOK_SPAN 0x10000u
#define GEMBOK_MEM_BASE 0x8000u
#define GEMBOK_MEM_SIZE 8192u

#define GEMBOK_ID_VALUE 0x47454D42u
#define GEMBOK_VERSION_MAJOR 1u
#define GEMBOK_CLOCK_HZ 50000000u

#define GEMBOK_ST_BUSY 0x001u
#define GEMBOK_ST_DONE 0x002u
#define GEMBOK_ST_ERROR 0x004u
#define GEMBOK_ST_ERR_SHIFT 4u
#define GEMBOK_ST_ERR_MASK 0xFu
#define GEMBOK_ST_PUF_READY 0x100u
#define GEMBOK_ST_COOLDOWN 0x200u
#define GEMBOK_ST_BOOTED 0x400u

#define GEMBOK_CMD_KEYGEN 1u
#define GEMBOK_CMD_ENCAPS 2u
#define GEMBOK_CMD_DECAPS 3u
#define GEMBOK_CMD_CHECK_EK 4u
#define GEMBOK_CMD_CHECK_DK 5u
#define GEMBOK_CMD_ENROLL 6u
#define GEMBOK_CMD_PROVE 7u
#define GEMBOK_CMD_PUF_ENROLL 8u
#define GEMBOK_CMD_PUF_RECON 9u
#define GEMBOK_CMD_PUF_MEASURE 10u
#define GEMBOK_CMD_WIPE 15u

#define GEMBOK_MEM_EK 0x0000u
#define GEMBOK_MEM_CT 0x0800u
#define GEMBOK_MEM_DK 0x1000u
#define GEMBOK_MEM_D 0x1800u
#define GEMBOK_MEM_Z 0x1820u
#define GEMBOK_MEM_M 0x1840u
#define GEMBOK_MEM_K 0x1860u
#define GEMBOK_MEM_SIGMA 0x1880u
#define GEMBOK_MEM_KBAR 0x18A0u
#define GEMBOK_MEM_H 0x18C0u
#define GEMBOK_MEM_TAG 0x18E0u
#define GEMBOK_MEM_CTX 0x1900u
#define GEMBOK_MEM_HELPER_MASK 0x1A00u
#define GEMBOK_MEM_HELPER_CHK 0x1A80u

#define GEMBOK_SEED_LEN 32u
#define GEMBOK_SHARED_LEN 32u
#define GEMBOK_HASH_LEN 32u
#define GEMBOK_TAG_LEN 32u
#define GEMBOK_CTX_MAX 255u
#define GEMBOK_HELPER_MASK_MAX 128u
#define GEMBOK_HELPER_CHK_LEN 16u
#define GEMBOK_N_RO_MIN 2u
#define GEMBOK_N_RO_MAX 1025u
#define GEMBOK_EK_MAX 1568u
#define GEMBOK_DK_MAX 3168u
#define GEMBOK_CT_MAX 1568u
#define GEMBOK_PUF_IDX_MAX 1023u
#define GEMBOK_PUF_COUNT_MASK 0xFFFFFu

#define GEMBOK_TAG_LABEL "GEMBOK-v1/bukti"

#define GEMBOK_POLL_LIMIT_DEFAULT 50000000u
#define GEMBOK_COOLDOWN_SLACK 1024u
#define GEMBOK_PUF_VOTE_SLACK 32u
#define GEMBOK_PUF_PAIR_SLACK 16u
#define GEMBOK_PUF_FIXED_SLACK 100000u
#define GEMBOK_PUF_MEASURE_SLACK 2048u

typedef enum {
    GEMBOK_OK = 0,
    GEMBOK_CHIP_BAD_EK = 1,
    GEMBOK_CHIP_BAD_DK = 2,
    GEMBOK_CHIP_BAD_HELPER = 3,
    GEMBOK_CHIP_BAD_PARAM = 4,
    GEMBOK_CHIP_NO_KEY = 5,
    GEMBOK_CHIP_RATE = 6,
    GEMBOK_CHIP_BAD_CMD = 7,
    GEMBOK_CHIP_PUF_FAIL = 8,
    GEMBOK_CHIP_LAST = 15,
    GEMBOK_DRV_ARG = 0x100,
    GEMBOK_DRV_K = 0x101,
    GEMBOK_DRV_LEN = 0x102,
    GEMBOK_DRV_ID = 0x103,
    GEMBOK_DRV_VERSION = 0x104,
    GEMBOK_DRV_BUSY = 0x105,
    GEMBOK_DRV_TIMEOUT = 0x106,
    GEMBOK_DRV_CAPS = 0x107
} gembok_err;

typedef uint32_t (*gembok_read32_fn)(void *ctx, uint32_t offset);
typedef void (*gembok_write32_fn)(void *ctx, uint32_t offset, uint32_t value);

typedef struct {
    gembok_read32_fn read32;
    gembok_write32_fn write32;
    void *ctx;
} gembok_io;

typedef struct {
    gembok_io io;
    uint32_t poll_limit;
    uint32_t last_status;
    uint32_t last_cycles;
    uint32_t caps;
} gembok_dev;

typedef struct {
    uint32_t raw;
    unsigned puf_mode;
    unsigned debug;
    unsigned win_log2;
    unsigned votes;
    unsigned oscillators;
    size_t helper_mask_len;
    uint64_t puf_cycle_bound;
} gembok_caps;

int gembok_k_valid(unsigned k);
size_t gembok_ek_len(unsigned k);
size_t gembok_dk_len(unsigned k);
size_t gembok_ct_len(unsigned k);

int gembok_err_is_chip(gembok_err e);
const char *gembok_strerror(gembok_err e);

gembok_err gembok_init(gembok_dev *dev, const gembok_io *io);
void gembok_set_poll_limit(gembok_dev *dev, uint32_t polls);
uint64_t gembok_command_poll_limit(gembok_dev *dev, uint32_t cmd);
gembok_err gembok_probe(gembok_dev *dev, uint32_t *version);
gembok_err gembok_wait_ready(gembok_dev *dev);

uint32_t gembok_reg_read(gembok_dev *dev, uint32_t offset);
uint32_t gembok_status(gembok_dev *dev);
void gembok_decode_caps(uint32_t raw, gembok_caps *caps);
void gembok_get_caps(gembok_dev *dev, gembok_caps *caps);
size_t gembok_helper_mask_len(gembok_dev *dev);
uint32_t gembok_cycles(gembok_dev *dev);
uint32_t gembok_last_cycles(const gembok_dev *dev);
uint32_t gembok_cooldown(gembok_dev *dev);
uint32_t gembok_proofs(gembok_dev *dev);
gembok_err gembok_cooldown_wait(gembok_dev *dev);
uint16_t gembok_get_thresh(gembok_dev *dev);
gembok_err gembok_set_thresh(gembok_dev *dev, uint16_t thresh);

gembok_err gembok_command(gembok_dev *dev, uint32_t cmd);
gembok_err gembok_mem_write(gembok_dev *dev, uint32_t addr, const uint8_t *data, size_t len);
gembok_err gembok_mem_read(gembok_dev *dev, uint32_t addr, uint8_t *data, size_t len);

gembok_err gembok_keygen(gembok_dev *dev, unsigned k,
                         const uint8_t d[GEMBOK_SEED_LEN], const uint8_t z[GEMBOK_SEED_LEN],
                         uint8_t *ek, size_t ek_len, uint8_t *dk, size_t dk_len);
gembok_err gembok_encaps(gembok_dev *dev, unsigned k,
                         const uint8_t *ek, size_t ek_len, const uint8_t m[GEMBOK_SEED_LEN],
                         uint8_t *ct, size_t ct_len, uint8_t shared[GEMBOK_SHARED_LEN]);
gembok_err gembok_decaps(gembok_dev *dev, unsigned k,
                         const uint8_t *dk, size_t dk_len, const uint8_t *ct, size_t ct_len,
                         uint8_t shared[GEMBOK_SHARED_LEN]);
gembok_err gembok_check_ek(gembok_dev *dev, unsigned k, const uint8_t *ek, size_t ek_len);
gembok_err gembok_check_dk(gembok_dev *dev, unsigned k, const uint8_t *dk, size_t dk_len);

gembok_err gembok_puf_enroll(gembok_dev *dev, uint8_t *mask, size_t mask_len,
                             uint8_t chk[GEMBOK_HELPER_CHK_LEN]);
gembok_err gembok_puf_recon(gembok_dev *dev, const uint8_t *mask, size_t mask_len,
                            const uint8_t chk[GEMBOK_HELPER_CHK_LEN]);
gembok_err gembok_puf_measure(gembok_dev *dev, unsigned pair,
                              uint32_t *count_c, uint32_t *count_c1);
gembok_err gembok_enroll(gembok_dev *dev, unsigned k, uint8_t *ek, size_t ek_len);
gembok_err gembok_prove(gembok_dev *dev, unsigned k,
                        const uint8_t *ct, size_t ct_len,
                        const uint8_t *ctx, size_t ctx_len,
                        uint8_t tag[GEMBOK_TAG_LEN]);
gembok_err gembok_wipe(gembok_dev *dev);

#ifdef __cplusplus
}
#endif

#endif
