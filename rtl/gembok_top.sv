module gembok_top #(
    parameter integer PUF_MODE        = 0,
    parameter integer N_RO            = 768,
    parameter integer RO_STAGES       = 5,
    parameter integer WIN_LOG2        = 12,
    parameter integer VOTES           = 5,
    parameter [31:0]  COOLDOWN_CYCLES = 32'd1048576,
    parameter [255:0] DEV_KEY         = 256'h6b6f626d65672d6b6f626d65672d6b6f626d65672d6b6f626d65672d76656421,
    parameter [31:0]  CHIP_SEED       = 32'h0000_0001,
    parameter integer SIM_NOISE       = 0,
    parameter integer SIM_RO_HALF_PS  = 1500,
    parameter integer PUF_DEBUG       = 0,
    parameter [15:0]  THRESH_RESET    = 16'd64
) (
    input  logic        clk,
    input  logic        reset,

    input  logic [13:0] avs_address,
    input  logic        avs_read,
    input  logic        avs_write,
    input  logic [31:0] avs_writedata,
    output logic [31:0] avs_readdata,

    output logic [7:0]  led
);

    logic [1:0] rst_sync;
    always_ff @(posedge clk) rst_sync <= {rst_sync[0], reset};
    wire rst = rst_sync[1];

    localparam integer MASK_BYTES = (N_RO + 6) / 8;
    localparam [31:0] CAPS = (N_RO * 65536) + (VOTES * 256) + (WIN_LOG2 * 8) + ((PUF_DEBUG != 0) ? 4 : 0) + PUF_MODE;

    wire        cmd_valid;
    wire [3:0]  cmd_code;
    wire [2:0]  cmd_k;
    wire [7:0]  ctx_len;
    wire [15:0] puf_thresh;
    wire [9:0]  puf_dbg_idx;
    wire        busy;
    wire        done;
    wire [3:0]  err;
    wire        puf_ready;
    wire        cooling;
    wire        booted;
    wire [31:0] cycles;
    wire [31:0] cooldown_left;
    wire [31:0] proofs;
    wire [31:0] puf_dbg;
    wire [31:0] puf_dbg1;
    wire [12:0] h_addr;
    wire        h_we;
    wire [7:0]  h_wdata;
    wire [7:0]  h_rdata;
    wire [12:0] m_addr;
    wire        m_we;
    wire [7:0]  m_wdata;
    wire [7:0]  m_rdata;
    wire        core_start;
    wire [3:0]  core_sel;
    wire [2:0]  core_k;
    wire [7:0]  core_ctx_len;
    wire        core_busy;
    wire        core_done;
    wire [3:0]  core_status;
    wire [4:0]  puf_idx;
    wire [7:0]  puf_byte;

    avmm_bridge #(
        .CAPS         (CAPS),
        .THRESH_RESET (THRESH_RESET)
    ) u_bridge (
        .clk           (clk),
        .rst           (rst),
        .avs_address   (avs_address),
        .avs_read      (avs_read),
        .avs_write     (avs_write),
        .avs_writedata (avs_writedata),
        .avs_readdata  (avs_readdata),
        .cmd_valid     (cmd_valid),
        .cmd_code      (cmd_code),
        .cmd_k         (cmd_k),
        .ctx_len       (ctx_len),
        .puf_thresh    (puf_thresh),
        .puf_dbg_idx   (puf_dbg_idx),
        .busy          (busy),
        .done          (done),
        .err           (err),
        .puf_ready     (puf_ready),
        .cooling       (cooling),
        .booted        (booted),
        .cycles        (cycles),
        .cooldown_left (cooldown_left),
        .proofs        (proofs),
        .puf_dbg       (puf_dbg),
        .puf_dbg1      (puf_dbg1),
        .h_addr        (h_addr),
        .h_we          (h_we),
        .h_wdata       (h_wdata),
        .h_rdata       (h_rdata)
    );

    vault #(
        .PUF_MODE        (PUF_MODE),
        .N_RO            (N_RO),
        .RO_STAGES       (RO_STAGES),
        .WIN_LOG2        (WIN_LOG2),
        .VOTES           (VOTES),
        .COOLDOWN_CYCLES (COOLDOWN_CYCLES),
        .DEV_KEY         (DEV_KEY),
        .CHIP_SEED       (CHIP_SEED),
        .SIM_NOISE       (SIM_NOISE),
        .SIM_RO_HALF_PS  (SIM_RO_HALF_PS),
        .PUF_DEBUG       (PUF_DEBUG)
    ) u_vault (
        .clk           (clk),
        .rst           (rst),
        .cmd_valid     (cmd_valid),
        .cmd_code      (cmd_code),
        .cmd_k         (cmd_k),
        .cmd_ctx_len   (ctx_len),
        .puf_thresh    (puf_thresh),
        .puf_dbg_idx   (puf_dbg_idx),
        .busy          (busy),
        .done          (done),
        .err           (err),
        .puf_ready     (puf_ready),
        .cooling       (cooling),
        .booted        (booted),
        .cycles        (cycles),
        .cooldown_left (cooldown_left),
        .proofs        (proofs),
        .puf_dbg       (puf_dbg),
        .puf_dbg1      (puf_dbg1),
        .core_start    (core_start),
        .core_sel      (core_sel),
        .core_k        (core_k),
        .core_ctx_len  (core_ctx_len),
        .core_busy     (core_busy),
        .core_done     (core_done),
        .core_status   (core_status),
        .puf_idx       (puf_idx),
        .puf_byte      (puf_byte),
        .h_addr        (h_addr),
        .h_we          (h_we),
        .h_wdata       (h_wdata),
        .h_rdata       (h_rdata),
        .m_addr        (m_addr),
        .m_we          (m_we),
        .m_wdata       (m_wdata),
        .m_rdata       (m_rdata)
    );

    mlkem_ctrl #(
        .HELPER_MASK_LEN (MASK_BYTES)
    ) u_core (
        .clk       (clk),
        .rst       (rst),
        .cmd_start (core_start),
        .cmd_sel   (core_sel),
        .cmd_k     (core_k),
        .ctx_len   (core_ctx_len),
        .busy      (core_busy),
        .done      (core_done),
        .status    (core_status),
        .puf_idx   (puf_idx),
        .puf_byte  (puf_byte),
        .h_addr    (m_addr),
        .h_we      (m_we),
        .h_wdata   (m_wdata),
        .h_rdata   (m_rdata)
    );

    assign led = {3'd0, booted, puf_ready, (err != 4'd0), done, busy};

endmodule
