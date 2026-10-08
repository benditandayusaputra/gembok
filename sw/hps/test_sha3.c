#include <stdio.h>
#include <stdlib.h>
#include <string.h>

#include "sha3.h"

static unsigned failures;
static unsigned checks;

static void pattern(uint8_t *buf, size_t n)
{
    for (size_t i = 0; i < n; i++)
        buf[i] = (uint8_t)(i * 7 + 3);
}

static void expect_hex(const char *name, const uint8_t *got, size_t len, const char *want_hex)
{
    static const char digits[] = "0123456789abcdef";
    char *got_hex = malloc(2 * len + 1);

    if (got_hex == NULL) {
        fprintf(stderr, "kehabisan memori\n");
        exit(2);
    }
    for (size_t i = 0; i < len; i++) {
        got_hex[2 * i] = digits[got[i] >> 4];
        got_hex[2 * i + 1] = digits[got[i] & 15];
    }
    got_hex[2 * len] = '\0';
    checks++;
    if (strcmp(got_hex, want_hex) != 0) {
        failures++;
        printf("GAGAL %s\n  hasil    %s\n  harapan  %s\n", name, got_hex, want_hex);
    }
    free(got_hex);
}

static void test_pattern(size_t n, const char *want_hex)
{
    uint8_t buf[512];
    uint8_t out[SHA3_256_LEN];
    char name[32];

    pattern(buf, n);
    sha3_256(out, buf, n);
    snprintf(name, sizeof name, "pola %zu byte", n);
    expect_hex(name, out, sizeof out, want_hex);
}

static void test_known_answers(void)
{
    uint8_t out[SHA3_256_LEN];
    uint8_t block[200];

    sha3_256(out, "", 0);
    expect_hex("pesan kosong", out, sizeof out,
               "a7ffc6f8bf1ed76651c14756a061d662f580ff4de43b49fa82d80a4b80f8434a");
    sha3_256(out, "abc", 3);
    expect_hex("abc", out, sizeof out,
               "3a985da74fe225b2045c172d6bd390bd855f086e3e9d525b46bfe24511431532");
    memset(block, 0xA3, sizeof block);
    sha3_256(out, block, sizeof block);
    expect_hex("200 byte 0xA3", out, sizeof out,
               "79f38adec5c20307a98ef76e8324afbfd46cfd81b22e3973c65fa1bd9de31787");

    test_pattern(135, "d9dcf1f98e49a79b0643a9e68fef48079ff8777c5e7e7f93469ded65f192ac71");
    test_pattern(136, "743bd32e775ac7387a57d4d574c89ddef5ebcb08bb5cc6b88c55a27b5035cc45");
    test_pattern(137, "01d47e8d6dce6e3dcbf1baa6f845b6ace4ef74bd17da8176ecc49bc35dbe5d21");
    test_pattern(271, "20bf2d2a8b462cf032927768963867773b04bcfdbf1419d1ac06aa23637e5b19");
    test_pattern(272, "ddeb5151c079739970e780e6257d0c4d52d83bf82c6aa8d47d5195530b5d5f4b");
    test_pattern(273, "ca74f68cfcda3bbd9068ae31dd3156e6df046aeb68e3176e4e17bf1557174553");
}

static void test_million(void)
{
    sha3_256_ctx c;
    uint8_t chunk[1000];
    uint8_t out[SHA3_256_LEN];

    memset(chunk, 'a', sizeof chunk);
    sha3_256_init(&c);
    for (unsigned i = 0; i < 1000; i++)
        sha3_256_update(&c, chunk, sizeof chunk);
    sha3_256_final(&c, out);
    expect_hex("sejuta huruf a", out, sizeof out,
               "5c8875ae474a3634ba4fd55ec85bffd661f32aca75c6d699d0cdcb6c115891c1");
}

static void test_every_length(void)
{
    sha3_256_ctx chain;
    uint8_t buf[600];
    uint8_t out[SHA3_256_LEN];

    sha3_256_init(&chain);
    for (size_t n = 0; n <= 600; n++) {
        pattern(buf, n);
        sha3_256(out, buf, n);
        sha3_256_update(&chain, out, sizeof out);
    }
    sha3_256_final(&chain, out);
    expect_hex("semua panjang 0 sampai 600", out, sizeof out,
               "83a211de2c370b1ab699c8a94103c39a8feb102abb605722085e090e80c86536");
}

static void test_incremental(void)
{
    uint8_t buf[500];
    uint8_t whole[SHA3_256_LEN];
    uint8_t parts[SHA3_256_LEN];

    pattern(buf, sizeof buf);
    sha3_256(whole, buf, sizeof buf);
    for (size_t step = 1; step <= 300; step += 13) {
        sha3_256_ctx c;

        sha3_256_init(&c);
        for (size_t off = 0; off < sizeof buf; off += step) {
            size_t n = sizeof buf - off < step ? sizeof buf - off : step;
            sha3_256_update(&c, buf + off, n);
        }
        sha3_256_final(&c, parts);
        checks++;
        if (memcmp(whole, parts, sizeof whole) != 0) {
            failures++;
            printf("GAGAL bertahap, potongan %zu byte\n", step);
        }
    }
}

static void test_shake(void)
{
    static const size_t out_lens[] = {1, 16, 135, 136, 137, 300};
    sha3_256_ctx chain;
    uint8_t buf[300];
    uint8_t out[300];
    uint8_t digest[SHA3_256_LEN];

    shake256(out, 64, "", 0);
    expect_hex("SHAKE256 pesan kosong", out, 64,
               "46b9dd2b0ba88d13233b3feb743eeb243fcd52ea62b81b82b50c27646ed5762f"
               "d75dc4ddd8c0f200cb05019d67b592f6fc821c49479ab48640292eacb3b7c4be");
    shake256(out, 32, "abc", 3);
    expect_hex("SHAKE256 abc", out, 32,
               "483366601360a8771c6863080cc4114d8db44530f8f1e1ee4f94ea37e78b5739");

    sha3_256_init(&chain);
    for (size_t n = 0; n <= 300; n++) {
        pattern(buf, n);
        for (size_t i = 0; i < sizeof out_lens / sizeof out_lens[0]; i++) {
            shake256(out, out_lens[i], buf, n);
            sha3_256_update(&chain, out, out_lens[i]);
        }
    }
    sha3_256_final(&chain, digest);
    expect_hex("SHAKE256 semua panjang 0 sampai 300", digest, sizeof digest,
               "7fd0dedd781058a0de8dee4685396437ce6118102e2e47b1604c6868471f82a4");
}

int main(void)
{
    test_known_answers();
    test_million();
    test_every_length();
    test_incremental();
    test_shake();
    printf("SHA3: %u dari %u pemeriksaan lolos\n", checks - failures, checks);
    return failures == 0 ? 0 : 1;
}
