import os
import random

import cocotb

import gembok
import mlkem
import ucode as U
from mlkem_dev import CoreDev
from rtl_driver import ADriver, load_acvp, params_of, run_acvp_case, slot_addr

@cocotb.test()
async def asap(dut):
    rng = random.Random(41)
    dev = CoreDev(dut)
    await dev.init()
    for k in (2, 3, 4):
        p = params_of(k)
        drv = ADriver(dev, k)
        d, z, m = rng.randbytes(32), rng.randbytes(32), rng.randbytes(32)
        ek, dk = await drv.keygen(d, z)
        c_kg = dev.last_cycles
        assert (ek, dk) == mlkem.keygen_internal(p, d, z), f"KeyGen k={k} salah"
        st, kk, c = await drv.encaps(ek, m)
        c_en = dev.last_cycles
        assert st == 0 and (kk, c) == mlkem.encaps_internal(p, ek, m), f"Encaps k={k} salah"
        st, k2 = await drv.decaps(dk, c)
        c_de = dev.last_cycles
        assert st == 0 and k2 == kk, f"Decaps k={k} salah"
        bad = bytes([c[5] ^ 0x10]) + c[1:] if False else c[:5] + bytes([c[5] ^ 0x10]) + c[6:]
        st, k3 = await drv.decaps(dk, bad)
        assert st == 0 and k3 == mlkem.decaps_internal(p, dk, bad) and k3 != kk, f"penolakan implisit k={k} salah"
        dut._log.info("k=%d siklus: KeyGen %d, Encaps %d, Decaps %d (ditolak: %d)", k, c_kg, c_en, c_de, dev.last_cycles)

@cocotb.test(skip=os.environ.get("ACVP", "1") == "0")
async def vektor_nist(dut):
    dev = CoreDev(dut)
    await dev.init()
    limit = int(os.environ.get("ACVP_LIMIT", "0"))
    seen = {}
    total = passed = 0
    gagal = []
    for pset, fn, tcid, t in load_acvp():
        key = (pset, fn)
        seen[key] = seen.get(key, 0) + 1
        if limit and seen[key] > limit:
            continue
        drv = ADriver(dev, mlkem.PARAMS[pset].k)
        ok = await run_acvp_case(drv, fn, t)
        total += 1
        passed += ok
        if not ok:
            gagal.append((pset, fn, tcid))
    dut._log.info("ACVP di RTL: %d dari %d kasus lolos", passed, total)
    assert passed == total, f"gagal: {gagal[:10]}"
    if not limit:
        assert total == 240

@cocotb.test()
async def identitas(dut):
    rng = random.Random(42)
    dev = CoreDev(dut)
    await dev.init()
    for k in (2, 3, 4):
        p = params_of(k)
        puf = rng.randbytes(32)
        dut.puf_key.value = int.from_bytes(puf, "little")
        ref = gembok.Chip(p, puf)
        drv = ADriver(dev, k)
        ek = await drv.enroll()
        c_en = dev.last_cycles
        assert ek == ref.enroll(), f"ENROLL k={k} salah"
        assert await dev.read(U.BASE_SLOT, 192) == bytes(192), "rahasia tidak terhapus setelah ENROLL"
        ver = gembok.Verifier(p, ek)
        for ctx in (b"", b"scan-001", rng.randbytes(255)):
            c, kk = ver.challenge()
            tag = await drv.prove(c, ctx)
            assert tag == ref.prove(c, ctx) and ver.check(kk, ctx, tag), f"PROVE k={k} salah"
            assert await dev.read(U.BASE_SLOT, 192) == bytes(192), "rahasia tidak terhapus setelah PROVE"
            assert await dev.read(U.BASE_DK, 384 * k) == bytes(384 * k)
        c_pr = dev.last_cycles
        bad = c[:9] + bytes([c[9] ^ 0x80]) + c[10:]
        tag2 = await drv.prove(bad, b"x")
        assert tag2 == ref.prove(bad, b"x") and not ver.check(kk, b"x", tag2)
        dut._log.info("k=%d siklus: ENROLL %d, PROVE %d (konteks 255 byte)", k, c_en, c_pr)
