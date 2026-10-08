#!/usr/bin/env python3

from __future__ import annotations

from dataclasses import dataclass
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

OPS = [
    "NOP", "END", "JMP", "CALL", "RET", "LOOPI", "LOOPJ", "SETR", "INCN", "BRF",
    "HINIT", "HABS", "HABSC", "HFIN", "HSQZ", "SNTT", "CBD", "DEC", "ENC",
    "NTT", "INTT", "PWM", "LIN", "CSEL", "WIPE",
]
OP = {name: i for i, name in enumerate(OPS)}

SHA3_256, SHA3_512, SHAKE128, SHAKE256 = 0, 1, 2, 3

R_I, R_J, R_N, R_NEQ, R_CMP, R_BAD = 0, 1, 2, 3, 4, 5

F_NEQ, F_BAD = 0, 1

C_LIT, C_I, C_J, C_N, C_K, C_CTXLEN = 0, 1, 2, 3, 4, 5

D_12, D_U, D_V, D_1 = 0, 1, 2, 3

ETA1, ETA2 = 0, 1

PWM_SET, PWM_ACC, PWM_ACC_J, PWM_ACC_I = 0, 1, 2, 3

W_WRITE, W_CMP, W_FLAG = 0, 1, 2

WIPE_POLY, WIPE_SECRET, WIPE_KECCAK, WIPE_ALLBYTES = 1, 2, 4, 8

ST_OK, ST_BAD_EK, ST_BAD_DK, ST_BAD_HELPER = 0, 1, 2, 3

DESC = {
    "D": 0, "Z": 1, "M": 2, "K": 3, "SIGMA": 4, "KBAR": 5, "H": 6, "TAG": 7,
    "CTX": 8, "EK_RHO": 9, "EK_ALL": 10, "EK_T_I": 11, "DK_S_I": 12,
    "CT_U_I": 13, "CT_V": 14, "PUFKEY": 15, "HELPER_MASK": 16, "HELPER_CHK": 17,
}

BASE_EK, BASE_CT, BASE_DK, BASE_SLOT, BASE_CTX = 0x0000, 0x0800, 0x1000, 0x1800, 0x1900
BASE_HELPER_MASK, BASE_HELPER_CHK = 0x1A00, 0x1A80
HELPER_MASK_LEN = 96
HELPER_CHK_LEN = 16

def slot(base: int, sel: str = "") -> int:
    return ({"": 0, "i": 1, "j": 2}[sel] << 4) | base

S_I, S_J = slot(0, "i"), slot(0, "j")
T_I, T_J = slot(4, "i"), slot(4, "j")
ACC, TMP = slot(8), slot(9)

PROGRAMS = ["WIPE_ALL", "KEYGEN", "ENCAPS", "DECAPS", "CHECK_EK", "CHECK_DK", "ENROLL", "PROVE",
            "PUF_SEAL", "PUF_OPEN"]

LABEL_SEED = b"GEMBOK-v1/benih"
LABEL_TAG = b"GEMBOK-v1/bukti"
LABEL_CHECK = b"GEMBOK-v1/cek"

@dataclass
class Ins:
    op: str
    f1: int = 0
    f2: int = 0
    f3: int = 0
    target: str | None = None
    text: str = ""

class Asm:
    def __init__(self) -> None:
        self.code: list[Ins] = []
        self.labels: dict[str, int] = {}

    def label(self, name: str) -> None:
        assert name not in self.labels, name
        self.labels[name] = len(self.code)

    def _e(self, op, f1=0, f2=0, f3=0, target=None, text=""):
        assert 0 <= f1 < 128 and 0 <= f2 < 128 and 0 <= f3 < 128, (op, f1, f2, f3)
        self.code.append(Ins(op, f1, f2, f3, target, text))

    def _imm(self, op, f1, imm, text):
        assert 0 <= imm < (1 << 14)
        self._e(op, f1, imm >> 7, imm & 0x7F, text=text)

    def END(self, st):           self._imm("END", 0, st, f"END {st}")
    def JMP(self, t):            self._e("JMP", target=t, text=f"JMP {t}")
    def CALL(self, t):           self._e("CALL", target=t, text=f"CALL {t}")
    def RET(self):               self._e("RET", text="RET")
    def LOOPI(self, t):          self._e("LOOPI", target=t, text=f"LOOPI {t}")
    def LOOPJ(self, t):          self._e("LOOPJ", target=t, text=f"LOOPJ {t}")
    def SETR(self, r, v):        self._imm("SETR", r, v, f"SETR {['i','j','n','neq','cmp','bad'][r]} = {v}")
    def INCN(self):              self._e("INCN", text="INCN")
    def BRF(self, f, t):         self._e("BRF", f1=f, target=t, text=f"BRF {['neq','bad'][f]} -> {t}")

    def HINIT(self, mode):       self._e("HINIT", mode, text=f"HINIT {['SHA3-256','SHA3-512','SHAKE128','SHAKE256'][mode]}")
    def HABS(self, d):           self._e("HABS", DESC[d], text=f"HABS {d}")
    def HABSC(self, sel, lit=0): self._imm("HABSC", sel, lit, f"HABSC {['lit','i','j','n','k','ctxlen'][sel]}" + (f" 0x{lit:02x}" if sel == C_LIT else ""))
    def HFIN(self):              self._e("HFIN", text="HFIN")
    def HSQZ(self, d, cmp=0):    self._e("HSQZ", DESC[d], cmp, text=f"HSQZ {d}" + (" (banding)" if cmp else ""))

    def SNTT(self, s):           self._e("SNTT", s, text=f"SNTT {sl(s)}")
    def CBD(self, s, eta):       self._e("CBD", s, eta, text=f"CBD {sl(s)} eta{eta + 1}")
    def DEC(self, dsel, d, s, tee=0, chk=0):
        self._e("DEC", s, DESC[d], (chk << 3) | (tee << 2) | dsel,
                text=f"DEC {sl(s)} <- {d} d={dn(dsel)}" + (" +serap" if tee else "") + (" +cek" if chk else ""))
    def ENC(self, dsel, s, d, mode=W_WRITE):
        self._e("ENC", s, DESC[d], (mode << 2) | dsel,
                text=f"ENC {d} <- {sl(s)} d={dn(dsel)}" + ["", " (banding)", " (tulis/banding)"][mode])

    def NTT(self, s):            self._e("NTT", s, text=f"NTT {sl(s)}")
    def INTT(self, s):           self._e("INTT", s, text=f"INTT {sl(s)}")
    def PWM(self, dst, a, b, acc):
        self._e("PWM", dst | ((acc & 1) << 6), a | (((acc >> 1) & 1) << 6), b,
                text=f"PWM {sl(dst)} {['=', '+=', '+= (j>0)', '+= (i>0)'][acc]} {sl(a)} o {sl(b)}")
    def LIN(self, dst, x, y, scale, sub):
        self._e("LIN", dst | (scale << 6), x | (sub << 6), y,
                text=f"LIN {sl(dst)} = {sl(y)} {'-' if sub else '+'} {'3303*' if scale else ''}{sl(x)}")

    def CSEL(self, dst, alt):    self._e("CSEL", DESC[dst], DESC[alt], text=f"CSEL {dst} = neq ? {alt} : {dst}")
    def WIPE(self, mask):        self._imm("WIPE", 0, mask, f"WIPE 0x{mask:x}")

    def words(self) -> list[int]:
        out = []
        for ins in self.code:
            f2, f3 = ins.f2, ins.f3
            if ins.target is not None:
                t = self.labels[ins.target]
                f2, f3 = t >> 7, t & 0x7F
            out.append((OP[ins.op] << 21) | (ins.f1 << 14) | (f2 << 7) | f3)
        return out

def sl(s: int) -> str:
    base, sel = s & 0xF, (s >> 4) & 3
    name = {8: "ACC", 9: "TMP"}.get(base)
    if name and sel == 0:
        return name
    grp = "S" if base < 4 else "T"
    b = base & 3
    return f"{grp}[{['', 'i', 'j'][sel]}{'+' + str(b) if (b and sel) else (str(b) if not sel else '')}]"

def dn(dsel: int) -> str:
    return ["12", "du", "dv", "1"][dsel]

def prf(a: Asm) -> None:
    a.HINIT(SHAKE256)
    a.HABS("SIGMA")
    a.HABSC(C_N)
    a.HFIN()

def xof(a: Asm, first: int, second: int) -> None:
    a.HINIT(SHAKE128)
    a.HABS("EK_RHO")
    a.HABSC(first)
    a.HABSC(second)
    a.HFIN()

def build() -> Asm:
    a = Asm()

    a.label("WIPE_ALL")
    a.WIPE(WIPE_POLY | WIPE_KECCAK | WIPE_ALLBYTES)
    a.END(ST_OK)

    a.label("KEYGEN")
    a.CALL("kg_core")
    a.CALL("kg_out_ek")
    a.SETR(R_I, 0)
    a.label("kg_dk")
    a.ENC(D_12, S_I, "DK_S_I")
    a.LOOPI("kg_dk")
    a.END(ST_OK)

    a.label("ENCAPS")
    a.HINIT(SHA3_256)
    a.SETR(R_BAD, 0)
    a.SETR(R_I, 0)
    a.label("ec_t")
    a.DEC(D_12, "EK_T_I", T_I, tee=1, chk=1)
    a.LOOPI("ec_t")
    a.HABS("EK_RHO")
    a.HFIN()
    a.HSQZ("H")
    a.BRF(F_BAD, "fail_ek")
    a.HINIT(SHA3_512)
    a.HABS("M")
    a.HABS("H")
    a.HFIN()
    a.HSQZ("K")
    a.HSQZ("SIGMA")
    a.SETR(R_CMP, 0)
    a.CALL("enc_core")
    a.END(ST_OK)

    a.label("DECAPS")
    a.SETR(R_I, 0)
    a.label("dc_s")
    a.DEC(D_12, "DK_S_I", S_I)
    a.LOOPI("dc_s")
    a.HINIT(SHA3_256)
    a.SETR(R_I, 0)
    a.label("dc_t")
    a.DEC(D_12, "EK_T_I", T_I, tee=1)
    a.LOOPI("dc_t")
    a.HABS("EK_RHO")
    a.HFIN()
    a.SETR(R_NEQ, 0)
    a.HSQZ("H", cmp=1)
    a.BRF(F_NEQ, "fail_dk")
    a.CALL("dec_core")
    a.END(ST_OK)

    a.label("CHECK_EK")
    a.SETR(R_BAD, 0)
    a.SETR(R_I, 0)
    a.label("ce_t")
    a.DEC(D_12, "EK_T_I", T_I, chk=1)
    a.LOOPI("ce_t")
    a.BRF(F_BAD, "fail_ek")
    a.END(ST_OK)

    a.label("CHECK_DK")
    a.HINIT(SHA3_256)
    a.HABS("EK_ALL")
    a.HFIN()
    a.SETR(R_NEQ, 0)
    a.HSQZ("H", cmp=1)
    a.BRF(F_NEQ, "fail_dk")
    a.END(ST_OK)

    a.label("ENROLL")
    a.CALL("v_seeds")
    a.CALL("kg_core")
    a.CALL("kg_out_ek")
    a.WIPE(WIPE_POLY | WIPE_SECRET | WIPE_KECCAK)
    a.END(ST_OK)

    a.label("PROVE")
    a.CALL("v_seeds")
    a.CALL("kg_core")
    a.CALL("kg_out_ek")
    a.CALL("dec_core")
    a.HINIT(SHA3_256)
    for b in LABEL_TAG:
        a.HABSC(C_LIT, b)
    a.HABS("K")
    a.HABSC(C_CTXLEN)
    a.HABS("CTX")
    a.HFIN()
    a.HSQZ("TAG")
    a.WIPE(WIPE_POLY | WIPE_SECRET | WIPE_KECCAK)
    a.END(ST_OK)

    a.label("PUF_SEAL")
    a.CALL("v_check")
    a.HSQZ("HELPER_CHK")
    a.WIPE(WIPE_KECCAK)
    a.END(ST_OK)

    a.label("PUF_OPEN")
    a.CALL("v_check")
    a.SETR(R_NEQ, 0)
    a.HSQZ("HELPER_CHK", cmp=1)
    a.WIPE(WIPE_KECCAK)
    a.BRF(F_NEQ, "fail_helper")
    a.END(ST_OK)

    a.label("fail_ek")
    a.END(ST_BAD_EK)
    a.label("fail_dk")
    a.END(ST_BAD_DK)
    a.label("fail_helper")
    a.END(ST_BAD_HELPER)

    a.label("v_check")
    a.HINIT(SHAKE256)
    for b in LABEL_CHECK:
        a.HABSC(C_LIT, b)
    a.HABS("PUFKEY")
    a.HABS("HELPER_MASK")
    a.HFIN()
    a.RET()

    a.label("v_seeds")
    a.HINIT(SHAKE256)
    for b in LABEL_SEED:
        a.HABSC(C_LIT, b)
    a.HABS("PUFKEY")
    a.HFIN()
    a.HSQZ("D")
    a.HSQZ("Z")
    a.RET()

    a.label("kg_core")
    a.HINIT(SHA3_512)
    a.HABS("D")
    a.HABSC(C_K)
    a.HFIN()
    a.HSQZ("EK_RHO")
    a.HSQZ("SIGMA")
    a.SETR(R_N, 0)
    a.SETR(R_I, 0)
    a.label("kg_s")
    prf(a)
    a.CBD(S_I, ETA1)
    a.NTT(S_I)
    a.INCN()
    a.LOOPI("kg_s")
    a.SETR(R_I, 0)
    a.label("kg_e")
    prf(a)
    a.CBD(T_I, ETA1)
    a.NTT(T_I)
    a.INCN()
    a.LOOPI("kg_e")
    a.SETR(R_I, 0)
    a.label("kg_ti")
    a.SETR(R_J, 0)
    a.label("kg_tj")
    xof(a, C_J, C_I)
    a.SNTT(TMP)
    a.PWM(T_I, TMP, S_J, PWM_ACC)
    a.LOOPJ("kg_tj")
    a.LOOPI("kg_ti")
    a.RET()

    a.label("kg_out_ek")
    a.SETR(R_I, 0)
    a.label("ko_t")
    a.ENC(D_12, T_I, "EK_T_I")
    a.LOOPI("ko_t")
    a.HINIT(SHA3_256)
    a.HABS("EK_ALL")
    a.HFIN()
    a.HSQZ("H")
    a.RET()

    a.label("enc_core")
    a.SETR(R_N, 0)
    a.SETR(R_I, 0)
    a.label("en_y")
    prf(a)
    a.CBD(S_I, ETA1)
    a.NTT(S_I)
    a.INCN()
    a.LOOPI("en_y")
    a.SETR(R_I, 0)
    a.label("en_ui")
    a.SETR(R_J, 0)
    a.label("en_uj")
    xof(a, C_I, C_J)
    a.SNTT(TMP)
    a.PWM(ACC, TMP, S_J, PWM_ACC_J)
    a.LOOPJ("en_uj")
    a.INTT(ACC)
    prf(a)
    a.CBD(TMP, ETA2)
    a.INCN()
    a.LIN(ACC, ACC, TMP, scale=1, sub=0)
    a.ENC(D_U, ACC, "CT_U_I", W_FLAG)
    a.LOOPI("en_ui")
    a.SETR(R_J, 0)
    a.label("en_vj")
    a.PWM(ACC, T_J, S_J, PWM_ACC_J)
    a.LOOPJ("en_vj")
    a.INTT(ACC)
    prf(a)
    a.CBD(TMP, ETA2)
    a.LIN(ACC, ACC, TMP, scale=1, sub=0)
    a.DEC(D_1, "M", TMP)
    a.LIN(ACC, TMP, ACC, scale=0, sub=0)
    a.ENC(D_V, ACC, "CT_V", W_FLAG)
    a.RET()

    a.label("dec_core")
    a.HINIT(SHAKE256)
    a.HABS("Z")
    a.SETR(R_I, 0)
    a.label("dc_u")
    a.DEC(D_U, "CT_U_I", TMP, tee=1)
    a.NTT(TMP)
    a.PWM(ACC, TMP, S_I, PWM_ACC_I)
    a.LOOPI("dc_u")
    a.INTT(ACC)
    a.DEC(D_V, "CT_V", TMP, tee=1)
    a.HFIN()
    a.HSQZ("KBAR")
    a.LIN(ACC, ACC, TMP, scale=1, sub=1)
    a.ENC(D_1, ACC, "M")
    a.HINIT(SHA3_512)
    a.HABS("M")
    a.HABS("H")
    a.HFIN()
    a.HSQZ("K")
    a.HSQZ("SIGMA")
    a.SETR(R_NEQ, 0)
    a.SETR(R_CMP, 1)
    a.CALL("enc_core")
    a.CSEL("K", "KBAR")
    a.RET()

    return a

def emit_rom(a: Asm) -> str:
    words = a.words()
    nbits = max(8, (len(words) - 1).bit_length())
    assert nbits <= 9
    lines = [
        "module ucode_rom (",
        "    input  logic [8:0]  pc,",
        "    output logic [25:0] instr,",
        "    input  logic [3:0]  sel,",
        "    output logic [8:0]  entry",
        ");",
        "    always_comb begin",
        "        case (pc)",
    ]
    for i, w in enumerate(words):
        lines.append(f"            9'd{i}: instr = 26'h{w:07x};")
    lines += [
        "            default: instr = 26'h0200000;",
        "        endcase",
        "    end",
        "",
        "    always_comb begin",
        "        case (sel)",
    ]
    for i, name in enumerate(PROGRAMS):
        lines.append(f"            4'd{i}: entry = 9'd{a.labels[name]};")
    lines += [
        "            default: entry = 9'd0;",
        "        endcase",
        "    end",
        "endmodule",
        "",
    ]
    return "\n".join(lines)

def emit_listing(a: Asm) -> str:
    inv: dict[int, list[str]] = {}
    for name, addr in a.labels.items():
        inv.setdefault(addr, []).append(name)
    words = a.words()
    out = ["Tabel langkah pengendali ML-KEM (dibangkitkan oleh tools/ucode.py)", ""]
    for i, ins in enumerate(a.code):
        for name in inv.get(i, []):
            out.append(f"{name}:")
        out.append(f"  {i:3d}  {words[i]:07x}  {ins.text}")
    out.append("")
    out.append(f"Jumlah langkah: {len(words)}")
    return "\n".join(out) + "\n"

def main() -> None:
    import argparse
    ap = argparse.ArgumentParser()
    ap.add_argument("--rom", default=str(ROOT / "rtl" / "ucode_rom.sv"))
    ap.add_argument("--listing", default=str(ROOT / "docs" / "tabel-langkah.txt"))
    args = ap.parse_args()
    a = build()
    Path(args.rom).write_text(emit_rom(a))
    Path(args.listing).write_text(emit_listing(a))
    print(f"{len(a.code)} langkah ditulis ke {args.rom} dan {args.listing}")

if __name__ == "__main__":
    main()
