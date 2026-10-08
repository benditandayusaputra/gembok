RTL ?= $(abspath ../../rtl)
all:
	verilator --binary --timing -Wno-fatal -O3 --top-module tb_modmul -Mdir obj_dir tb_modmul.sv $(RTL)/modmul.sv > build.log 2>&1 || (cat build.log; false)
	./obj_dir/Vtb_modmul
clean:
	rm -rf obj_dir build.log
