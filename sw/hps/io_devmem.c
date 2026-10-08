#define _FILE_OFFSET_BITS 64
#define _POSIX_C_SOURCE 200809L

#include <errno.h>
#include <fcntl.h>
#include <stdio.h>
#include <stdlib.h>
#include <string.h>
#include <sys/mman.h>
#include <sys/types.h>
#include <unistd.h>

#include "gembok_io.h"

#define DEVMEM_PATH "/dev/mem"

typedef struct {
    int fd;
    void *map;
    size_t map_len;
    volatile uint32_t *reg;
} devmem;

static uint32_t devmem_read32(void *ctx, uint32_t offset)
{
    devmem *m = ctx;

    return m->reg[(offset & (GEMBOK_SPAN - 1u)) / 4u];
}

static void devmem_write32(void *ctx, uint32_t offset, uint32_t value)
{
    devmem *m = ctx;

    m->reg[(offset & (GEMBOK_SPAN - 1u)) / 4u] = value;
}

const char *gembok_io_name(void)
{
    return DEVMEM_PATH;
}

int gembok_io_open(gembok_io *io, uint32_t base, char *err, size_t err_len)
{
    devmem *m;
    long page = sysconf(_SC_PAGESIZE);
    size_t in_page;

    if (base % 4u != 0) {
        snprintf(err, err_len, "alamat dasar 0x%08lX bukan kelipatan 4", (unsigned long)base);
        return -1;
    }
    if (page <= 0) {
        snprintf(err, err_len, "ukuran halaman memori tidak diketahui");
        return -1;
    }
    m = malloc(sizeof *m);
    if (m == NULL) {
        snprintf(err, err_len, "kehabisan memori");
        return -1;
    }
    m->fd = open(DEVMEM_PATH, O_RDWR | O_SYNC);
    if (m->fd < 0) {
        int why = errno;

        snprintf(err, err_len, "tidak bisa membuka %s: %s%s", DEVMEM_PATH, strerror(why),
                 (why == EACCES || why == EPERM) ? " (jalankan sebagai root)" : "");
        free(m);
        return -1;
    }
    in_page = (size_t)(base % (uint32_t)page);
    m->map_len = GEMBOK_SPAN + in_page;
    m->map = mmap(NULL, m->map_len, PROT_READ | PROT_WRITE, MAP_SHARED, m->fd,
                  (off_t)(base - (uint32_t)in_page));
    if (m->map == MAP_FAILED) {
        snprintf(err, err_len, "mmap %s pada 0x%08lX gagal: %s",
                 DEVMEM_PATH, (unsigned long)base, strerror(errno));
        close(m->fd);
        free(m);
        return -1;
    }
    m->reg = (volatile uint32_t *)(void *)((uint8_t *)m->map + in_page);
    io->read32 = devmem_read32;
    io->write32 = devmem_write32;
    io->ctx = m;
    return 0;
}

void gembok_io_close(gembok_io *io)
{
    devmem *m = io->ctx;

    if (m == NULL)
        return;
    munmap(m->map, m->map_len);
    close(m->fd);
    free(m);
    io->ctx = NULL;
}
