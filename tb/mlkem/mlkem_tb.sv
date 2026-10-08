`timescale 1ns/1ps

module mlkem_tb (
    output logic         clk,
    input  logic         rst,
    input  logic         cmd_start,
    input  logic [3:0]   cmd_sel,
    input  logic [2:0]   cmd_k,
    input  logic [7:0]   ctx_len,
    output logic         busy,
    output logic         done,
    output logic [3:0]   status,
    input  logic [255:0] puf_key,
    input  logic [12:0]  h_addr,
    input  logic         h_we,
    input  logic [7:0]   h_wdata,
    output logic [7:0]   h_rdata,
    output logic [31:0]  cycles
);
    initial clk = 1'b0;
    always #10 clk = ~clk;

    logic [4:0] puf_idx;
    wire  [7:0] puf_byte = puf_key[8*puf_idx +: 8];

    mlkem_ctrl dut (
        .clk(clk), .rst(rst),
        .cmd_start(cmd_start), .cmd_sel(cmd_sel), .cmd_k(cmd_k), .ctx_len(ctx_len),
        .busy(busy), .done(done), .status(status),
        .puf_idx(puf_idx), .puf_byte(puf_byte),
        .h_addr(h_addr), .h_we(h_we), .h_wdata(h_wdata), .h_rdata(h_rdata)
    );

    logic [31:0] cnt;
    always_ff @(posedge clk) begin
        if (rst) begin
            cnt    <= 32'd0;
            cycles <= 32'd0;
        end else begin
            if (cmd_start && !busy) cnt <= 32'd1;
            else if (busy)          cnt <= cnt + 32'd1;
            if (done) cycles <= cnt;
        end
    end
endmodule
