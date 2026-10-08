#include "sha3.h"

#define RATE 136u
#define SUFFIX_SHA3 0x06u
#define SUFFIX_SHAKE 0x1Fu

static const uint64_t ROUND_CONST[24] = {
    0x0000000000000001ull, 0x0000000000008082ull, 0x800000000000808Aull, 0x8000000080008000ull,
    0x000000000000808Bull, 0x0000000080000001ull, 0x8000000080008081ull, 0x8000000000008009ull,
    0x000000000000008Aull, 0x0000000000000088ull, 0x0000000080008009ull, 0x000000008000000Aull,
    0x000000008000808Bull, 0x800000000000008Bull, 0x8000000000008089ull, 0x8000000000008003ull,
    0x8000000000008002ull, 0x8000000000000080ull, 0x000000000000800Aull, 0x800000008000000Aull,
    0x8000000080008081ull, 0x8000000000008080ull, 0x0000000080000001ull, 0x8000000080008008ull
};

static const unsigned RHO[24] = {
    1, 3, 6, 10, 15, 21, 28, 36, 45, 55, 2, 14, 27, 41, 56, 8, 25, 43, 62, 18, 39, 61, 20, 44
};

static const unsigned PI[24] = {
    10, 7, 11, 17, 18, 3, 5, 16, 8, 21, 24, 4, 15, 23, 19, 13, 12, 2, 20, 14, 22, 9, 6, 1
};

static uint64_t rotl(uint64_t x, unsigned n)
{
    return (x << n) | (x >> (64u - n));
}

static void keccak_f1600(uint64_t s[25])
{
    for (unsigned round = 0; round < 24; round++) {
        uint64_t c[5];
        uint64_t t;

        for (unsigned x = 0; x < 5; x++)
            c[x] = s[x] ^ s[x + 5] ^ s[x + 10] ^ s[x + 15] ^ s[x + 20];
        for (unsigned x = 0; x < 5; x++) {
            t = c[(x + 4) % 5] ^ rotl(c[(x + 1) % 5], 1);
            for (unsigned y = 0; y < 25; y += 5)
                s[y + x] ^= t;
        }

        t = s[1];
        for (unsigned i = 0; i < 24; i++) {
            uint64_t next = s[PI[i]];
            s[PI[i]] = rotl(t, RHO[i]);
            t = next;
        }

        for (unsigned y = 0; y < 25; y += 5) {
            for (unsigned x = 0; x < 5; x++)
                c[x] = s[y + x];
            for (unsigned x = 0; x < 5; x++)
                s[y + x] = c[x] ^ (~c[(x + 1) % 5] & c[(x + 2) % 5]);
        }

        s[0] ^= ROUND_CONST[round];
    }
}

static void xor_byte(uint64_t s[25], size_t pos, uint8_t b)
{
    s[pos / 8] ^= (uint64_t)b << (8 * (pos % 8));
}

static uint8_t get_byte(const uint64_t s[25], size_t pos)
{
    return (uint8_t)(s[pos / 8] >> (8 * (pos % 8)));
}

static void sponge_init(sha3_256_ctx *c)
{
    for (unsigned i = 0; i < 25; i++)
        c->lane[i] = 0;
    c->pos = 0;
}

static void sponge_absorb(sha3_256_ctx *c, const uint8_t *data, size_t len)
{
    for (size_t i = 0; i < len; i++) {
        xor_byte(c->lane, c->pos, data[i]);
        c->pos++;
        if (c->pos == RATE) {
            keccak_f1600(c->lane);
            c->pos = 0;
        }
    }
}

static void sponge_pad(sha3_256_ctx *c, uint8_t suffix)
{
    xor_byte(c->lane, c->pos, suffix);
    xor_byte(c->lane, RATE - 1, 0x80);
    keccak_f1600(c->lane);
    c->pos = 0;
}

static void sponge_squeeze(sha3_256_ctx *c, uint8_t *out, size_t len)
{
    for (size_t i = 0; i < len; i++) {
        if (c->pos == RATE) {
            keccak_f1600(c->lane);
            c->pos = 0;
        }
        out[i] = get_byte(c->lane, c->pos);
        c->pos++;
    }
}

void sha3_256_init(sha3_256_ctx *c)
{
    sponge_init(c);
}

void sha3_256_update(sha3_256_ctx *c, const void *data, size_t len)
{
    sponge_absorb(c, (const uint8_t *)data, len);
}

void sha3_256_final(sha3_256_ctx *c, uint8_t out[SHA3_256_LEN])
{
    sponge_pad(c, SUFFIX_SHA3);
    sponge_squeeze(c, out, SHA3_256_LEN);
    sponge_init(c);
}

void sha3_256(uint8_t out[SHA3_256_LEN], const void *data, size_t len)
{
    sha3_256_ctx c;

    sha3_256_init(&c);
    sha3_256_update(&c, data, len);
    sha3_256_final(&c, out);
}

void shake256(uint8_t *out, size_t out_len, const void *data, size_t len)
{
    sha3_256_ctx c;

    sponge_init(&c);
    sponge_absorb(&c, (const uint8_t *)data, len);
    sponge_pad(&c, SUFFIX_SHAKE);
    sponge_squeeze(&c, out, out_len);
    sponge_init(&c);
}
