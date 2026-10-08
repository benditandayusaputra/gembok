`timescale 1ns/1ps

module gembok_tb #(
    parameter integer PUF_MODE        = 2,
    parameter integer N_RO            = 768,
    parameter integer WIN_LOG2        = 3,
    parameter integer VOTES           = 5,
    parameter [31:0]  COOLDOWN_CYCLES = 32'd3000,
    parameter [31:0]  CHIP_SEED       = 32'h0000_0001,
    parameter integer SIM_NOISE       = 0,
    parameter integer SIM_RO_HALF_PS  = 1500,
    parameter integer PUF_DEBUG       = 0
) (
    output logic        clk,
    input  logic        reset,
    input  logic [13:0] avs_address,
    input  logic        avs_read,
    input  logic        avs_write,
    input  logic [31:0] avs_writedata,
    output logic [31:0] avs_readdata,
    output logic [7:0]  led,
    output logic        busy_o,
    output logic        done_o
);
    initial clk = 1'b0;
    always #10 clk = ~clk;

    gembok_top #(
        .PUF_MODE        (PUF_MODE),
        .N_RO            (N_RO),
        .WIN_LOG2        (WIN_LOG2),
        .VOTES           (VOTES),
        .COOLDOWN_CYCLES (COOLDOWN_CYCLES),
        .CHIP_SEED       (CHIP_SEED),
        .SIM_NOISE       (SIM_NOISE),
        .SIM_RO_HALF_PS  (SIM_RO_HALF_PS),
        .PUF_DEBUG       (PUF_DEBUG)
    ) dut (
        .clk(clk), .reset(reset),
        .avs_address(avs_address), .avs_read(avs_read), .avs_write(avs_write),
        .avs_writedata(avs_writedata), .avs_readdata(avs_readdata),
        .led(led)
    );

    assign busy_o = led[0];
    assign done_o = led[1];
endmodule
