.DEFAULT_GOAL := all

REPO := $(abspath ../..)
RTL := $(sort $(wildcard $(REPO)/rtl/*.sv))
VECTORS := $(wildcard $(REPO)/vectors/*.json)
ACVP_TOOL := $(REPO)/tools/acvp_to_txt.py

CROSS_CC := arm-linux-gnueabihf-gcc
ifeq ($(origin CC),default)
CC := $(if $(shell command -v $(CROSS_CC) 2>/dev/null),$(CROSS_CC),cc)
endif
HOSTCC ?= cc
HOSTCXX ?= c++
PYTHON ?= python3
VERILATOR ?= verilator

CSTD := -std=c11
WARN ?= -Wall -Wextra -Werror -pedantic
CFLAGS ?= -O2
HOSTCFLAGS ?= -O2
HOSTCXXFLAGS ?= -O2

SIM ?= sim
VPARAMS ?= PUF_MODE=2 COOLDOWN_CYCLES=20000
VFLAGS := -Wno-fatal -Wno-WIDTH -Wno-UNUSED -Wno-UNOPTFLAT -Wno-DECLFILENAME -Wno-PINCONNECTEMPTY \
          --x-assign 0 --x-initial 0 -O3
VROOT = $(shell $(VERILATOR) --getenv VERILATOR_ROOT)
PUF_VPARAMS := PUF_MODE=1 CHIP_SEED=1 WIN_LOG2=3 PUF_DEBUG=1 COOLDOWN_CYCLES=20000
PUF_WIDE_VPARAMS := PUF_MODE=1 CHIP_SEED=1 WIN_LOG2=4 VOTES=15 PUF_DEBUG=1 COOLDOWN_CYCLES=20000 N_RO=1025
ACVP_CASES := 240
JEDA ?= 5

B := build
S := $(B)/$(SIM)
T := $(S)/uji

HDRS := gembok.h gembok_io.h sha3.h
CLI_SRC := gembok_cli.c gembok.c sha3.c
MODEL := $(S)/obj/libVgembok_top.a
SIM_LIBS := $(MODEL) $(S)/obj/libverilated.a
SIM_CORE := $(S)/gembok.o $(S)/sha3.o $(S)/io_sim.o

.PHONY: all sim vectors test test-puf test-unit test-acvp test-selftest test-driver test-cli \
        clean FORCE

all: $(B)/gembok-cli

sim: $(S)/gembok-sim $(S)/test-driver

vectors: $(B)/acvp.txt

$(B) $(S) $(T):
	mkdir -p $@

$(B)/gembok-cli: $(CLI_SRC) io_devmem.c $(HDRS) | $(B)
	$(CC) $(CSTD) $(WARN) $(CFLAGS) -o $@ $(CLI_SRC) io_devmem.c

$(B)/test-sha3: test_sha3.c sha3.c sha3.h | $(B)
	$(HOSTCC) $(CSTD) $(WARN) $(HOSTCFLAGS) -o $@ test_sha3.c sha3.c

$(B)/acvp.txt: $(ACVP_TOOL) $(VECTORS) | $(B)
	$(PYTHON) $(ACVP_TOOL) -o $@

$(S)/params: FORCE | $(S)
	@echo '$(VPARAMS)' | cmp -s - $@ || echo '$(VPARAMS)' > $@

$(MODEL): $(RTL) $(S)/params
	$(VERILATOR) --cc --build -j 0 --Mdir $(S)/obj --top-module gembok_top --prefix Vgembok_top \
	    $(VFLAGS) $(addprefix -G,$(VPARAMS)) -CFLAGS "$(HOSTCXXFLAGS)" $(RTL)
	@touch $@

$(S)/io_sim.o: io_sim.cpp gembok.h gembok_io.h $(MODEL)
	$(HOSTCXX) -std=c++17 -Wall -Wextra -Werror $(HOSTCXXFLAGS) -I. -isystem $(S)/obj \
	    -isystem $(VROOT)/include -isystem $(VROOT)/include/vltstd -c -o $@ io_sim.cpp

$(S)/%.o: %.c $(HDRS) $(S)/params | $(S)
	$(HOSTCC) $(CSTD) $(WARN) $(HOSTCFLAGS) -c -o $@ $<

$(S)/gembok-sim: $(S)/gembok_cli.o $(SIM_CORE)
	$(HOSTCXX) -o $@ $^ $(SIM_LIBS) -pthread

$(S)/test-driver: $(S)/test_driver.o $(SIM_CORE)
	$(HOSTCXX) -o $@ $^ $(SIM_LIBS) -pthread

test: test-unit test-acvp test-selftest test-driver test-cli
	@echo "SEMUA UJI LOLOS"

test-puf:
	$(MAKE) SIM=sim-puf VPARAMS="$(PUF_VPARAMS)" test-selftest test-driver test-cli
	$(MAKE) SIM=sim-puf1025 VPARAMS="$(PUF_WIDE_VPARAMS)" test-selftest test-driver test-cli
	@echo "SEMUA UJI MODEL PUF LOLOS"

test-unit: $(B)/test-sha3
	$(B)/test-sha3

test-acvp: $(S)/gembok-sim $(B)/acvp.txt
	GEMBOK_SIM_STAT=1 $(S)/gembok-sim acvp $(B)/acvp.txt --jumlah $(ACVP_CASES)

test-selftest: $(S)/gembok-sim
	$(S)/gembok-sim info
	$(S)/gembok-sim selftest
	GEMBOK_SIM_JEDA=$(JEDA) $(S)/gembok-sim selftest

test-driver: $(S)/test-driver
	$(S)/test-driver
	GEMBOK_SIM_JEDA=$(JEDA) $(S)/test-driver

test-cli: $(S)/gembok-sim | $(T)
	$(S)/gembok-sim keygen -k 4 --ek $(T)/buka.ek --dk $(T)/buka.dk \
	    + encaps -k 4 --ek $(T)/buka.ek --ct $(T)/buka.ct --kunci-bersama $(T)/buka.k1 \
	    + decaps -k 4 --dk $(T)/buka.dk --ct $(T)/buka.ct --kunci-bersama $(T)/buka.k2
	cmp $(T)/buka.k1 $(T)/buka.k2
	$(S)/gembok-sim puf-enroll --helper $(T)/bantu.txt \
	    + enroll -k 3 --ek $(T)/chip.ek \
	    + encaps -k 3 --ek $(T)/chip.ek --ct $(T)/chip.ct --kunci-bersama $(T)/chip.k \
	    + prove -k 3 --ct $(T)/chip.ct --ctx pintu-1 --tag $(T)/chip.tag --kunci-bersama $(T)/chip.k \
	    + wipe \
	    + puf-recon --helper $(T)/bantu.txt \
	    + enroll -k 3 --ek $(T)/chip2.ek \
	    + prove -k 3 --ct $(T)/chip.ct --ctx pintu-1 --tag $(T)/chip2.tag --kunci-bersama $(T)/chip.k
	cmp $(T)/chip.ek $(T)/chip2.ek
	cmp $(T)/chip.tag $(T)/chip2.tag
	awk '$$1 == "osilator" { $$2 = $$2 - 1 } { print }' $(T)/bantu.txt > $(T)/bantu-lain.txt
	$(S)/gembok-sim puf-recon --helper $(T)/bantu-lain.txt 2> $(T)/tolak-lain.txt; test $$? -eq 1
	grep -q 'tidak cocok dengan chip' $(T)/tolak-lain.txt
	sed -e 's/^\(topeng .*\)..$$/\1/' $(T)/bantu.txt > $(T)/bantu-pendek.txt
	$(S)/gembok-sim puf-recon --helper $(T)/bantu-pendek.txt 2> $(T)/tolak-pendek.txt; test $$? -eq 1
	grep -q 'seharusnya' $(T)/tolak-pendek.txt
	$(S)/gembok-sim puf-recon --helper $(T)/bantu.txt \
	    + prove -k 3 --ct $(T)/chip.ct --ctx pintu-1 --kunci-bersama $(T)/buka.k1 > $(T)/palsu.txt; \
	    test $$? -eq 1
	grep -qx PALSU $(T)/palsu.txt

clean:
	rm -rf $(B)
