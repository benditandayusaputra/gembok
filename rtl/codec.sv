module codec (
    input  logic        clk,
    input  logic        rst,

    input  logic        start,
    input  logic        dir,
    input  logic [3:0]  d,
    output logic        busy,
    output logic        done,
    output logic        range_err,

    input  logic        i_valid,
    input  logic [7:0]  i_data,
    output logic        i_ready,

    output logic        o_valid,
    output logic [7:0]  o_data,
    input  logic        o_ready,

    output logic [6:0]  m_idx,
    output logic        m_we,
    output logic [23:0] m_wdata,
    output logic        m_re,
    input  logic [23:0] m_rdata
);

    logic       run;
    logic       dir_r;
    logic [3:0] d_r;

    wire [5:0] two_d = {1'b0, d_r, 1'b0};

    assign busy = run;

    logic [31:0] dbuf;
    logic [5:0]  dcnt;
    logic [8:0]  d_bytes_left;
    logic [7:0]  d_pairs;
    logic        p_v;
    logic [11:0] p_v0;
    logic [11:0] p_v1;
    logic [6:0]  d_widx;

    wire        dec_run = run && !dir_r;
    wire        d_emit  = dec_run && (dcnt >= two_d) && (d_pairs != 8'd128);
    wire [5:0]  dcnt1   = d_emit ? (dcnt - two_d) : dcnt;
    wire [31:0] dbuf1   = d_emit ? (dbuf >> two_d) : dbuf;

    assign i_ready = dec_run && (d_bytes_left != 9'd0) && (dcnt1 <= 6'd24);
    wire        d_take  = i_valid && i_ready;
    wire [31:0] dbuf2   = d_take ? (dbuf1 | ({24'd0, i_data} << dcnt1)) : dbuf1;
    wire [5:0]  dcnt2   = d_take ? (dcnt1 + 6'd8) : dcnt1;

    logic [11:0] x_v0;
    logic [11:0] x_v1;
    always_comb begin
        case (d_r)
            4'd1:    begin x_v0 = {11'd0, dbuf[0]};   x_v1 = {11'd0, dbuf[1]};     end
            4'd4:    begin x_v0 = {8'd0, dbuf[3:0]};  x_v1 = {8'd0, dbuf[7:4]};    end
            4'd5:    begin x_v0 = {7'd0, dbuf[4:0]};  x_v1 = {7'd0, dbuf[9:5]};    end
            4'd10:   begin x_v0 = {2'd0, dbuf[9:0]};  x_v1 = {2'd0, dbuf[19:10]};  end
            4'd11:   begin x_v0 = {1'd0, dbuf[10:0]}; x_v1 = {1'd0, dbuf[21:11]};  end
            default: begin x_v0 = dbuf[11:0];         x_v1 = dbuf[23:12];          end
        endcase
    end

    function automatic logic [11:0] decomp(input logic [11:0] y, input logic [3:0] dd);
        logic [23:0] qy;
        logic [23:0] s;
        begin
            qy = {1'b0, y, 11'd0} + {2'b0, y, 10'd0} + {4'b0, y, 8'd0} + {12'd0, y};
            case (dd)
                4'd1:    begin s = qy + 24'd1;    decomp = s[12:1];  end
                4'd4:    begin s = qy + 24'd8;    decomp = s[15:4];  end
                4'd5:    begin s = qy + 24'd16;   decomp = s[16:5];  end
                4'd10:   begin s = qy + 24'd512;  decomp = s[21:10]; end
                4'd11:   begin s = qy + 24'd1024; decomp = s[22:11]; end
                default: begin s = 24'd0;         decomp = (y >= 12'd3329) ? (y - 12'd3329) : y; end
            endcase
        end
    endfunction

    wire d_over = (d_r == 4'd12) && ((p_v0 >= 12'd3329) || (p_v1 >= 12'd3329));

    logic [7:0]  e_ridx;
    logic        rd_v;
    logic [11:0] wlo;
    logic [11:0] whi;
    logic        lo_v;
    logic        hi_v;
    logic [5:0]  commit;
    logic [47:0] obuf;
    logic [5:0]  ocnt;
    logic [8:0]  e_bytes;
    logic        s1_v;
    logic [11:0] s1_x;
    logic        s2_v;
    logic [44:0] s2_p;
    logic [11:0] s2_x;

    wire enc_run = run && dir_r;

    wire        src_v = rd_v || lo_v || hi_v;
    wire [11:0] src_x = rd_v ? m_rdata[11:0] : (lo_v ? wlo : whi);
    wire        e_can = ({1'b0, commit} + {3'd0, d_r} <= 7'd48);
    wire        e_issue = enc_run && src_v && e_can;

    wire        e_empty_next = !rd_v && !lo_v && (!hi_v || e_issue);
    assign      m_re = enc_run && (e_ridx != 8'd128) && e_empty_next;

    logic [22:0] e_n;
    always_comb begin
        case (d_r)
            4'd1:    e_n = {10'd0, s1_x, 1'd0}  + 23'd1664;
            4'd4:    e_n = {7'd0,  s1_x, 4'd0}  + 23'd1664;
            4'd5:    e_n = {6'd0,  s1_x, 5'd0}  + 23'd1664;
            4'd10:   e_n = {1'd0,  s1_x, 10'd0} + 23'd1664;
            default: e_n = {s1_x, 11'd0}        + 23'd1664;
        endcase
    end

    wire [11:0] e_quot = s2_p[44:33];
    logic [11:0] e_val;
    always_comb begin
        case (d_r)
            4'd1:    e_val = {11'd0, e_quot[0]};
            4'd4:    e_val = {8'd0,  e_quot[3:0]};
            4'd5:    e_val = {7'd0,  e_quot[4:0]};
            4'd10:   e_val = {2'd0,  e_quot[9:0]};
            4'd11:   e_val = {1'd0,  e_quot[10:0]};
            default: e_val = s2_x;
        endcase
    end

    assign o_valid = enc_run && (ocnt >= 6'd8);
    assign o_data  = obuf[7:0];
    wire        e_drain = o_valid && o_ready;
    wire [5:0]  ocnt1   = e_drain ? (ocnt - 6'd8) : ocnt;
    wire [47:0] obuf1   = e_drain ? {8'd0, obuf[47:8]} : obuf;
    wire [47:0] obuf2   = s2_v ? (obuf1 | ({36'd0, e_val} << ocnt1)) : obuf1;
    wire [5:0]  ocnt2   = s2_v ? (ocnt1 + {2'd0, d_r}) : ocnt1;

    wire [8:0]  e_total = {d_r, 5'd0};

    assign m_we    = dec_run && p_v;
    assign m_wdata = {decomp(p_v1, d_r), decomp(p_v0, d_r)};
    assign m_idx   = dir_r ? e_ridx[6:0] : d_widx;

    always_ff @(posedge clk) begin
        done <= 1'b0;
        if (rst) begin
            run          <= 1'b0;
            dir_r        <= 1'b0;
            d_r          <= 4'd12;
            range_err    <= 1'b0;
            dbuf         <= 32'd0;
            dcnt         <= 6'd0;
            d_bytes_left <= 9'd0;
            d_pairs      <= 8'd0;
            p_v          <= 1'b0;
            p_v0         <= 12'd0;
            p_v1         <= 12'd0;
            d_widx       <= 7'd0;
            e_ridx       <= 8'd0;
            rd_v         <= 1'b0;
            wlo          <= 12'd0;
            whi          <= 12'd0;
            lo_v         <= 1'b0;
            hi_v         <= 1'b0;
            commit       <= 6'd0;
            obuf         <= 48'd0;
            ocnt         <= 6'd0;
            e_bytes      <= 9'd0;
            s1_v         <= 1'b0;
            s1_x         <= 12'd0;
            s2_v         <= 1'b0;
            s2_p         <= 45'd0;
            s2_x         <= 12'd0;
        end else if (!run) begin
            if (start) begin
                run          <= 1'b1;
                dir_r        <= dir;
                d_r          <= d;
                range_err    <= 1'b0;
                dbuf         <= 32'd0;
                dcnt         <= 6'd0;
                d_bytes_left <= {d, 5'd0};
                d_pairs      <= 8'd0;
                p_v          <= 1'b0;
                d_widx       <= 7'd0;
                e_ridx       <= 8'd0;
                rd_v         <= 1'b0;
                lo_v         <= 1'b0;
                hi_v         <= 1'b0;
                commit       <= 6'd0;
                obuf         <= 48'd0;
                ocnt         <= 6'd0;
                e_bytes      <= 9'd0;
                s1_v         <= 1'b0;
                s2_v         <= 1'b0;
            end
        end else if (!dir_r) begin
            dbuf <= dbuf2;
            dcnt <= dcnt2;
            if (d_take) d_bytes_left <= d_bytes_left - 9'd1;
            p_v <= d_emit;
            if (d_emit) begin
                p_v0    <= x_v0;
                p_v1    <= x_v1;
                d_pairs <= d_pairs + 8'd1;
            end
            if (p_v) begin
                if (d_over) range_err <= 1'b1;
                if (d_widx == 7'd127) begin
                    run  <= 1'b0;
                    done <= 1'b1;
                    p_v  <= 1'b0;
                    p_v0 <= 12'd0;
                    p_v1 <= 12'd0;
                    dbuf <= 32'd0;
                end else begin
                    d_widx <= d_widx + 7'd1;
                end
            end
        end else begin
            rd_v <= m_re;
            if (m_re) e_ridx <= e_ridx + 8'd1;

            if (rd_v) begin
                wlo  <= m_rdata[11:0];
                whi  <= m_rdata[23:12];
                lo_v <= !e_issue;
                hi_v <= 1'b1;
            end else if (e_issue) begin
                if (lo_v) lo_v <= 1'b0;
                else      hi_v <= 1'b0;
            end

            s1_v <= e_issue;
            if (e_issue) s1_x <= src_x;
            s2_v <= s1_v;
            s2_p <= e_n * 22'd2580335;
            s2_x <= s1_x;

            commit <= commit + (e_issue ? {2'd0, d_r} : 6'd0) - (e_drain ? 6'd8 : 6'd0);
            obuf   <= obuf2;
            ocnt   <= ocnt2;

            if (e_drain) begin
                e_bytes <= e_bytes + 9'd1;
                if (e_bytes + 9'd1 == e_total) begin
                    run  <= 1'b0;
                    done <= 1'b1;
                    obuf <= 48'd0;
                    ocnt <= 6'd0;
                    wlo  <= 12'd0;
                    whi  <= 12'd0;
                    s1_x <= 12'd0;
                    s2_p <= 45'd0;
                    s2_x <= 12'd0;
                end
            end
        end
    end

endmodule
