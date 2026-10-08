module modmul (
    input  logic        clk,
    input  logic [11:0] x,
    input  logic [11:0] y,
    output logic [11:0] t,
    output logic [11:0] t_r
);

    logic [11:0] x_r;
    logic [11:0] y_r;
    logic [23:0] p_r;
    logic [12:0] p_lo;
    logic [12:0] m_r;

    wire [36:0] pm = p_r * 37'd5039;

    always_ff @(posedge clk) begin
        x_r  <= x;
        y_r  <= y;
        p_r  <= x_r * y_r;
        m_r  <= pm[36:24];
        p_lo <= p_r[12:0];
        t_r  <= t;
    end

    wire [12:0] mq = m_r + {m_r[4:0], 8'h00} + {m_r[2:0], 10'h000} + {m_r[1:0], 11'h000};
    wire [12:0] r  = p_lo - mq;
    wire [12:0] r2 = r - 13'd3329;

    assign t = r2[12] ? r[11:0] : r2[11:0];

endmodule
