module avmm_bridge #(
    parameter [31:0] CAPS         = 32'd0,
    parameter [15:0] THRESH_RESET = 16'd64
) (
    input  logic        clk,
    input  logic        rst,

    input  logic [13:0] avs_address,
    input  logic        avs_read,
    input  logic        avs_write,
    input  logic [31:0] avs_writedata,
    output logic [31:0] avs_readdata,

    output logic        cmd_valid,
    output logic [3:0]  cmd_code,
    output logic [2:0]  cmd_k,
    output logic [7:0]  ctx_len,
    output logic [15:0] puf_thresh,
    output logic [9:0]  puf_dbg_idx,

    input  logic        busy,
    input  logic        done,
    input  logic [3:0]  err,
    input  logic        puf_ready,
    input  logic        cooling,
    input  logic        booted,
    input  logic [31:0] cycles,
    input  logic [31:0] cooldown_left,
    input  logic [31:0] proofs,
    input  logic [31:0] puf_dbg,
    input  logic [31:0] puf_dbg1,

    output logic [12:0] h_addr,
    output logic        h_we,
    output logic [7:0]  h_wdata,
    input  logic [7:0]  h_rdata
);

    localparam [31:0] ID_VALUE      = 32'h47454D42;
    localparam [31:0] VERSION_VALUE = 32'h00010000;

    wire        sel_mem = avs_address[13];
    wire [5:0]  reg_idx = avs_address[5:0];
    wire        sel_reg = !sel_mem && (avs_address[12:6] == 7'd0);

    logic cmd_pend;

    assign h_addr  = avs_address[12:0];
    assign h_we    = avs_write && sel_mem && !cmd_pend;
    assign h_wdata = avs_writedata[7:0];
    wire  busy_any = busy || cmd_pend;

    wire [31:0] status = {21'd0, booted, cooling, puf_ready, err, 1'b0,
                          (err != 4'd0) && !cmd_pend, done && !cmd_pend, busy_any};

    logic [31:0] rd_reg;
    logic        rd_mem;

    always_ff @(posedge clk) begin
        cmd_valid <= 1'b0;
        cmd_pend  <= 1'b0;
        if (rst) begin
            cmd_code    <= 4'd0;
            cmd_k       <= 3'd3;
            ctx_len     <= 8'd0;
            puf_thresh  <= THRESH_RESET;
            puf_dbg_idx <= 10'd0;
            rd_reg      <= 32'd0;
            rd_mem      <= 1'b0;
        end else begin
            if (avs_write && sel_reg && !busy_any) begin
                case (reg_idx)
                    6'd2: begin
                        cmd_code  <= avs_writedata[3:0];
                        cmd_valid <= 1'b1;
                        cmd_pend  <= 1'b1;
                    end
                    6'd4:  cmd_k       <= avs_writedata[2:0];
                    6'd5:  ctx_len     <= avs_writedata[7:0];
                    6'd10: puf_thresh  <= avs_writedata[15:0];
                    6'd11: puf_dbg_idx <= avs_writedata[9:0];
                    default: ;
                endcase
            end

            rd_mem <= avs_read && sel_mem;
            rd_reg <= 32'd0;
            if (avs_read && sel_reg) begin
                case (reg_idx)
                    6'd0:    rd_reg <= ID_VALUE;
                    6'd1:    rd_reg <= VERSION_VALUE;
                    6'd3:    rd_reg <= status;
                    6'd4:    rd_reg <= {29'd0, cmd_k};
                    6'd5:    rd_reg <= {24'd0, ctx_len};
                    6'd6:    rd_reg <= cycles;
                    6'd7:    rd_reg <= CAPS;
                    6'd8:    rd_reg <= cooldown_left;
                    6'd9:    rd_reg <= proofs;
                    6'd10:   rd_reg <= {16'd0, puf_thresh};
                    6'd11:   rd_reg <= {22'd0, puf_dbg_idx};
                    6'd12:   rd_reg <= puf_dbg;
                    6'd13:   rd_reg <= puf_dbg1;
                    default: rd_reg <= 32'd0;
                endcase
            end
        end
    end

    assign avs_readdata = rd_mem ? {24'd0, h_rdata} : rd_reg;

endmodule
