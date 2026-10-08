#define _POSIX_C_SOURCE 200809L

#include <ctype.h>
#include <errno.h>
#include <fcntl.h>
#include <inttypes.h>
#include <stdarg.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/stat.h>
#include <unistd.h>

#include "gembok.h"
#include "gembok_io.h"
#include "sha3.h"

#if defined(__GNUC__)
#define PRINTF_LIKE(fmt, args) __attribute__((format(printf, fmt, args)))
#else
#define PRINTF_LIKE(fmt, args)
#endif

#define EXIT_FAIL 1
#define EXIT_USAGE 2

#define K_DEFAULT 3u
#define KV_LINE_CAP 8192
#define HEX_FILE_LIMIT 65536u
#define PROVE_ATTEMPTS 3
#define ACVP_MAGIC "gembok-acvp"
#define ACVP_FORMAT "1"
#define HELPER_MAGIC "gembok-helper"
#define HELPER_FORMAT "2"

typedef struct {
    gembok_dev dev;
    gembok_caps caps;
    uint32_t version;
    uint8_t seed[32];
    int seeded;
    uint64_t draws;
} app;

typedef struct {
    const char *name;
    const char **value;
} option;

typedef struct {
    unsigned oscillators;
    unsigned thresh;
    size_t mask_len;
    uint8_t mask[GEMBOK_HELPER_MASK_MAX];
    uint8_t chk[GEMBOK_HELPER_CHK_LEN];
} helper_data;

typedef int (*command_fn)(app *a, int argc, char **argv);

typedef struct {
    const char *name;
    command_fn run;
} command;

static const char *prog = "gembok-cli";

static void say_error(const char *fmt, ...) PRINTF_LIKE(1, 2);
static void note(const char *fmt, ...) PRINTF_LIKE(1, 2);
static int usage_error(const char *fmt, ...) PRINTF_LIKE(1, 2);

static void say_error(const char *fmt, ...)
{
    va_list ap;

    fflush(stdout);
    fprintf(stderr, "%s: ", prog);
    va_start(ap, fmt);
    vfprintf(stderr, fmt, ap);
    va_end(ap);
    fputc('\n', stderr);
}

static void note(const char *fmt, ...)
{
    va_list ap;

    fflush(stdout);
    va_start(ap, fmt);
    vfprintf(stderr, fmt, ap);
    va_end(ap);
    fputc('\n', stderr);
}

static int usage_error(const char *fmt, ...)
{
    va_list ap;

    fflush(stdout);
    fprintf(stderr, "%s: ", prog);
    va_start(ap, fmt);
    vfprintf(stderr, fmt, ap);
    va_end(ap);
    fprintf(stderr, "\nJalankan \"%s --help\" untuk daftar perintah.\n", prog);
    return EXIT_USAGE;
}

static int chip_error(const char *what, gembok_err e)
{
    say_error("%s gagal: %s (kode 0x%X)", what, gembok_strerror(e), (unsigned)e);
    return EXIT_FAIL;
}

static void print_usage(FILE *out)
{
    fprintf(out,
            "Pemakaian: %s [--base ALAMAT] [--benih HEX] PERINTAH [OPSI] [+ PERINTAH [OPSI] ...]\n"
            "\n"
            "Perintah:\n"
            "  info\n"
            "      Tampilkan register identitas, kemampuan, dan status.\n"
            "  selftest [-k K] [--thresh N]\n"
            "      Uji mandiri: mode terbuka lalu mode brankas. Tanpa -k, ketiga tingkat diuji.\n"
            "  acvp BERKAS [--jumlah N]\n"
            "      Jalankan vektor NIST ACVP dari berkas hasil tools/acvp_to_txt.py.\n"
            "  puf-enroll [--thresh N] [--helper BERKAS]\n"
            "      Daftarkan PUF dan simpan data bantu.\n"
            "  puf-recon --helper BERKAS\n"
            "      Pulihkan kunci PUF dari data bantu.\n"
            "  enroll [-k K] [--ek BERKAS]\n"
            "      Perintah DAFTAR: keluarkan kunci enkapsulasi chip.\n"
            "  prove [-k K] --ct BERKAS [--ctx TEKS | --ctx-hex HEX] [--tag BERKAS]\n"
            "        [--kunci-bersama BERKAS]\n"
            "      Perintah BUKTIKAN. Dengan --kunci-bersama, bukti diperiksa: ASLI atau PALSU.\n"
            "  wipe\n"
            "      Hapus semua memori dan lupakan kunci PUF.\n"
            "  keygen [-k K] --ek BERKAS --dk BERKAS [--d BERKAS --z BERKAS]\n"
            "  encaps [-k K] --ek BERKAS --ct BERKAS --kunci-bersama BERKAS [--m BERKAS]\n"
            "  decaps [-k K] --dk BERKAS --ct BERKAS [--kunci-bersama BERKAS]\n"
            "      Mode terbuka. Benih yang tidak diberikan diambil acak.\n"
            "  puf-measure [--idx N] [--jumlah N]\n"
            "      Baca hitungan mentah osilator (hanya bangunan debug).\n"
            "\n"
            "Opsi:\n"
            "  --base ALAMAT  alamat fisik dasar IP (bawaan 0x%08" PRIX32 ")\n"
            "  --benih HEX    benih 32 byte untuk bilangan acak, supaya uji bisa diulang persis\n"
            "  -k K           2, 3, 4 atau 512, 768, 1024 (bawaan %u)\n"
            "  --thresh N     ambang PUF (register PUF_THRESH)\n"
            "\n"
            "Berkas kunci, sandi, dan bukti berisi heksadesimal. Beberapa perintah bisa\n"
            "dirangkai dengan tanda + dan dijalankan berurutan pada sambungan yang sama.\n",
            prog, (uint32_t)GEMBOK_IO_BASE_DEFAULT, K_DEFAULT);
}

static int parse_u32(const char *text, uint32_t *out)
{
    char *end;
    unsigned long long v;

    if (text == NULL || !isdigit((unsigned char)text[0]))
        return -1;
    errno = 0;
    v = strtoull(text, &end, 0);
    if (errno != 0 || *end != '\0' || v > 0xFFFFFFFFull)
        return -1;
    *out = (uint32_t)v;
    return 0;
}

static int parse_level(const char *text, unsigned *k)
{
    uint32_t v;

    if (text == NULL) {
        *k = K_DEFAULT;
        return 0;
    }
    if (parse_u32(text, &v) != 0)
        return -1;
    switch (v) {
    case 2:
    case 512:
        *k = 2;
        return 0;
    case 3:
    case 768:
        *k = 3;
        return 0;
    case 4:
    case 1024:
        *k = 4;
        return 0;
    default:
        return -1;
    }
}

static const char *level_name(unsigned k)
{
    switch (k) {
    case 2:
        return "ML-KEM-512";
    case 3:
        return "ML-KEM-768";
    case 4:
        return "ML-KEM-1024";
    default:
        return "tingkat tidak dikenal";
    }
}

static const char *puf_mode_name(unsigned mode)
{
    switch (mode) {
    case 0:
        return "osilator asli";
    case 1:
        return "model simulasi";
    case 2:
        return "kunci pengembangan tetap, tidak aman";
    default:
        return "tidak dikenal";
    }
}

static const char *version_text(uint32_t version, char buf[32])
{
    snprintf(buf, 32, "%" PRIu32 ".%" PRIu32 ".%" PRIu32,
             version >> 16, (version >> 8) & 0xFFu, version & 0xFFu);
    return buf;
}

static const char *ms_text(uint64_t cycles, char buf[32])
{
    uint64_t us = (cycles * 1000000u + GEMBOK_CLOCK_HZ / 2u) / GEMBOK_CLOCK_HZ;

    snprintf(buf, 32, "%" PRIu64 ",%03u ms", us / 1000u, (unsigned)(us % 1000u));
    return buf;
}

static const char *window_text(unsigned win_log2, char buf[48])
{
    uint64_t cycles = (uint64_t)1 << win_log2;
    uint64_t centi_us = cycles * 100000000u / GEMBOK_CLOCK_HZ;

    snprintf(buf, 48, "2^%u siklus (%" PRIu64 ",%02u us)", win_log2, centi_us / 100u,
             (unsigned)(centi_us % 100u));
    return buf;
}

static int parse_options(int argc, char **argv, const option *opts, size_t n_opts,
                         const char **positional)
{
    for (int i = 1; i < argc; i++) {
        const char *arg = argv[i];
        size_t j = 0;

        while (j < n_opts && strcmp(arg, opts[j].name) != 0)
            j++;
        if (j < n_opts) {
            if (i + 1 >= argc)
                return usage_error("opsi %s pada perintah %s butuh nilai", arg, argv[0]);
            if (*opts[j].value != NULL)
                return usage_error("opsi %s diberikan dua kali", arg);
            *opts[j].value = argv[++i];
        } else if (arg[0] == '-' && arg[1] != '\0') {
            return usage_error("opsi tidak dikenal untuk perintah %s: %s", argv[0], arg);
        } else if (positional != NULL && *positional == NULL) {
            *positional = arg;
        } else {
            return usage_error("argumen berlebih untuk perintah %s: %s", argv[0], arg);
        }
    }
    return 0;
}

static int option_level(const char *text, unsigned *k)
{
    if (parse_level(text, k) != 0)
        return usage_error("nilai -k tidak sah: %s (pakai 2, 3, 4 atau 512, 768, 1024)", text);
    return 0;
}

static int option_thresh(app *a, const char *text)
{
    uint32_t v;
    gembok_err e;

    if (text == NULL)
        return 0;
    if (parse_u32(text, &v) != 0 || v > 0xFFFFu)
        return usage_error("nilai --thresh tidak sah: %s (0 sampai 65535)", text);
    e = gembok_set_thresh(&a->dev, (uint16_t)v);
    return e == GEMBOK_OK ? 0 : chip_error("mengatur PUF_THRESH", e);
}

static int hex_digit(int c)
{
    if (c >= '0' && c <= '9')
        return c - '0';
    if (c >= 'a' && c <= 'f')
        return c - 'a' + 10;
    if (c >= 'A' && c <= 'F')
        return c - 'A' + 10;
    return -1;
}

static int hex_decode(const char *text, uint8_t *out, size_t cap, size_t *len)
{
    size_t n = 0;
    int high = -1;

    for (; *text != '\0'; text++) {
        int v;

        if (isspace((unsigned char)*text))
            continue;
        v = hex_digit(*text);
        if (v < 0)
            return -1;
        if (high < 0) {
            high = v;
            continue;
        }
        if (n == cap)
            return -1;
        out[n++] = (uint8_t)((high << 4) | v);
        high = -1;
    }
    if (high >= 0)
        return -1;
    *len = n;
    return 0;
}

static void hex_write(FILE *out, const uint8_t *data, size_t len)
{
    for (size_t i = 0; i < len; i++)
        fprintf(out, "%02x", data[i]);
}

static int load_hex(const char *path, const char *what, uint8_t *out, size_t want)
{
    FILE *f = fopen(path, "r");
    char *text;
    size_t got;
    size_t len = 0;
    int bad;

    if (f == NULL) {
        say_error("tidak bisa membuka %s (%s): %s", path, what, strerror(errno));
        return -1;
    }
    text = malloc(HEX_FILE_LIMIT + 1u);
    if (text == NULL) {
        fclose(f);
        say_error("kehabisan memori");
        return -1;
    }
    got = fread(text, 1, HEX_FILE_LIMIT, f);
    bad = ferror(f) || !feof(f);
    fclose(f);
    text[got] = '\0';
    if (!bad)
        bad = hex_decode(text, out, want, &len) != 0 || len != want;
    free(text);
    if (bad) {
        say_error("%s (%s) harus berisi tepat %zu byte heksadesimal", path, what, want);
        return -1;
    }
    return 0;
}

static FILE *create_file(const char *path, int secret)
{
    int fd;
    FILE *f;

    if (!secret)
        return fopen(path, "w");
    fd = open(path, O_WRONLY | O_CREAT | O_TRUNC, S_IRUSR | S_IWUSR);
    if (fd < 0)
        return NULL;
    if (fchmod(fd, S_IRUSR | S_IWUSR) != 0) {
        close(fd);
        return NULL;
    }
    f = fdopen(fd, "w");
    if (f == NULL)
        close(fd);
    return f;
}

static int save_hex(const char *path, const char *what, const uint8_t *data, size_t len,
                    int secret)
{
    FILE *f = create_file(path, secret);

    if (f == NULL) {
        say_error("tidak bisa menulis %s (%s): %s", path, what, strerror(errno));
        return -1;
    }
    hex_write(f, data, len);
    fputc('\n', f);
    if (fclose(f) != 0) {
        say_error("gagal menulis %s (%s): %s", path, what, strerror(errno));
        return -1;
    }
    return 0;
}

static int emit_hex(const char *path, const char *what, const uint8_t *data, size_t len)
{
    if (path != NULL)
        return save_hex(path, what, data, len, 0);
    printf("%s ", what);
    hex_write(stdout, data, len);
    putchar('\n');
    return 0;
}

static void seed_random(app *a)
{
    FILE *f;
    size_t got;

    if (a->seeded)
        return;
    f = fopen("/dev/urandom", "rb");
    got = f != NULL ? fread(a->seed, 1, sizeof a->seed, f) : 0;
    if (f != NULL)
        fclose(f);
    if (got != sizeof a->seed) {
        say_error("tidak bisa membaca /dev/urandom, berikan --benih");
        exit(EXIT_FAIL);
    }
    a->seeded = 1;
}

static void random_bytes(app *a, uint8_t *out, size_t len)
{
    uint8_t block[40];

    seed_random(a);
    memcpy(block, a->seed, sizeof a->seed);
    for (unsigned i = 0; i < 8; i++)
        block[sizeof a->seed + i] = (uint8_t)(a->draws >> (8u * i));
    a->draws++;
    shake256(out, len, block, sizeof block);
}

static void flip_random_bit(app *a, uint8_t *data, size_t len)
{
    uint8_t r[5];
    uint32_t pos;

    random_bytes(a, r, sizeof r);
    pos = (uint32_t)r[0] | ((uint32_t)r[1] << 8) | ((uint32_t)r[2] << 16) | ((uint32_t)r[3] << 24);
    data[pos % len] ^= (uint8_t)(1u << (r[4] & 7u));
}

static int equal_bytes(const uint8_t *x, const uint8_t *y, size_t len)
{
    unsigned diff = 0;

    for (size_t i = 0; i < len; i++)
        diff |= (unsigned)(x[i] ^ y[i]);
    return diff == 0;
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

static gembok_err prove_patiently(app *a, unsigned k, const uint8_t *ct, size_t ct_len,
                                  const uint8_t *ctx, size_t ctx_len,
                                  uint8_t tag[GEMBOK_TAG_LEN], FILE *log, const char *indent)
{
    gembok_err e = GEMBOK_CHIP_RATE;

    for (int attempt = 0; attempt < PROVE_ATTEMPTS && e == GEMBOK_CHIP_RATE; attempt++) {
        e = gembok_prove(&a->dev, k, ct, ct_len, ctx, ctx_len, tag);
        if (e == GEMBOK_CHIP_RATE) {
            uint32_t left = gembok_cooldown(&a->dev);
            char t[32];
            gembok_err w;

            fflush(stdout);
            fprintf(log, "%spembatas laju aktif, menunggu %" PRIu32 " siklus (%s)\n",
                    indent, left, ms_text(left, t));
            fflush(log);
            w = gembok_cooldown_wait(&a->dev);
            if (w != GEMBOK_OK)
                return w;
        }
    }
    return e;
}

static int cmd_info(app *a, int argc, char **argv)
{
    gembok_dev *dev = &a->dev;
    uint32_t st;
    uint32_t err;
    uint32_t param;
    uint32_t cycles;
    uint32_t cooldown;
    char t[32];
    char w[48];
    int rc = parse_options(argc, argv, NULL, 0, NULL);

    if (rc != 0)
        return rc;
    st = gembok_status(dev);
    err = (st >> GEMBOK_ST_ERR_SHIFT) & GEMBOK_ST_ERR_MASK;
    param = gembok_reg_read(dev, GEMBOK_REG_PARAM);
    cycles = gembok_cycles(dev);
    cooldown = gembok_cooldown(dev);
    printf("Sambungan   %s\n", gembok_io_name());
    printf("ID          0x%08" PRIX32 " (GEMB)\n", gembok_reg_read(dev, GEMBOK_REG_ID));
    printf("VERSION     0x%08" PRIX32 " (%s)\n", a->version, version_text(a->version, t));
    printf("CAPS        0x%08" PRIX32 ": mode PUF %u (%s), %s\n",
           a->caps.raw, a->caps.puf_mode, puf_mode_name(a->caps.puf_mode),
           a->caps.debug ? "bangunan debug" : "bukan bangunan debug");
    printf("            %u osilator, %u suara, jendela %s, topeng data bantu %zu byte\n",
           a->caps.oscillators, a->caps.votes, window_text(a->caps.win_log2, w),
           a->caps.helper_mask_len);
    printf("            PUF_ENROLL dan PUF_RECON paling lama %" PRIu64 " siklus (%s)\n",
           a->caps.puf_cycle_bound, ms_text(a->caps.puf_cycle_bound, t));
    printf("STATUS      0x%08" PRIX32 ":%s%s%s%s%s%s, galat %" PRIu32 " (%s)\n", st,
           (st & GEMBOK_ST_BOOTED) ? " BOOTED" : "",
           (st & GEMBOK_ST_BUSY) ? " BUSY" : "",
           (st & GEMBOK_ST_DONE) ? " DONE" : "",
           (st & GEMBOK_ST_ERROR) ? " ERROR" : "",
           (st & GEMBOK_ST_PUF_READY) ? " PUF_READY" : "",
           (st & GEMBOK_ST_COOLDOWN) ? " COOLDOWN" : "",
           err, gembok_strerror((gembok_err)err));
    printf("PARAM       %" PRIu32 " (%s)\n", param, level_name((unsigned)param));
    printf("CTXLEN      %" PRIu32 "\n", gembok_reg_read(dev, GEMBOK_REG_CTXLEN));
    printf("CYCLES      %" PRIu32 " (%s)\n", cycles, ms_text(cycles, t));
    printf("COOLDOWN    %" PRIu32 " (%s)\n", cooldown, ms_text(cooldown, t));
    printf("PROOFS      %" PRIu32 "\n", gembok_proofs(dev));
    printf("PUF_THRESH  %u\n", (unsigned)gembok_get_thresh(dev));
    return 0;
}

static void step_row(const char *name, uint32_t cycles, const char *remark)
{
    char t[32];

    printf("     %-22s %9" PRIu32 " siklus %12s", name, cycles, ms_text(cycles, t));
    if (remark[0] != '\0')
        printf("   %s", remark);
    putchar('\n');
    fflush(stdout);
}

static unsigned step_failed(const char *name, gembok_err e)
{
    printf("     %-22s GAGAL: %s (kode 0x%X)\n", name, gembok_strerror(e), (unsigned)e);
    fflush(stdout);
    return 1;
}

static unsigned selftest_sha3(void)
{
    static const uint8_t want[SHA3_256_LEN] = {
        0x3a, 0x98, 0x5d, 0xa7, 0x4f, 0xe2, 0x25, 0xb2, 0x04, 0x5c, 0x17, 0x2d, 0x6b, 0xd3, 0x90, 0xbd,
        0x85, 0x5f, 0x08, 0x6e, 0x3e, 0x9d, 0x52, 0x5b, 0x46, 0xbf, 0xe2, 0x45, 0x11, 0x43, 0x15, 0x32
    };
    uint8_t got[SHA3_256_LEN];
    int ok;

    sha3_256(got, "abc", 3);
    ok = equal_bytes(got, want, sizeof want);
    printf("     %-22s %s\n", "vektor uji \"abc\"", ok ? "cocok" : "GAGAL");
    return ok ? 0 : 1;
}

static unsigned selftest_open(app *a, unsigned k)
{
    static uint8_t ek[GEMBOK_EK_MAX];
    static uint8_t dk[GEMBOK_DK_MAX];
    static uint8_t ct[GEMBOK_CT_MAX];
    static uint8_t reject_input[GEMBOK_SEED_LEN + GEMBOK_CT_MAX];
    uint8_t d[GEMBOK_SEED_LEN];
    uint8_t z[GEMBOK_SEED_LEN];
    uint8_t m[GEMBOK_SEED_LEN];
    uint8_t shared_enc[GEMBOK_SHARED_LEN];
    uint8_t shared_dec[GEMBOK_SHARED_LEN];
    uint8_t shared_bad[GEMBOK_SHARED_LEN];
    uint8_t shared_reject[GEMBOK_SHARED_LEN];
    size_t ek_len = gembok_ek_len(k);
    size_t dk_len = gembok_dk_len(k);
    size_t ct_len = gembok_ct_len(k);
    uint8_t *ct_bad = reject_input + GEMBOK_SEED_LEN;
    gembok_dev *dev = &a->dev;
    unsigned failures = 0;
    gembok_err e;
    int same;
    int differs;
    int rejected;

    printf("   %s\n", level_name(k));
    random_bytes(a, d, sizeof d);
    random_bytes(a, z, sizeof z);
    random_bytes(a, m, sizeof m);

    e = gembok_keygen(dev, k, d, z, ek, ek_len, dk, dk_len);
    if (e != GEMBOK_OK)
        return step_failed("KeyGen", e);
    step_row("KeyGen", gembok_last_cycles(dev), "");

    e = gembok_check_ek(dev, k, ek, ek_len);
    if (e != GEMBOK_OK)
        return step_failed("Cek EK", e);
    step_row("Cek EK", gembok_last_cycles(dev), "lolos");
    e = gembok_check_dk(dev, k, dk, dk_len);
    if (e != GEMBOK_OK)
        return step_failed("Cek DK", e);
    step_row("Cek DK", gembok_last_cycles(dev), "lolos");

    e = gembok_encaps(dev, k, ek, ek_len, m, ct, ct_len, shared_enc);
    if (e != GEMBOK_OK)
        return step_failed("Encaps", e);
    step_row("Encaps", gembok_last_cycles(dev), "");

    e = gembok_decaps(dev, k, dk, dk_len, ct, ct_len, shared_dec);
    if (e != GEMBOK_OK)
        return step_failed("Decaps", e);
    same = equal_bytes(shared_enc, shared_dec, sizeof shared_enc);
    step_row("Decaps", gembok_last_cycles(dev),
             same ? "kunci bersama sama" : "GAGAL: kunci bersama berbeda");
    failures += same ? 0 : 1;

    memcpy(reject_input, z, sizeof z);
    memcpy(ct_bad, ct, ct_len);
    flip_random_bit(a, ct_bad, ct_len);
    e = gembok_decaps(dev, k, dk, dk_len, ct_bad, ct_len, shared_bad);
    if (e != GEMBOK_OK)
        return failures + step_failed("Decaps sandi rusak", e);
    shake256(shared_reject, sizeof shared_reject, reject_input, sizeof z + ct_len);
    differs = !equal_bytes(shared_bad, shared_enc, sizeof shared_bad);
    rejected = equal_bytes(shared_bad, shared_reject, sizeof shared_bad);
    step_row("Decaps sandi rusak", gembok_last_cycles(dev),
             !differs ? "GAGAL: kunci tetap sama"
             : rejected ? "kunci berbeda, penolakan implisit cocok"
             : "GAGAL: kunci bukan J(z || c)");
    failures += (differs && rejected) ? 0 : 1;
    return failures;
}

static unsigned selftest_vault(app *a, unsigned k)
{
    static uint8_t ek[GEMBOK_EK_MAX];
    static uint8_t ct[GEMBOK_CT_MAX];
    uint8_t m[GEMBOK_SEED_LEN];
    uint8_t shared[GEMBOK_SHARED_LEN];
    uint8_t tag[GEMBOK_TAG_LEN];
    uint8_t want[GEMBOK_TAG_LEN];
    char ctx[40];
    size_t ek_len = gembok_ek_len(k);
    size_t ct_len = gembok_ct_len(k);
    size_t ctx_len;
    gembok_dev *dev = &a->dev;
    unsigned failures = 0;
    gembok_err e;
    int genuine;

    printf("   %s\n", level_name(k));
    ctx_len = (size_t)snprintf(ctx, sizeof ctx, "GEMBOK uji mandiri k=%u", k);
    random_bytes(a, m, sizeof m);

    e = gembok_enroll(dev, k, ek, ek_len);
    if (e != GEMBOK_OK)
        return step_failed("DAFTAR (ENROLL)", e);
    step_row("DAFTAR (ENROLL)", gembok_last_cycles(dev), "");

    e = gembok_encaps(dev, k, ek, ek_len, m, ct, ct_len, shared);
    if (e != GEMBOK_OK)
        return step_failed("Encaps tantangan", e);
    step_row("Encaps tantangan", gembok_last_cycles(dev), "");
    expected_tag(shared, (const uint8_t *)ctx, ctx_len, want);

    e = prove_patiently(a, k, ct, ct_len, (const uint8_t *)ctx, ctx_len, tag, stdout, "     ");
    if (e != GEMBOK_OK)
        return step_failed("BUKTIKAN (PROVE)", e);
    genuine = equal_bytes(tag, want, sizeof tag);
    step_row("BUKTIKAN (PROVE)", gembok_last_cycles(dev), genuine ? "ASLI" : "PALSU");
    failures += genuine ? 0 : 1;

    flip_random_bit(a, ct, ct_len);
    e = prove_patiently(a, k, ct, ct_len, (const uint8_t *)ctx, ctx_len, tag, stdout, "     ");
    if (e != GEMBOK_OK)
        return failures + step_failed("BUKTIKAN sandi rusak", e);
    genuine = equal_bytes(tag, want, sizeof tag);
    step_row("BUKTIKAN sandi rusak", gembok_last_cycles(dev),
             genuine ? "ASLI, seharusnya PALSU" : "PALSU, sesuai harapan");
    failures += genuine ? 1 : 0;
    return failures;
}

static unsigned selftest_puf_enroll(app *a)
{
    uint8_t mask[GEMBOK_HELPER_MASK_MAX];
    uint8_t chk[GEMBOK_HELPER_CHK_LEN];
    gembok_err e = gembok_puf_enroll(&a->dev, mask, a->caps.helper_mask_len, chk);
    int ready;

    if (e != GEMBOK_OK)
        return step_failed("PUF_ENROLL", e);
    ready = (gembok_status(&a->dev) & GEMBOK_ST_PUF_READY) != 0;
    step_row("PUF_ENROLL", gembok_last_cycles(&a->dev),
             ready ? "kunci PUF siap" : "GAGAL: PUF_READY tidak menyala");
    return ready ? 0 : 1;
}

static int cmd_selftest(app *a, int argc, char **argv)
{
    static const unsigned all_levels[] = {2, 3, 4};
    const char *k_text = NULL;
    const char *thresh_text = NULL;
    const option opts[] = {{"-k", &k_text}, {"--thresh", &thresh_text}};
    unsigned one_level[1];
    const unsigned *levels = all_levels;
    size_t n_levels = sizeof all_levels / sizeof all_levels[0];
    unsigned failures = 0;
    unsigned vault_failures;
    char t[32];
    char w[48];
    int rc = parse_options(argc, argv, opts, sizeof opts / sizeof opts[0], NULL);

    if (rc != 0)
        return rc;
    if (k_text != NULL) {
        rc = option_level(k_text, &one_level[0]);
        if (rc != 0)
            return rc;
        levels = one_level;
        n_levels = 1;
    }
    rc = option_thresh(a, thresh_text);
    if (rc != 0)
        return rc;

    seed_random(a);
    printf("GEMBOK uji mandiri\n");
    printf("Sambungan  : %s\n", gembok_io_name());
    printf("Perangkat  : versi %s, mode PUF %u (%s), %u osilator, %u suara, jendela %s\n",
           version_text(a->version, t), a->caps.puf_mode, puf_mode_name(a->caps.puf_mode),
           a->caps.oscillators, a->caps.votes, window_text(a->caps.win_log2, w));
    printf("Benih acak : ");
    hex_write(stdout, a->seed, sizeof a->seed);
    printf("\n\n1. SHA3-256 lokal\n");
    failures += selftest_sha3();

    printf("\n2. Mode terbuka: KeyGen, Encaps, Decaps\n");
    for (size_t i = 0; i < n_levels; i++)
        failures += selftest_open(a, levels[i]);

    printf("\n3. Mode brankas: PUF_ENROLL, DAFTAR, BUKTIKAN\n");
    vault_failures = selftest_puf_enroll(a);
    for (size_t i = 0; i < n_levels && vault_failures == 0; i++)
        failures += selftest_vault(a, levels[i]);
    failures += vault_failures;

    if (failures == 0) {
        printf("\nHASIL: LULUS, semua pemeriksaan lolos\n");
        return 0;
    }
    printf("\nHASIL: GAGAL, %u pemeriksaan tidak lolos\n", failures);
    return EXIT_FAIL;
}

typedef struct {
    FILE *f;
    const char *path;
    unsigned long line_no;
    char line[KV_LINE_CAP];
    char *key;
    char *value;
} kv_reader;

static int kv_open(kv_reader *r, const char *path)
{
    r->f = fopen(path, "r");
    r->path = path;
    r->line_no = 0;
    if (r->f == NULL) {
        say_error("tidak bisa membuka %s: %s", path, strerror(errno));
        return -1;
    }
    return 0;
}

static int kv_next(kv_reader *r)
{
    while (fgets(r->line, sizeof r->line, r->f) != NULL) {
        size_t n = strlen(r->line);
        char *p = r->line;

        r->line_no++;
        if (n + 1 == sizeof r->line && r->line[n - 1] != '\n') {
            say_error("%s baris %lu: baris terlalu panjang", r->path, r->line_no);
            return -1;
        }
        while (n > 0 && isspace((unsigned char)r->line[n - 1]))
            r->line[--n] = '\0';
        while (isspace((unsigned char)*p))
            p++;
        if (*p == '\0')
            continue;
        r->key = p;
        while (*p != '\0' && !isspace((unsigned char)*p))
            p++;
        if (*p != '\0')
            *p++ = '\0';
        while (isspace((unsigned char)*p))
            p++;
        r->value = p;
        return 1;
    }
    if (ferror(r->f)) {
        say_error("gagal membaca %s: %s", r->path, strerror(errno));
        return -1;
    }
    return 0;
}

static int kv_malformed(const kv_reader *r, const char *why)
{
    say_error("%s baris %lu: %s", r->path, r->line_no, why);
    return -1;
}

static int kv_expect_magic(kv_reader *r, const char *magic, const char *format)
{
    int got = kv_next(r);

    if (got < 0)
        return -1;
    if (got == 0 || strcmp(r->key, magic) != 0 || strcmp(r->value, format) != 0) {
        say_error("%s: baris pertama harus \"%s %s\"", r->path, magic, format);
        return -1;
    }
    return 0;
}

enum { FN_KEYGEN, FN_ENCAPS, FN_DECAPS, FN_CHECK_EK, FN_CHECK_DK, FN_COUNT };
enum { FIELD_D, FIELD_Z, FIELD_M, FIELD_K, FIELD_EK, FIELD_DK, FIELD_C, FIELD_COUNT };

#define FIELD_BIT(f) (1u << (f))

static const char *const FN_NAMES[FN_COUNT] = {
    "keyGen", "encapsulation", "decapsulation", "encapsulationKeyCheck", "decapsulationKeyCheck"
};

static const char *const FIELD_NAMES[FIELD_COUNT] = {"d", "z", "m", "k", "ek", "dk", "c"};

static const unsigned FN_FIELDS[FN_COUNT] = {
    FIELD_BIT(FIELD_D) | FIELD_BIT(FIELD_Z) | FIELD_BIT(FIELD_EK) | FIELD_BIT(FIELD_DK),
    FIELD_BIT(FIELD_EK) | FIELD_BIT(FIELD_M) | FIELD_BIT(FIELD_C) | FIELD_BIT(FIELD_K),
    FIELD_BIT(FIELD_DK) | FIELD_BIT(FIELD_C) | FIELD_BIT(FIELD_K),
    FIELD_BIT(FIELD_EK),
    FIELD_BIT(FIELD_DK)
};

typedef struct {
    uint8_t bytes[GEMBOK_DK_MAX];
    size_t len;
} acvp_field;

typedef struct {
    int fn;
    unsigned k;
    unsigned long tc_id;
    unsigned present;
    int verdict;
    acvp_field field[FIELD_COUNT];
} acvp_case;

typedef struct {
    unsigned total;
    unsigned passed;
    uint32_t cycles_min;
    uint32_t cycles_max;
} acvp_tally;

static int field_is(const acvp_case *c, int field, const uint8_t *data, size_t len)
{
    return c->field[field].len == len && memcmp(c->field[field].bytes, data, len) == 0;
}

static int key_check_passes(gembok_err e, gembok_err rejection, int expect_valid)
{
    if (e == GEMBOK_OK)
        return expect_valid == 1;
    if (e == rejection || e == GEMBOK_DRV_LEN)
        return expect_valid == 0;
    return 0;
}

static int acvp_run_case(app *a, const acvp_case *c, gembok_err *err)
{
    static uint8_t ek[GEMBOK_EK_MAX];
    static uint8_t dk[GEMBOK_DK_MAX];
    static uint8_t ct[GEMBOK_CT_MAX];
    uint8_t shared[GEMBOK_SHARED_LEN];
    const acvp_field *f = c->field;
    gembok_dev *dev = &a->dev;
    size_t ek_len = gembok_ek_len(c->k);
    size_t dk_len = gembok_dk_len(c->k);
    size_t ct_len = gembok_ct_len(c->k);
    gembok_err e = GEMBOK_DRV_LEN;
    int pass = 0;

    switch (c->fn) {
    case FN_KEYGEN:
        if (f[FIELD_D].len != GEMBOK_SEED_LEN || f[FIELD_Z].len != GEMBOK_SEED_LEN)
            break;
        e = gembok_keygen(dev, c->k, f[FIELD_D].bytes, f[FIELD_Z].bytes, ek, ek_len, dk, dk_len);
        pass = e == GEMBOK_OK && field_is(c, FIELD_EK, ek, ek_len) && field_is(c, FIELD_DK, dk, dk_len);
        break;
    case FN_ENCAPS:
        if (f[FIELD_M].len != GEMBOK_SEED_LEN)
            break;
        e = gembok_encaps(dev, c->k, f[FIELD_EK].bytes, f[FIELD_EK].len, f[FIELD_M].bytes,
                          ct, ct_len, shared);
        pass = e == GEMBOK_OK && field_is(c, FIELD_C, ct, ct_len)
               && field_is(c, FIELD_K, shared, sizeof shared);
        break;
    case FN_DECAPS:
        e = gembok_decaps(dev, c->k, f[FIELD_DK].bytes, f[FIELD_DK].len,
                          f[FIELD_C].bytes, f[FIELD_C].len, shared);
        pass = e == GEMBOK_OK && field_is(c, FIELD_K, shared, sizeof shared);
        break;
    case FN_CHECK_EK:
        e = gembok_check_ek(dev, c->k, f[FIELD_EK].bytes, f[FIELD_EK].len);
        pass = key_check_passes(e, GEMBOK_CHIP_BAD_EK, c->verdict);
        break;
    case FN_CHECK_DK:
        e = gembok_check_dk(dev, c->k, f[FIELD_DK].bytes, f[FIELD_DK].len);
        pass = key_check_passes(e, GEMBOK_CHIP_BAD_DK, c->verdict);
        break;
    default:
        break;
    }
    *err = e;
    return pass;
}

static int acvp_begin_case(kv_reader *r, acvp_case *c)
{
    char fn_name[32];
    unsigned k;
    unsigned long tc_id;
    int fn = 0;

    if (sscanf(r->value, "%31s %u %lu", fn_name, &k, &tc_id) != 3)
        return kv_malformed(r, "bentuk baris harus \"uji FUNGSI K NOMOR\"");
    while (fn < FN_COUNT && strcmp(fn_name, FN_NAMES[fn]) != 0)
        fn++;
    if (fn == FN_COUNT)
        return kv_malformed(r, "fungsi tidak dikenal");
    if (!gembok_k_valid(k))
        return kv_malformed(r, "k harus 2, 3, atau 4");
    c->fn = fn;
    c->k = k;
    c->tc_id = tc_id;
    c->present = 0;
    c->verdict = -1;
    return 0;
}

static int acvp_read_field(kv_reader *r, acvp_case *c)
{
    int field = 0;

    if (strcmp(r->key, "sah") == 0) {
        if (strcmp(r->value, "0") != 0 && strcmp(r->value, "1") != 0)
            return kv_malformed(r, "nilai sah harus 0 atau 1");
        c->verdict = r->value[0] - '0';
        return 0;
    }
    while (field < FIELD_COUNT && strcmp(r->key, FIELD_NAMES[field]) != 0)
        field++;
    if (field == FIELD_COUNT)
        return kv_malformed(r, "nama isian tidak dikenal");
    if (hex_decode(r->value, c->field[field].bytes, sizeof c->field[field].bytes,
                   &c->field[field].len) != 0)
        return kv_malformed(r, "heksadesimal tidak sah atau terlalu panjang");
    c->present |= FIELD_BIT(field);
    return 0;
}

static int acvp_case_complete(const kv_reader *r, const acvp_case *c)
{
    int is_check = c->fn == FN_CHECK_EK || c->fn == FN_CHECK_DK;

    if ((c->present & FN_FIELDS[c->fn]) != FN_FIELDS[c->fn])
        return kv_malformed(r, "isian kasus tidak lengkap");
    if (is_check && c->verdict < 0)
        return kv_malformed(r, "kasus cek kunci butuh baris sah");
    return 0;
}

static void acvp_print_table(acvp_tally tally[3][FN_COUNT])
{
    printf("%-13s%-24s%7s%7s%13s%13s\n", "parameter", "fungsi", "lolos", "total",
           "siklus min", "siklus maks");
    for (unsigned ki = 0; ki < 3; ki++) {
        for (int fn = 0; fn < FN_COUNT; fn++) {
            const acvp_tally *t = &tally[ki][fn];

            if (t->total == 0)
                continue;
            printf("%-13s%-24s%7u%7u%13" PRIu32 "%13" PRIu32 "%s\n", level_name(ki + 2),
                   FN_NAMES[fn], t->passed, t->total, t->cycles_min, t->cycles_max,
                   t->passed == t->total ? "" : "   GAGAL");
        }
    }
}

static int cmd_acvp(app *a, int argc, char **argv)
{
    static acvp_case c;
    static kv_reader r;
    static acvp_tally tally[3][FN_COUNT];
    const char *path = NULL;
    const char *count_text = NULL;
    const option opts[] = {{"--jumlah", &count_text}};
    uint32_t want_count = 0;
    uint32_t declared = 0;
    int have_declared = 0;
    unsigned total = 0;
    unsigned passed = 0;
    int in_case = 0;
    int got;
    int rc = parse_options(argc, argv, opts, sizeof opts / sizeof opts[0], &path);

    if (rc != 0)
        return rc;
    if (path == NULL)
        return usage_error("perintah acvp butuh nama berkas vektor");
    if (count_text != NULL && parse_u32(count_text, &want_count) != 0)
        return usage_error("nilai --jumlah tidak sah: %s", count_text);
    if (kv_open(&r, path) != 0)
        return EXIT_FAIL;
    memset(tally, 0, sizeof tally);
    if (kv_expect_magic(&r, ACVP_MAGIC, ACVP_FORMAT) != 0) {
        fclose(r.f);
        return EXIT_FAIL;
    }

    while ((got = kv_next(&r)) > 0) {
        if (!in_case) {
            if (strcmp(r.key, "kasus") == 0) {
                if (parse_u32(r.value, &declared) != 0) {
                    got = kv_malformed(&r, "jumlah kasus tidak sah");
                    break;
                }
                have_declared = 1;
            } else if (strcmp(r.key, "uji") == 0) {
                if (acvp_begin_case(&r, &c) != 0) {
                    got = -1;
                    break;
                }
                in_case = 1;
            } else {
                got = kv_malformed(&r, "baris di luar kasus uji");
                break;
            }
        } else if (strcmp(r.key, "selesai") == 0) {
            acvp_tally *t = &tally[c.k - 2][c.fn];
            gembok_err e;
            uint32_t cycles;
            int pass;

            if (acvp_case_complete(&r, &c) != 0) {
                got = -1;
                break;
            }
            pass = acvp_run_case(a, &c, &e);
            cycles = gembok_last_cycles(&a->dev);
            if (t->total == 0 || cycles < t->cycles_min)
                t->cycles_min = cycles;
            if (cycles > t->cycles_max)
                t->cycles_max = cycles;
            t->total++;
            t->passed += pass ? 1u : 0u;
            total++;
            passed += pass ? 1u : 0u;
            if (!pass && e == GEMBOK_OK)
                printf("GAGAL: %s %s tcId %lu: keluaran berbeda dari vektor\n", level_name(c.k),
                       FN_NAMES[c.fn], c.tc_id);
            else if (!pass)
                printf("GAGAL: %s %s tcId %lu: %s (kode 0x%X)\n", level_name(c.k), FN_NAMES[c.fn],
                       c.tc_id, gembok_strerror(e), (unsigned)e);
            fflush(stdout);
            in_case = 0;
        } else if (acvp_read_field(&r, &c) != 0) {
            got = -1;
            break;
        }
    }
    fclose(r.f);
    if (got < 0)
        return EXIT_FAIL;
    if (in_case) {
        say_error("%s: kasus terakhir tidak ditutup dengan baris selesai", path);
        return EXIT_FAIL;
    }

    acvp_print_table(tally);
    printf("\nTOTAL %u dari %u kasus lolos\n", passed, total);
    if (have_declared && declared != total) {
        say_error("%s menyatakan %" PRIu32 " kasus, tetapi yang terbaca %u", path, declared, total);
        return EXIT_FAIL;
    }
    if (count_text != NULL && want_count != total) {
        say_error("diharapkan %" PRIu32 " kasus, tetapi yang dijalankan %u", want_count, total);
        return EXIT_FAIL;
    }
    return (total > 0 && passed == total) ? 0 : EXIT_FAIL;
}

static int save_helper(const char *path, const helper_data *h)
{
    FILE *f = path != NULL ? fopen(path, "w") : stdout;

    if (f == NULL) {
        say_error("tidak bisa menulis %s: %s", path, strerror(errno));
        return -1;
    }
    fprintf(f, "%s %s\nosilator %u\nambang %u\ntopeng ", HELPER_MAGIC, HELPER_FORMAT,
            h->oscillators, h->thresh);
    hex_write(f, h->mask, h->mask_len);
    fprintf(f, "\ncek ");
    hex_write(f, h->chk, GEMBOK_HELPER_CHK_LEN);
    fputc('\n', f);
    if (path != NULL && fclose(f) != 0) {
        say_error("gagal menulis %s: %s", path, strerror(errno));
        return -1;
    }
    return 0;
}

static int read_helper_line(kv_reader *r, helper_data *h, unsigned *seen)
{
    uint32_t v;
    size_t len = 0;

    if (strcmp(r->key, "osilator") == 0) {
        if (parse_u32(r->value, &v) != 0 || v < GEMBOK_N_RO_MIN || v > GEMBOK_N_RO_MAX)
            return kv_malformed(r, "osilator harus 2 sampai 1025");
        h->oscillators = (unsigned)v;
        *seen |= 1u;
    } else if (strcmp(r->key, "ambang") == 0) {
        if (parse_u32(r->value, &v) != 0 || v > 0xFFFFu)
            return kv_malformed(r, "ambang harus 0 sampai 65535");
        h->thresh = (unsigned)v;
        *seen |= 2u;
    } else if (strcmp(r->key, "topeng") == 0) {
        if (hex_decode(r->value, h->mask, sizeof h->mask, &len) != 0)
            return kv_malformed(r, "topeng harus heksadesimal, paling panjang 128 byte");
        h->mask_len = len;
        *seen |= 4u;
    } else if (strcmp(r->key, "cek") == 0) {
        if (hex_decode(r->value, h->chk, sizeof h->chk, &len) != 0 || len != sizeof h->chk)
            return kv_malformed(r, "cek harus 16 byte heksadesimal");
        *seen |= 8u;
    } else {
        return kv_malformed(r, "baris tidak dikenal");
    }
    return 0;
}

static int load_helper(const char *path, helper_data *h)
{
    static kv_reader r;
    unsigned seen = 0;
    size_t want;
    int got;

    if (kv_open(&r, path) != 0)
        return -1;
    if (kv_expect_magic(&r, HELPER_MAGIC, HELPER_FORMAT) != 0) {
        fclose(r.f);
        return -1;
    }
    while ((got = kv_next(&r)) > 0) {
        if (read_helper_line(&r, h, &seen) != 0) {
            got = -1;
            break;
        }
    }
    fclose(r.f);
    if (got < 0)
        return -1;
    if (seen != 15u) {
        say_error("%s: data bantu butuh baris osilator, ambang, topeng, dan cek", path);
        return -1;
    }
    want = (h->oscillators + 6u) / 8u;
    if (h->mask_len != want) {
        say_error("%s: topeng %zu byte, seharusnya %zu byte untuk %u osilator",
                  path, h->mask_len, want, h->oscillators);
        return -1;
    }
    return 0;
}

static int cmd_puf_enroll(app *a, int argc, char **argv)
{
    const char *thresh_text = NULL;
    const char *helper_path = NULL;
    const option opts[] = {{"--thresh", &thresh_text}, {"--helper", &helper_path}};
    helper_data h;
    char t[32];
    gembok_err e;
    int rc = parse_options(argc, argv, opts, sizeof opts / sizeof opts[0], NULL);

    if (rc != 0)
        return rc;
    rc = option_thresh(a, thresh_text);
    if (rc != 0)
        return rc;
    h.oscillators = a->caps.oscillators;
    h.mask_len = a->caps.helper_mask_len;
    e = gembok_puf_enroll(&a->dev, h.mask, h.mask_len, h.chk);
    if (e != GEMBOK_OK)
        return chip_error("PUF_ENROLL", e);
    h.thresh = gembok_get_thresh(&a->dev);
    if (save_helper(helper_path, &h) != 0)
        return EXIT_FAIL;
    note("PUF_ENROLL berhasil, %" PRIu32 " siklus (%s), kunci PUF siap",
         gembok_last_cycles(&a->dev), ms_text(gembok_last_cycles(&a->dev), t));
    return 0;
}

static int cmd_puf_recon(app *a, int argc, char **argv)
{
    const char *helper_path = NULL;
    const option opts[] = {{"--helper", &helper_path}};
    helper_data h;
    char t[32];
    gembok_err e;
    int rc = parse_options(argc, argv, opts, sizeof opts / sizeof opts[0], NULL);

    if (rc != 0)
        return rc;
    if (helper_path == NULL)
        return usage_error("perintah puf-recon butuh --helper BERKAS");
    if (load_helper(helper_path, &h) != 0)
        return EXIT_FAIL;
    if (h.oscillators != a->caps.oscillators) {
        say_error("%s tidak cocok dengan chip: data bantu untuk %u osilator, chip ini %u osilator",
                  helper_path, h.oscillators, a->caps.oscillators);
        return EXIT_FAIL;
    }
    e = gembok_puf_recon(&a->dev, h.mask, h.mask_len, h.chk);
    if (e != GEMBOK_OK)
        return chip_error("PUF_RECON", e);
    note("PUF_RECON berhasil, %" PRIu32 " siklus (%s), kunci PUF siap",
         gembok_last_cycles(&a->dev), ms_text(gembok_last_cycles(&a->dev), t));
    return 0;
}

static int cmd_enroll(app *a, int argc, char **argv)
{
    static uint8_t ek[GEMBOK_EK_MAX];
    const char *k_text = NULL;
    const char *ek_path = NULL;
    const option opts[] = {{"-k", &k_text}, {"--ek", &ek_path}};
    unsigned k;
    char t[32];
    gembok_err e;
    int rc = parse_options(argc, argv, opts, sizeof opts / sizeof opts[0], NULL);

    if (rc != 0)
        return rc;
    rc = option_level(k_text, &k);
    if (rc != 0)
        return rc;
    e = gembok_enroll(&a->dev, k, ek, gembok_ek_len(k));
    if (e != GEMBOK_OK)
        return chip_error("DAFTAR (ENROLL)", e);
    if (emit_hex(ek_path, "ek", ek, gembok_ek_len(k)) != 0)
        return EXIT_FAIL;
    note("DAFTAR %s berhasil, %" PRIu32 " siklus (%s)", level_name(k),
         gembok_last_cycles(&a->dev), ms_text(gembok_last_cycles(&a->dev), t));
    return 0;
}

static int cmd_prove(app *a, int argc, char **argv)
{
    static uint8_t ct[GEMBOK_CT_MAX];
    const char *k_text = NULL;
    const char *ct_path = NULL;
    const char *ctx_text = NULL;
    const char *ctx_hex = NULL;
    const char *tag_path = NULL;
    const char *shared_path = NULL;
    const option opts[] = {
        {"-k", &k_text}, {"--ct", &ct_path}, {"--ctx", &ctx_text}, {"--ctx-hex", &ctx_hex},
        {"--tag", &tag_path}, {"--kunci-bersama", &shared_path}
    };
    uint8_t ctx_bytes[GEMBOK_CTX_MAX];
    const uint8_t *ctx = ctx_bytes;
    size_t ctx_len = 0;
    uint8_t shared[GEMBOK_SHARED_LEN];
    uint8_t tag[GEMBOK_TAG_LEN];
    uint8_t want[GEMBOK_TAG_LEN];
    unsigned k;
    char t[32];
    gembok_err e;
    int rc = parse_options(argc, argv, opts, sizeof opts / sizeof opts[0], NULL);

    if (rc != 0)
        return rc;
    rc = option_level(k_text, &k);
    if (rc != 0)
        return rc;
    if (ct_path == NULL)
        return usage_error("perintah prove butuh --ct BERKAS");
    if (ctx_text != NULL && ctx_hex != NULL)
        return usage_error("pakai salah satu saja: --ctx atau --ctx-hex");
    if (ctx_text != NULL) {
        ctx = (const uint8_t *)ctx_text;
        ctx_len = strlen(ctx_text);
        if (ctx_len > GEMBOK_CTX_MAX)
            return usage_error("konteks paling panjang %u byte", GEMBOK_CTX_MAX);
    }
    if (ctx_hex != NULL && hex_decode(ctx_hex, ctx_bytes, sizeof ctx_bytes, &ctx_len) != 0)
        return usage_error("nilai --ctx-hex harus heksadesimal, paling panjang %u byte",
                           GEMBOK_CTX_MAX);
    if (load_hex(ct_path, "ct", ct, gembok_ct_len(k)) != 0)
        return EXIT_FAIL;
    if (shared_path != NULL && load_hex(shared_path, "kunci bersama", shared, sizeof shared) != 0)
        return EXIT_FAIL;

    e = prove_patiently(a, k, ct, gembok_ct_len(k), ctx, ctx_len, tag, stderr, "");
    if (e != GEMBOK_OK)
        return chip_error("BUKTIKAN (PROVE)", e);
    if (emit_hex(tag_path, "tag", tag, sizeof tag) != 0)
        return EXIT_FAIL;
    note("BUKTIKAN %s berhasil, %" PRIu32 " siklus (%s)", level_name(k),
         gembok_last_cycles(&a->dev), ms_text(gembok_last_cycles(&a->dev), t));
    if (shared_path == NULL)
        return 0;
    expected_tag(shared, ctx, ctx_len, want);
    if (equal_bytes(tag, want, sizeof tag)) {
        printf("ASLI\n");
        return 0;
    }
    printf("PALSU\n");
    return EXIT_FAIL;
}

static int cmd_wipe(app *a, int argc, char **argv)
{
    gembok_err e;
    int rc = parse_options(argc, argv, NULL, 0, NULL);

    if (rc != 0)
        return rc;
    e = gembok_wipe(&a->dev);
    if (e != GEMBOK_OK)
        return chip_error("WIPE", e);
    note("WIPE berhasil: semua memori nol, kunci PUF dilupakan");
    return 0;
}

static int seed_input(app *a, const char *path, const char *what, uint8_t seed[GEMBOK_SEED_LEN])
{
    if (path == NULL) {
        random_bytes(a, seed, GEMBOK_SEED_LEN);
        return 0;
    }
    return load_hex(path, what, seed, GEMBOK_SEED_LEN);
}

static int cmd_keygen(app *a, int argc, char **argv)
{
    static uint8_t ek[GEMBOK_EK_MAX];
    static uint8_t dk[GEMBOK_DK_MAX];
    const char *k_text = NULL;
    const char *ek_path = NULL;
    const char *dk_path = NULL;
    const char *d_path = NULL;
    const char *z_path = NULL;
    const option opts[] = {
        {"-k", &k_text}, {"--ek", &ek_path}, {"--dk", &dk_path}, {"--d", &d_path}, {"--z", &z_path}
    };
    uint8_t d[GEMBOK_SEED_LEN];
    uint8_t z[GEMBOK_SEED_LEN];
    unsigned k;
    char t[32];
    gembok_err e;
    int rc = parse_options(argc, argv, opts, sizeof opts / sizeof opts[0], NULL);

    if (rc != 0)
        return rc;
    rc = option_level(k_text, &k);
    if (rc != 0)
        return rc;
    if (ek_path == NULL || dk_path == NULL)
        return usage_error("perintah keygen butuh --ek BERKAS dan --dk BERKAS");
    if ((d_path == NULL) != (z_path == NULL))
        return usage_error("berikan --d dan --z bersama-sama, atau tidak keduanya");
    if (seed_input(a, d_path, "d", d) != 0 || seed_input(a, z_path, "z", z) != 0)
        return EXIT_FAIL;
    e = gembok_keygen(&a->dev, k, d, z, ek, gembok_ek_len(k), dk, gembok_dk_len(k));
    if (e != GEMBOK_OK)
        return chip_error("KeyGen", e);
    if (save_hex(ek_path, "ek", ek, gembok_ek_len(k), 0) != 0
        || save_hex(dk_path, "dk", dk, gembok_dk_len(k), 1) != 0)
        return EXIT_FAIL;
    note("KeyGen %s berhasil, %" PRIu32 " siklus (%s)", level_name(k),
         gembok_last_cycles(&a->dev), ms_text(gembok_last_cycles(&a->dev), t));
    return 0;
}

static int cmd_encaps(app *a, int argc, char **argv)
{
    static uint8_t ek[GEMBOK_EK_MAX];
    static uint8_t ct[GEMBOK_CT_MAX];
    const char *k_text = NULL;
    const char *ek_path = NULL;
    const char *ct_path = NULL;
    const char *shared_path = NULL;
    const char *m_path = NULL;
    const option opts[] = {
        {"-k", &k_text}, {"--ek", &ek_path}, {"--ct", &ct_path},
        {"--kunci-bersama", &shared_path}, {"--m", &m_path}
    };
    uint8_t m[GEMBOK_SEED_LEN];
    uint8_t shared[GEMBOK_SHARED_LEN];
    unsigned k;
    char t[32];
    gembok_err e;
    int rc = parse_options(argc, argv, opts, sizeof opts / sizeof opts[0], NULL);

    if (rc != 0)
        return rc;
    rc = option_level(k_text, &k);
    if (rc != 0)
        return rc;
    if (ek_path == NULL || ct_path == NULL || shared_path == NULL)
        return usage_error("perintah encaps butuh --ek, --ct, dan --kunci-bersama");
    if (load_hex(ek_path, "ek", ek, gembok_ek_len(k)) != 0 || seed_input(a, m_path, "m", m) != 0)
        return EXIT_FAIL;
    e = gembok_encaps(&a->dev, k, ek, gembok_ek_len(k), m, ct, gembok_ct_len(k), shared);
    if (e != GEMBOK_OK)
        return chip_error("Encaps", e);
    if (save_hex(ct_path, "ct", ct, gembok_ct_len(k), 0) != 0
        || save_hex(shared_path, "kunci bersama", shared, sizeof shared, 1) != 0)
        return EXIT_FAIL;
    note("Encaps %s berhasil, %" PRIu32 " siklus (%s)", level_name(k),
         gembok_last_cycles(&a->dev), ms_text(gembok_last_cycles(&a->dev), t));
    return 0;
}

static int cmd_decaps(app *a, int argc, char **argv)
{
    static uint8_t dk[GEMBOK_DK_MAX];
    static uint8_t ct[GEMBOK_CT_MAX];
    const char *k_text = NULL;
    const char *dk_path = NULL;
    const char *ct_path = NULL;
    const char *shared_path = NULL;
    const option opts[] = {
        {"-k", &k_text}, {"--dk", &dk_path}, {"--ct", &ct_path}, {"--kunci-bersama", &shared_path}
    };
    uint8_t shared[GEMBOK_SHARED_LEN];
    unsigned k;
    char t[32];
    gembok_err e;
    int rc = parse_options(argc, argv, opts, sizeof opts / sizeof opts[0], NULL);

    if (rc != 0)
        return rc;
    rc = option_level(k_text, &k);
    if (rc != 0)
        return rc;
    if (dk_path == NULL || ct_path == NULL)
        return usage_error("perintah decaps butuh --dk BERKAS dan --ct BERKAS");
    if (load_hex(dk_path, "dk", dk, gembok_dk_len(k)) != 0
        || load_hex(ct_path, "ct", ct, gembok_ct_len(k)) != 0)
        return EXIT_FAIL;
    e = gembok_decaps(&a->dev, k, dk, gembok_dk_len(k), ct, gembok_ct_len(k), shared);
    if (e != GEMBOK_OK)
        return chip_error("Decaps", e);
    if (shared_path != NULL) {
        if (save_hex(shared_path, "kunci bersama", shared, sizeof shared, 1) != 0)
            return EXIT_FAIL;
    } else {
        emit_hex(NULL, "kunci-bersama", shared, sizeof shared);
    }
    note("Decaps %s berhasil, %" PRIu32 " siklus (%s)", level_name(k),
         gembok_last_cycles(&a->dev), ms_text(gembok_last_cycles(&a->dev), t));
    return 0;
}

static int cmd_puf_measure(app *a, int argc, char **argv)
{
    const char *idx_text = NULL;
    const char *count_text = NULL;
    const option opts[] = {{"--idx", &idx_text}, {"--jumlah", &count_text}};
    uint32_t pairs = a->caps.oscillators > 1 ? a->caps.oscillators - 1u : 0;
    uint32_t first = 0;
    uint32_t count = 1;
    int rc = parse_options(argc, argv, opts, sizeof opts / sizeof opts[0], NULL);

    if (rc != 0)
        return rc;
    if (pairs > GEMBOK_PUF_IDX_MAX + 1u)
        pairs = GEMBOK_PUF_IDX_MAX + 1u;
    if (idx_text != NULL && (parse_u32(idx_text, &first) != 0 || first >= pairs))
        return usage_error("nilai --idx tidak sah: %s (0 sampai %" PRIu32 ")", idx_text, pairs - 1u);
    if (count_text != NULL
        && (parse_u32(count_text, &count) != 0 || count == 0 || count > pairs - first))
        return usage_error("nilai --jumlah tidak sah: %s (1 sampai %" PRIu32 ")", count_text,
                           pairs - first);
    for (uint32_t pair = first; pair < first + count; pair++) {
        uint32_t c0;
        uint32_t c1;
        gembok_err e = gembok_puf_measure(&a->dev, (unsigned)pair, &c0, &c1);

        if (e != GEMBOK_OK)
            return chip_error("PUF_MEASURE", e);
        if (pair == first)
            printf("%-9s%10s%10s%10s\n", "pasangan", "osc c", "osc c+1", "selisih");
        printf("%-9" PRIu32 "%10" PRIu32 "%10" PRIu32 "%10ld\n", pair, c0, c1, (long)c0 - (long)c1);
    }
    return 0;
}

static const command COMMANDS[] = {
    {"info", cmd_info},
    {"selftest", cmd_selftest},
    {"acvp", cmd_acvp},
    {"puf-enroll", cmd_puf_enroll},
    {"puf-recon", cmd_puf_recon},
    {"enroll", cmd_enroll},
    {"prove", cmd_prove},
    {"wipe", cmd_wipe},
    {"keygen", cmd_keygen},
    {"encaps", cmd_encaps},
    {"decaps", cmd_decaps},
    {"puf-measure", cmd_puf_measure}
};

static const command *find_command(const char *name)
{
    for (size_t i = 0; i < sizeof COMMANDS / sizeof COMMANDS[0]; i++) {
        if (strcmp(name, COMMANDS[i].name) == 0)
            return &COMMANDS[i];
    }
    return NULL;
}

static int check_commands(int argc, char **argv, int first)
{
    int start = first;

    for (int i = first; i <= argc; i++) {
        if (i < argc && strcmp(argv[i], "+") != 0)
            continue;
        if (i == start)
            return usage_error("perintah kosong di sekitar tanda +");
        if (find_command(argv[start]) == NULL)
            return usage_error("perintah tidak dikenal: %s", argv[start]);
        start = i + 1;
    }
    return 0;
}

static int run_commands(app *a, int argc, char **argv, int first)
{
    int start = first;

    for (int i = first; i <= argc; i++) {
        int rc;

        if (i < argc && strcmp(argv[i], "+") != 0)
            continue;
        rc = find_command(argv[start])->run(a, i - start, argv + start);
        fflush(stdout);
        if (rc != 0)
            return rc;
        start = i + 1;
    }
    return 0;
}

static int connect_chip(app *a, gembok_io *io, uint32_t base)
{
    char why[160];
    gembok_err e;

    if (gembok_io_open(io, base, why, sizeof why) != 0) {
        say_error("%s", why);
        return EXIT_FAIL;
    }
    gembok_init(&a->dev, io);
    e = gembok_probe(&a->dev, &a->version);
    if (e == GEMBOK_DRV_ID) {
        say_error("register ID di 0x%08" PRIX32 " berisi 0x%08" PRIX32 ", seharusnya 0x%08" PRIX32
                  ". Periksa alamat dasar (--base) dan bitstream FPGA.",
                  base, gembok_reg_read(&a->dev, GEMBOK_REG_ID), (uint32_t)GEMBOK_ID_VALUE);
        return EXIT_FAIL;
    }
    if (e != GEMBOK_OK)
        return chip_error("pengenalan IP", e);
    e = gembok_wait_ready(&a->dev);
    if (e != GEMBOK_OK)
        return chip_error("menunggu chip siap", e);
    gembok_get_caps(&a->dev, &a->caps);
    return 0;
}

int main(int argc, char **argv)
{
    static app a;
    gembok_io io = {NULL, NULL, NULL};
    uint32_t base = GEMBOK_IO_BASE_DEFAULT;
    int first = 1;
    int rc;

    if (argc > 0 && argv[0] != NULL) {
        const char *slash = strrchr(argv[0], '/');

        prog = slash != NULL ? slash + 1 : argv[0];
    }
    while (first < argc && argv[first][0] == '-') {
        const char *name = argv[first];
        const char *value = first + 1 < argc ? argv[first + 1] : NULL;
        size_t seed_len = 0;

        if (strcmp(name, "-h") == 0 || strcmp(name, "--help") == 0) {
            print_usage(stdout);
            return 0;
        }
        if (strcmp(name, "--base") == 0) {
            if (parse_u32(value, &base) != 0)
                return usage_error("opsi --base butuh alamat, misalnya 0x%08" PRIX32, (uint32_t)GEMBOK_IO_BASE_DEFAULT);
        } else if (strcmp(name, "--benih") == 0) {
            if (value == NULL || hex_decode(value, a.seed, sizeof a.seed, &seed_len) != 0
                || seed_len != sizeof a.seed)
                return usage_error("opsi --benih butuh 64 digit heksadesimal");
            a.seeded = 1;
        } else {
            return usage_error("opsi tidak dikenal: %s", name);
        }
        first += 2;
    }
    if (first >= argc) {
        print_usage(stderr);
        return EXIT_USAGE;
    }
    rc = check_commands(argc, argv, first);
    if (rc != 0)
        return rc;

    rc = connect_chip(&a, &io, base);
    if (rc == 0)
        rc = run_commands(&a, argc, argv, first);
    if (io.ctx != NULL)
        gembok_io_close(&io);
    return rc;
}
