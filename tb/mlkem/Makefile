RTL ?= $(abspath ../../rtl)
TOPLEVEL = mlkem_tb
MODULE ?= test_mlkem
VERILOG_SOURCES = $(abspath mlkem_tb.sv) $(RTL)/mlkem_ctrl.sv $(RTL)/ucode_rom.sv \
    $(RTL)/keccak_core.sv $(RTL)/keccak_round.sv $(RTL)/sampler.sv $(RTL)/codec.sv \
    $(RTL)/poly_arith.sv $(RTL)/modmul.sv $(RTL)/zeta_rom.sv $(RTL)/poly_mem.sv $(RTL)/ram_tdp.sv
ifneq ($(SIM),icarus)
  EXTRA_ARGS += --timing
endif
include ../Makefile.inc
