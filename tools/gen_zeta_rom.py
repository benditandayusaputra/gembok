#!/usr/bin/env python3

from __future__ import annotations

import sys
from pathlib import Path

ROOT = Path(__file__).resolve().parent.parent
sys.path.insert(0, str(ROOT / "model"))

import mlkem

def main() -> None:
    for j in range(64):
        assert mlkem.GAMMAS[2 * j] == mlkem.ZETAS[64 + j]
        assert mlkem.GAMMAS[2 * j + 1] == mlkem.Q - mlkem.ZETAS[64 + j]

    lines = [
        "module zeta_rom (",
        "    input  logic [6:0]  idx,",
        "    output logic [11:0] z",
        ");",
        "    always_comb begin",
        "        case (idx)",
    ]
    for i, z in enumerate(mlkem.ZETAS):
        lines.append(f"            7'd{i}: z = 12'd{z};")
    lines += [
        "            default: z = 12'd0;",
        "        endcase",
        "    end",
        "endmodule",
        "",
    ]
    out = ROOT / "rtl" / "zeta_rom.sv"
    out.write_text("\n".join(lines))
    print(f"ditulis: {out} ({len(mlkem.ZETAS)} entri)")

if __name__ == "__main__":
    main()
