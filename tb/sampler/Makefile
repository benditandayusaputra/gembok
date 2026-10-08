RTL ?= $(abspath ../../rtl)
TOPLEVEL = sampler_tb
MODULE = test_sampler
VERILOG_SOURCES = $(abspath sampler_tb.sv) $(RTL)/sampler.sv $(RTL)/keccak_core.sv $(RTL)/keccak_round.sv $(RTL)/poly_mem.sv $(RTL)/ram_tdp.sv
include ../Makefile.inc
