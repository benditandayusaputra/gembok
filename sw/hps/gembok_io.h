#ifndef GEMBOK_IO_H
#define GEMBOK_IO_H

#include "gembok.h"

#ifdef __cplusplus
extern "C" {
#endif

#define GEMBOK_IO_BASE_DEFAULT 0xFF240000u

const char *gembok_io_name(void);
int gembok_io_open(gembok_io *io, uint32_t base, char *err, size_t err_len);
void gembok_io_close(gembok_io *io);

#ifdef __cplusplus
}
#endif

#endif
