package require -exact qsys 16.1

set_module_property NAME gembok
set_module_property VERSION 1.0
set_module_property DISPLAY_NAME "GEMBOK ML-KEM identity core"
set_module_property GROUP "GEMBOK"
set_module_property AUTHOR "Tim GEMBOK"
set_module_property EDITABLE false
set_module_property INSTANTIATE_IN_SYSTEM_MODULE true

add_fileset QUARTUS_SYNTH QUARTUS_SYNTH "" ""
set_fileset_property QUARTUS_SYNTH TOP_LEVEL gembok_top
set_fileset_property QUARTUS_SYNTH ENABLE_RELATIVE_INCLUDE_PATHS false
foreach f {keccak_round keccak_core modmul zeta_rom poly_arith sampler codec ram_tdp poly_mem ucode_rom mlkem_ctrl puf_ro fuzzy_extractor vault avmm_bridge} {
    add_fileset_file $f.sv SYSTEM_VERILOG PATH rtl/$f.sv
}
add_fileset_file gembok_top.sv SYSTEM_VERILOG PATH rtl/gembok_top.sv TOP_LEVEL_FILE
add_fileset_file gembok_puf.sdc SDC PATH gembok_puf.sdc

proc gembok_param {name value range label} {
    add_parameter $name INTEGER $value
    set_parameter_property $name HDL_PARAMETER true
    set_parameter_property $name ALLOWED_RANGES $range
    set_parameter_property $name DISPLAY_NAME $label
}

gembok_param PUF_MODE        0       {0 2}                  "Sumber kunci (0 = PUF osilator, 2 = kunci pengembangan, tidak aman)"
gembok_param N_RO            768     {600:1025}             "Jumlah osilator"
gembok_param RO_STAGES       5       {3 5 7 9 11 13}        "Tahap tiap osilator (harus ganjil)"
gembok_param WIN_LOG2        12      {4:16}                 "Lama jendela ukur (2^n siklus clock)"
gembok_param VOTES           5       {1 3 5 7 9 11 13 15}   "Jumlah pengukuran per pasangan (harus ganjil)"
gembok_param PUF_DEBUG       0       {0 1}                  "Bangunan debug dengan PUF_MEASURE (jangan untuk demo)"
gembok_param COOLDOWN_CYCLES 1048576 {0:2147483647}         "Jeda pembatas laju sesudah BUKTIKAN (siklus)"
gembok_param THRESH_RESET    64      {1:65535}              "Ambang PUF awal"

add_interface clock clock end
set_interface_property clock clockRate 50000000
add_interface_port clock clk clk Input 1

add_interface reset reset end
set_interface_property reset associatedClock clock
set_interface_property reset synchronousEdges DEASSERT
add_interface_port reset reset reset Input 1

add_interface s0 avalon end
set_interface_property s0 addressUnits WORDS
set_interface_property s0 associatedClock clock
set_interface_property s0 associatedReset reset
set_interface_property s0 bitsPerSymbol 8
set_interface_property s0 readLatency 1
set_interface_property s0 readWaitTime 0
set_interface_property s0 writeWaitTime 0
set_interface_property s0 maximumPendingReadTransactions 0
set_interface_property s0 timingUnits Cycles
add_interface_port s0 avs_address address Input 14
add_interface_port s0 avs_read read Input 1
add_interface_port s0 avs_write write Input 1
add_interface_port s0 avs_writedata writedata Input 32
add_interface_port s0 avs_readdata readdata Output 32

add_interface led conduit end
set_interface_property led associatedClock clock
add_interface_port led led export Output 8
