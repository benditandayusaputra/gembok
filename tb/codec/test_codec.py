import random

import cocotb
from cocotb.triggers import FallingEdge

import mlkem
from tbutil import reset, start_clock, tick, val

Q = mlkem.Q
ALL_D = (1, 4, 5, 10, 11, 12)

def idle(dut):
    for s in ("start", "dir", "d", "slot", "i_valid", "i_data", "o_ready", "tb_addr", "tb_we", "tb_wdata"):
        getattr(dut, s).value = 0

async def load(dut, slot, f):
    dut.tb_we.value = 1
    for w in range(128):
        dut.tb_addr.value = (slot << 7) | w
        dut.tb_wdata.value = f[2 * w] | (f[2 * w + 1] << 12)
        await FallingEdge(dut.clk)
    dut.tb_we.value = 0

async def dump(dut, slot):
    f = []
    for w in range(128):
        dut.tb_addr.value = (slot << 7) | w
        await FallingEdge(dut.clk)
        await FallingEdge(dut.clk)
        x = val(dut.tb_rdata)
        f += [x & 0xFFF, x >> 12]
    return f

async def decode(dut, d, slot, data, rng, stall=0.0):
    dut.dir.value = 0
    dut.d.value = d
    dut.slot.value = slot
    dut.start.value = 1
    await FallingEdge(dut.clk)
    dut.start.value = 0
    i = 0
    n = 1
    extra_ready = 0
    while not val(dut.done):
        if i < len(data) and rng.random() >= stall:
            dut.i_valid.value = 1
            dut.i_data.value = data[i]
            if val(dut.i_ready):
                i += 1
        else:
            dut.i_valid.value = 1 if i >= len(data) else 0
            dut.i_data.value = 0xA5
            if i >= len(data) and val(dut.i_ready):
                extra_ready += 1
        await FallingEdge(dut.clk)
        n += 1
        assert n < 20000, "dekode macet"
    dut.i_valid.value = 0
    assert i == len(data), f"byte terpakai {i} dari {len(data)}"
    assert extra_ready == 0, "dekode mengambil byte melebihi panjangnya"
    return n

async def encode(dut, d, slot, rng, stall=0.0):
    dut.dir.value = 1
    dut.d.value = d
    dut.slot.value = slot
    dut.start.value = 1
    await FallingEdge(dut.clk)
    dut.start.value = 0
    out = bytearray()
    n = 1
    while not val(dut.done):
        if val(dut.o_valid) and rng.random() >= stall:
            dut.o_ready.value = 1
            out.append(val(dut.o_data))
        else:
            dut.o_ready.value = 0
        await FallingEdge(dut.clk)
        n += 1
        assert n < 20000, "enkode macet"
    dut.o_ready.value = 0
    return bytes(out), n

def polys(rng, bound):
    return [
        [0] * 256,
        [bound - 1] * 256,
        [(i * 37) % bound for i in range(256)],
    ] + [[rng.randrange(bound) for _ in range(256)] for _ in range(4)]

@cocotb.test()
async def dekode(dut):
    rng = random.Random(31)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    for d in ALL_D:
        bound = Q if d == 12 else (1 << d)
        for t, vals in enumerate(polys(rng, bound)):
            data = mlkem.byte_encode(d, vals)
            slot = rng.randrange(16)
            c = await decode(dut, d, slot, data, rng, stall=0.3 if t % 2 else 0.0)
            want = vals if d == 12 else mlkem.decompress_poly(d, vals)
            assert await dump(dut, slot) == want, f"dekode d={d} salah pada kasus {t}"
            assert val(dut.range_err) == 0
        dut._log.info("dekode d=%d: %d siklus tanpa jeda", d, await decode(dut, d, 0, mlkem.byte_encode(d, [0] * 256), rng))

@cocotb.test()
async def dekode_cek_modulus(dut):
    rng = random.Random(32)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    for pos in (0, 1, 127, 254, 255):
        for bad in (3329, 3330, 4095):
            vals = [rng.randrange(Q) for _ in range(256)]
            raw = list(vals)
            raw[pos] = bad
            bits = []
            for x in raw:
                bits += [(x >> b) & 1 for b in range(12)]
            data = mlkem.bits_to_bytes(bits)
            await decode(dut, 12, 3, data, rng)
            assert val(dut.range_err) == 1, f"nilai {bad} di posisi {pos} tidak terdeteksi"
            vals[pos] = bad - Q
            assert await dump(dut, 3) == vals
    vals = [rng.randrange(Q) for _ in range(256)]
    await decode(dut, 12, 3, mlkem.byte_encode(12, vals), rng)
    assert val(dut.range_err) == 0

@cocotb.test()
async def enkode(dut):
    rng = random.Random(33)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    for d in ALL_D:
        for t, vals in enumerate(polys(rng, Q)):
            slot = rng.randrange(16)
            await load(dut, slot, vals)
            got, c = await encode(dut, d, slot, rng, stall=0.3 if t % 2 else 0.0)
            want = mlkem.byte_encode(d, vals if d == 12 else mlkem.compress_poly(d, vals))
            assert got == want, f"enkode d={d} salah pada kasus {t}"
            if t == 0:
                dut._log.info("enkode d=%d: %d siklus tanpa jeda, %d byte", d, c, len(got))

@cocotb.test()
async def kompresi_semua_nilai(dut):
    rng = random.Random(34)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    allv = list(range(Q))
    chunks = [allv[i:i + 256] for i in range(0, Q, 256)]
    chunks[-1] = chunks[-1] + [0] * (256 - len(chunks[-1]))
    for d in (1, 4, 5, 10, 11):
        for vals in chunks:
            await load(dut, 2, vals)
            got, _ = await encode(dut, d, 2, rng)
            assert got == mlkem.byte_encode(d, mlkem.compress_poly(d, vals)), f"kompresi d={d} salah"
