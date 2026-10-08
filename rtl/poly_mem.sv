module poly_mem (
    input  logic        clk,

    input  logic [10:0] a_addr,
    input  logic        a_we,
    input  logic [23:0] a_wdata,
    output logic [23:0] a_rdata,

    input  logic [10:0] b_addr,
    input  logic        b_we,
    input  logic [23:0] b_wdata,
    output logic [23:0] b_rdata
);

    ram_tdp #(.AW(11), .DW(24)) u_ram (
        .clk     (clk),
        .a_addr  (a_addr),
        .a_we    (a_we),
        .a_wdata (a_wdata),
        .a_rdata (a_rdata),
        .b_addr  (b_addr),
        .b_we    (b_we),
        .b_wdata (b_wdata),
        .b_rdata (b_rdata)
    );

endmodule
