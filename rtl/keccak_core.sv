module keccak_core (
    input  logic        clk,
    input  logic        rst,

    input  logic        init,
    input  logic [1:0]  mode,
    input  logic        finish,
    input  logic        triple,

    input  logic        a_valid,
    input  logic [7:0]  a_data,
    output logic        a_ready,

    output logic        q_valid,
    output logic [7:0]  q_data,
    input  logic        q_ready,

    output logic        t_valid,
    output logic [23:0] t_data,
    input  logic        t_ready,

    output logic        busy
);

    localparam [2:0] PH_IDLE  = 3'd0;
    localparam [2:0] PH_ABS   = 3'd1;
    localparam [2:0] PH_PRM_A = 3'd2;
    localparam [2:0] PH_PRM_F = 3'd3;
    localparam [2:0] PH_SQZ   = 3'd4;
    localparam [2:0] PH_PRM_S = 3'd5;

    logic [2:0]    ph;
    logic [1599:0] s;
    logic [4:0]    rnd;
    logic [7:0]    ptr;
    logic [7:0]    rate;
    logic [7:0]    sfx;
    logic [5:0]    grp;
    logic [7:0]    left;
    logic [23:0]   hold;
    logic [1:0]    hcnt;

    function automatic logic [63:0] round_const(input logic [4:0] r);
        case (r)
            5'd0:  round_const = 64'h0000000000000001;
            5'd1:  round_const = 64'h0000000000008082;
            5'd2:  round_const = 64'h800000000000808A;
            5'd3:  round_const = 64'h8000000080008000;
            5'd4:  round_const = 64'h000000000000808B;
            5'd5:  round_const = 64'h0000000080000001;
            5'd6:  round_const = 64'h8000000080008081;
            5'd7:  round_const = 64'h8000000000008009;
            5'd8:  round_const = 64'h000000000000008A;
            5'd9:  round_const = 64'h0000000000000088;
            5'd10: round_const = 64'h0000000080008009;
            5'd11: round_const = 64'h000000008000000A;
            5'd12: round_const = 64'h000000008000808B;
            5'd13: round_const = 64'h800000000000008B;
            5'd14: round_const = 64'h8000000000008089;
            5'd15: round_const = 64'h8000000000008003;
            5'd16: round_const = 64'h8000000000008002;
            5'd17: round_const = 64'h8000000000000080;
            5'd18: round_const = 64'h000000000000800A;
            5'd19: round_const = 64'h800000008000000A;
            5'd20: round_const = 64'h8000000080008081;
            5'd21: round_const = 64'h8000000000008080;
            5'd22: round_const = 64'h0000000080000001;
            5'd23: round_const = 64'h8000000080008008;
            default: round_const = 64'h0;
        endcase
    endfunction

    logic [1599:0] s_round;
    keccak_round u_round (
        .a  (s),
        .rc (round_const(rnd)),
        .o  (s_round)
    );

    assign a_ready = (ph == PH_ABS);

    wire abs_fire = (ph == PH_ABS) && a_valid && !finish;
    wire fin_fire = (ph == PH_ABS) && finish;
    wire [7:0] xbyte = fin_fire ? sfx : a_data;
    wire       xen   = abs_fire || fin_fire;

    wire  [1599:0] s_abs;
    genvar bi;
    generate
        for (bi = 0; bi < 200; bi = bi + 1) begin : g_abs
            if (bi == 71 || bi == 135 || bi == 167) begin : g_last
                wire       hit  = xen && (ptr == bi);
                wire       tail = fin_fire && (rate == (bi + 1));
                assign s_abs[8*bi +: 8] = s[8*bi +: 8] ^ (hit ? xbyte : 8'h00) ^ (tail ? 8'h80 : 8'h00);
            end else if (bi < 168) begin : g_rate
                wire hit = xen && (ptr == bi);
                assign s_abs[8*bi +: 8] = s[8*bi +: 8] ^ (hit ? xbyte : 8'h00);
            end else begin : g_cap
                assign s_abs[8*bi +: 8] = s[8*bi +: 8];
            end
        end
    endgenerate

    wire [23:0] grp_mux [0:63];
    genvar gi;
    generate
        for (gi = 0; gi < 64; gi = gi + 1) begin : g_grp
            if (gi < 56) begin : g_in
                assign grp_mux[gi] = s[24*gi +: 24];
            end else begin : g_out
                assign grp_mux[gi] = 24'h0;
            end
        end
    endgenerate
    wire [23:0] grp_now = grp_mux[grp];

    assign q_valid = (ph == PH_SQZ) && !triple && (hcnt != 2'd0);
    assign q_data  = hold[7:0];
    assign t_valid = (ph == PH_SQZ) && triple && (hcnt == 2'd3);
    assign t_data  = hold;

    wire q_fire = q_valid && q_ready;
    wire t_fire = t_valid && t_ready;
    wire [1:0] hcnt_after = t_fire ? 2'd0 : (q_fire ? (hcnt - 2'd1) : hcnt);
    wire [1:0] take       = (left >= 8'd3) ? 2'd3 : left[1:0];

    assign busy = (ph == PH_PRM_A) || (ph == PH_PRM_F) || (ph == PH_PRM_S);

    always_ff @(posedge clk) begin
        if (rst) begin
            ph   <= PH_IDLE;
            s    <= 1600'h0;
            rnd  <= 5'd0;
            ptr  <= 8'd0;
            rate <= 8'd136;
            sfx  <= 8'h06;
            grp  <= 6'd0;
            left <= 8'd0;
            hold <= 24'h0;
            hcnt <= 2'd0;
        end else if (init) begin
            ph   <= PH_ABS;
            s    <= 1600'h0;
            rnd  <= 5'd0;
            ptr  <= 8'd0;
            grp  <= 6'd0;
            left <= 8'd0;
            hold <= 24'h0;
            hcnt <= 2'd0;
            case (mode)
                2'd0:    begin rate <= 8'd136; sfx <= 8'h06; end
                2'd1:    begin rate <= 8'd72;  sfx <= 8'h06; end
                2'd2:    begin rate <= 8'd168; sfx <= 8'h1F; end
                default: begin rate <= 8'd136; sfx <= 8'h1F; end
            endcase
        end else begin
            case (ph)
                PH_ABS: begin
                    if (fin_fire) begin
                        s   <= s_abs;
                        ph  <= PH_PRM_F;
                        rnd <= 5'd0;
                    end else if (abs_fire) begin
                        s <= s_abs;
                        if (ptr == rate - 8'd1) begin
                            ptr <= 8'd0;
                            ph  <= PH_PRM_A;
                            rnd <= 5'd0;
                        end else begin
                            ptr <= ptr + 8'd1;
                        end
                    end
                end

                PH_PRM_A, PH_PRM_F, PH_PRM_S: begin
                    s   <= s_round;
                    rnd <= rnd + 5'd1;
                    if (rnd == 5'd23) begin
                        if (ph == PH_PRM_A) begin
                            ph <= PH_ABS;
                        end else begin
                            ph   <= PH_SQZ;
                            grp  <= 6'd0;
                            left <= rate;
                            hcnt <= 2'd0;
                        end
                    end
                end

                PH_SQZ: begin
                    if (hcnt_after == 2'd0) begin
                        if (left != 8'd0) begin
                            hold <= grp_now;
                            hcnt <= take;
                            left <= left - {6'd0, take};
                            grp  <= grp + 6'd1;
                        end else begin
                            hcnt <= 2'd0;
                            ph   <= PH_PRM_S;
                            rnd  <= 5'd0;
                        end
                    end else begin
                        hcnt <= hcnt_after;
                        if (q_fire) hold <= {8'h00, hold[23:8]};
                    end
                end

                default: ;
            endcase
        end
    end

endmodule
