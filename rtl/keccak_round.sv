module keccak_round (
    input  logic [1599:0] a,
    input  logic [63:0]   rc,
    output wire  [1599:0] o
);

    function automatic integer rho_off(input integer idx);
        case (idx)
            0:  rho_off = 0;
            1:  rho_off = 1;
            2:  rho_off = 62;
            3:  rho_off = 28;
            4:  rho_off = 27;
            5:  rho_off = 36;
            6:  rho_off = 44;
            7:  rho_off = 6;
            8:  rho_off = 55;
            9:  rho_off = 20;
            10: rho_off = 3;
            11: rho_off = 10;
            12: rho_off = 43;
            13: rho_off = 25;
            14: rho_off = 39;
            15: rho_off = 41;
            16: rho_off = 45;
            17: rho_off = 15;
            18: rho_off = 21;
            19: rho_off = 8;
            20: rho_off = 18;
            21: rho_off = 2;
            22: rho_off = 61;
            23: rho_off = 56;
            24: rho_off = 14;
            default: rho_off = 0;
        endcase
    endfunction

    wire [63:0] c [0:4];
    wire [63:0] d [0:4];
    wire [63:0] b [0:24];

    genvar x, y;
    generate
        for (x = 0; x < 5; x = x + 1) begin : g_c
            assign c[x] = a[64*x +: 64] ^ a[64*(x+5) +: 64] ^ a[64*(x+10) +: 64]
                        ^ a[64*(x+15) +: 64] ^ a[64*(x+20) +: 64];
        end
        for (x = 0; x < 5; x = x + 1) begin : g_d
            assign d[x] = c[(x+4)%5] ^ {c[(x+1)%5][62:0], c[(x+1)%5][63]};
        end

        for (y = 0; y < 5; y = y + 1) begin : g_by
            for (x = 0; x < 5; x = x + 1) begin : g_bx
                localparam integer R   = rho_off(x + 5*y);
                localparam integer DST = y + 5*((2*x + 3*y) % 5);
                wire [63:0] t = a[64*(x+5*y) +: 64] ^ d[x];
                if (R == 0) begin : g_r0
                    assign b[DST] = t;
                end else begin : g_rn
                    assign b[DST] = {t[63-R:0], t[63:64-R]};
                end
            end
        end

        for (y = 0; y < 5; y = y + 1) begin : g_ey
            for (x = 0; x < 5; x = x + 1) begin : g_ex
                wire [63:0] e = b[x+5*y] ^ (~b[(x+1)%5 + 5*y] & b[(x+2)%5 + 5*y]);
                if (x == 0 && y == 0) begin : g_iota
                    assign o[63:0] = e ^ rc;
                end else begin : g_plain
                    assign o[64*(x+5*y) +: 64] = e;
                end
            end
        end
    endgenerate

endmodule
