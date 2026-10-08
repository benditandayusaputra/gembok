#ifndef GEMBOK_SHA3_H
#define GEMBOK_SHA3_H

#include <stddef.h>
#include <stdint.h>

#ifdef __cplusplus
extern "C" {
#endif

#define SHA3_256_LEN 32u

typedef struct {
    uint64_t lane[25];
    size_t pos;
} sha3_256_ctx;

void sha3_256_init(sha3_256_ctx *c);
void sha3_256_update(sha3_256_ctx *c, const void *data, size_t len);
void sha3_256_final(sha3_256_ctx *c, uint8_t out[SHA3_256_LEN]);
void sha3_256(uint8_t out[SHA3_256_LEN], const void *data, size_t len);
void shake256(uint8_t *out, size_t out_len, const void *data, size_t len);

#ifdef __cplusplus
}
#endif

#endif
