import random

import cocotb
from cocotb.triggers import FallingEdge

import mlkem
from tbutil import reset, start_clock, tick, val

def idle(dut):
    for s in ("init", "kmode", "finish", "a_valid", "a_data", "start", "smode", "slot", "tb_addr"):
        getattr(dut, s).value = 0

async def absorb(dut, kmode, msg):
    dut.init.value = 1
    dut.kmode.value = kmode
    await FallingEdge(dut.clk)
    dut.init.value = 0
    for b in msg:
        while not val(dut.a_ready):
            await FallingEdge(dut.clk)
        dut.a_valid.value = 1
        dut.a_data.value = b
        await FallingEdge(dut.clk)
    dut.a_valid.value = 0
    while not val(dut.a_ready):
        await FallingEdge(dut.clk)
    dut.finish.value = 1
    await FallingEdge(dut.clk)
    dut.finish.value = 0

async def sample(dut, smode, slot):
    dut.smode.value = smode
    dut.slot.value = slot
    dut.start.value = 1
    await FallingEdge(dut.clk)
    dut.start.value = 0
    n = 1
    while not val(dut.done):
        await FallingEdge(dut.clk)
        n += 1
        assert n < 5000, "sampler macet"
    return n

async def dump(dut, slot):
    f = []
    for w in range(128):
        dut.tb_addr.value = (slot << 7) | w
        await FallingEdge(dut.clk)
        await FallingEdge(dut.clk)
        d = val(dut.tb_rdata)
        f += [d & 0xFFF, d >> 12]
    return f

@cocotb.test()
async def sample_ntt(dut):
    rng = random.Random(21)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    cycles = []
    for t in range(24):
        rho = rng.randbytes(32)
        i, j = rng.randrange(4), rng.randrange(4)
        slot = rng.randrange(16)
        await absorb(dut, 2, rho + bytes([j, i]))
        cycles.append(await sample(dut, 0, slot))
        assert await dump(dut, slot) == mlkem.sample_ntt(rho, j, i), f"SampleNTT salah pada kasus {t}"
    dut._log.info("SampleNTT siklus: min %d, maks %d", min(cycles), max(cycles))

@cocotb.test()
async def sample_ntt_blok_keempat(dut):
    import hashlib
    rng = random.Random(22)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    found = 0
    tries = 0
    while found < 2 and tries < 200000:
        tries += 1
        rho = rng.randbytes(32)
        s = hashlib.shake_128(rho + b"\x00\x00").digest(504)
        n = 0
        for p in range(0, 504, 3):
            d1 = s[p] + 256 * (s[p + 1] % 16)
            d2 = (s[p + 1] // 16) + 16 * s[p + 2]
            n += (d1 < mlkem.Q) + (d2 < mlkem.Q)
        if n < 256:
            found += 1
            await absorb(dut, 2, rho + bytes([0, 0]))
            c = await sample(dut, 0, 7)
            assert await dump(dut, 7) == mlkem.sample_ntt(rho, 0, 0)
            dut._log.info("rho dengan 4 blok ditemukan setelah %d percobaan, %d siklus", tries, c)
    assert found == 2, "tidak menemukan rho yang butuh 4 blok"

@cocotb.test()
async def cbd(dut):
    rng = random.Random(23)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    for eta, smode in ((2, 1), (3, 2)):
        seeds = [bytes(32), b"\xff" * 32] + [rng.randbytes(32) for _ in range(10)]
        for t, seed in enumerate(seeds):
            nonce = rng.randrange(256)
            slot = rng.randrange(16)
            await absorb(dut, 3, seed + bytes([nonce]))
            c = await sample(dut, smode, slot)
            want = mlkem.sample_poly_cbd(eta, mlkem.PRF(eta, seed, nonce))
            assert await dump(dut, slot) == want, f"CBD eta={eta} salah pada kasus {t}"
        dut._log.info("CBD eta=%d: %d siklus", eta, c)
