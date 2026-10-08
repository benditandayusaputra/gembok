module poly_arith (
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

    output logic [10:0] pa_addr,
    output logic        pa_we,
    output logic [23:0] pa_wdata,
    input  logic [23:0] pa_rdata,

    output logic [10:0] pb_addr,
    output logic        pb_we,
    output logic [23:0] pb_wdata,
    input  logic [23:0] pb_rdata
);

    localparam [1:0]  OP_NTT  = 2'd0;
    localparam [1:0]  OP_INTT = 2'd1;
    localparam [1:0]  OP_PWM  = 2'd2;
    localparam [1:0]  OP_LIN  = 2'd3;
    localparam [11:0] Q       = 12'd3329;
    localparam [11:0] INV128  = 12'd3303;

    function automatic logic [11:0] madd(input logic [11:0] a, input logic [11:0] b);
        logic [12:0] s;
        logic [12:0] d;
        begin
            s    = {1'b0, a} + {1'b0, b};
            d    = s - 13'd3329;
            madd = d[12] ? s[11:0] : d[11:0];
        end
    endfunction

    function automatic logic [11:0] msub(input logic [11:0] a, input logic [11:0] b);
        logic [12:0] d;
        begin
            d    = {1'b0, a} - {1'b0, b};
            msub = d[12] ? (d[11:0] + 12'd3329) : d[11:0];
        end
    endfunction

    logic        run;
    logic        issuing;
    logic [1:0]  op_r;
    logic [3:0]  sd;
    logic [3:0]  sa;
    logic [3:0]  sb;
    logic [1:0]  opt_r;
    logic [2:0]  lvl;
    logic [6:0]  n;
    logic        ph2;
    logic [1:0]  ph4;
    logic [7:1]  v;
    logic [9:1]  pv;
    logic [76:0] m_wa;
    logic [76:0] m_wb;

    wire is_pwm = (op_r == OP_PWM);
    wire is_lin = (op_r == OP_LIN);
    wire gs     = (op_r == OP_INTT);

    wire issue2 = run && issuing && !is_pwm && !ph2 && !((n == 7'd0) && (v != 7'd0));
    wire issue4 = run && issuing &&  is_pwm && (ph4 == 2'd0);

    assign busy = run;

    logic [6:0] wa2;
    logic [6:0] wb2;
    logic [5:0] zblk;
    logic [6:0] zidx2;

    always_comb begin
        case (lvl)
            3'd0:    begin wa2 = {1'b0, n[5:0]};          wb2 = wa2 | 7'd64; zblk = 6'd0;              end
            3'd1:    begin wa2 = {n[5], 1'b0, n[4:0]};    wb2 = wa2 | 7'd32; zblk = {5'd0, n[5]};      end
            3'd2:    begin wa2 = {n[5:4], 1'b0, n[3:0]};  wb2 = wa2 | 7'd16; zblk = {4'd0, n[5:4]};    end
            3'd3:    begin wa2 = {n[5:3], 1'b0, n[2:0]};  wb2 = wa2 | 7'd8;  zblk = {3'd0, n[5:3]};    end
            3'd4:    begin wa2 = {n[5:2], 1'b0, n[1:0]};  wb2 = wa2 | 7'd4;  zblk = {2'd0, n[5:2]};    end
            3'd5:    begin wa2 = {n[5:1], 1'b0, n[0]};    wb2 = wa2 | 7'd2;  zblk = {1'd0, n[5:1]};    end
            default: begin wa2 = {n[5:0], 1'b0};          wb2 = wa2 | 7'd1;  zblk = n[5:0];            end
        endcase
    end

    always_comb begin
        case (lvl)
            3'd0:    zidx2 = 7'd1;
            3'd1:    zidx2 = {5'd0, 1'b1, gs ? ~zblk[0]   : zblk[0]};
            3'd2:    zidx2 = {4'd0, 1'b1, gs ? ~zblk[1:0] : zblk[1:0]};
            3'd3:    zidx2 = {3'd0, 1'b1, gs ? ~zblk[2:0] : zblk[2:0]};
            3'd4:    zidx2 = {2'd0, 1'b1, gs ? ~zblk[3:0] : zblk[3:0]};
            3'd5:    zidx2 = {1'd0, 1'b1, gs ? ~zblk[4:0] : zblk[4:0]};
            default: zidx2 = {1'b1, gs ? ~zblk[5:0] : zblk[5:0]};
        endcase
    end

    logic [6:0]  n_q;
    wire  [6:0]  zeta_idx = is_pwm ? {1'b1, n_q[6:1]} : zidx2;
    wire  [11:0] zeta;

    zeta_rom u_zeta (.idx(zeta_idx), .z(zeta));

    logic [11:0] mm_x;
    logic [11:0] mm_y;
    wire  [11:0] mm_t;
    wire  [11:0] mm_tr;

    modmul u_mm (.clk(clk), .x(mm_x), .y(mm_y), .t(mm_t), .t_r(mm_tr));

    logic [11:0] w_hold;
    logic [11:0] a1_h;
    logic [11:0] b1_h;
    logic [11:0] aa1, aa2, aa3, aa4;
    logic [11:0] oa_r, ob_r, oa_h, ob_h;

    wire [11:0] opa   = v[1] ? pa_rdata[11:0] : a1_h;
    wire [11:0] opb   = v[1] ? pb_rdata[11:0] : b1_h;
    wire [11:0] pre_x = gs ? msub(opb, opa) : opb;
    wire [11:0] pre_a = gs ? madd(opa, opb) : opa;
    wire [11:0] oa_c  = gs ? aa4   : madd(aa4, mm_tr);
    wire [11:0] ob_c  = gs ? mm_tr : msub(aa4, mm_tr);

    logic [6:0]  nw;
    logic [11:0] ha0, ha1, hb0, hb1;
    logic [11:0] acc0_h, acc1_h;
    logic [11:0] gam_h;
    logic [11:0] r0, r1;

    always_comb begin
        mm_x = pre_x;
        mm_y = w_hold;
        if (is_pwm) begin
            if (pv[1]) begin
                mm_x = pa_rdata[23:12];
                mm_y = pb_rdata[23:12];
            end else if (pv[2]) begin
                mm_x = ha0;
                mm_y = hb0;
            end else if (pv[3]) begin
                mm_x = madd(ha0, ha1);
                mm_y = madd(hb0, hb1);
            end else begin
                mm_x = mm_t;
                mm_y = gam_h;
            end
        end
    end

    always_comb begin
        pa_addr  = 11'd0;
        pa_we    = 1'b0;
        pa_wdata = 24'd0;
        pb_addr  = 11'd0;
        pb_we    = 1'b0;
        pb_wdata = 24'd0;
        if (is_pwm) begin
            if (issue4) begin
                pa_addr = {sa, n};
                pb_addr = {sb, n};
            end
            if (pv[1]) begin
                pa_addr = {sd, n_q};
            end
            if (pv[9]) begin
                pb_addr  = {sd, nw};
                pb_we    = 1'b1;
                pb_wdata = {r1, r0};
            end
        end else begin
            if (issue2) begin
                pa_addr = is_lin ? {sb, n} : {sd, wa2};
                pb_addr = is_lin ? {sa, n} : {sd, wb2};
            end
            if (v[7]) begin
                pa_addr  = m_wa[76:66];
                pa_we    = 1'b1;
                pa_wdata = (is_lin && opt_r[0]) ? {ob_r, ob_h} : {oa_r, oa_h};
                if (!is_lin) begin
                    pb_addr  = m_wb[76:66];
                    pb_we    = 1'b1;
                    pb_wdata = {ob_r, ob_h};
                end
            end
        end
    end

    always_ff @(posedge clk) begin
        done <= 1'b0;

        if (rst) begin
            run     <= 1'b0;
            issuing <= 1'b0;
            op_r    <= OP_NTT;
            sd      <= 4'd0;
            sa      <= 4'd0;
            sb      <= 4'd0;
            opt_r   <= 2'd0;
            lvl     <= 3'd0;
            n       <= 7'd0;
            ph2     <= 1'b0;
            ph4     <= 2'd0;
            v       <= 7'd0;
            pv      <= 9'd0;
            m_wa    <= 77'd0;
            m_wb    <= 77'd0;
            n_q     <= 7'd0;
            nw      <= 7'd0;
        end else begin
            v    <= {v[6:1], issue2};
            pv   <= {pv[8:1], issue4};
            m_wa <= {m_wa[65:0], (is_lin ? {sd, n} : {sd, wa2})};
            m_wb <= {m_wb[65:0], {sd, wb2}};

            if (!run) begin
                if (start) begin
                    run     <= 1'b1;
                    issuing <= 1'b1;
                    op_r    <= op;
                    sd      <= s_dst;
                    sa      <= s_a;
                    sb      <= s_b;
                    opt_r   <= opt;
                    n       <= 7'd0;
                    lvl     <= (op == OP_INTT) ? 3'd6 : 3'd0;
                    ph2     <= 1'b0;
                    ph4     <= 2'd0;
                    nw      <= 7'd0;
                end
            end else begin
                ph2 <= ~ph2;
                ph4 <= ph4 + 2'd1;

                if (issue2) begin
                    if (is_lin) begin
                        if (n == 7'd127) issuing <= 1'b0;
                        else             n <= n + 7'd1;
                    end else if (n == 7'd63) begin
                        n <= 7'd0;
                        if (gs) begin
                            if (lvl == 3'd0) issuing <= 1'b0;
                            else             lvl <= lvl - 3'd1;
                        end else begin
                            if (lvl == 3'd6) issuing <= 1'b0;
                            else             lvl <= lvl + 3'd1;
                        end
                    end else begin
                        n <= n + 7'd1;
                    end
                end

                if (issue4) begin
                    n_q <= n;
                    if (n == 7'd127) issuing <= 1'b0;
                    else             n <= n + 7'd1;
                end

                if (pv[9]) nw <= nw + 7'd1;

                if (!issuing && (v == 7'd0) && (pv == 9'd0)) begin
                    run  <= 1'b0;
                    done <= 1'b1;
                end
            end
        end
    end

    always_ff @(posedge clk) begin
        if (issue2) w_hold <= is_lin ? (opt_r[1] ? INV128 : 12'd1) : zeta;
        if (v[1]) begin
            a1_h <= pa_rdata[23:12];
            b1_h <= pb_rdata[23:12];
        end
        aa1  <= pre_a;
        aa2  <= aa1;
        aa3  <= aa2;
        aa4  <= aa3;
        oa_r <= oa_c;
        ob_r <= ob_c;
        oa_h <= oa_r;
        ob_h <= ob_r;

        if (pv[1]) begin
            ha0 <= pa_rdata[11:0];
            ha1 <= pa_rdata[23:12];
            hb0 <= pb_rdata[11:0];
            hb1 <= pb_rdata[23:12];
        end
        if (pv[2]) begin
            acc0_h <= opt_r[0] ? pa_rdata[11:0]  : 12'd0;
            acc1_h <= opt_r[0] ? pa_rdata[23:12] : 12'd0;
        end
        if (pv[3]) gam_h <= n_q[0] ? (Q - zeta) : zeta;

        if (pv[5])      r1 <= msub(acc1_h, mm_tr);
        else if (pv[6]) r1 <= msub(r1, mm_tr);
        else if (pv[7]) r1 <= madd(r1, mm_tr);

        if (pv[6])      r0 <= madd(acc0_h, mm_tr);
        else if (pv[8]) r0 <= madd(r0, mm_tr);
    end

endmodule
