import hashlib
import random

import cocotb
from cocotb.triggers import FallingEdge

from tbutil import reset, start_clock, tick, val

MODES = {
    0: ("sha3_256", 136),
    1: ("sha3_512", 72),
    2: ("shake_128", 168),
    3: ("shake_256", 136),
}

def ref(mode: int, msg: bytes, n: int) -> bytes:
    name, _ = MODES[mode]
    h = hashlib.new(name, msg)
    if name.startswith("shake"):
        return h.digest(n)
    return h.digest()[:n]

def idle(dut):
    dut.init.value = 0
    dut.finish.value = 0
    dut.triple.value = 0
    dut.a_valid.value = 0
    dut.a_data.value = 0
    dut.q_ready.value = 0
    dut.t_ready.value = 0
    dut.mode.value = 0

async def do_init(dut, mode):
    dut.init.value = 1
    dut.mode.value = mode
    await FallingEdge(dut.clk)
    dut.init.value = 0

async def absorb(dut, msg: bytes, rng, stall: float):
    i = 0
    while i < len(msg):
        if val(dut.a_ready) and rng.random() >= stall:
            dut.a_valid.value = 1
            dut.a_data.value = msg[i]
            i += 1
        else:
            dut.a_valid.value = 0
        await FallingEdge(dut.clk)
    dut.a_valid.value = 0
    while not val(dut.a_ready):
        await FallingEdge(dut.clk)
    dut.finish.value = 1
    await FallingEdge(dut.clk)
    dut.finish.value = 0

async def squeeze_bytes(dut, n: int, rng, stall: float) -> bytes:
    out = bytearray()
    guard = 0
    while len(out) < n:
        if val(dut.q_valid) and rng.random() >= stall:
            dut.q_ready.value = 1
            out.append(val(dut.q_data))
        else:
            dut.q_ready.value = 0
        await FallingEdge(dut.clk)
        guard += 1
        assert guard < 200000, "peras macet"
    dut.q_ready.value = 0
    return bytes(out)

async def squeeze_triples(dut, n_triples: int, rng, stall: float) -> bytes:
    out = bytearray()
    dut.triple.value = 1
    guard = 0
    while len(out) < 3 * n_triples:
        if val(dut.t_valid) and rng.random() >= stall:
            dut.t_ready.value = 1
            out += val(dut.t_data).to_bytes(3, "little")
        else:
            dut.t_ready.value = 0
        await FallingEdge(dut.clk)
        guard += 1
        assert guard < 200000, "peras macet"
    dut.t_ready.value = 0
    dut.triple.value = 0
    return bytes(out)

async def one_hash(dut, mode, msg, n_out, rng, stall=0.0):
    await do_init(dut, mode)
    await absorb(dut, msg, rng, stall)
    got = await squeeze_bytes(dut, n_out, rng, stall)
    want = ref(mode, msg, n_out)
    assert got == want, f"mode {mode} len {len(msg)} out {n_out}: {got.hex()} != {want.hex()}"

@cocotb.test()
async def panjang_batas_blok(dut):
    rng = random.Random(1)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    for mode, (name, rate) in MODES.items():
        n_out = {0: 32, 1: 64, 2: 2 * rate + 5, 3: 2 * rate + 5}[mode]
        for n in (0, 1, 2, rate - 2, rate - 1, rate, rate + 1, 2 * rate - 1, 2 * rate, 2 * rate + 1, 3 * rate + 7):
            await one_hash(dut, mode, rng.randbytes(n), n_out, rng)

@cocotb.test()
async def acak_dengan_jeda(dut):
    rng = random.Random(2)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    for _ in range(40):
        mode = rng.randrange(4)
        rate = MODES[mode][1]
        n = rng.randrange(0, 500)
        n_out = {0: 32, 1: 64}.get(mode, rng.randrange(1, 3 * rate))
        await one_hash(dut, mode, rng.randbytes(n), n_out, rng, stall=0.3)

@cocotb.test()
async def peras_tiga_byte(dut):
    rng = random.Random(3)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    for stall in (0.0, 0.4):
        for _ in range(4):
            msg = rng.randbytes(34)
            await do_init(dut, 2)
            await absorb(dut, msg, rng, 0.0)
            await tick(dut, rng.randrange(0, 5))
            got = await squeeze_triples(dut, 56 * 3 + 11, rng, stall)
            want = ref(2, msg, len(got))
            assert got == want

@cocotb.test()
async def init_di_tengah_permutasi(dut):
    rng = random.Random(4)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    for cut in (1, 5, 23, 24, 30, 60):
        await do_init(dut, 3)
        await absorb(dut, rng.randbytes(200), rng, 0.0)
        await tick(dut, cut)
        await one_hash(dut, rng.randrange(4), rng.randbytes(rng.randrange(300)), 32, rng)

@cocotb.test()
async def jumlah_siklus(dut):
    rng = random.Random(5)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    await do_init(dut, 3)
    await absorb(dut, b"abc", rng, 0.0)
    n = 0
    while not val(dut.q_valid):
        await FallingEdge(dut.clk)
        n += 1
    assert n == 25, n
    t0 = cocotb.utils.get_sim_time("ns")
    got = await squeeze_bytes(dut, 136, rng, 0.0)
    t1 = cocotb.utils.get_sim_time("ns")
    assert (t1 - t0) == 136 * 20, (t1 - t0)
    assert got == ref(3, b"abc", 136)
