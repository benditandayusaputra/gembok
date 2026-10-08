CONFIG ?= dev
RTL ?= $(abspath ../../rtl)
TOPLEVEL = gembok_tb
MODULE = test_top
SIM_BUILD = sim_build_$(CONFIG)
export GEMBOK_CONFIG := $(CONFIG)

PARAMS_dev   = PUF_MODE=2
PARAMS_puf   = PUF_MODE=1 CHIP_SEED=1
PARAMS_clone = PUF_MODE=1 CHIP_SEED=2
PARAMS_noise = PUF_MODE=1 CHIP_SEED=1 SIM_NOISE=10
PARAMS_debug = PUF_MODE=1 CHIP_SEED=1 PUF_DEBUG=1
PARAMS_votes15 = PUF_MODE=1 CHIP_SEED=1 VOTES=15
PARAMS_ro    = PUF_MODE=0 CHIP_SEED=1 WIN_LOG2=6
PARAMS_rodbg = PUF_MODE=0 CHIP_SEED=1 WIN_LOG2=12 PUF_DEBUG=1 SIM_RO_HALF_PS=400

VERILOG_SOURCES = $(abspath gembok_tb.sv) $(wildcard $(RTL)/*.sv)
SIM ?= verilator
ifeq ($(SIM),icarus)
  COMPILE_ARGS += $(foreach p,$(PARAMS_$(CONFIG)),-P$(TOPLEVEL).$(p)) -DGEMBOK_SIM_RO
else
  EXTRA_ARGS += --timing $(foreach p,$(PARAMS_$(CONFIG)),-G$(p)) +define+GEMBOK_SIM_RO
endif
include ../Makefile.inc
