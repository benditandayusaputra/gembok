module sampler_tb (
    input  logic        clk,
    input  logic        rst,
    input  logic        init,
    input  logic [1:0]  kmode,
    input  logic        finish,
    input  logic        a_valid,
    input  logic [7:0]  a_data,
    output logic        a_ready,
    input  logic        start,
    input  logic [1:0]  smode,
    input  logic [3:0]  slot,
    output logic        busy,
    output logic        done,
    input  logic [10:0] tb_addr,
    output logic [23:0] tb_rdata
);
    logic        triple, q_valid, q_ready, t_valid, t_ready, kbusy;
    logic [7:0]  q_data;
    logic [23:0] t_data;
    logic        w_valid;
    logic [6:0]  w_idx;
    logic [23:0] w_data;
    logic [23:0] unused_a;

    keccak_core u_k (
        .clk(clk), .rst(rst), .init(init), .mode(kmode), .finish(finish), .triple(triple),
        .a_valid(a_valid), .a_data(a_data), .a_ready(a_ready),
        .q_valid(q_valid), .q_data(q_data), .q_ready(q_ready),
        .t_valid(t_valid), .t_data(t_data), .t_ready(t_ready), .busy(kbusy)
    );

    sampler u_s (
        .clk(clk), .rst(rst), .start(start), .mode(smode), .busy(busy), .done(done),
        .triple(triple), .q_valid(q_valid), .q_data(q_data), .q_ready(q_ready),
        .t_valid(t_valid), .t_data(t_data), .t_ready(t_ready),
        .w_valid(w_valid), .w_idx(w_idx), .w_data(w_data)
    );

    poly_mem u_m (
        .clk(clk),
        .a_addr({slot, w_idx}), .a_we(w_valid), .a_wdata(w_data), .a_rdata(unused_a),
        .b_addr(tb_addr), .b_we(1'b0), .b_wdata(24'd0), .b_rdata(tb_rdata)
    );
endmodule
