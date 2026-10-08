#!/usr/bin/env python3

from __future__ import annotations

import hashlib
import json
import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "model"))
sys.path.insert(0, str(ROOT / "tools"))

import gembok
import mlkem
import ucode as U

Q = mlkem.Q

def params_of(k: int):
    return {2: mlkem.ML_KEM_512, 3: mlkem.ML_KEM_768, 4: mlkem.ML_KEM_1024}[k]

def desc_resolve(did: int, k: int, i: int, ctx_len: int) -> tuple[int, int]:
    p = params_of(k)
    if did <= 7:
        return U.BASE_SLOT + 32 * did, 32
    if did == U.DESC["CTX"]:
        return U.BASE_CTX, ctx_len
    if did == U.DESC["EK_RHO"]:
        return U.BASE_EK + 384 * k, 32
    if did == U.DESC["EK_ALL"]:
        return U.BASE_EK, 384 * k + 32
    if did == U.DESC["EK_T_I"]:
        return U.BASE_EK + 384 * i, 384
    if did == U.DESC["DK_S_I"]:
        return U.BASE_DK + 384 * i, 384
    if did == U.DESC["CT_U_I"]:
        return U.BASE_CT + 32 * p.du * i, 32 * p.du
    if did == U.DESC["CT_V"]:
        return U.BASE_CT + 32 * p.du * k, 32 * p.dv
    if did == U.DESC["PUFKEY"]:
        return 0, 32
    if did == U.DESC["HELPER_MASK"]:
        return U.BASE_HELPER_MASK, U.HELPER_MASK_LEN
    if did == U.DESC["HELPER_CHK"]:
        return U.BASE_HELPER_CHK, U.HELPER_CHK_LEN
    raise ValueError(did)

class Machine:
    def __init__(self, k: int, puf_key: bytes | None = None) -> None:
        asm = U.build()
        self.words = asm.words()
        self.labels = asm.labels
        self.k = k
        self.p = params_of(k)
        self.puf_key = puf_key
        self.bmem = bytearray(8192)
        self.poly = [[0] * 256 for _ in range(16)]
        self.ctx_len = 0
        self.steps = 0

    def write(self, addr: int, data: bytes) -> None:
        self.bmem[addr:addr + len(data)] = data

    def read(self, addr: int, n: int) -> bytes:
        return bytes(self.bmem[addr:addr + n])

    def slot_addr(self, name: str) -> int:
        return U.BASE_SLOT + 32 * U.DESC[name]

    def _h_stream(self, n: int) -> bytes:
        name = ["sha3_256", "sha3_512", "shake_128", "shake_256"][self.h_mode]
        h = hashlib.new(name, bytes(self.h_in))
        if name.startswith("shake"):
            out = h.digest(self.h_pos + n)[self.h_pos:]
        else:
            out = h.digest()[self.h_pos:self.h_pos + n]
            assert len(out) == n, "membaca melewati panjang digest SHA3"
        self.h_pos += n
        return out

    def _slot(self, f: int) -> int:
        base, sel = f & 0xF, (f >> 4) & 3
        return base + (self.i if sel == 1 else self.j if sel == 2 else 0)

    def _d(self, dsel: int) -> int:
        return [12, self.p.du, self.p.dv, 1][dsel]

    def run(self, program: str) -> int:
        pc = self.labels[program]
        stack: list[int] = []
        self.i = self.j = self.n = 0
        self.neq = self.bad = self.cmp = 0
        self.h_mode, self.h_in, self.h_pos = 0, bytearray(), 0
        k = self.k
        while True:
            w = self.words[pc]
            pc += 1
            self.steps += 1
            op = U.OPS[w >> 21]
            f1, f2, f3 = (w >> 14) & 0x7F, (w >> 7) & 0x7F, w & 0x7F
            imm = (f2 << 7) | f3
            if op == "NOP":
                pass
            elif op == "END":
                return imm & 0xF
            elif op == "JMP":
                pc = imm
            elif op == "CALL":
                stack.append(pc)
                assert len(stack) <= 2, "tumpukan panggilan lebih dari 2"
                pc = imm
            elif op == "RET":
                pc = stack.pop()
            elif op == "LOOPI":
                if self.i + 1 != k:
                    pc = imm
                self.i = (self.i + 1) & 3
            elif op == "LOOPJ":
                if self.j + 1 != k:
                    pc = imm
                self.j = (self.j + 1) & 3
            elif op == "SETR":
                v = imm
                if f1 == U.R_I: self.i = v
                elif f1 == U.R_J: self.j = v
                elif f1 == U.R_N: self.n = v
                elif f1 == U.R_NEQ: self.neq = v & 1
                elif f1 == U.R_CMP: self.cmp = v & 1
                elif f1 == U.R_BAD: self.bad = v & 1
            elif op == "INCN":
                self.n += 1
            elif op == "BRF":
                if (self.neq if f1 == U.F_NEQ else self.bad):
                    pc = imm
            elif op == "HINIT":
                self.h_mode, self.h_in, self.h_pos = f1 & 3, bytearray(), 0
            elif op == "HABS":
                f1 &= 0x1F
                if f1 == U.DESC["PUFKEY"]:
                    assert self.puf_key is not None, "kunci PUF tidak tersedia"
                    self.h_in += self.puf_key
                else:
                    base, ln = desc_resolve(f1, k, self.i, self.ctx_len)
                    self.h_in += self.bmem[base:base + ln]
            elif op == "HABSC":
                self.h_in.append([imm & 0xFF, self.i, self.j, self.n, k, self.ctx_len][f1])
            elif op == "HFIN":
                self.h_pos = 0
            elif op == "HSQZ":
                base, ln = desc_resolve(f1 & 0x1F, k, self.i, self.ctx_len)
                data = self._h_stream(ln)
                if f2 & 1:
                    if bytes(self.bmem[base:base + ln]) != data:
                        self.neq = 1
                else:
                    self.bmem[base:base + ln] = data
            elif op == "SNTT":
                f: list[int] = []
                while len(f) < 256:
                    c = self._h_stream(3)
                    d1 = c[0] + 256 * (c[1] % 16)
                    d2 = (c[1] // 16) + 16 * c[2]
                    if d1 < Q:
                        f.append(d1)
                    if d2 < Q and len(f) < 256:
                        f.append(d2)
                self.poly[self._slot(f1)] = f
            elif op == "CBD":
                eta = self.p.eta1 if (f2 & 1) == U.ETA1 else self.p.eta2
                self.poly[self._slot(f1)] = mlkem.sample_poly_cbd(eta, self._h_stream(64 * eta))
            elif op == "DEC":
                d = self._d(f3 & 3)
                base, _ = desc_resolve(f2 & 0x1F, k, self.i, self.ctx_len)
                data = bytes(self.bmem[base:base + 32 * d])
                if f3 & 4:
                    self.h_in += data
                if d == 12:
                    bits = mlkem.bytes_to_bits(data)
                    raw = [sum(bits[12 * c + b] << b for b in range(12)) for c in range(256)]
                    if (f3 & 8) and any(x >= Q for x in raw):
                        self.bad = 1
                    vals = [x % Q for x in raw]
                else:
                    vals = mlkem.decompress_poly(d, mlkem.byte_decode(d, data))
                self.poly[self._slot(f1)] = vals
            elif op == "ENC":
                d = self._d(f3 & 3)
                base, _ = desc_resolve(f2 & 0x1F, k, self.i, self.ctx_len)
                f = self.poly[self._slot(f1)]
                data = mlkem.byte_encode(d, f if d == 12 else mlkem.compress_poly(d, f))
                mode = (f3 >> 2) & 3
                compare = mode == U.W_CMP or (mode == U.W_FLAG and self.cmp)
                if compare:
                    if bytes(self.bmem[base:base + len(data)]) != data:
                        self.neq = 1
                else:
                    self.bmem[base:base + len(data)] = data
            elif op == "NTT":
                s = self._slot(f1)
                self.poly[s] = mlkem.ntt(self.poly[s])
            elif op == "INTT":
                s = self._slot(f1)
                self.poly[s] = [(x * 128) % Q for x in mlkem.ntt_inv(self.poly[s])]
            elif op == "PWM":
                dst, sa, sb = self._slot(f1 & 0x3F), self._slot(f2 & 0x3F), self._slot(f3 & 0x3F)
                accm = ((f1 >> 6) & 1) | (((f2 >> 6) & 1) << 1)
                acc = [False, True, self.j != 0, self.i != 0][accm]
                prod = mlkem.multiply_ntts(self.poly[sa], self.poly[sb])
                self.poly[dst] = mlkem.poly_add(self.poly[dst], prod) if acc else prod
            elif op == "LIN":
                dst, sx, sy = self._slot(f1 & 0x3F), self._slot(f2 & 0x3F), self._slot(f3 & 0x3F)
                scale, sub = (f1 >> 6) & 1, (f2 >> 6) & 1
                w_ = 3303 if scale else 1
                x, y = self.poly[sx], self.poly[sy]
                self.poly[dst] = [((b - w_ * a) if sub else (b + w_ * a)) % Q for a, b in zip(x, y)]
            elif op == "CSEL":
                bd, _ = desc_resolve(f1 & 0x1F, k, self.i, self.ctx_len)
                ba, _ = desc_resolve(f2 & 0x1F, k, self.i, self.ctx_len)
                if self.neq:
                    self.bmem[bd:bd + 32] = self.bmem[ba:ba + 32]
            elif op == "WIPE":
                if imm & U.WIPE_POLY:
                    self.poly = [[0] * 256 for _ in range(16)]
                if imm & U.WIPE_SECRET:
                    self.bmem[U.BASE_SLOT:U.BASE_SLOT + 192] = bytes(192)
                if imm & U.WIPE_KECCAK:
                    self.h_in, self.h_pos = bytearray(), 0
                if imm & U.WIPE_ALLBYTES:
                    self.bmem = bytearray(8192)
            else:
                raise ValueError(op)

class Driver:

    def __init__(self, dev, k: int) -> None:
        self.dev = dev
        self.k = k
        self.p = params_of(k)

    def _slot(self, name: str) -> int:
        return U.BASE_SLOT + 32 * U.DESC[name]

    def keygen(self, d: bytes, z: bytes) -> tuple[bytes, bytes]:
        k = self.k
        self.dev.write(self._slot("D"), d)
        self.dev.write(self._slot("Z"), z)
        st = self.dev.run("KEYGEN")
        assert st == U.ST_OK, st
        ek = self.dev.read(U.BASE_EK, 384 * k + 32)
        dk_pke = self.dev.read(U.BASE_DK, 384 * k)
        h = self.dev.read(self._slot("H"), 32)
        return ek, dk_pke + ek + h + z

    def encaps(self, ek: bytes, m: bytes) -> tuple[int, bytes, bytes]:
        self.dev.write(U.BASE_EK, ek)
        self.dev.write(self._slot("M"), m)
        st = self.dev.run("ENCAPS")
        if st != U.ST_OK:
            return st, b"", b""
        return st, self.dev.read(self._slot("K"), 32), self.dev.read(U.BASE_CT, self.p.ct_len)

    def _load_dk(self, dk: bytes) -> None:
        k = self.k
        self.dev.write(U.BASE_DK, dk[:384 * k])
        self.dev.write(U.BASE_EK, dk[384 * k:768 * k + 32])
        self.dev.write(self._slot("H"), dk[768 * k + 32:768 * k + 64])
        self.dev.write(self._slot("Z"), dk[768 * k + 64:768 * k + 96])

    def decaps(self, dk: bytes, c: bytes) -> tuple[int, bytes]:
        self._load_dk(dk)
        self.dev.write(U.BASE_CT, c)
        st = self.dev.run("DECAPS")
        if st != U.ST_OK:
            return st, b""
        return st, self.dev.read(self._slot("K"), 32)

    def check_ek(self, ek: bytes) -> bool:
        if len(ek) != self.p.ek_len:
            return False
        self.dev.write(U.BASE_EK, ek)
        return self.dev.run("CHECK_EK") == U.ST_OK

    def check_dk(self, dk: bytes) -> bool:
        if len(dk) != self.p.dk_len:
            return False
        self._load_dk(dk)
        return self.dev.run("CHECK_DK") == U.ST_OK

    def enroll(self) -> bytes:
        st = self.dev.run("ENROLL")
        assert st == U.ST_OK, st
        return self.dev.read(U.BASE_EK, 384 * self.k + 32)

    def prove(self, c: bytes, context: bytes = b"") -> bytes:
        assert len(c) == self.p.ct_len and len(context) <= 255
        self.dev.write(U.BASE_CT, c)
        self.dev.write(U.BASE_CTX, context)
        self.dev.ctx_len = len(context)
        st = self.dev.run("PROVE")
        assert st == U.ST_OK, st
        return self.dev.read(self._slot("TAG"), 32)

def run_acvp(make_driver) -> tuple[int, int]:
    hx = bytes.fromhex
    total = passed = 0
    kg = json.loads((ROOT / "vectors" / "ML-KEM-keyGen-FIPS203.json").read_text())
    for g in kg["testGroups"]:
        p = mlkem.PARAMS[g["parameterSet"]]
        drv = make_driver(p.k)
        for t in g["tests"]:
            ek, dk = drv.keygen(hx(t["d"]), hx(t["z"]))
            total += 1
            passed += (ek == hx(t["ek"]) and dk == hx(t["dk"]))
    ed = json.loads((ROOT / "vectors" / "ML-KEM-encapDecap-FIPS203.json").read_text())
    for g in ed["testGroups"]:
        p = mlkem.PARAMS[g["parameterSet"]]
        drv = make_driver(p.k)
        fn = g["function"]
        for t in g["tests"]:
            total += 1
            if fn == "encapsulation":
                st, kk, c = drv.encaps(hx(t["ek"]), hx(t["m"]))
                passed += (st == 0 and kk == hx(t["k"]) and c == hx(t["c"]))
            elif fn == "decapsulation":
                st, kk = drv.decaps(hx(t["dk"]), hx(t["c"]))
                passed += (st == 0 and kk == hx(t["k"]))
            elif fn == "encapsulationKeyCheck":
                passed += (drv.check_ek(hx(t["ek"])) == t["testPassed"])
            elif fn == "decapsulationKeyCheck":
                passed += (drv.check_dk(hx(t["dk"])) == t["testPassed"])
    return passed, total

def main() -> int:
    import random
    passed, total = run_acvp(lambda k: Driver(Machine(k), k))
    print(f"ACVP lewat tabel langkah: {passed} dari {total}")
    ok = passed == total == 240

    rng = random.Random(7)
    n_ok = n_all = 0
    for k in (2, 3, 4):
        p = params_of(k)
        for _ in range(4):
            puf = rng.randbytes(32)
            ref = gembok.Chip(p, puf)
            m = Machine(k, puf)
            drv = Driver(m, k)
            ek = drv.enroll()
            n_all += 1
            n_ok += (ek == ref.enroll())
            assert m.read(U.BASE_SLOT, 192) == bytes(192)
            assert all(x == [0] * 256 for x in m.poly)
            ver = gembok.Verifier(p, ek)
            for ctx in (b"", b"scan-001", rng.randbytes(255)):
                c, kk = ver.challenge()
                tag = drv.prove(c, ctx)
                n_all += 1
                n_ok += (tag == ref.prove(c, ctx) and ver.check(kk, ctx, tag))
                assert m.read(U.BASE_SLOT, 192) == bytes(192)
                bad = bytes([c[0] ^ 1]) + c[1:]
                tag2 = drv.prove(bad, ctx)
                n_all += 1
                n_ok += (tag2 == ref.prove(bad, ctx) and not ver.check(kk, ctx, tag2))
    print(f"Protokol identitas lewat tabel langkah: {n_ok} dari {n_all}")
    ok = ok and n_ok == n_all
    print("LOLOS" if ok else "GAGAL")
    return 0 if ok else 1

if __name__ == "__main__":
    sys.exit(main())
