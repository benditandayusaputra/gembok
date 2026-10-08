#!/usr/bin/env python3
from __future__ import annotations

import os
import re
import shutil
import subprocess
import sys
import tempfile
import time
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent

MLKEM_CEPAT = ("mlkem", {}, "asap")
MLKEM_NIST = ("mlkem", {"ACVP_LIMIT": "2"}, "asap,vektor_nist")
MLKEM_ID = ("mlkem", {"ACVP": "0"}, "identitas")
TOP_DEV = lambda t: ("top", {"CONFIG": "dev"}, t)
TOP_PUF = lambda t: ("top", {"CONFIG": "puf"}, t)
TOP_RODBG = lambda t: ("top", {"CONFIG": "rodbg"}, t)

MUTASI = [
    ("Konstanta Barrett salah", "rtl/modmul.sv",
     [("37'd5039", "37'd5038")], MLKEM_CEPAT),
    ("Konstanta ronde Keccak salah", "rtl/keccak_core.sv",
     [("5'd7:  round_const = 64'h8000000000008009;", "5'd7:  round_const = 64'h800000000000800A;")], MLKEM_CEPAT),
    ("Zeta NTT salah", "rtl/zeta_rom.sv",
     [("7'd17: z = 12'd", "7'd17: z = 12'd1+12'd")], MLKEM_CEPAT),
    ("Pembulatan kompresi salah", "rtl/codec.sv",
     [("4'd10:   e_n = {1'd0,  s1_x, 10'd0} + 23'd1664;", "4'd10:   e_n = {1'd0,  s1_x, 10'd0} + 23'd1665;")],
     ("codec", {}, "kompresi_semua_nilai")),
    ("Batas penolakan SampleNTT salah", "rtl/sampler.sv",
     [("wire        a1 = (d1 < 12'd3329);", "wire        a1 = (d1 <= 12'd3329);")], MLKEM_NIST),
    ("Penolakan implisit dimatikan", "rtl/mlkem_ctrl.sv",
     [("bm_wdata = f_neq ? bm_rdata : cs_a;", "bm_wdata = cs_a;")], MLKEM_CEPAT),
    ("Penghapusan rahasia dihapus", "tools/ucode.py",
     [('    a.CALL("kg_out_ek")\n    a.WIPE(WIPE_POLY | WIPE_SECRET | WIPE_KECCAK)\n    a.END(ST_OK)\n\n    a.label("PROVE")',
       '    a.CALL("kg_out_ek")\n    a.SETR(R_N, 0)\n    a.END(ST_OK)\n\n    a.label("PROVE")')], MLKEM_ID),
    ("Cek modulus kunci dimatikan", "rtl/codec.sv",
     [("wire d_over = (d_r == 4'd12) && ((p_v0 >= 12'd3329) || (p_v1 >= 12'd3329));", "wire d_over = 1'b0;")],
     TOP_DEV("status_tidak_basi")),
    ("Penjaga akses dimatikan", "rtl/vault.sv",
     [("(host_ok ? h_addr : 13'd0)", "h_addr"),
      ("assign h_rdata = (host_ok && host_ok_q) ? m_rdata : 8'd0;", "assign h_rdata = m_rdata;")],
     TOP_DEV("penjaga_akses")),
    ("Pembatas laju dimatikan", "rtl/vault.sv",
     [("cooldown <= COOLDOWN_CYCLES;", "cooldown <= 32'd0;")], TOP_DEV("pembatas_laju")),
    ("STATUS basi sesudah CTRL", "rtl/avmm_bridge.sv",
     [("cmd_pend  <= 1'b1;", "cmd_pend  <= 1'b0;")], TOP_DEV("status_tidak_basi")),
    ("Register bisa diubah saat sibuk", "rtl/avmm_bridge.sv",
     [("if (avs_write && sel_reg && !busy_any) begin", "if (avs_write && sel_reg && (reg_idx != 6'd2 || !busy_any)) begin"),
      (".ctx_len   (core_ctx_len),", ".ctx_len   (ctx_len),")],
     TOP_DEV("register_terkunci_saat_sibuk")),
    ("Cek data bantu PUF dilewati", "tools/ucode.py",
     [('    a.BRF(F_NEQ, "fail_helper")\n', "")], TOP_PUF("data_bantu_diubah")),
    ("Tulisan memori diterima sesudah CTRL", "rtl/avmm_bridge.sv",
     [("assign h_we    = avs_write && sel_mem && !cmd_pend;", "assign h_we    = avs_write && sel_mem;")],
     TOP_DEV("tulis_memori_sesudah_ctrl")),
    ("Reset mengosongkan pembatas laju", "rtl/vault.sv",
     [("            st        <= V_BOOT;\n", "            st        <= V_BOOT;\n            cooldown  <= 32'd0;\n")],
     TOP_DEV("pembatas_laju_tahan_reset")),
    ("Pencacah osilator 16 bit", "rtl/puf_ro.sv",
     [("reg [18:0] ha;", "reg [14:0] ha;"), ("reg [18:0] hb;", "reg [14:0] hb;")],
     TOP_RODBG("ro_hitungan_20_bit")),
]

RINGKAS = re.compile(r"TESTS=(\d+)\s+PASS=(\d+)\s+FAIL=(\d+)")

def run_case(work: Path, tb: str, env_extra: dict, tests: str, rtl: Path) -> tuple[int, int, str]:
    build = work / f"sim_{tb}"
    env = dict(os.environ)
    env.update(env_extra)
    args = ["make", "-C", str(ROOT / "tb" / tb), f"RTL={rtl}", f"SIM_BUILD={build}", f"TESTCASE={tests}",
            f"COCOTB_RESULTS_FILE={work / 'results.xml'}"]
    if "CONFIG" in env_extra:
        args.append(f"CONFIG={env_extra['CONFIG']}")
    out = subprocess.run(args, env=env, capture_output=True, text=True).stdout
    m = RINGKAS.findall(out)
    if not m:
        return -1, -1, out[-2000:]
    tests_n, passed, failed = (int(x) for x in m[-1])
    return passed, failed, out[-2000:]

def main() -> int:
    only = sys.argv[1:]
    base = Path(tempfile.mkdtemp(prefix="mutasi_", dir=os.environ.get("TMPDIR")))
    hasil = []
    for nama, path, edits, (tb, env_extra, tests) in MUTASI:
        if only and not any(o.lower() in nama.lower() for o in only):
            continue
        work = base / re.sub(r"\W+", "_", nama)
        rtl = work / "rtl"
        shutil.copytree(ROOT / "rtl", rtl)
        if path.startswith("tools/"):
            src = (ROOT / path).read_text()
            for a, b in edits:
                assert src.count(a) >= 1, (nama, a)
                src = src.replace(a, b, 1)
            tool = work / "ucode.py"
            tool.write_text(src)
            subprocess.run([sys.executable, str(tool), "--rom", str(rtl / "ucode_rom.sv"),
                            "--listing", str(work / "tabel.txt")], check=True, capture_output=True,
                           env=dict(os.environ, PYTHONPATH=str(ROOT / "tools")))
        else:
            f = rtl / Path(path).name
            src = f.read_text()
            for a, b in edits:
                if a not in src:
                    other = rtl / "gembok_top.sv"
                    o = other.read_text()
                    assert a in o, (nama, a)
                    other.write_text(o.replace(a, b, 1))
                    continue
                src = src.replace(a, b, 1)
            f.write_text(src)
        t0 = time.time()
        passed, failed, tail = run_case(work, tb, env_extra, tests, rtl)
        dt = time.time() - t0
        tertangkap = failed > 0
        hasil.append((nama, tests, passed, failed, tertangkap, dt))
        print(f"{'TERTANGKAP' if tertangkap else 'LOLOS (BURUK)':14s} {nama:38s} uji={tests} lolos={passed} gagal={failed} ({dt:.0f} s)", flush=True)
        if failed < 0:
            print(tail)
    n_ok = sum(1 for h in hasil if h[4])
    print(f"\n{n_ok} dari {len(hasil)} mutasi tertangkap oleh uji")
    shutil.rmtree(base, ignore_errors=True)
    return 0 if n_ok == len(hasil) else 1

if __name__ == "__main__":
    sys.exit(main())
