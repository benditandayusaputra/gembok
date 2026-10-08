module vault #(
    parameter integer PUF_MODE        = 0,
    parameter integer N_RO            = 768,
    parameter integer RO_STAGES       = 5,
    parameter integer WIN_LOG2        = 12,
    parameter integer VOTES           = 5,
    parameter [31:0]  COOLDOWN_CYCLES = 32'd1048576,
    parameter [255:0] DEV_KEY         = 256'h0,
    parameter [31:0]  CHIP_SEED       = 32'h0000_0001,
    parameter integer SIM_NOISE       = 0,
    parameter integer SIM_RO_HALF_PS  = 1500,
    parameter integer PUF_DEBUG       = 0
) (
    input  logic        clk,
    input  logic        rst,

    input  logic        cmd_valid,
    input  logic [3:0]  cmd_code,
    input  logic [2:0]  cmd_k,
    input  logic [7:0]  cmd_ctx_len,
    input  logic [15:0] puf_thresh,
    input  logic [9:0]  puf_dbg_idx,

    output logic        busy,
    output logic        done,
    output logic [3:0]  err,
    output logic        puf_ready,
    output logic        cooling,
    output logic        booted,
    output logic [31:0] cycles,
    output logic [31:0] cooldown_left,
    output logic [31:0] proofs,
    output logic [31:0] puf_dbg,
    output logic [31:0] puf_dbg1,

    output logic        core_start,
    output logic [3:0]  core_sel,
    output logic [2:0]  core_k,
    output logic [7:0]  core_ctx_len,
    input  logic        core_busy,
    input  logic        core_done,
    input  logic [3:0]  core_status,
    input  logic [4:0]  puf_idx,
    output logic [7:0]  puf_byte,

    input  logic [12:0] h_addr,
    input  logic        h_we,
    input  logic [7:0]  h_wdata,
    output logic [7:0]  h_rdata,
    output logic [12:0] m_addr,
    output logic        m_we,
    output logic [7:0]  m_wdata,
    input  logic [7:0]  m_rdata
);

    localparam [3:0] C_KEYGEN = 4'd1, C_ENCAPS = 4'd2, C_DECAPS = 4'd3, C_CHECK_EK = 4'd4,
                     C_CHECK_DK = 4'd5, C_ENROLL = 4'd6, C_PROVE = 4'd7, C_PUF_ENROLL = 4'd8,
                     C_PUF_RECON = 4'd9, C_PUF_MEASURE = 4'd10, C_WIPE = 4'd15;

    localparam [3:0] E_OK = 4'd0, E_BAD_PARAM = 4'd4, E_NO_KEY = 4'd5, E_RATE = 4'd6,
                     E_BAD_CMD = 4'd7, E_PUF_FAIL = 4'd8;

    localparam [3:0] P_WIPE_ALL = 4'd0, P_PUF_SEAL = 4'd8, P_PUF_OPEN = 4'd9;

    localparam [2:0] V_BOOT = 3'd0, V_IDLE = 3'd1, V_CORE = 3'd2, V_FUZZY = 3'd3,
                     V_DBG_STEP = 3'd4, V_DBG_MEAS = 3'd5;

    logic [2:0]   st;
    logic [3:0]   cur;
    logic [255:0] key;
    logic         grant;
    logic [31:0]  cooldown = 32'd0;
    logic [31:0]  cyc;
    logic [9:0]   dbg_n;
    logic         host_ok_q;

    wire k_ok = (cmd_k == 3'd2) || (cmd_k == 3'd3) || (cmd_k == 3'd4);

    assign busy          = (st != V_IDLE);
    assign cooling       = (cooldown != 32'd0);
    assign cooldown_left = cooldown;
    assign cycles        = cyc;
    assign puf_byte      = grant ? key[8*puf_idx +: 8] : 8'd0;

    logic        fz_start;
    logic        fz_mode;
    wire         fz_done;
    wire         fz_ok;
    wire  [6:0]  hm_addr;
    wire         hm_we;
    wire  [7:0]  hm_wdata;
    wire         key_shift;
    wire         key_bit;
    logic        dbg_restart;
    logic        dbg_next;
    logic        dbg_measure;
    wire         puf_done;
    wire  [19:0] puf_cnt_c;
    wire  [19:0] puf_cnt_c1;

    generate
        if (PUF_MODE == 2) begin : g_devkey
            assign fz_done    = 1'b0;
            assign fz_ok      = 1'b0;
            assign hm_addr    = 7'd0;
            assign hm_we      = 1'b0;
            assign hm_wdata   = 8'd0;
            assign key_shift  = 1'b0;
            assign key_bit    = 1'b0;
            assign puf_done   = 1'b0;
            assign puf_cnt_c  = 20'd0;
            assign puf_cnt_c1 = 20'd0;
            wire unused_dev = fz_start | fz_mode | dbg_restart | dbg_next | dbg_measure;
        end else begin : g_puf
            wire fz_restart, fz_next, fz_measure, fz_busy_unused, puf_busy_unused;
            wire puf_bit, puf_solid;

            puf_ro #(
                .N_RO      (N_RO),
                .STAGES    (RO_STAGES),
                .WIN_LOG2  (WIN_LOG2),
                .MODE      (PUF_MODE),
                .CHIP_SEED (CHIP_SEED),
                .SIM_NOISE (SIM_NOISE),
                .SIM_RO_HALF_PS (SIM_RO_HALF_PS)
            ) u_puf (
                .clk     (clk),
                .rst     (rst),
                .restart (fz_restart | dbg_restart),
                .next    (fz_next | dbg_next),
                .measure (fz_measure | dbg_measure),
                .thresh  (puf_thresh),
                .busy    (puf_busy_unused),
                .done    (puf_done),
                .bit_o   (puf_bit),
                .solid  (puf_solid),
                .cnt_c   (puf_cnt_c),
                .cnt_c1  (puf_cnt_c1)
            );

            fuzzy_extractor #(
                .N_CAND   (N_RO - 1),
                .KEY_BITS (256),
                .VOTES    (VOTES)
            ) u_fuzzy (
                .clk         (clk),
                .rst         (rst),
                .start       (fz_start),
                .mode        (fz_mode),
                .busy        (fz_busy_unused),
                .done        (fz_done),
                .ok          (fz_ok),
                .puf_restart (fz_restart),
                .puf_next    (fz_next),
                .puf_measure (fz_measure),
                .puf_done    (puf_done),
                .puf_bit     (puf_bit),
                .puf_solid  (puf_solid),
                .hm_addr     (hm_addr),
                .hm_we       (hm_we),
                .hm_wdata    (hm_wdata),
                .hm_rdata    (m_rdata),
                .key_shift   (key_shift),
                .key_bit     (key_bit)
            );
        end
    endgenerate

    wire host_ok   = (st == V_IDLE);
    wire fz_active = (st == V_FUZZY);

    assign m_addr  = fz_active ? (13'h1A00 + {6'd0, hm_addr}) : (host_ok ? h_addr : 13'd0);
    assign m_we    = fz_active ? hm_we : (host_ok && h_we);
    assign m_wdata = fz_active ? hm_wdata : h_wdata;
    assign h_rdata = (host_ok && host_ok_q) ? m_rdata : 8'd0;

    always_ff @(posedge clk) begin
        core_start  <= 1'b0;
        fz_start    <= 1'b0;
        dbg_restart <= 1'b0;
        dbg_next    <= 1'b0;
        dbg_measure <= 1'b0;
        host_ok_q   <= host_ok;

        if (rst) begin
            st        <= V_BOOT;
            cur       <= 4'd0;
            key       <= 256'd0;
            grant     <= 1'b0;
            cyc       <= 32'd0;
            dbg_n     <= 10'd0;
            done      <= 1'b0;
            err       <= E_OK;
            puf_ready <= 1'b0;
            booted    <= 1'b0;
            proofs    <= 32'd0;
            puf_dbg   <= 32'd0;
            puf_dbg1  <= 32'd0;
            core_sel  <= 4'd0;
            core_k    <= 3'd3;
            core_ctx_len <= 8'd0;
            fz_mode   <= 1'b0;
            host_ok_q <= 1'b0;
        end else begin
            if (cooldown != 32'd0) cooldown <= cooldown - 32'd1;
            if (st != V_IDLE)      cyc <= cyc + 32'd1;
            if (key_shift)         key <= {key_bit, key[255:1]};

            case (st)
                V_BOOT: begin
                    if (!core_busy) begin
                        booted <= 1'b1;
                        st     <= V_IDLE;
                    end
                end

                V_IDLE: begin
                    if (cmd_valid) begin
                        done   <= 1'b0;
                        err    <= E_OK;
                        cyc    <= 32'd1;
                        cur          <= cmd_code;
                        core_k       <= cmd_k;
                        core_ctx_len <= cmd_ctx_len;
                        grant  <= 1'b0;
                        case (cmd_code)
                            C_KEYGEN, C_ENCAPS, C_DECAPS, C_CHECK_EK, C_CHECK_DK: begin
                                if (!k_ok) begin
                                    err  <= E_BAD_PARAM;
                                    done <= 1'b1;
                                end else begin
                                    core_sel   <= cmd_code;
                                    core_start <= 1'b1;
                                    st         <= V_CORE;
                                end
                            end
                            C_ENROLL, C_PROVE: begin
                                if (!k_ok) begin
                                    err  <= E_BAD_PARAM;
                                    done <= 1'b1;
                                end else if (!puf_ready) begin
                                    err  <= E_NO_KEY;
                                    done <= 1'b1;
                                end else if ((cmd_code == C_PROVE) && cooling) begin
                                    err  <= E_RATE;
                                    done <= 1'b1;
                                end else begin
                                    core_sel   <= cmd_code;
                                    core_start <= 1'b1;
                                    grant      <= 1'b1;
                                    st         <= V_CORE;
                                end
                            end
                            C_PUF_ENROLL, C_PUF_RECON: begin
                                puf_ready <= 1'b0;
                                if (PUF_MODE == 2) begin
                                    key        <= DEV_KEY;
                                    core_sel   <= (cmd_code == C_PUF_ENROLL) ? P_PUF_SEAL : P_PUF_OPEN;
                                    core_start <= 1'b1;
                                    grant      <= 1'b1;
                                    st         <= V_CORE;
                                end else begin
                                    key      <= 256'd0;
                                    fz_mode  <= (cmd_code == C_PUF_RECON);
                                    fz_start <= 1'b1;
                                    st       <= V_FUZZY;
                                end
                            end
                            C_PUF_MEASURE: begin
                                if ((PUF_DEBUG != 0) && (PUF_MODE != 2)) begin
                                    dbg_restart <= 1'b1;
                                    dbg_n       <= puf_dbg_idx;
                                    st          <= V_DBG_STEP;
                                end else begin
                                    err  <= E_BAD_CMD;
                                    done <= 1'b1;
                                end
                            end
                            C_WIPE: begin
                                key        <= 256'd0;
                                puf_ready  <= 1'b0;
                                core_sel   <= P_WIPE_ALL;
                                core_start <= 1'b1;
                                st         <= V_CORE;
                            end
                            default: begin
                                err  <= E_BAD_CMD;
                                done <= 1'b1;
                            end
                        endcase
                    end
                end

                V_CORE: begin
                    if (core_done) begin
                        grant <= 1'b0;
                        err   <= core_status;
                        done  <= 1'b1;
                        st    <= V_IDLE;
                        case (cur)
                            C_PROVE: begin
                                cooldown <= COOLDOWN_CYCLES;
                                proofs   <= proofs + 32'd1;
                            end
                            C_PUF_ENROLL: puf_ready <= 1'b1;
                            C_PUF_RECON: begin
                                if (core_status == 4'd0) puf_ready <= 1'b1;
                                else                     key <= 256'd0;
                            end
                            default: ;
                        endcase
                    end
                end

                V_FUZZY: begin
                    if (fz_done) begin
                        if (fz_ok) begin
                            core_sel   <= (cur == C_PUF_ENROLL) ? P_PUF_SEAL : P_PUF_OPEN;
                            core_start <= 1'b1;
                            grant      <= 1'b1;
                            st         <= V_CORE;
                        end else begin
                            key  <= 256'd0;
                            err  <= E_PUF_FAIL;
                            done <= 1'b1;
                            st   <= V_IDLE;
                        end
                    end
                end

                V_DBG_STEP: begin
                    if (dbg_n == 10'd0) begin
                        if (!dbg_restart && !dbg_next) begin
                            dbg_measure <= 1'b1;
                            st          <= V_DBG_MEAS;
                        end
                    end else if (!dbg_restart) begin
                        dbg_next <= 1'b1;
                        dbg_n    <= dbg_n - 10'd1;
                    end
                end

                V_DBG_MEAS: begin
                    if (puf_done) begin
                        puf_dbg  <= {12'd0, puf_cnt_c};
                        puf_dbg1 <= {12'd0, puf_cnt_c1};
                        done    <= 1'b1;
                        st      <= V_IDLE;
                    end
                end

                default: st <= V_BOOT;
            endcase
        end
    end

endmodule
