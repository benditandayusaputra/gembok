`timescale 1ns/1ps

module tb_modmul;
    logic        clk = 1'b0;
    logic [11:0] x = 12'd0;
    logic [11:0] y = 12'd0;
    wire  [11:0] t;
    wire  [11:0] t_r;

    modmul dut (.clk(clk), .x(x), .y(y), .t(t), .t_r(t_r));

    always #5 clk = ~clk;

    logic [11:0] e1, e2, e3, e4;
    logic        v1 = 0, v2 = 0, v3 = 0, v4 = 0;
    logic        feed = 1'b1;
    longint      checked = 0;
    longint      errors  = 0;

    always @(posedge clk) begin
        e1 <= 12'((24'(x) * 24'(y)) % 24'd3329);
        e2 <= e1;
        e3 <= e2;
        e4 <= e3;
        v1 <= feed;
        v2 <= v1;
        v3 <= v2;
        v4 <= v3;
        if (v3) begin
            if (t !== e3) errors <= errors + 1;
        end
        if (v4) begin
            checked <= checked + 1;
            if (t_r !== e4) errors <= errors + 1;
        end
    end

    initial begin
        for (int a = 0; a < 3329; a++) begin
            for (int b = 0; b < 3329; b++) begin
                @(negedge clk);
                x = 12'(a);
                y = 12'(b);
            end
        end
        @(negedge clk);
        feed = 1'b0;
        repeat (8) @(negedge clk);
        $display("modmul: %0d pasangan diperiksa, %0d salah", checked, errors);
        if (errors != 0 || checked < 64'd11082241) begin
            $display("GAGAL");
            $fatal(1);
        end
        $display("LOLOS");
        $finish;
    end
endmodule
