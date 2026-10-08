from cocotb.triggers import FallingEdge, RisingEdge

import ucode as U
from tbutil import val

REG_ID, REG_VERSION, REG_CTRL, REG_STATUS, REG_PARAM, REG_CTXLEN = 0x00, 0x04, 0x08, 0x0C, 0x10, 0x14
REG_CYCLES, REG_CAPS, REG_COOLDOWN, REG_PROOFS, REG_THRESH, REG_PUF_IDX, REG_PUF_DBG = 0x18, 0x1C, 0x20, 0x24, 0x28, 0x2C, 0x30
MEM_BASE = 0x8000

CMD = {"KEYGEN": 1, "ENCAPS": 2, "DECAPS": 3, "CHECK_EK": 4, "CHECK_DK": 5, "ENROLL": 6, "PROVE": 7,
       "PUF_ENROLL": 8, "PUF_RECON": 9, "PUF_MEASURE": 10, "WIPE": 15}

ST_BUSY, ST_DONE, ST_ERROR, ST_PUF_READY, ST_COOLING, ST_BOOTED = 1, 2, 4, 0x100, 0x200, 0x400
E_OK, E_BAD_EK, E_BAD_DK, E_BAD_HELPER, E_BAD_PARAM, E_NO_KEY, E_RATE, E_BAD_CMD, E_PUF_FAIL = range(9)

class BusDev:
    def __init__(self, dut):
        self.dut = dut
        self.ctx_len = 0
        self.last_cycles = 0

    async def reset(self):
        d = self.dut
        d.avs_address.value = 0
        d.avs_read.value = 0
        d.avs_write.value = 0
        d.avs_writedata.value = 0
        d.reset.value = 1
        for _ in range(5):
            await FallingEdge(d.clk)
        d.reset.value = 0
        for _ in range(4):
            await FallingEdge(d.clk)
        while not (await self.reg_read(REG_STATUS)) & ST_BOOTED:
            for _ in range(200):
                await FallingEdge(d.clk)

    async def reg_write(self, off, value):
        d = self.dut
        d.avs_address.value = off >> 2
        d.avs_writedata.value = value
        d.avs_write.value = 1
        await FallingEdge(d.clk)
        d.avs_write.value = 0

    async def reg_read(self, off):
        d = self.dut
        d.avs_address.value = off >> 2
        d.avs_read.value = 1
        await FallingEdge(d.clk)
        d.avs_read.value = 0
        return val(d.avs_readdata)

    async def write(self, addr, data):
        d = self.dut
        d.avs_write.value = 1
        for i, b in enumerate(data):
            d.avs_address.value = (MEM_BASE >> 2) + addr + i
            d.avs_writedata.value = b
            await FallingEdge(d.clk)
        d.avs_write.value = 0

    async def read(self, addr, n):
        d = self.dut
        out = bytearray()
        d.avs_read.value = 1
        d.avs_address.value = (MEM_BASE >> 2) + addr
        await FallingEdge(d.clk)
        for i in range(n):
            out.append(val(d.avs_readdata) & 0xFF)
            if i + 1 < n:
                d.avs_address.value = (MEM_BASE >> 2) + addr + i + 1
            await FallingEdge(d.clk)
        d.avs_read.value = 0
        return bytes(out)

    async def start(self, name, k=3):
        await self.reg_write(REG_PARAM, k)
        await self.reg_write(REG_CTXLEN, self.ctx_len)
        await self.reg_write(REG_CTRL, CMD[name])
        await FallingEdge(self.dut.clk)

    async def wait_done(self):
        d = self.dut
        while not val(d.done_o):
            await RisingEdge(d.done_o)
            await FallingEdge(d.clk)
        st = await self.reg_read(REG_STATUS)
        assert st & ST_DONE and not st & ST_BUSY, hex(st)
        self.last_cycles = await self.reg_read(REG_CYCLES)
        return (st >> 4) & 0xF

    async def cmd(self, name, k=3):
        await self.start(name, k)
        return await self.wait_done()

    async def run(self, program, k):
        return await self.cmd(program, k)

    async def status(self):
        return await self.reg_read(REG_STATUS)
