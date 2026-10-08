module mlkem_ctrl #(
    parameter integer HELPER_MASK_LEN = 96
) (
    input  logic        clk,
    input  logic        rst,

    input  logic        cmd_start,
    input  logic [3:0]  cmd_sel,
    input  logic [2:0]  cmd_k,
    input  logic [7:0]  ctx_len,
    output logic        busy,
    output logic        done,
    output logic [3:0]  status,

    output logic [4:0]  puf_idx,
    input  logic [7:0]  puf_byte,

    input  logic [12:0] h_addr,
    input  logic        h_we,
    input  logic [7:0]  h_wdata,
    output logic [7:0]  h_rdata
);

    localparam [4:0] O_NOP   = 5'd0;
    localparam [4:0] O_END   = 5'd1;
    localparam [4:0] O_JMP   = 5'd2;
    localparam [4:0] O_CALL  = 5'd3;
    localparam [4:0] O_RET   = 5'd4;
    localparam [4:0] O_LOOPI = 5'd5;
    localparam [4:0] O_LOOPJ = 5'd6;
    localparam [4:0] O_SETR  = 5'd7;
    localparam [4:0] O_INCN  = 5'd8;
    localparam [4:0] O_BRF   = 5'd9;
    localparam [4:0] O_HINIT = 5'd10;
    localparam [4:0] O_HABS  = 5'd11;
    localparam [4:0] O_HABSC = 5'd12;
    localparam [4:0] O_HFIN  = 5'd13;
    localparam [4:0] O_HSQZ  = 5'd14;
    localparam [4:0] O_SNTT  = 5'd15;
    localparam [4:0] O_CBD   = 5'd16;
    localparam [4:0] O_DEC   = 5'd17;
    localparam [4:0] O_ENC   = 5'd18;
    localparam [4:0] O_NTT   = 5'd19;
    localparam [4:0] O_INTT  = 5'd20;
    localparam [4:0] O_PWM   = 5'd21;
    localparam [4:0] O_LIN   = 5'd22;
    localparam [4:0] O_CSEL  = 5'd23;
    localparam [4:0] O_WIPE  = 5'd24;

    localparam [2:0] S_BOOT  = 3'd0;
    localparam [2:0] S_IDLE  = 3'd1;
    localparam [2:0] S_FETCH = 3'd2;
    localparam [2:0] S_EXEC  = 3'd3;
    localparam [2:0] S_WAIT  = 3'd4;

    logic [2:0]  st;
    logic [8:0]  pc;
    logic [25:0] ir;
    logic [8:0]  rs0;
    logic [8:0]  rs1;
    logic [1:0]  sp;
    logic [2:0]  k_r;
    logic [1:0]  ri;
    logic [1:0]  rj;
    logic [3:0]  rn;
    logic        f_neq;
    logic        f_bad;
    logic        f_cmp;

    wire [4:0]  op  = ir[25:21];
    wire [6:0]  f1  = ir[20:14];
    wire [6:0]  f2  = ir[13:7];
    wire [6:0]  f3  = ir[6:0];
    wire [13:0] imm = ir[13:0];

    wire in_exec = (st == S_EXEC);
    wire in_wait = (st == S_WAIT);

    logic [12:0] rd_addr;
    logic [10:0] rd_left;
    logic        rd_vld;
    logic        rd_puf;
    wire  [7:0]  bm_rdata;
    wire  [7:0]  rd_data  = rd_puf ? puf_byte : bm_rdata;
    wire         rd_valid = rd_vld && (rd_left != 11'd0);
    logic        rd_ready;
    wire         rd_fire  = rd_valid && rd_ready;

    assign busy = (st != S_IDLE);

    wire [25:0] rom_instr;
    wire [8:0]  rom_entry;
    ucode_rom u_rom (.pc(pc), .instr(rom_instr), .sel(cmd_sel), .entry(rom_entry));

    wire        k_is2 = (k_r == 3'd2);
    wire        k_is4 = (k_r == 3'd4);
    wire [3:0]  p_du  = k_is4 ? 4'd11 : 4'd10;
    wire [3:0]  p_dv  = k_is4 ? 4'd5  : 4'd4;

    function automatic logic [3:0] slot_of(input logic [5:0] f, input logic [1:0] i, input logic [1:0] j);
        begin
            case (f[5:4])
                2'd1:    slot_of = f[3:0] + {2'd0, i};
                2'd2:    slot_of = f[3:0] + {2'd0, j};
                default: slot_of = f[3:0];
            endcase
        end
    endfunction

    wire [3:0] sl1 = slot_of(f1[5:0], ri, rj);
    wire [3:0] sl2 = slot_of(f2[5:0], ri, rj);
    wire [3:0] sl3 = slot_of(f3[5:0], ri, rj);

    logic [3:0] d_cur;
    always_comb begin
        case (f3[1:0])
            2'd0:    d_cur = 4'd12;
            2'd1:    d_cur = p_du;
            2'd2:    d_cur = p_dv;
            default: d_cur = 4'd1;
        endcase
    end

    wire [10:0] k384  = k_is2 ? 11'd768 : (k_is4 ? 11'd1536 : 11'd1152);
    wire [10:0] i384  = {1'b0, ri, 8'd0} + {2'b0, ri, 7'd0};
    wire [10:0] du32  = k_is4 ? 11'd352 : 11'd320;
    wire [10:0] idu32 = {1'b0, ri, 8'd0} + {3'b0, ri, 6'd0} + (k_is4 ? {4'b0, ri, 5'd0} : 11'd0);
    wire [10:0] kdu32 = k_is2 ? 11'd640 : (k_is4 ? 11'd1408 : 11'd960);
    wire [10:0] dv32  = k_is4 ? 11'd160 : 11'd128;

    function automatic logic [23:0] desc_of(
        input logic [4:0]  id,
        input logic [10:0] a_k384,
        input logic [10:0] a_i384,
        input logic [10:0] a_du32,
        input logic [10:0] a_idu32,
        input logic [10:0] a_kdu32,
        input logic [10:0] a_dv32,
        input logic [7:0]  a_ctx,
        input logic [10:0] a_hlen
    );
        logic [12:0] base;
        logic [10:0] len;
        begin
            base = 13'd0;
            len  = 11'd32;
            case (id)
                5'd0, 5'd1, 5'd2, 5'd3, 5'd4, 5'd5, 5'd6, 5'd7: begin
                    base = {5'b11000, id[2:0], 5'd0};
                end
                5'd8:  begin base = 13'h1900;                   len = {3'd0, a_ctx}; end
                5'd9:  begin base = {2'b00, a_k384};            len = 11'd32;        end
                5'd10: begin base = 13'h0000;                   len = a_k384 + 11'd32; end
                5'd11: begin base = {2'b00, a_i384};            len = 11'd384;       end
                5'd12: begin base = 13'h1000 + {2'b00, a_i384}; len = 11'd384;       end
                5'd13: begin base = 13'h0800 + {2'b00, a_idu32}; len = a_du32;       end
                5'd14: begin base = 13'h0800 + {2'b00, a_kdu32}; len = a_dv32;       end
                5'd16: begin base = 13'h1A00;                   len = a_hlen;        end
                5'd17: begin base = 13'h1A80;                   len = 11'd16;        end
                default: begin base = 13'd0;                    len = 11'd32;        end
            endcase
            desc_of = {base, len};
        end
    endfunction

    wire [31:0] hlen_w = HELPER_MASK_LEN;
    wire [23:0] dsc1 = desc_of(f1[4:0], k384, i384, du32, idu32, kdu32, dv32, ctx_len, hlen_w[10:0]);
    wire [23:0] dsc2 = desc_of(f2[4:0], k384, i384, du32, idu32, kdu32, dv32, ctx_len, hlen_w[10:0]);
    wire [12:0] dsc1_base = dsc1[23:11];
    wire [10:0] dsc1_len  = dsc1[10:0];
    wire [12:0] dsc2_base = dsc2[23:11];

    logic        kc_init;
    logic [1:0]  kc_mode;
    logic        kc_finish;
    wire         kc_triple;
    logic        kc_a_valid;
    logic [7:0]  kc_a_data;
    wire         kc_a_ready;
    wire         kc_q_valid;
    wire  [7:0]  kc_q_data;
    logic        kc_q_ready;
    wire         kc_t_valid;
    wire  [23:0] kc_t_data;
    wire         kc_t_ready;
    wire         kc_busy_unused;

    keccak_core u_keccak (
        .clk     (clk),
        .rst     (rst),
        .init    (kc_init),
        .mode    (kc_mode),
        .finish  (kc_finish),
        .triple  (kc_triple),
        .a_valid (kc_a_valid),
        .a_data  (kc_a_data),
        .a_ready (kc_a_ready),
        .q_valid (kc_q_valid),
        .q_data  (kc_q_data),
        .q_ready (kc_q_ready),
        .t_valid (kc_t_valid),
        .t_data  (kc_t_data),
        .t_ready (kc_t_ready),
        .busy    (kc_busy_unused)
    );

    wire        sm_start = in_exec && ((op == O_SNTT) || (op == O_CBD));
    wire [1:0]  sm_mode  = (op == O_SNTT) ? 2'd0 :
                           ((!f2[0] && k_is2) ? 2'd2 : 2'd1);
    wire        sm_done;
    wire        sm_q_ready;
    wire        sm_w_valid;
    wire [6:0]  sm_w_idx;
    wire [23:0] sm_w_data;
    wire        sm_busy_unused;

    sampler u_sampler (
        .clk     (clk),
        .rst     (rst),
        .start   (sm_start),
        .mode    (sm_mode),
        .busy    (sm_busy_unused),
        .done    (sm_done),
        .triple  (kc_triple),
        .q_valid (kc_q_valid),
        .q_data  (kc_q_data),
        .q_ready (sm_q_ready),
        .t_valid (kc_t_valid),
        .t_data  (kc_t_data),
        .t_ready (kc_t_ready),
        .w_valid (sm_w_valid),
        .w_idx   (sm_w_idx),
        .w_data  (sm_w_data)
    );

    wire        cd_start = in_exec && ((op == O_DEC) || (op == O_ENC));
    wire        cd_done;
    wire        cd_range_err;
    logic       cd_i_valid;
    wire        cd_i_ready;
    wire        cd_o_valid;
    wire [7:0]  cd_o_data;
    wire [6:0]  cd_m_idx;
    wire        cd_m_we;
    wire [23:0] cd_m_wdata;
    wire        cd_m_re_unused;
    wire        cd_busy_unused;

    wire [23:0] pa_rdata;
    wire [23:0] pb_rdata;

    codec u_codec (
        .clk       (clk),
        .rst       (rst),
        .start     (cd_start),
        .dir       (op == O_ENC),
        .d         (d_cur),
        .busy      (cd_busy_unused),
        .done      (cd_done),
        .range_err (cd_range_err),
        .i_valid   (cd_i_valid),
        .i_data    (rd_data),
        .i_ready   (cd_i_ready),
        .o_valid   (cd_o_valid),
        .o_data    (cd_o_data),
        .o_ready   (1'b1),
        .m_idx     (cd_m_idx),
        .m_we      (cd_m_we),
        .m_wdata   (cd_m_wdata),
        .m_re      (cd_m_re_unused),
        .m_rdata   (pa_rdata)
    );

    wire is_arith = (op == O_NTT) || (op == O_INTT) || (op == O_PWM) || (op == O_LIN);
    wire ar_start = in_exec && is_arith;
    wire [1:0] ar_op = (op == O_NTT) ? 2'd0 : (op == O_INTT) ? 2'd1 : (op == O_PWM) ? 2'd2 : 2'd3;

    logic pwm_acc;
    always_comb begin
        case ({f2[6], f1[6]})
            2'd0:    pwm_acc = 1'b0;
            2'd1:    pwm_acc = 1'b1;
            2'd2:    pwm_acc = (rj != 2'd0);
            default: pwm_acc = (ri != 2'd0);
        endcase
    end
    wire [1:0] ar_opt = (op == O_PWM) ? {1'b0, pwm_acc} : {f1[6], f2[6]};

    wire        ar_done;
    wire        ar_busy_unused;
    wire [10:0] ar_pa_addr;
    wire        ar_pa_we;
    wire [23:0] ar_pa_wdata;
    wire [10:0] ar_pb_addr;
    wire        ar_pb_we;
    wire [23:0] ar_pb_wdata;

    poly_arith u_arith (
        .clk      (clk),
        .rst      (rst),
        .start    (ar_start),
        .op       (ar_op),
        .s_dst    (sl1),
        .s_a      (sl2),
        .s_b      (sl3),
        .opt      (ar_opt),
        .busy     (ar_busy_unused),
        .done     (ar_done),
        .pa_addr  (ar_pa_addr),
        .pa_we    (ar_pa_we),
        .pa_wdata (ar_pa_wdata),
        .pa_rdata (pa_rdata),
        .pb_addr  (ar_pb_addr),
        .pb_we    (ar_pb_we),
        .pb_wdata (ar_pb_wdata),
        .pb_rdata (pb_rdata)
    );

    assign puf_idx = rd_addr[4:0];

    wire dec_tee = f3[2];
    wire dec_chk = f3[3];

    always_comb begin
        rd_ready   = 1'b0;
        cd_i_valid = 1'b0;
        if (in_wait && (op == O_HABS)) begin
            rd_ready = kc_a_ready;
        end else if (in_wait && (op == O_DEC)) begin
            rd_ready   = cd_i_ready && (!dec_tee || kc_a_ready);
            cd_i_valid = rd_valid && (!dec_tee || kc_a_ready);
        end
    end

    logic [7:0] const_byte;
    always_comb begin
        case (f1[2:0])
            3'd0:    const_byte = imm[7:0];
            3'd1:    const_byte = {6'd0, ri};
            3'd2:    const_byte = {6'd0, rj};
            3'd3:    const_byte = {4'd0, rn};
            3'd4:    const_byte = {5'd0, k_r};
            default: const_byte = ctx_len;
        endcase
    end

    always_comb begin
        kc_a_valid = 1'b0;
        kc_a_data  = rd_data;
        if (in_wait && (op == O_HABS)) begin
            kc_a_valid = rd_valid;
        end else if (in_wait && (op == O_DEC) && dec_tee) begin
            kc_a_valid = rd_valid && cd_i_ready;
        end else if (in_wait && (op == O_HABSC)) begin
            kc_a_valid = 1'b1;
            kc_a_data  = const_byte;
        end
    end

    wire wipe_keccak = in_exec && (op == O_WIPE) && imm[2];
    assign kc_init   = (in_exec && (op == O_HINIT)) || wipe_keccak;
    assign kc_mode   = f1[1:0];
    assign kc_finish = in_wait && (op == O_HFIN);

    logic [12:0] wr_addr;
    logic [10:0] wr_left;
    logic        cmp_pend;
    logic [7:0]  cmp_byte;
    logic        enc_fin;

    wire sqz_wait = in_wait && (op == O_HSQZ);
    wire enc_wait = in_wait && (op == O_ENC);

    wire wr_is_cmp = (op == O_HSQZ) ? f2[0] :
                     ((f3[3:2] == 2'd1) || ((f3[3:2] == 2'd2) && f_cmp));

    wire       sqz_ready = sqz_wait && (wr_left != 11'd0);
    wire       wr_fire   = (sqz_wait && kc_q_valid && sqz_ready) || (enc_wait && cd_o_valid);
    wire [7:0] wr_byte   = (op == O_HSQZ) ? kc_q_data : cd_o_data;

    assign kc_q_ready = sm_q_ready || sqz_ready;

    logic [5:0] cs_i;
    logic [1:0] cs_ph;
    logic [7:0] cs_a;
    wire csel_wait = in_wait && (op == O_CSEL);

    logic [13:0] wp;
    wire wipe_wait = in_wait && (op == O_WIPE);
    wire wipe_poly = wipe_wait && imm[0] && (wp < 14'd1024);
    wire wipe_sec  = wipe_wait && imm[1] && !imm[3] && (wp < 14'd192);
    wire wipe_all  = wipe_wait && imm[3] && (wp < 14'd8192);
    wire [13:0] wipe_last = imm[3] ? 14'd8191 : (imm[0] ? 14'd1023 : (imm[1] ? 14'd191 : 14'd0));

    logic [12:0] bm_addr;
    logic        bm_we;
    logic [7:0]  bm_wdata;

    always_comb begin
        bm_addr  = rd_fire ? (rd_addr + 13'd1) : rd_addr;
        bm_we    = 1'b0;
        bm_wdata = 8'd0;
        if (sqz_wait || enc_wait) begin
            bm_addr  = wr_addr;
            bm_we    = wr_fire && !wr_is_cmp;
            bm_wdata = wr_byte;
        end else if (csel_wait) begin
            case (cs_ph)
                2'd0:    bm_addr = dsc1_base + {7'd0, cs_i};
                2'd1:    bm_addr = dsc2_base + {7'd0, cs_i};
                default: begin
                    bm_addr  = dsc1_base + {7'd0, cs_i};
                    bm_we    = 1'b1;
                    bm_wdata = f_neq ? bm_rdata : cs_a;
                end
            endcase
        end else if (wipe_wait) begin
            bm_addr  = imm[3] ? wp[12:0] : (13'h1800 + wp[12:0]);
            bm_we    = wipe_sec || wipe_all;
            bm_wdata = 8'd0;
        end
    end

    ram_tdp #(.AW(13), .DW(8)) u_bmem (
        .clk     (clk),
        .a_addr  (bm_addr),
        .a_we    (bm_we),
        .a_wdata (bm_wdata),
        .a_rdata (bm_rdata),
        .b_addr  (h_addr),
        .b_we    (h_we),
        .b_wdata (h_wdata),
        .b_rdata (h_rdata)
    );

    logic [10:0] pa_addr;
    logic        pa_we;
    logic [23:0] pa_wdata;
    logic [10:0] pb_addr;
    logic        pb_we;
    logic [23:0] pb_wdata;

    always_comb begin
        pa_addr  = ar_pa_addr;
        pa_we    = ar_pa_we;
        pa_wdata = ar_pa_wdata;
        pb_addr  = ar_pb_addr;
        pb_we    = ar_pb_we;
        pb_wdata = ar_pb_wdata;
        if ((op == O_SNTT) || (op == O_CBD)) begin
            pa_addr  = {sl1, sm_w_idx};
            pa_we    = sm_w_valid;
            pa_wdata = sm_w_data;
            pb_we    = 1'b0;
        end else if ((op == O_DEC) || (op == O_ENC)) begin
            pa_addr  = {sl1, cd_m_idx};
            pa_we    = cd_m_we;
            pa_wdata = cd_m_wdata;
            pb_we    = 1'b0;
        end else if (op == O_WIPE) begin
            pa_addr  = {1'b0, wp[9:0]};
            pa_we    = wipe_poly;
            pa_wdata = 24'd0;
            pb_addr  = {1'b1, wp[9:0]};
            pb_we    = wipe_poly;
            pb_wdata = 24'd0;
        end
    end

    poly_mem u_pmem (
        .clk     (clk),
        .a_addr  (pa_addr),
        .a_we    (pa_we),
        .a_wdata (pa_wdata),
        .a_rdata (pa_rdata),
        .b_addr  (pb_addr),
        .b_we    (pb_we),
        .b_wdata (pb_wdata),
        .b_rdata (pb_rdata)
    );

    logic step_done;
    always_comb begin
        case (op)
            O_HABS:  step_done = (rd_left == 11'd0);
            O_HABSC: step_done = kc_a_ready;
            O_HFIN:  step_done = kc_a_ready;
            O_HSQZ:  step_done = (wr_left == 11'd0) && !cmp_pend;
            O_SNTT,
            O_CBD:   step_done = sm_done;
            O_DEC:   step_done = cd_done;
            O_ENC:   step_done = enc_fin && !cmp_pend;
            O_NTT,
            O_INTT,
            O_PWM,
            O_LIN:   step_done = ar_done;
            O_CSEL:  step_done = (cs_ph == 2'd2) && (cs_i == 6'd31);
            O_WIPE:  step_done = (wp == wipe_last);
            default: step_done = 1'b1;
        endcase
    end

    wire [2:0] ri_next = {1'b0, ri} + 3'd1;
    wire [2:0] rj_next = {1'b0, rj} + 3'd1;

    always_ff @(posedge clk) begin
        done <= 1'b0;

        if (rst) begin
            st       <= S_BOOT;
            pc       <= 9'd0;
            ir       <= 26'd0;
            rs0      <= 9'd0;
            rs1      <= 9'd0;
            sp       <= 2'd0;
            k_r      <= 3'd3;
            ri       <= 2'd0;
            rj       <= 2'd0;
            rn       <= 4'd0;
            f_neq    <= 1'b0;
            f_bad    <= 1'b0;
            f_cmp    <= 1'b0;
            status   <= 4'd0;
            rd_addr  <= 13'd0;
            rd_left  <= 11'd0;
            rd_vld   <= 1'b0;
            rd_puf   <= 1'b0;
            wr_addr  <= 13'd0;
            wr_left  <= 11'd0;
            cmp_pend <= 1'b0;
            cmp_byte <= 8'd0;
            enc_fin  <= 1'b0;
            cs_i     <= 6'd0;
            cs_ph    <= 2'd0;
            cs_a     <= 8'd0;
            wp       <= 14'd0;
        end else begin
            if (rd_fire) begin
                rd_addr <= rd_addr + 13'd1;
                rd_left <= rd_left - 11'd1;
            end
            if (in_wait) rd_vld <= 1'b1;

            cmp_pend <= wr_fire && wr_is_cmp;
            if (wr_fire) begin
                cmp_byte <= wr_byte;
                wr_addr  <= wr_addr + 13'd1;
                if (op == O_HSQZ) wr_left <= wr_left - 11'd1;
            end
            if (cmp_pend && (bm_rdata != cmp_byte)) f_neq <= 1'b1;
            if (enc_wait && cd_done) enc_fin <= 1'b1;

            case (st)
                S_BOOT: begin
                    pc    <= 9'd0;
                    sp    <= 2'd0;
                    st    <= S_FETCH;
                end

                S_IDLE: begin
                    if (cmd_start) begin
                        pc    <= rom_entry;
                        k_r   <= cmd_k;
                        sp    <= 2'd0;
                        ri    <= 2'd0;
                        rj    <= 2'd0;
                        rn    <= 4'd0;
                        f_neq <= 1'b0;
                        f_bad <= 1'b0;
                        f_cmp <= 1'b0;
                        st    <= S_FETCH;
                    end
                end

                S_FETCH: begin
                    ir <= rom_instr;
                    pc <= pc + 9'd1;
                    st <= S_EXEC;
                end

                S_EXEC: begin
                    st <= S_FETCH;
                    case (op)
                        O_END: begin
                            status <= imm[3:0];
                            done   <= 1'b1;
                            st     <= S_IDLE;
                        end
                        O_JMP: pc <= imm[8:0];
                        O_CALL: begin
                            if (sp == 2'd0) rs0 <= pc;
                            else            rs1 <= pc;
                            sp <= sp + 2'd1;
                            pc <= imm[8:0];
                        end
                        O_RET: begin
                            pc <= (sp == 2'd1) ? rs0 : rs1;
                            sp <= sp - 2'd1;
                        end
                        O_LOOPI: begin
                            ri <= ri + 2'd1;
                            if (ri_next != k_r) pc <= imm[8:0];
                        end
                        O_LOOPJ: begin
                            rj <= rj + 2'd1;
                            if (rj_next != k_r) pc <= imm[8:0];
                        end
                        O_SETR: begin
                            case (f1[2:0])
                                3'd0:    ri    <= imm[1:0];
                                3'd1:    rj    <= imm[1:0];
                                3'd2:    rn    <= imm[3:0];
                                3'd3:    f_neq <= imm[0];
                                3'd4:    f_cmp <= imm[0];
                                default: f_bad <= imm[0];
                            endcase
                        end
                        O_INCN: rn <= rn + 4'd1;
                        O_BRF: begin
                            if (f1[0] ? f_bad : f_neq) pc <= imm[8:0];
                        end
                        O_HABS: begin
                            rd_addr <= dsc1_base;
                            rd_left <= dsc1_len;
                            rd_vld  <= 1'b0;
                            rd_puf  <= (f1[4:0] == 5'd15);
                            st      <= S_WAIT;
                        end
                        O_HSQZ: begin
                            wr_addr <= dsc1_base;
                            wr_left <= dsc1_len;
                            st      <= S_WAIT;
                        end
                        O_DEC: begin
                            rd_addr <= dsc2_base;
                            rd_left <= {2'd0, d_cur, 5'd0};
                            rd_vld  <= 1'b0;
                            rd_puf  <= 1'b0;
                            st      <= S_WAIT;
                        end
                        O_ENC: begin
                            wr_addr <= dsc2_base;
                            enc_fin <= 1'b0;
                            st      <= S_WAIT;
                        end
                        O_CSEL: begin
                            cs_i  <= 6'd0;
                            cs_ph <= 2'd0;
                            st    <= S_WAIT;
                        end
                        O_WIPE: begin
                            wp <= 14'd0;
                            st <= S_WAIT;
                        end
                        O_HABSC, O_HFIN, O_SNTT, O_CBD, O_NTT, O_INTT, O_PWM, O_LIN: st <= S_WAIT;
                        default: ;
                    endcase
                end

                S_WAIT: begin
                    if ((op == O_DEC) && cd_done && dec_chk && cd_range_err) f_bad <= 1'b1;

                    if (op == O_CSEL) begin
                        if (cs_ph == 2'd1) cs_a <= bm_rdata;
                        if (cs_ph == 2'd2) begin
                            cs_ph <= 2'd0;
                            cs_i  <= cs_i + 6'd1;
                        end else begin
                            cs_ph <= cs_ph + 2'd1;
                        end
                    end
                    if (op == O_WIPE) wp <= wp + 14'd1;

                    if (step_done) begin
                        st     <= S_FETCH;
                        rd_vld <= 1'b0;
                        if (op == O_CSEL) cs_a <= 8'd0;
                    end
                end

                default: st <= S_BOOT;
            endcase
        end
    end

endmodule
