TOPLEVEL_LANG ?= verilog
SIM ?= verilator

TB_DIR  := $(abspath $(dir $(lastword $(MAKEFILE_LIST))))
REPO    := $(abspath $(TB_DIR)/..)
RTL     := $(REPO)/rtl

export PYTHONPATH := $(TB_DIR)/common:$(REPO)/model:$(REPO)/tools:$(PYTHONPATH)
export PYTHONDONTWRITEBYTECODE := 1

ifeq ($(SIM),verilator)
  EXTRA_ARGS += -Wno-fatal -Wno-WIDTH -Wno-UNUSED -Wno-UNOPTFLAT -Wno-DECLFILENAME -Wno-PINCONNECTEMPTY
  EXTRA_ARGS += --x-assign 0 --x-initial 0 -O3
  ifneq ($(WAVES),)
    EXTRA_ARGS += --trace-fst --trace-structs
  endif
endif
ifeq ($(SIM),icarus)
  COMPILE_ARGS += -g2012
endif

include $(shell cocotb-config --makefiles)/Makefile.sim
