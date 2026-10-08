#include <cstdint>
#include <cstdio>
#include <cstdlib>
#include <new>

#include "Vgembok_top.h"
#include "verilated.h"

#include "gembok_io.h"

namespace {

const unsigned RESET_CYCLES = 5;
const unsigned SETTLE_CYCLES = 4;

struct Sim {
    VerilatedContext context;
    Vgembok_top top;
    unsigned long long cycles;
    unsigned long long reads;
    unsigned long long writes;
    unsigned long idle_gap;
    bool report;

    Sim() : top(&context), cycles(0), reads(0), writes(0), idle_gap(0), report(false) {}
};

void clock_cycle(Sim *s)
{
    s->top.eval();
    s->top.clk = 1;
    s->top.eval();
    s->top.clk = 0;
    s->top.eval();
    s->cycles++;
}

void idle_cycles(Sim *s, unsigned long n)
{
    for (unsigned long i = 0; i < n; i++)
        clock_cycle(s);
}

void check_offset(uint32_t offset)
{
    if (offset >= GEMBOK_SPAN || (offset & 3u) != 0) {
        std::fprintf(stderr, "simulasi: offset bus tidak sah 0x%08lX\n", (unsigned long)offset);
        std::abort();
    }
}

uint32_t sim_read32(void *ctx, uint32_t offset)
{
    Sim *s = static_cast<Sim *>(ctx);
    uint32_t value;

    check_offset(offset);
    s->top.avs_address = offset >> 2;
    s->top.avs_read = 1;
    clock_cycle(s);
    s->top.avs_read = 0;
    value = s->top.avs_readdata;
    s->reads++;
    idle_cycles(s, s->idle_gap);
    return value;
}

void sim_write32(void *ctx, uint32_t offset, uint32_t value)
{
    Sim *s = static_cast<Sim *>(ctx);

    check_offset(offset);
    s->top.avs_address = offset >> 2;
    s->top.avs_writedata = value;
    s->top.avs_write = 1;
    clock_cycle(s);
    s->top.avs_write = 0;
    s->writes++;
    idle_cycles(s, s->idle_gap);
}

unsigned long env_number(const char *name)
{
    const char *text = std::getenv(name);

    return text != nullptr ? std::strtoul(text, nullptr, 0) : 0;
}

}

extern "C" const char *gembok_io_name(void)
{
    return "simulasi Verilator gembok_top";
}

extern "C" int gembok_io_open(gembok_io *io, uint32_t base, char *err, size_t err_len)
{
    Sim *s = new (std::nothrow) Sim;

    (void)base;
    if (s == nullptr) {
        std::snprintf(err, err_len, "kehabisan memori");
        return -1;
    }
    s->idle_gap = env_number("GEMBOK_SIM_JEDA");
    s->report = env_number("GEMBOK_SIM_STAT") != 0;
    s->top.clk = 0;
    s->top.reset = 1;
    s->top.avs_address = 0;
    s->top.avs_read = 0;
    s->top.avs_write = 0;
    s->top.avs_writedata = 0;
    idle_cycles(s, RESET_CYCLES);
    s->top.reset = 0;
    idle_cycles(s, SETTLE_CYCLES);
    io->read32 = sim_read32;
    io->write32 = sim_write32;
    io->ctx = s;
    return 0;
}

extern "C" void gembok_io_close(gembok_io *io)
{
    Sim *s = static_cast<Sim *>(io->ctx);

    if (s == nullptr)
        return;
    if (s->report)
        std::fprintf(stderr, "simulasi: %llu siklus clock, %llu baca, %llu tulis, jeda %lu siklus\n",
                     s->cycles, s->reads, s->writes, s->idle_gap);
    s->top.final();
    delete s;
    io->ctx = nullptr;
}
