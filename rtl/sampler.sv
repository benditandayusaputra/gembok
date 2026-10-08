module sampler (
    input  logic        clk,
    input  logic        rst,

    input  logic        start,
    input  logic [1:0]  mode,
    output logic        busy,
    output logic        done,

    output logic        triple,
    input  logic        q_valid,
    input  logic [7:0]  q_data,
    output logic        q_ready,
    input  logic        t_valid,
    input  logic [23:0] t_data,
    output logic        t_ready,

    output logic        w_valid,
    output logic [6:0]  w_idx,
    output logic [23:0] w_data
);

    localparam [1:0] M_NTT  = 2'd0;
    localparam [1:0] M_CBD2 = 2'd1;
    localparam [1:0] M_CBD3 = 2'd2;

    logic        run;
    logic [1:0]  mode_r;
    logic [6:0]  idx;
    logic        hold_v;
    logic [11:0] hold_c;
    logic [7:0]  bbuf;
    logic [3:0]  bcnt;

    assign busy   = run;
    assign w_idx  = idx;
    assign triple = run && (mode_r == M_NTT);

    assign t_ready = run && (mode_r == M_NTT);
    assign q_ready = run && (mode_r != M_NTT);

    wire t_fire = t_valid && t_ready;
    wire q_fire = q_valid && q_ready;

    wire [11:0] d1 = t_data[11:0];
    wire [11:0] d2 = t_data[23:12];
    wire        a1 = (d1 < 12'd3329);
    wire        a2 = (d2 < 12'd3329);

    logic        sn_w;
    logic [23:0] sn_data;
    logic        sn_hold_v;
    logic [11:0] sn_hold_c;

    always_comb begin
        sn_w      = 1'b0;
        sn_data   = 24'd0;
        sn_hold_v = hold_v;
        sn_hold_c = hold_c;
        if (hold_v) begin
            if (a1) begin
                sn_w      = 1'b1;
                sn_data   = {d1, hold_c};
                sn_hold_v = a2;
                sn_hold_c = d2;
            end else if (a2) begin
                sn_w      = 1'b1;
                sn_data   = {d2, hold_c};
                sn_hold_v = 1'b0;
            end
        end else begin
            if (a1 && a2) begin
                sn_w    = 1'b1;
                sn_data = {d2, d1};
            end else if (a1) begin
                sn_hold_v = 1'b1;
                sn_hold_c = d1;
            end else if (a2) begin
                sn_hold_v = 1'b1;
                sn_hold_c = d2;
            end
        end
    end

    function automatic logic [11:0] cbd_coef(input logic [1:0] x, input logic [1:0] y);
        logic [1:0] df;
        begin
            if (x >= y) begin
                df       = x - y;
                cbd_coef = {10'd0, df};
            end else begin
                df       = y - x;
                cbd_coef = 12'd3329 - {10'd0, df};
            end
        end
    endfunction

    wire [1:0]  c2_x0 = {1'b0, q_data[0]} + {1'b0, q_data[1]};
    wire [1:0]  c2_y0 = {1'b0, q_data[2]} + {1'b0, q_data[3]};
    wire [1:0]  c2_x1 = {1'b0, q_data[4]} + {1'b0, q_data[5]};
    wire [1:0]  c2_y1 = {1'b0, q_data[6]} + {1'b0, q_data[7]};
    wire [23:0] c2_data = {cbd_coef(c2_x1, c2_y1), cbd_coef(c2_x0, c2_y0)};

    wire [15:0] c3_cat  = (bcnt == 4'd8) ? {q_data, bbuf} :
                          (bcnt == 4'd4) ? {4'd0, q_data, bbuf[3:0]} :
                                           {8'd0, q_data};
    wire        c3_emit = (bcnt != 4'd0);
    wire [1:0]  c3_x0 = {1'b0, c3_cat[0]} + {1'b0, c3_cat[1]} + {1'b0, c3_cat[2]};
    wire [1:0]  c3_y0 = {1'b0, c3_cat[3]} + {1'b0, c3_cat[4]} + {1'b0, c3_cat[5]};
    wire [1:0]  c3_x1 = {1'b0, c3_cat[6]} + {1'b0, c3_cat[7]} + {1'b0, c3_cat[8]};
    wire [1:0]  c3_y1 = {1'b0, c3_cat[9]} + {1'b0, c3_cat[10]} + {1'b0, c3_cat[11]};
    wire [23:0] c3_data = {cbd_coef(c3_x1, c3_y1), cbd_coef(c3_x0, c3_y0)};

    always_comb begin
        w_valid = 1'b0;
        w_data  = 24'd0;
        case (mode_r)
            M_NTT: begin
                w_valid = t_fire && sn_w;
                w_data  = sn_data;
            end
            M_CBD2: begin
                w_valid = q_fire;
                w_data  = c2_data;
            end
            default: begin
                w_valid = q_fire && c3_emit;
                w_data  = c3_data;
            end
        endcase
    end

    always_ff @(posedge clk) begin
        done <= 1'b0;
        if (rst) begin
            run    <= 1'b0;
            mode_r <= M_NTT;
            idx    <= 7'd0;
            hold_v <= 1'b0;
            hold_c <= 12'd0;
            bbuf   <= 8'd0;
            bcnt   <= 4'd0;
        end else if (!run) begin
            if (start) begin
                run    <= 1'b1;
                mode_r <= mode;
                idx    <= 7'd0;
                hold_v <= 1'b0;
                bcnt   <= 4'd0;
            end
        end else begin
            if (t_fire) begin
                hold_v <= sn_hold_v;
                hold_c <= sn_hold_c;
            end
            if (q_fire && (mode_r == M_CBD3)) begin
                if (bcnt == 4'd0) begin
                    bbuf <= q_data;
                    bcnt <= 4'd8;
                end else if (bcnt == 4'd8) begin
                    bbuf <= {4'd0, c3_cat[15:12]};
                    bcnt <= 4'd4;
                end else begin
                    bcnt <= 4'd0;
                end
            end
            if (w_valid) begin
                if (idx == 7'd127) begin
                    run    <= 1'b0;
                    done   <= 1'b1;
                    hold_v <= 1'b0;
                    hold_c <= 12'd0;
                    bbuf   <= 8'd0;
                    bcnt   <= 4'd0;
                end else begin
                    idx <= idx + 7'd1;
                end
            end
        end
    end

endmodule
