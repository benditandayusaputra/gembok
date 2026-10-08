module poly_arith_tb (
    input  logic        clk,
    input  logic        rst,
    input  logic        start,
    input  logic [1:0]  op,
    input  logic [3:0]  s_dst,
    input  logic [3:0]  s_a,
    input  logic [3:0]  s_b,
    input  logic [1:0]  opt,
    output logic        busy,
    output logic        done,
    input  logic        tb_sel,
    input  logic [10:0] tb_addr,
    input  logic        tb_we,
    input  logic [23:0] tb_wdata,
    output logic [23:0] tb_rdata
);
    logic [10:0] pa_addr, pb_addr;
    logic        pa_we, pb_we;
    logic [23:0] pa_wdata, pb_wdata, pa_rdata, pb_rdata;

    poly_arith dut (
        .clk(clk), .rst(rst), .start(start), .op(op), .s_dst(s_dst), .s_a(s_a), .s_b(s_b), .opt(opt),
        .busy(busy), .done(done),
        .pa_addr(pa_addr), .pa_we(pa_we), .pa_wdata(pa_wdata), .pa_rdata(pa_rdata),
        .pb_addr(pb_addr), .pb_we(pb_we), .pb_wdata(pb_wdata), .pb_rdata(pb_rdata)
    );

    poly_mem mem (
        .clk(clk),
        .a_addr (tb_sel ? tb_addr  : pa_addr),
        .a_we   (tb_sel ? tb_we    : pa_we),
        .a_wdata(tb_sel ? tb_wdata : pa_wdata),
        .a_rdata(pa_rdata),
        .b_addr (pb_addr), .b_we(pb_we), .b_wdata(pb_wdata), .b_rdata(pb_rdata)
    );
    assign tb_rdata = pa_rdata;
endmodule
