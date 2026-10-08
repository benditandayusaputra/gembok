from cocotb.triggers import FallingEdge, RisingEdge

import ucode as U
from tbutil import val

class CoreDev:
    def __init__(self, dut):
        self.dut = dut
        self.ctx_len = 0
        self.last_cycles = 0

    async def init(self):
        d = self.dut
        for s in ("cmd_start", "cmd_sel", "cmd_k", "ctx_len", "h_addr", "h_we", "h_wdata", "puf_key"):
            getattr(d, s).value = 0
        d.rst.value = 1
        for _ in range(4):
            await FallingEdge(d.clk)
        d.rst.value = 0
        await FallingEdge(d.clk)
        while val(d.busy):
            await RisingEdge(d.done)
            await FallingEdge(d.clk)

    async def write(self, addr, data):
        d = self.dut
        d.h_we.value = 1
        for i, b in enumerate(data):
            d.h_addr.value = addr + i
            d.h_wdata.value = b
            await FallingEdge(d.clk)
        d.h_we.value = 0

    async def read(self, addr, n):
        d = self.dut
        out = bytearray()
        d.h_we.value = 0
        d.h_addr.value = addr
        await FallingEdge(d.clk)
        for i in range(n):
            out.append(val(d.h_rdata))
            if i + 1 < n:
                d.h_addr.value = addr + i + 1
            await FallingEdge(d.clk)
        return bytes(out)

    async def run(self, program, k):
        d = self.dut
        d.cmd_sel.value = U.PROGRAMS.index(program)
        d.cmd_k.value = k
        d.ctx_len.value = self.ctx_len
        d.cmd_start.value = 1
        await FallingEdge(d.clk)
        d.cmd_start.value = 0
        await RisingEdge(d.done)
        await FallingEdge(d.clk)
        await FallingEdge(d.clk)
        self.last_cycles = val(d.cycles)
        return val(d.status)
