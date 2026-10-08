import os
import random

import cocotb
from cocotb.triggers import FallingEdge, RisingEdge

import gembok
import mlkem
import puf_model
import ucode as U
from rtl_driver import ADriver, load_acvp, params_of, run_acvp_case, slot_addr
from tbutil import val
from top_dev import (CMD, E_BAD_CMD, E_BAD_HELPER, E_BAD_PARAM, E_NO_KEY, E_OK, E_PUF_FAIL,
                     E_RATE, MEM_BASE, REG_CAPS, REG_COOLDOWN, REG_CTRL, REG_CYCLES, REG_ID, REG_PARAM,
                     REG_PROOFS, REG_PUF_DBG, REG_PUF_IDX, REG_STATUS, REG_THRESH, REG_VERSION,
                     ST_BOOTED, ST_BUSY, ST_COOLING, ST_DONE, ST_PUF_READY, BusDev)

CONFIG = os.environ.get("GEMBOK_CONFIG", "dev")
DEV_KEY = (0x6b6f626d65672d6b6f626d65672d6b6f626d65672d6b6f626d65672d76656421).to_bytes(32, "little")
COOLDOWN = 3000
N_RO = 768
MASK_LEN = 96

def only(*configs):
    return cocotb.test(skip=CONFIG not in configs)

async def cooldown(dev):
    for _ in range(COOLDOWN + 50):
        await FallingEdge(dev.dut.clk)

async def setup(dut):
    dev = BusDev(dut)
    await dev.reset()
    return dev

async def dump_all(dev):
    return await dev.read(0, 8192)

def contains_any(haystack: bytes, secrets, n=8):
    for name, s in secrets:
        for off in range(0, len(s) - n + 1, n):
            piece = s[off:off + n]
            if piece != bytes(n) and piece in haystack:
                return name
    return None

@only("dev")
async def register_dan_boot(dut):
    dev = await setup(dut)
    assert await dev.reg_read(REG_ID) == 0x47454D42
    assert await dev.reg_read(REG_VERSION) == 0x00010000
    caps = await dev.reg_read(REG_CAPS)
    assert caps & 3 == 2 and (caps >> 16) & 0xFFF == N_RO and (caps >> 8) & 0xFF == 5, hex(caps)
    assert (caps >> 3) & 0x1F == 3 and not caps & 4 and caps >> 28 == 0, hex(caps)
    st = await dev.status()
    assert st & ST_BOOTED and not st & (ST_BUSY | ST_PUF_READY), hex(st)
    assert await dump_all(dev) == bytes(8192), "memori tidak kosong setelah boot"
    await dev.reg_write(REG_THRESH, 123)
    assert await dev.reg_read(REG_THRESH) == 123

@only("dev")
async def galat_perintah(dut):
    dev = await setup(dut)
    assert await dev.cmd("KEYGEN", k=5) == E_BAD_PARAM
    assert await dev.cmd("ENCAPS", k=0) == E_BAD_PARAM
    assert await dev.cmd("ENROLL", k=3) == E_NO_KEY
    assert await dev.cmd("PROVE", k=3) == E_NO_KEY
    assert await dev.cmd("PUF_MEASURE") == E_BAD_CMD
    await dev.reg_write(REG_CTRL, 12)
    await FallingEdge(dut.clk)
    assert await dev.wait_done() == E_BAD_CMD
    drv = ADriver(dev, 2)
    d, z = bytes(range(32)), bytes(range(32, 64))
    assert await drv.keygen(d, z) == mlkem.keygen_internal(params_of(2), d, z)

@only("dev")
async def nist_lewat_bus(dut):
    dev = await setup(dut)
    limit = int(os.environ.get("ACVP_LIMIT", "0"))
    seen, total, passed, gagal = {}, 0, 0, []
    for pset, fn, tcid, t in load_acvp():
        seen[(pset, fn)] = seen.get((pset, fn), 0) + 1
        if limit and seen[(pset, fn)] > limit:
            continue
        ok = await run_acvp_case(ADriver(dev, mlkem.PARAMS[pset].k), fn, t)
        total += 1
        passed += ok
        if not ok:
            gagal.append((pset, fn, tcid))
    dut._log.info("ACVP lewat bus: %d dari %d kasus lolos", passed, total)
    assert passed == total, f"gagal: {gagal[:10]}"
    if not limit:
        assert total == 240

@only("dev")
async def brankas_terhadap_model(dut):
    dev = await setup(dut)
    rng = random.Random(51)
    assert await dev.cmd("PUF_ENROLL") == E_OK
    assert (await dev.status()) & ST_PUF_READY
    chk = await dev.read(U.BASE_HELPER_CHK, 16)
    assert chk == gembok.helper_check(DEV_KEY, bytes(MASK_LEN)), "nilai cek data bantu salah"
    for k in (2, 3, 4):
        p = params_of(k)
        ref = gembok.Chip(p, DEV_KEY)
        drv = ADriver(dev, k)
        ek = await drv.enroll()
        assert ek == ref.enroll(), f"DAFTAR k={k} salah"
        dut._log.info("k=%d DAFTAR %d siklus", k, dev.last_cycles)
        ver = gembok.Verifier(p, ek)
        for ctx in (b"", b"scan-001", rng.randbytes(255)):
            c, kk = ver.challenge()
            tag = await drv.prove(c, ctx)
            assert tag == ref.prove(c, ctx) and ver.check(kk, ctx, tag), f"BUKTIKAN k={k} salah"
            dut._log.info("k=%d BUKTIKAN %d siklus (konteks %d byte)", k, dev.last_cycles, len(ctx))
            await cooldown(dev)
        bad = c[:17] + bytes([c[17] ^ 4]) + c[18:]
        tag = await drv.prove(bad, b"x")
        assert tag == ref.prove(bad, b"x") and not ver.check(kk, b"x", tag)
        await cooldown(dev)

@only("dev")
async def penjaga_akses(dut):
    dev = await setup(dut)
    rng = random.Random(52)
    k = 3
    p = params_of(k)
    assert await dev.cmd("PUF_ENROLL") == E_OK
    drv = ADriver(dev, k)
    ek = await drv.enroll()
    ver = gembok.Verifier(p, ek)
    c, kk = ver.challenge()
    ctx = b"uji-penjaga"

    d, z = gembok.derive_seeds(DEV_KEY)
    _, dk = mlkem.keygen_internal(p, d, z)
    _, sigma = mlkem.G(d + bytes([k]))
    m_prime = mlkem.kpke_decrypt(p, dk[:384 * k], c)
    _, r_prime = mlkem.G(m_prime + mlkem.H(ek))
    secrets = [("kunci PUF", DEV_KEY), ("d", d), ("z", z), ("sigma", sigma), ("dk_pke", dk[:384 * k]),
               ("m'", m_prime), ("r'", r_prime), ("K", kk), ("K_bar", mlkem.J(z + c))]

    await dev.write(U.BASE_CT, c)
    await dev.write(U.BASE_CTX, ctx)
    dev.ctx_len = len(ctx)
    await dev.start("PROVE", k)
    assert val(dut.busy_o)

    seen = bytearray()
    probes = 0
    addrs = list(range(U.BASE_SLOT, U.BASE_SLOT + 256)) + list(range(U.BASE_DK, U.BASE_DK + 64)) + list(range(0, 64))
    while val(dut.busy_o):
        a = rng.choice(addrs)
        seen += await dev.read(a, 4)
        probes += 1
        if probes % 50 == 0:
            await dev.write(U.BASE_CT + rng.randrange(1088), bytes([rng.randrange(256)]))
            await dev.write(slot_addr("TAG"), b"\xAA" * 32)
            await dev.reg_write(REG_CTRL, CMD["WIPE"])
        for _ in range(rng.randrange(1, 40)):
            await FallingEdge(dut.clk)
    assert probes > 100
    assert bytes(seen) == bytes(len(seen)), "host bisa membaca memori saat brankas bekerja"
    assert await dev.wait_done() == E_OK
    tag = await dev.read(slot_addr("TAG"), 32)
    assert ver.check(kk, ctx, tag), "tulisan host saat sibuk memengaruhi hasil"
    assert (await dev.status()) & ST_PUF_READY, "perintah saat sibuk tidak diabaikan"

    mem = await dump_all(dev)
    assert mem[U.BASE_SLOT:U.BASE_SLOT + 192] == bytes(192), "slot rahasia tidak terhapus"
    assert mem[U.BASE_DK:U.BASE_DK + 2048] == bytes(2048), "wilayah DK terisi pada mode brankas"
    bocor = contains_any(mem, secrets)
    assert bocor is None, f"rahasia terbaca host: {bocor}"
    assert mem[U.BASE_EK:U.BASE_EK + len(ek)] == ek
    regs = b"".join([(await dev.reg_read(o)).to_bytes(4, "little") for o in range(0, 0x100, 4)])
    assert contains_any(regs, secrets, n=4) is None

@only("dev")
async def pembatas_laju(dut):
    dev = await setup(dut)
    k = 2
    p = params_of(k)
    assert await dev.cmd("PUF_ENROLL") == E_OK
    drv = ADriver(dev, k)
    ver = gembok.Verifier(p, await drv.enroll())
    c, kk = ver.challenge()
    assert await dev.reg_read(REG_PROOFS) == 0
    tag = await drv.prove(c)
    assert ver.check(kk, b"", tag)
    assert await dev.reg_read(REG_PROOFS) == 1
    assert (await dev.status()) & ST_COOLING
    assert await dev.cmd("PROVE", k) == E_RATE
    assert await dev.reg_read(REG_PROOFS) == 1
    left = await dev.reg_read(REG_COOLDOWN)
    assert 0 < left <= COOLDOWN
    assert await dev.cmd("ENROLL", k) == E_OK
    await cooldown(dev)
    assert not (await dev.status()) & ST_COOLING
    c2, kk2 = ver.challenge()
    assert ver.check(kk2, b"", await drv.prove(c2))
    assert await dev.reg_read(REG_PROOFS) == 2

@only("dev")
async def wipe_melupakan_kunci(dut):
    dev = await setup(dut)
    assert await dev.cmd("PUF_ENROLL") == E_OK
    assert await dev.cmd("ENROLL", 3) == E_OK
    assert await dev.cmd("WIPE") == E_OK
    assert not (await dev.status()) & ST_PUF_READY
    assert await dump_all(dev) == bytes(8192)
    assert await dev.cmd("ENROLL", 3) == E_NO_KEY

@only("dev")
async def reset_di_tengah_operasi(dut):
    dev = await setup(dut)
    rng = random.Random(53)
    assert await dev.cmd("PUF_ENROLL") == E_OK
    drv = ADriver(dev, 3)
    ver = gembok.Verifier(params_of(3), await drv.enroll())
    for wait in (500, 9000, 21000, 40000):
        c, _ = ver.challenge()
        await dev.write(U.BASE_CT, c)
        dev.ctx_len = 0
        await dev.start("PROVE", 3)
        for _ in range(wait):
            await FallingEdge(dut.clk)
        assert val(dut.busy_o)
        await dev.reset()
        st = await dev.status()
        assert st & ST_BOOTED and not st & ST_PUF_READY
        assert await dump_all(dev) == bytes(8192), "memori tidak dihapus setelah reset"
        assert await dev.cmd("PUF_ENROLL") == E_OK

@only("dev")
async def siklus_konstan(dut):
    dev = await setup(dut)
    rng = random.Random(54)
    for k in (2, 3, 4):
        p = params_of(k)
        drv = ADriver(dev, k)
        d, z = rng.randbytes(32), rng.randbytes(32)
        ek, dk = mlkem.keygen_internal(p, d, z)
        cycles = set()
        for t in range(6):
            _, c = mlkem.encaps_internal(p, ek, rng.randbytes(32))
            if t % 2:
                pos = rng.randrange(len(c))
                c = c[:pos] + bytes([c[pos] ^ (1 << rng.randrange(8))]) + c[pos + 1:]
            st, kk = await drv.decaps(dk, c)
            assert st == 0 and kk == mlkem.decaps_internal(p, dk, c)
            cycles.add(dev.last_cycles)
        for t in range(4):
            s_fake = b"".join(mlkem.byte_encode(12, [rng.randrange(mlkem.Q) for _ in range(256)]) for _ in range(k))
            dk2 = s_fake + dk[384 * k:768 * k + 64] + rng.randbytes(32)
            _, c = mlkem.encaps_internal(p, ek, rng.randbytes(32))
            st, kk = await drv.decaps(dk2, c)
            assert st == 0 and kk == mlkem.decaps_internal(p, dk2, c)
            cycles.add(dev.last_cycles)
        assert len(cycles) == 1, f"k={k}: jumlah siklus Decaps berubah-ubah: {sorted(cycles)}"
        dut._log.info("k=%d: Decaps selalu %d siklus pada 10 masukan rahasia berbeda", k, cycles.pop())

@only("puf", "votes15")
async def pendaftaran_puf(dut):
    dev = await setup(dut)
    key, mask, nsel = puf_model.enroll(1, N_RO, 64)
    assert nsel == 256
    assert await dev.cmd("PUF_ENROLL") == E_OK
    dut._log.info("PUF_ENROLL %d siklus", dev.last_cycles)
    assert (await dev.status()) & ST_PUF_READY
    assert await dev.read(U.BASE_HELPER_MASK, MASK_LEN) == mask, "topeng pasangan berbeda dari model"
    assert await dev.read(U.BASE_HELPER_CHK, 16) == gembok.helper_check(key, mask)
    for k in (2, 3, 4):
        p = params_of(k)
        ref = gembok.Chip(p, key)
        drv = ADriver(dev, k)
        ek = await drv.enroll()
        assert ek == ref.enroll(), "kunci dari PUF berbeda dari model"
        ver = gembok.Verifier(p, ek)
        c, kk = ver.challenge()
        assert ver.check(kk, b"a", await drv.prove(c, b"a"))
        await cooldown(dev)
    mem = await dump_all(dev)
    assert contains_any(mem, [("kunci PUF", key)]) is None

@only("puf", "votes15")
async def pemulihan_setelah_reset(dut):
    dev = await setup(dut)
    key, mask, _ = puf_model.enroll(1, N_RO, 64)
    chk = gembok.helper_check(key, mask)
    drv = ADriver(dev, 3)
    ek_ref = gembok.Chip(params_of(3), key).enroll()
    for _ in range(3):
        await dev.reset()
        assert await dev.cmd("ENROLL", 3) == E_NO_KEY
        await dev.write(U.BASE_HELPER_MASK, mask)
        await dev.write(U.BASE_HELPER_CHK, chk)
        assert await dev.cmd("PUF_RECON") == E_OK
        assert await drv.enroll() == ek_ref
    dut._log.info("PUF_RECON %d siklus", dev.last_cycles)

@only("puf")
async def data_bantu_diubah(dut):
    dev = await setup(dut)
    key, mask, _ = puf_model.enroll(1, N_RO, 64)
    chk = gembok.helper_check(key, mask)
    drv = ADriver(dev, 3)
    ek_ref = gembok.Chip(params_of(3), key).enroll()

    def bit(m, c):
        return (m[c >> 3] >> (c & 7)) & 1

    ones = [c for c in range(N_RO - 1) if bit(mask, c)]
    zeros = [c for c in range(N_RO - 1) if not bit(mask, c)]

    def flip(m, *cs):
        m = bytearray(m)
        for c in cs:
            m[c >> 3] ^= 1 << (c & 7)
        return bytes(m)

    cases = [
        ("satu pasangan dibuang", flip(mask, ones[10]), chk, (E_PUF_FAIL,)),
        ("satu pasangan ditambah", flip(mask, zeros[5]), chk, (E_PUF_FAIL,)),
        ("satu pasangan ditukar", flip(mask, ones[40], zeros[40]), chk, (E_BAD_HELPER,)),
        ("nilai cek diubah", mask, bytes([chk[0] ^ 1]) + chk[1:], (E_BAD_HELPER,)),
        ("topeng kosong", bytes(MASK_LEN), chk, (E_PUF_FAIL,)),
        ("topeng penuh", b"\xff" * MASK_LEN, chk, (E_PUF_FAIL,)),
    ]
    for nama, m, ck, harap in cases:
        await dev.write(U.BASE_HELPER_MASK, m)
        await dev.write(U.BASE_HELPER_CHK, ck)
        e = await dev.cmd("PUF_RECON")
        assert e in harap, f"{nama}: galat {e}"
        assert not (await dev.status()) & ST_PUF_READY, nama
        assert await dev.cmd("ENROLL", 3) == E_NO_KEY, f"{nama}: kunci tetap bisa dipakai"
        assert await dev.cmd("PROVE", 3) == E_NO_KEY, nama
    await dev.write(U.BASE_HELPER_MASK, mask)
    await dev.write(U.BASE_HELPER_CHK, chk)
    assert await dev.cmd("PUF_RECON") == E_OK
    assert await drv.enroll() == ek_ref

@only("puf")
async def ambang_terlalu_tinggi(dut):
    dev = await setup(dut)
    await dev.reg_write(REG_THRESH, 900)
    _, _, nsel = puf_model.enroll(1, N_RO, 900)
    assert nsel < 256
    assert await dev.cmd("PUF_ENROLL") == E_PUF_FAIL
    assert not (await dev.status()) & ST_PUF_READY
    assert await dev.cmd("ENROLL", 3) == E_NO_KEY
    await dev.reg_write(REG_THRESH, 200)
    key, mask, nsel = puf_model.enroll(1, N_RO, 200)
    assert nsel == 256
    assert await dev.cmd("PUF_ENROLL") == E_OK
    assert await dev.read(U.BASE_HELPER_MASK, MASK_LEN) == mask
    assert await ADriver(dev, 2).enroll() == gembok.Chip(params_of(2), key).enroll()

@only("clone")
async def chip_tiruan(dut):
    dev = await setup(dut)
    key1, mask1, _ = puf_model.enroll(1, N_RO, 64)
    key2, mask2, _ = puf_model.enroll(2, N_RO, 64)
    assert key1 != key2
    p = params_of(3)
    asli = gembok.Chip(p, key1)
    ver = gembok.Verifier(p, asli.enroll())
    await dev.write(U.BASE_HELPER_MASK, mask1)
    await dev.write(U.BASE_HELPER_CHK, gembok.helper_check(key1, mask1))
    assert await dev.cmd("PUF_RECON") == E_BAD_HELPER
    assert await dev.cmd("PROVE", 3) == E_NO_KEY
    assert await dev.cmd("PUF_ENROLL") == E_OK
    drv = ADriver(dev, 3)
    ek2 = await drv.enroll()
    assert ek2 == gembok.Chip(p, key2).enroll() and ek2 != asli.enroll()
    c, kk = ver.challenge()
    tag = await drv.prove(c, b"")
    assert not ver.check(kk, b"", tag), "chip tiruan lolos pemeriksaan"

@only("noise")
async def kunci_stabil_dengan_derau(dut):
    dev = await setup(dut)
    drv = ADriver(dev, 2)
    assert await dev.cmd("PUF_ENROLL") == E_OK
    mask = await dev.read(U.BASE_HELPER_MASK, MASK_LEN)
    chk = await dev.read(U.BASE_HELPER_CHK, 16)
    ek0 = await drv.enroll()
    for i in range(8):
        await dev.reset()
        for _ in range(17 * i):
            await FallingEdge(dut.clk)
        await dev.write(U.BASE_HELPER_MASK, mask)
        await dev.write(U.BASE_HELPER_CHK, chk)
        assert await dev.cmd("PUF_RECON") == E_OK, f"pemulihan ke-{i} gagal"
        assert await drv.enroll() == ek0, "kunci berubah antar-penyalaan"

@only("noise")
async def ambang_rendah_terdeteksi(dut):
    dev = await setup(dut)
    drv = ADriver(dev, 2)
    await dev.reg_write(REG_THRESH, 1)
    assert await dev.cmd("PUF_ENROLL") == E_OK
    mask = await dev.read(U.BASE_HELPER_MASK, MASK_LEN)
    chk = await dev.read(U.BASE_HELPER_CHK, 16)
    ek0 = await drv.enroll()
    hasil = {"baik": 0, "ditolak": 0}
    for i in range(12):
        await dev.reset()
        for _ in range(29 * i + 3):
            await FallingEdge(dut.clk)
        await dev.reg_write(REG_THRESH, 1)
        await dev.write(U.BASE_HELPER_MASK, mask)
        await dev.write(U.BASE_HELPER_CHK, chk)
        e = await dev.cmd("PUF_RECON")
        if e == E_OK:
            assert await drv.enroll() == ek0, "kunci salah diterima tanpa terdeteksi"
            hasil["baik"] += 1
        else:
            assert e == E_BAD_HELPER
            assert await dev.cmd("ENROLL", 2) == E_NO_KEY
            hasil["ditolak"] += 1
    dut._log.info("ambang 1 dengan derau: %s", hasil)

@only("debug")
async def baca_hitungan_mentah(dut):
    dev = await setup(dut)
    caps = await dev.reg_read(REG_CAPS)
    assert caps & 4, "bit debug di CAPS tidak menyala"
    for idx in (0, 1, 2, 77, 500, 766):
        await dev.reg_write(REG_PUF_IDX, idx)
        assert await dev.cmd("PUF_MEASURE") == E_OK
        c0 = await dev.reg_read(REG_PUF_DBG)
        c1 = await dev.reg_read(REG_PUF_DBG + 4)
        assert c0 == puf_model.ro_freq(1, idx), f"hitungan osilator {idx} salah"
        assert c1 == puf_model.ro_freq(1, idx + 1), f"hitungan osilator {idx + 1} salah"

@only("dev")
async def status_tidak_basi(dut):
    dev = await setup(dut)
    p = params_of(3)
    ek, _ = mlkem.keygen_internal(p, bytes(32), bytes(32))
    bad = bytearray(ek)
    bad[0], bad[1] = 0xFF, 0xFF
    await dev.write(U.BASE_EK, bytes(bad))
    assert await dev.cmd("CHECK_EK", 3) == 1
    await dev.write(U.BASE_EK, ek)
    for code, k, want in ((CMD["CHECK_EK"], 3, E_OK), (CMD["CHECK_EK"], 7, E_BAD_PARAM), (12, 3, E_BAD_CMD)):
        await dev.reg_write(0x10, k)
        await dev.reg_write(REG_CTRL, code)
        st = await dev.reg_read(REG_STATUS)
        assert st & ST_BUSY and not st & ST_DONE, f"STATUS basi sesudah CTRL: {st:#x}"
        while True:
            st = await dev.reg_read(REG_STATUS)
            if not st & ST_BUSY:
                break
        assert st & ST_DONE and (st >> 4) & 0xF == want, f"{st:#x}"

@only("dev")
async def register_terkunci_saat_sibuk(dut):
    dev = await setup(dut)
    k = 3
    p = params_of(k)
    assert await dev.cmd("PUF_ENROLL") == E_OK
    drv = ADriver(dev, k)
    ver = gembok.Verifier(p, await drv.enroll())
    c, kk = ver.challenge()
    ctx = b"konteks-10"
    await dev.write(U.BASE_CT, c)
    await dev.write(U.BASE_CTX, ctx)
    dev.ctx_len = len(ctx)
    await dev.start("PROVE", k)
    await dev.reg_write(0x14, 0)
    await dev.reg_write(0x10, 2)
    await dev.reg_write(REG_THRESH, 999)
    assert await dev.wait_done() == E_OK
    tag = await dev.read(slot_addr("TAG"), 32)
    assert ver.check(kk, ctx, tag), "CTXLEN yang ditulis saat sibuk ikut terpakai"
    assert await dev.reg_read(0x14) == len(ctx)
    assert await dev.reg_read(0x10) == k
    assert await dev.reg_read(REG_THRESH) == 64

@only("dev")
async def tulis_memori_sesudah_ctrl(dut):
    dev = await setup(dut)
    p = params_of(3)
    d, z = bytes(range(32)), bytes(range(32, 64))
    await dev.write(slot_addr("D"), d)
    await dev.write(slot_addr("Z"), z)
    await dev.reg_write(REG_PARAM, 3)
    await dev.reg_write(REG_CTRL, CMD["KEYGEN"])
    await dev.write(slot_addr("D"), b"\xAA")
    assert await dev.wait_done() == E_OK
    ek, _ = mlkem.keygen_internal(p, d, z)
    assert await dev.read(U.BASE_EK, len(ek)) == ek, "tulisan memori tepat sesudah CTRL ikut terpakai"
    assert await dev.read(slot_addr("D"), 1) == d[:1]

@only("dev")
async def pembatas_laju_tahan_reset(dut):
    dev = await setup(dut)
    k = 2
    assert await dev.cmd("PUF_ENROLL") == E_OK
    drv = ADriver(dev, k)
    ver = gembok.Verifier(params_of(k), await drv.enroll())
    c, kk = ver.challenge()
    assert ver.check(kk, b"", await drv.prove(c))
    before = await dev.reg_read(REG_COOLDOWN)
    assert before > 0
    dut.reset.value = 1
    for _ in range(5):
        await FallingEdge(dut.clk)
    dut.reset.value = 0
    for _ in range(4):
        await FallingEdge(dut.clk)
    after = await dev.reg_read(REG_COOLDOWN)
    st = await dev.status()
    assert 0 < after <= before and st & ST_COOLING, f"reset mengosongkan pembatas laju: {before} lalu {after}"
    while not (await dev.status()) & ST_BOOTED:
        for _ in range(200):
            await FallingEdge(dut.clk)
    dut._log.info("COOLDOWN %d sebelum reset, %d sesudah reset, boot %d siklus", before, after,
                  await dev.reg_read(REG_CYCLES))

@only("ro")
async def ro_daftar_dan_pulih(dut):
    dev = await setup(dut)
    await dev.reg_write(REG_THRESH, 4)
    assert await dev.cmd("PUF_ENROLL") == E_OK
    dut._log.info("PUF_ENROLL osilator perilaku: %d siklus", dev.last_cycles)
    mask = await dev.read(U.BASE_HELPER_MASK, MASK_LEN)
    chk = await dev.read(U.BASE_HELPER_CHK, 16)
    assert sum(bin(b).count("1") for b in mask) == 256
    drv = ADriver(dev, 3)
    ek0 = await drv.enroll()
    for _ in range(2):
        await dev.reset()
        await dev.write(U.BASE_HELPER_MASK, mask)
        await dev.write(U.BASE_HELPER_CHK, chk)
        assert await dev.cmd("PUF_RECON") == E_OK
        assert await drv.enroll() == ek0
    ones = [c for c in range(N_RO - 1) if (mask[c >> 3] >> (c & 7)) & 1]
    zeros = [c for c in range(N_RO - 1) if not (mask[c >> 3] >> (c & 7)) & 1]
    bad = bytearray(mask)
    bad[ones[3] >> 3] ^= 1 << (ones[3] & 7)
    bad[zeros[3] >> 3] ^= 1 << (zeros[3] & 7)
    await dev.write(U.BASE_HELPER_MASK, bytes(bad))
    assert await dev.cmd("PUF_RECON") == E_BAD_HELPER
    assert await dev.cmd("ENROLL", 3) == E_NO_KEY

@only("rodbg")
async def ro_hitungan_20_bit(dut):
    dev = await setup(dut)
    window_ps = (1 << 12) * 20_000
    besar = 0
    for idx in (0, 1, 2, 9, 30):
        await dev.reg_write(REG_PUF_IDX, idx)
        assert await dev.cmd("PUF_MEASURE") == E_OK
        got = [await dev.reg_read(REG_PUF_DBG), await dev.reg_read(REG_PUF_DBG + 4)]
        for g, v in zip((idx, idx + 1), got):
            harap = window_ps / (2 * puf_model.ro_half_ps(1, g, 400))
            assert abs(v - harap) <= 2, f"osilator {g}: {v}, harap sekitar {harap:.1f}"
            besar += v > 0xFFFF
        dut._log.info("pasangan %d: %d dan %d", idx, got[0], got[1])
    assert besar > 0, "tidak ada hitungan di atas 16 bit, uji tidak memeriksa apa-apa"
