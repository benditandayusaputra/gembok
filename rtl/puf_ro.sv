module ro_cell #(
    parameter integer STAGES      = 5,
    parameter integer SIM_HALF_PS = 1500
) (
    input  logic en,
    output logic osc
);
`ifdef GEMBOK_SIM_RO
    logic r;
    initial r = 1'b1;
    always begin
        if (en) begin
            #(SIM_HALF_PS * 1ps);
            r = en ? ~r : 1'b1;
        end else begin
            r = 1'b1;
            @(posedge en);
        end
    end
    assign osc = r;
`else
    (* keep = 1 *) wire [STAGES-1:0] n;

    assign n[0] = ~(en & n[STAGES-1]);
    genvar i;
    generate
        for (i = 1; i < STAGES; i = i + 1) begin : g_inv
            assign n[i] = ~n[i-1];
        end
    endgenerate
    assign osc = n[STAGES-1];
`endif
endmodule

module puf_ro #(
    parameter integer N_RO      = 768,
    parameter integer STAGES    = 5,
    parameter integer WIN_LOG2  = 12,
    parameter integer MODE      = 0,
    parameter [31:0]  CHIP_SEED = 32'h0000_0001,
    parameter integer SIM_NOISE = 0,
    parameter integer SIM_RO_HALF_PS = 1500
) (
    input  logic        clk,
    input  logic        rst,

    input  logic        restart,
    input  logic        next,
    input  logic        measure,
    input  logic [15:0] thresh,

    output logic        busy,
    output logic        done,
    output logic        bit_o,
    output logic        solid,
    output logic [19:0] cnt_c,
    output logic [19:0] cnt_c1
);

    localparam [2:0] M_IDLE   = 3'd0;
    localparam [2:0] M_CLEAR  = 3'd1;
    localparam [2:0] M_RUN    = 3'd2;
    localparam [2:0] M_SETTLE = 3'd3;
    localparam [2:0] M_EVAL   = 3'd4;

    logic [2:0]  st;
    logic [15:0] c;
    logic [15:0] wcnt;
    logic        win_en;
    logic        clr;

    wire [19:0] cnt_a;
    wire [19:0] cnt_b;

    assign busy = (st != M_IDLE);

    wire [19:0] raw_c  = c[0] ? cnt_b : cnt_a;
    wire [19:0] raw_c1 = c[0] ? cnt_a : cnt_b;
    wire [20:0] diff   = {1'b0, raw_c} - {1'b0, raw_c1};
    wire [19:0] adiff  = diff[20] ? (raw_c1 - raw_c) : (raw_c - raw_c1);

    always_ff @(posedge clk) begin
        done <= 1'b0;
        if (rst) begin
            st     <= M_IDLE;
            c      <= 16'd0;
            wcnt   <= 16'd0;
            win_en <= 1'b0;
            clr    <= 1'b1;
            bit_o  <= 1'b0;
            solid <= 1'b0;
            cnt_c  <= 20'd0;
            cnt_c1 <= 20'd0;
        end else begin
            case (st)
                M_IDLE: begin
                    win_en <= 1'b0;
                    if (restart)      c <= 16'd0;
                    else if (next)    c <= c + 16'd1;
                    if (measure) begin
                        clr  <= 1'b1;
                        wcnt <= 16'd0;
                        st   <= M_CLEAR;
                    end
                end
                M_CLEAR: begin
                    wcnt <= wcnt + 16'd1;
                    if (wcnt == 16'd2) clr <= 1'b0;
                    if (wcnt == 16'd3) begin
                        wcnt   <= 16'd0;
                        win_en <= 1'b1;
                        st     <= M_RUN;
                    end
                end
                M_RUN: begin
                    wcnt <= wcnt + 16'd1;
                    if (wcnt == ((16'd1 << WIN_LOG2) - 16'd1)) begin
                        win_en <= 1'b0;
                        wcnt   <= 16'd0;
                        st     <= M_SETTLE;
                    end
                end
                M_SETTLE: begin
                    wcnt <= wcnt + 16'd1;
                    if (wcnt == 16'd7) st <= M_EVAL;
                end
                M_EVAL: begin
                    bit_o  <= !diff[20] && (adiff != 20'd0);
                    solid  <= (adiff >= {4'd0, thresh});
                    cnt_c  <= raw_c;
                    cnt_c1 <= raw_c1;
                    done   <= 1'b1;
                    st     <= M_IDLE;
                end
                default: st <= M_IDLE;
            endcase
        end
    end

    function automatic logic [15:0] ro_freq(input logic [31:0] seed, input logic [15:0] idx);
        logic [31:0] x;
        begin
            x = seed ^ ({16'd0, idx} * 32'h9E3779B1);
            x = x ^ (x >> 16);
            x = x * 32'h85EBCA6B;
            x = x ^ (x >> 13);
            x = x * 32'hC2B2AE35;
            x = x ^ (x >> 16);
            ro_freq = 16'd20000 + {6'd0, x[9:0]};
        end
    endfunction

    generate
        if (MODE != 1) begin : g_real
            logic [N_RO-1:0] token;
            always_ff @(posedge clk) begin
                if (rst) begin
                    token <= {{(N_RO-1){1'b0}}, 1'b1};
                end else if (st == M_IDLE) begin
                    if (restart)   token <= {{(N_RO-1){1'b0}}, 1'b1};
                    else if (next) token <= {token[N_RO-2:0], 1'b0};
                end
            end

            wire [N_RO-1:0] osc;
            genvar g;
            for (g = 0; g < N_RO; g = g + 1) begin : g_ro
                wire sel;
                if (g == 0) begin : g_first
                    assign sel = token[0];
                end else begin : g_rest
                    assign sel = token[g] | token[g-1];
                end
                localparam integer HALF = SIM_RO_HALF_PS + (({16'd0, ro_freq(CHIP_SEED, g)} - 20000) / 4);
                ro_cell #(.STAGES(STAGES), .SIM_HALF_PS(HALF)) u_ro (.en(win_en & sel), .osc(osc[g]));
            end

            wire [N_RO-1:0] osc_even;
            wire [N_RO-1:0] osc_odd;
            for (g = 0; g < N_RO; g = g + 1) begin : g_side
                assign osc_even[g] = ((g % 2) == 0) ? osc[g] : 1'b1;
                assign osc_odd[g]  = ((g % 2) == 1) ? osc[g] : 1'b1;
            end
            (* keep = 1 *) wire osc_a = &osc_even;
            (* keep = 1 *) wire osc_b = &osc_odd;

            reg        ta;
            reg        tb;
            reg [18:0] ha;
            reg [18:0] hb;
            always @(posedge osc_a or posedge clr) begin
                if (clr) ta <= 1'b0;
                else     ta <= ~ta;
            end
            always @(posedge osc_b or posedge clr) begin
                if (clr) tb <= 1'b0;
                else     tb <= ~tb;
            end
            always @(negedge ta or posedge clr) begin
                if (clr) ha <= 19'd0;
                else     ha <= ha + 19'd1;
            end
            always @(negedge tb or posedge clr) begin
                if (clr) hb <= 19'd0;
                else     hb <= hb + 19'd1;
            end
            assign cnt_a = {ha, ta};
            assign cnt_b = {hb, tb};
        end else begin : g_sim

            logic [15:0] lfsr;
            always_ff @(posedge clk) begin
                if (rst) lfsr <= 16'hACE1;
                else     lfsr <= {lfsr[14:0], lfsr[15] ^ lfsr[13] ^ lfsr[12] ^ lfsr[10]};
            end

            wire [15:0] ev = c[0] ? (c + 16'd1) : c;
            wire [15:0] od = c[0] ? c : (c + 16'd1);

            localparam integer NOISE_M = SIM_NOISE + 1;
            wire [31:0] noise_a32 = {24'd0, lfsr[7:0]}  % NOISE_M;
            wire [31:0] noise_b32 = {24'd0, lfsr[15:8]} % NOISE_M;
            wire [15:0] noise_a   = noise_a32[15:0];
            wire [15:0] noise_b   = noise_b32[15:0];

            logic [19:0] sa;
            logic [19:0] sb;
            always_ff @(posedge clk) begin
                if (st == M_SETTLE && wcnt == 16'd0) begin
                    sa <= {4'd0, ro_freq(CHIP_SEED, ev) + noise_a};
                    sb <= {4'd0, ro_freq(CHIP_SEED, od) + noise_b};
                end
            end
            assign cnt_a = sa;
            assign cnt_b = sb;

            wire unused_sim = win_en | clr;
        end
    endgenerate

endmodule
