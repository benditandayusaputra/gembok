PY ?= python3
SIM ?= verilator
CEK = $(PY) $(CURDIR)/tb/cek_hasil.py

.PHONY: semua model ucode lint unit nist top hps verifier simulasi mutasi bersih

semua: model ucode lint unit nist top hps verifier

model:
	$(PY) tests/run_acvp.py
	$(PY) -m unittest discover -s tests

ucode:
	$(PY) tools/gen_zeta_rom.py
	$(PY) tools/ucode.py
	$(PY) tools/ucode_sim.py

lint:
	for m in 0 1 2; do verilator --lint-only -Wall tools/verilator_lint.vlt -GPUF_MODE=$$m rtl/*.sv --top-module gembok_top || exit 1; done
	verilator --lint-only -Wall tools/verilator_lint.vlt -GPUF_MODE=0 +define+GEMBOK_SIM_RO --timing rtl/*.sv --top-module gembok_top

unit:
	$(MAKE) -C tb/modmul
	for t in keccak poly_arith sampler codec; do $(MAKE) -C tb/$$t SIM=$(SIM) && $(CEK) tb/$$t/results.xml || exit 1; done

nist:
	$(MAKE) -C tb/mlkem SIM=$(SIM) && $(CEK) tb/mlkem/results.xml

top:
	for c in dev puf clone noise debug votes15 ro rodbg; do $(MAKE) -C tb/top CONFIG=$$c SIM=$(SIM) && $(CEK) tb/top/results.xml || exit 1; done

hps:
	$(MAKE) -C sw/hps test
	$(MAKE) -C sw/hps test-puf

verifier:
	cd sw/verifier && go vet ./... && go test ./...

simulasi:
	cd sw/verifier && GOOS=js GOARCH=wasm go build -trimpath -ldflags="-s -w" -o ../../simulasi/gembok.wasm ./cmd/gembok-wasm
	cp "$$(go env GOROOT)/lib/wasm/wasm_exec.js" simulasi/

mutasi:
	$(PY) tools/uji_mutasi.py

bersih:
	rm -rf tb/*/sim_build* tb/*/results.xml tb/modmul/obj_dir tb/modmul/build.log
	$(MAKE) -C sw/hps clean
