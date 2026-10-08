#!/usr/bin/env bash
set -euo pipefail
cd "$(dirname "$0")/.."
MODE="${1:-2}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT
cat > "$TMP/ram_bb.v" <<'VEOF'
(* blackbox *)
module ram_tdp #(parameter AW = 11, parameter DW = 24) (
    input clk,
    input [AW-1:0] a_addr, input a_we, input [DW-1:0] a_wdata, output [DW-1:0] a_rdata,
    input [AW-1:0] b_addr, input b_we, input [DW-1:0] b_wdata, output [DW-1:0] b_rdata
);
endmodule
VEOF
SRC="$(ls rtl/*.sv | grep -v ram_tdp.sv | tr '\n' ' ')"
yosys -q -l "$TMP/yosys.log" -p "read_verilog -sv $TMP/ram_bb.v $SRC; chparam -set PUF_MODE $MODE gembok_top; hierarchy -top gembok_top; synth_intel_alm -family cyclonev -top gembok_top; tee -o $TMP/stat.txt stat"
cat "$TMP/stat.txt"
