module fuzzy_extractor #(
    parameter integer N_CAND   = 767,
    parameter integer KEY_BITS = 256,
    parameter integer VOTES    = 5
) (
    input  logic        clk,
    input  logic        rst,

    input  logic        start,
    input  logic        mode,
    output logic        busy,
    output logic        done,
    output logic        ok,

    output logic        puf_restart,
    output logic        puf_next,
    output logic        puf_measure,
    input  logic        puf_done,
    input  logic        puf_bit,
    input  logic        puf_solid,

    output logic [6:0]  hm_addr,
    output logic        hm_we,
    output logic [7:0]  hm_wdata,
    input  logic [7:0]  hm_rdata,

    output logic        key_shift,
    output logic        key_bit
);

    localparam [3:0] F_IDLE  = 4'd0;
    localparam [3:0] F_INIT  = 4'd1;
    localparam [3:0] F_PAIR  = 4'd2;
    localparam [3:0] F_RDW   = 4'd3;
    localparam [3:0] F_DEC   = 4'd4;
    localparam [3:0] F_MEAS  = 4'd5;
    localparam [3:0] F_WAITM = 4'd6;
    localparam [3:0] F_EVAL  = 4'd7;
    localparam [3:0] F_NEXT  = 4'd8;
    localparam [3:0] F_FIN   = 4'd9;
    localparam [3:0] F_ADV   = 4'd10;

    logic [3:0]  st;
    logic        mode_r;
    logic [15:0] c;
    logic [15:0] nsel;
    logic        skip;
    logic [7:0]  mbyte;
    logic        mbit;
    logic [7:0]  votes;
    logic [7:0]  ones;
    logic        allsolid;

    wire last_cand = ({16'd0, c} == (N_CAND - 1));

    assign busy    = (st != F_IDLE);
    assign hm_addr = c[9:3];

    always_ff @(posedge clk) begin
        done        <= 1'b0;
        puf_restart <= 1'b0;
        puf_next    <= 1'b0;
        puf_measure <= 1'b0;
        hm_we       <= 1'b0;
        key_shift   <= 1'b0;

        if (rst) begin
            st        <= F_IDLE;
            mode_r    <= 1'b0;
            c         <= 16'd0;
            nsel      <= 16'd0;
            skip      <= 1'b0;
            mbyte     <= 8'd0;
            mbit      <= 1'b0;
            votes     <= 8'd0;
            ones      <= 8'd0;
            allsolid <= 1'b0;
            ok        <= 1'b0;
            hm_wdata  <= 8'd0;
            key_bit   <= 1'b0;
        end else begin
            case (st)
                F_IDLE: begin
                    if (start) begin
                        mode_r <= mode;
                        ok     <= 1'b0;
                        st     <= F_INIT;
                    end
                end

                F_INIT: begin
                    puf_restart <= 1'b1;
                    c           <= 16'd0;
                    nsel        <= 16'd0;
                    skip        <= 1'b0;
                    mbyte       <= 8'd0;
                    st          <= F_PAIR;
                end

                F_PAIR: begin
                    mbit <= 1'b0;
                    if (mode_r && (c[2:0] == 3'd0)) st <= F_RDW;
                    else                            st <= F_DEC;
                end

                F_RDW: begin
                    st <= F_DEC;
                end

                F_DEC: begin
                    if (mode_r && (c[2:0] == 3'd0)) mbyte <= hm_rdata;
                    votes     <= 8'd0;
                    ones      <= 8'd0;
                    allsolid <= 1'b1;
                    if (mode_r) begin
                        if ((c[2:0] == 3'd0) ? hm_rdata[0] : mbyte[c[2:0]]) st <= F_MEAS;
                        else                                                st <= F_NEXT;
                    end else begin
                        if (skip || ({16'd0, nsel} == KEY_BITS)) begin
                            skip <= 1'b0;
                            st   <= F_NEXT;
                        end else begin
                            st <= F_MEAS;
                        end
                    end
                end

                F_MEAS: begin
                    puf_measure <= 1'b1;
                    st          <= F_WAITM;
                end

                F_WAITM: begin
                    if (puf_done) begin
                        ones      <= ones + {7'd0, puf_bit};
                        allsolid <= allsolid && puf_solid;
                        votes     <= votes + 8'd1;
                        if ({24'd0, votes} == (VOTES - 1)) st <= F_EVAL;
                        else                      st <= F_MEAS;
                    end
                end

                F_EVAL: begin
                    if (mode_r) begin
                        key_shift <= 1'b1;
                        key_bit   <= ({24'd0, ones} > (VOTES / 2));
                        nsel      <= nsel + 16'd1;
                    end else if (allsolid && ((ones == 8'd0) || ({24'd0, ones} == VOTES))) begin
                        mbit      <= 1'b1;
                        key_shift <= 1'b1;
                        key_bit   <= (ones != 8'd0);
                        nsel      <= nsel + 16'd1;
                        skip      <= 1'b1;
                    end
                    st <= F_NEXT;
                end

                F_NEXT: begin
                    if (!mode_r) begin
                        if ((c[2:0] == 3'd7) || last_cand) begin
                            hm_we    <= 1'b1;
                            hm_wdata <= mbyte | ({7'd0, mbit} << c[2:0]);
                            mbyte    <= 8'd0;
                        end else begin
                            mbyte <= mbyte | ({7'd0, mbit} << c[2:0]);
                        end
                    end
                    if (last_cand) begin
                        st <= F_FIN;
                    end else begin
                        puf_next <= 1'b1;
                        st       <= F_ADV;
                    end
                end

                F_ADV: begin
                    c  <= c + 16'd1;
                    st <= F_PAIR;
                end

                F_FIN: begin
                    ok      <= ({16'd0, nsel} == KEY_BITS);
                    done    <= 1'b1;
                    key_bit <= 1'b0;
                    mbyte   <= 8'd0;
                    st      <= F_IDLE;
                end

                default: st <= F_IDLE;
            endcase
        end
    end

endmodule
