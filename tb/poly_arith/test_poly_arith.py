import random

import cocotb
from cocotb.triggers import FallingEdge

import mlkem
from tbutil import reset, start_clock, tick, val

Q = mlkem.Q
OP_NTT, OP_INTT, OP_PWM, OP_LIN = 0, 1, 2, 3

def idle(dut):
    dut.start.value = 0
    dut.op.value = 0
    dut.s_dst.value = 0
    dut.s_a.value = 0
    dut.s_b.value = 0
    dut.opt.value = 0
    dut.tb_sel.value = 0
    dut.tb_addr.value = 0
    dut.tb_we.value = 0
    dut.tb_wdata.value = 0

async def load(dut, slot, f):
    dut.tb_sel.value = 1
    dut.tb_we.value = 1
    for w in range(128):
        dut.tb_addr.value = (slot << 7) | w
        dut.tb_wdata.value = f[2 * w] | (f[2 * w + 1] << 12)
        await FallingEdge(dut.clk)
    dut.tb_we.value = 0
    dut.tb_sel.value = 0

async def dump(dut, slot):
    f = []
    dut.tb_sel.value = 1
    dut.tb_we.value = 0
    for w in range(128):
        dut.tb_addr.value = (slot << 7) | w
        await FallingEdge(dut.clk)
        await FallingEdge(dut.clk)
        d = val(dut.tb_rdata)
        f += [d & 0xFFF, d >> 12]
    dut.tb_sel.value = 0
    return f

async def run_op(dut, op, dst, a=0, b=0, opt=0):
    dut.op.value = op
    dut.s_dst.value = dst
    dut.s_a.value = a
    dut.s_b.value = b
    dut.opt.value = opt
    dut.start.value = 1
    await FallingEdge(dut.clk)
    dut.start.value = 0
    n = 1
    while not val(dut.done):
        await FallingEdge(dut.clk)
        n += 1
        assert n < 5000, "operasi macet"
    return n

def rand_poly(rng):
    return [rng.randrange(Q) for _ in range(256)]

def corner_polys():
    return [[0] * 256, [Q - 1] * 256, [1] + [0] * 255, [Q - 1 if i % 2 else 0 for i in range(256)]]

def intt_noscale(f):
    g = mlkem.ntt_inv(f)
    return [(x * 128) % Q for x in g]

@cocotb.test()
async def ntt_dan_intt(dut):
    rng = random.Random(11)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    polys = corner_polys() + [rand_poly(rng) for _ in range(6)]
    for t, f in enumerate(polys):
        slot = rng.randrange(16)
        await load(dut, slot, f)
        cyc = await run_op(dut, OP_NTT, slot)
        got = await dump(dut, slot)
        assert got == mlkem.ntt(f), f"NTT salah pada kasus {t}"
        cyc2 = await run_op(dut, OP_INTT, slot)
        got = await dump(dut, slot)
        assert got == [(x * 128) % Q for x in f], f"INTT salah pada kasus {t}"
        dut._log.info("kasus %d: NTT %d siklus, INTT %d siklus", t, cyc, cyc2)
    for _ in range(3):
        f = rand_poly(rng)
        await load(dut, 5, f)
        await run_op(dut, OP_INTT, 5)
        assert await dump(dut, 5) == intt_noscale(f)

@cocotb.test()
async def pwm(dut):
    rng = random.Random(12)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    cases = [(c, c2) for c in corner_polys() for c2 in corner_polys()[:2]]
    cases += [(rand_poly(rng), rand_poly(rng)) for _ in range(6)]
    for t, (f, g) in enumerate(cases):
        sd, sa, sb = rng.sample(range(16), 3)
        acc0 = rand_poly(rng)
        await load(dut, sa, f)
        await load(dut, sb, g)
        await load(dut, sd, acc0)
        prod = mlkem.multiply_ntts(f, g)
        cyc = await run_op(dut, OP_PWM, sd, sa, sb, opt=0)
        assert await dump(dut, sd) == prod, f"PWM salah pada kasus {t}"
        await load(dut, sd, acc0)
        await run_op(dut, OP_PWM, sd, sa, sb, opt=1)
        assert await dump(dut, sd) == mlkem.poly_add(acc0, prod), f"PWM akumulasi salah pada kasus {t}"
        assert await dump(dut, sa) == f
        assert await dump(dut, sb) == g
        if t == 0:
            dut._log.info("PWM %d siklus", cyc)
    f = rand_poly(rng)
    await load(dut, 3, f)
    await run_op(dut, OP_PWM, 9, 3, 3, opt=0)
    assert await dump(dut, 9) == mlkem.multiply_ntts(f, f)

@cocotb.test()
async def lin(dut):
    rng = random.Random(13)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    for t in range(8):
        x = rand_poly(rng) if t >= 2 else corner_polys()[t + 1]
        y = rand_poly(rng) if t >= 1 else [Q - 1] * 256
        for opt in range(4):
            sub, scale = opt & 1, (opt >> 1) & 1
            w = 3303 if scale else 1
            want = [((b - w * a) if sub else (b + w * a)) % Q for a, b in zip(x, y)]
            sd, sa, sb = rng.sample(range(16), 3)
            await load(dut, sa, x)
            await load(dut, sb, y)
            cyc = await run_op(dut, OP_LIN, sd, sa, sb, opt=opt)
            assert await dump(dut, sd) == want, f"LIN opt={opt} salah"
            await run_op(dut, OP_LIN, sa, sa, sb, opt=opt)
            assert await dump(dut, sa) == want, f"LIN di tempat (x) opt={opt} salah"
            await load(dut, sa, x)
            await run_op(dut, OP_LIN, sb, sa, sb, opt=opt)
            assert await dump(dut, sb) == want, f"LIN di tempat (y) opt={opt} salah"
        if t == 0:
            dut._log.info("LIN %d siklus", cyc)

@cocotb.test()
async def rantai_seperti_enkripsi(dut):
    rng = random.Random(14)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    k = 3
    a = [rand_poly(rng) for _ in range(k)]
    y = [rand_poly(rng) for _ in range(k)]
    e = rand_poly(rng)
    for j in range(k):
        await load(dut, j, y[j])
        await run_op(dut, OP_NTT, j)
    for j in range(k):
        await load(dut, 9, a[j])
        await run_op(dut, OP_PWM, 8, 9, j, opt=1 if j else 0)
    await run_op(dut, OP_INTT, 8)
    await load(dut, 9, e)
    await run_op(dut, OP_LIN, 8, 8, 9, opt=2)
    acc = [0] * 256
    for j in range(k):
        acc = mlkem.poly_add(acc, mlkem.multiply_ntts(a[j], mlkem.ntt(y[j])))
    want = mlkem.poly_add(mlkem.ntt_inv(acc), e)
    assert await dump(dut, 8) == want

@cocotb.test()
async def jumlah_siklus_tetap(dut):
    rng = random.Random(15)
    start_clock(dut)
    idle(dut)
    await reset(dut)
    seen = {}
    for f in corner_polys() + [rand_poly(rng) for _ in range(3)]:
        await load(dut, 0, f)
        await load(dut, 1, rand_poly(rng))
        for name, args in (("ntt", (OP_NTT, 0)), ("intt", (OP_INTT, 0)),
                           ("pwm", (OP_PWM, 2, 0, 1, 1)), ("lin", (OP_LIN, 2, 0, 1, 3))):
            c = await run_op(dut, *args)
            seen.setdefault(name, set()).add(c)
    for name, s in seen.items():
        assert len(s) == 1, f"{name}: jumlah siklus berubah-ubah {s}"
    dut._log.info("siklus: %s", {k: next(iter(v)) for k, v in seen.items()})
