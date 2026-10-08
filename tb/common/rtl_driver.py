from __future__ import annotations

import json
from pathlib import Path

import mlkem
import ucode as U

ROOT = Path(__file__).resolve().parent.parent.parent

def params_of(k: int):
    return {2: mlkem.ML_KEM_512, 3: mlkem.ML_KEM_768, 4: mlkem.ML_KEM_1024}[k]

def slot_addr(name: str) -> int:
    return U.BASE_SLOT + 32 * U.DESC[name]

class ADriver:
    def __init__(self, dev, k: int) -> None:
        self.dev = dev
        self.k = k
        self.p = params_of(k)

    async def keygen(self, d: bytes, z: bytes):
        k = self.k
        await self.dev.write(slot_addr("D"), d)
        await self.dev.write(slot_addr("Z"), z)
        st = await self.dev.run("KEYGEN", k)
        assert st == U.ST_OK, st
        ek = await self.dev.read(U.BASE_EK, 384 * k + 32)
        dk_pke = await self.dev.read(U.BASE_DK, 384 * k)
        h = await self.dev.read(slot_addr("H"), 32)
        return ek, dk_pke + ek + h + z

    async def encaps(self, ek: bytes, m: bytes):
        await self.dev.write(U.BASE_EK, ek)
        await self.dev.write(slot_addr("M"), m)
        st = await self.dev.run("ENCAPS", self.k)
        if st != U.ST_OK:
            return st, b"", b""
        kk = await self.dev.read(slot_addr("K"), 32)
        c = await self.dev.read(U.BASE_CT, self.p.ct_len)
        return st, kk, c

    async def load_dk(self, dk: bytes):
        k = self.k
        await self.dev.write(U.BASE_DK, dk[:384 * k])
        await self.dev.write(U.BASE_EK, dk[384 * k:768 * k + 32])
        await self.dev.write(slot_addr("H"), dk[768 * k + 32:768 * k + 64])
        await self.dev.write(slot_addr("Z"), dk[768 * k + 64:768 * k + 96])

    async def decaps(self, dk: bytes, c: bytes):
        await self.load_dk(dk)
        await self.dev.write(U.BASE_CT, c)
        st = await self.dev.run("DECAPS", self.k)
        if st != U.ST_OK:
            return st, b""
        return st, await self.dev.read(slot_addr("K"), 32)

    async def check_ek(self, ek: bytes) -> bool:
        if len(ek) != self.p.ek_len:
            return False
        await self.dev.write(U.BASE_EK, ek)
        return (await self.dev.run("CHECK_EK", self.k)) == U.ST_OK

    async def check_dk(self, dk: bytes) -> bool:
        if len(dk) != self.p.dk_len:
            return False
        await self.load_dk(dk)
        return (await self.dev.run("CHECK_DK", self.k)) == U.ST_OK

    async def enroll(self) -> bytes:
        st = await self.dev.run("ENROLL", self.k)
        assert st == U.ST_OK, st
        return await self.dev.read(U.BASE_EK, 384 * self.k + 32)

    async def prove(self, c: bytes, context: bytes = b"") -> bytes:
        assert len(c) == self.p.ct_len and len(context) <= 255
        await self.dev.write(U.BASE_CT, c)
        if context:
            await self.dev.write(U.BASE_CTX, context)
        self.dev.ctx_len = len(context)
        st = await self.dev.run("PROVE", self.k)
        assert st == U.ST_OK, st
        return await self.dev.read(slot_addr("TAG"), 32)

def load_acvp():
    cases = []
    kg = json.loads((ROOT / "vectors" / "ML-KEM-keyGen-FIPS203.json").read_text())
    for g in kg["testGroups"]:
        for t in g["tests"]:
            cases.append((g["parameterSet"], "keyGen", t["tcId"], t))
    ed = json.loads((ROOT / "vectors" / "ML-KEM-encapDecap-FIPS203.json").read_text())
    for g in ed["testGroups"]:
        for t in g["tests"]:
            cases.append((g["parameterSet"], g["function"], t["tcId"], t))
    return cases

async def run_acvp_case(drv: ADriver, fn: str, t: dict) -> bool:
    hx = bytes.fromhex
    if fn == "keyGen":
        ek, dk = await drv.keygen(hx(t["d"]), hx(t["z"]))
        return ek == hx(t["ek"]) and dk == hx(t["dk"])
    if fn == "encapsulation":
        st, kk, c = await drv.encaps(hx(t["ek"]), hx(t["m"]))
        return st == 0 and kk == hx(t["k"]) and c == hx(t["c"])
    if fn == "decapsulation":
        st, kk = await drv.decaps(hx(t["dk"]), hx(t["c"]))
        return st == 0 and kk == hx(t["k"])
    if fn == "encapsulationKeyCheck":
        return (await drv.check_ek(hx(t["ek"]))) == t["testPassed"]
    if fn == "decapsulationKeyCheck":
        return (await drv.check_dk(hx(t["dk"]))) == t["testPassed"]
    raise ValueError(fn)
