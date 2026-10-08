module ram_tdp #(
    parameter AW = 11,
    parameter DW = 24
) (
    input  logic          clk,

    input  logic [AW-1:0] a_addr,
    input  logic          a_we,
    input  logic [DW-1:0] a_wdata,
    output logic [DW-1:0] a_rdata,

    input  logic [AW-1:0] b_addr,
    input  logic          b_we,
    input  logic [DW-1:0] b_wdata,
    output logic [DW-1:0] b_rdata
);

    (* ramstyle = "M10K, no_rw_check" *) reg [DW-1:0] mem [0:(1<<AW)-1];

    always @(posedge clk) begin
        if (a_we) begin
            mem[a_addr] <= a_wdata;
            a_rdata     <= a_wdata;
        end else begin
            a_rdata     <= mem[a_addr];
        end
    end

    always @(posedge clk) begin
        if (b_we) begin
            mem[b_addr] <= b_wdata;
            b_rdata     <= b_wdata;
        end else begin
            b_rdata     <= mem[b_addr];
        end
    end

endmodule
