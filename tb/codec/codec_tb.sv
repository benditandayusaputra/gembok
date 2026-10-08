module codec_tb (
    input  logic        clk,
    input  logic        rst,
    input  logic        start,
    input  logic        dir,
    input  logic [3:0]  d,
    input  logic [3:0]  slot,
    output logic        busy,
    output logic        done,
    output logic        range_err,
    input  logic        i_valid,
    input  logic [7:0]  i_data,
    output logic        i_ready,
    output logic        o_valid,
    output logic [7:0]  o_data,
    input  logic        o_ready,
    input  logic [10:0] tb_addr,
    input  logic        tb_we,
    input  logic [23:0] tb_wdata,
    output logic [23:0] tb_rdata
);
    logic [6:0]  m_idx;
    logic        m_we, m_re;
    logic [23:0] m_wdata, m_rdata;

    codec dut (
        .clk(clk), .rst(rst), .start(start), .dir(dir), .d(d), .busy(busy), .done(done),
        .range_err(range_err),
        .i_valid(i_valid), .i_data(i_data), .i_ready(i_ready),
        .o_valid(o_valid), .o_data(o_data), .o_ready(o_ready),
        .m_idx(m_idx), .m_we(m_we), .m_wdata(m_wdata), .m_re(m_re), .m_rdata(m_rdata)
    );

    poly_mem u_m (
        .clk(clk),
        .a_addr({slot, m_idx}), .a_we(m_we), .a_wdata(m_wdata), .a_rdata(m_rdata),
        .b_addr(tb_addr), .b_we(tb_we), .b_wdata(tb_wdata), .b_rdata(tb_rdata)
    );
endmodule
