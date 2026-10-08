package main

import (
	"context"
	"crypto/rand"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"gembok/verifier/internal/chip"
	"gembok/verifier/internal/httpapi"
	"gembok/verifier/internal/issuer"
	"gembok/verifier/internal/store"
	"gembok/verifier/internal/verify"
)

type options struct {
	backend  string
	base     uint64
	data     string
	listen   string
	k        int
	thresh   uint16
	attempts int
}

func parseFlags(args []string) (options, error) {
	fs := flag.NewFlagSet("gembok-verifier", flag.ContinueOnError)
	backend := fs.String("backend", chip.KindSim, "backend chip: sim (simulasi perangkat lunak) atau mmio (/dev/mem di papan)")
	base := fs.String("base", "0xFF240000", "alamat fisik IP GEMBOK untuk backend mmio (jembatan ringan 0xFF200000 + ofset 0x40000)")
	data := fs.String("data", "data", "direktori data (kunci penerbit, sertifikat, data bantu)")
	listen := fs.String("listen", "127.0.0.1:8080", "alamat dengar HTTP")
	k := fs.Int("k", 3, "parameter ML-KEM untuk pendaftaran: 3 (ML-KEM-768) atau 4 (ML-KEM-1024)")
	thresh := fs.Int("ambang", int(chip.DefaultPUFThresh), "ambang PUF bawaan untuk pendaftaran bila permintaan tidak menyebutkan ambang (1 sampai 65535, dari karakterisasi papan)")
	attempts := fs.Int("coba-nyala", verify.DefaultPowerUpAttempts, fmt.Sprintf("berapa kali PUF_RECON dicoba sebelum chip dinyatakan gagal (1 sampai %d)", verify.MaxPowerUpAttempts))
	if err := fs.Parse(args); err != nil {
		return options{}, err
	}
	if fs.NArg() != 0 {
		return options{}, fmt.Errorf("argumen tidak dikenal: %v", fs.Args())
	}
	if *backend != chip.KindSim && *backend != chip.KindMMIO {
		return options{}, fmt.Errorf("--backend harus sim atau mmio, bukan %q", *backend)
	}
	if *k != 3 && *k != 4 {
		return options{}, fmt.Errorf("--k harus 3 atau 4 (pustaka standar Go tidak menyediakan ML-KEM-512)")
	}
	if *thresh < 1 || *thresh > 0xFFFF {
		return options{}, fmt.Errorf("--ambang harus 1 sampai 65535, bukan %d", *thresh)
	}
	if *attempts < 1 || *attempts > verify.MaxPowerUpAttempts {
		return options{}, fmt.Errorf("--coba-nyala harus 1 sampai %d, bukan %d", verify.MaxPowerUpAttempts, *attempts)
	}
	baseValue, err := strconv.ParseUint(*base, 0, 64)
	if err != nil {
		return options{}, fmt.Errorf("--base tidak sah: %v", err)
	}
	if *data == "" {
		return options{}, errors.New("--data tidak boleh kosong")
	}
	return options{backend: *backend, base: baseValue, data: *data, listen: *listen, k: *k, thresh: uint16(*thresh), attempts: *attempts}, nil
}

const (
	openTimeout          = time.Minute
	requestTimeoutMargin = 15 * time.Second
)

func requestTimeout(pufTimeout time.Duration, attempts int) time.Duration {
	return max(httpapi.DefaultRequestTimeout, time.Duration(attempts)*pufTimeout+requestTimeoutMargin)
}

func isLoopback(listen string) bool {
	host, _, err := net.SplitHostPort(listen)
	if err != nil {
		return false
	}
	if host == "localhost" {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func run(ctx context.Context, opts options, logger *slog.Logger) error {
	st, err := store.Open(opts.data)
	if err != nil {
		return err
	}
	defer st.Close()
	logger.Info("direktori data", "jalur", st.Dir())
	started := time.Now()
	keys, err := st.LoadOrCreateIssuer(rand.Reader, func(msg string) { logger.Info(msg) })
	if err != nil {
		return fmt.Errorf("kunci penerbit: %w", err)
	}
	if keys.Created || keys.TreeRebuilt {
		logger.Info("kunci penerbit siap", "durasi", time.Since(started).Round(time.Millisecond))
	}
	iss, err := issuer.New(keys.Signer, keys.Counter, nil)
	if err != nil {
		return err
	}
	trust, err := st.LoadIssuerPublicKey()
	if err != nil {
		return fmt.Errorf("kunci publik penerbit: %w", err)
	}
	logger.Info("penerbit", "algoritma", issuer.SignatureAlgorithm(), "tanda_tangan_terpakai", iss.Used(), "maksimum", iss.Capacity())

	openCtx, cancel := context.WithTimeout(ctx, openTimeout)
	device, err := chip.Open(openCtx, chip.Config{Backend: opts.backend, Base: opts.base})
	cancel()
	if err != nil {
		return fmt.Errorf("membuka chip (%s): %w", opts.backend, err)
	}
	defer device.Close()
	info, err := device.Info(ctx)
	if err != nil {
		return err
	}
	logger.Info("chip terhubung", "backend", opts.backend, "id", fmt.Sprintf("0x%08X", info.ID), "mode_puf", info.PUFMode, "osilator", info.Oscillators, "suara", info.Votes, "jendela_log2", info.WinLog2, "topeng_byte", info.HelperMaskLen, "siklus_simulasi", device.SimulatedCycles())
	for _, w := range verify.PUFWarnings(device.Kind(), info) {
		logger.Warn(w.Message, "kode", w.Code)
	}

	svc, err := verify.New(verify.Config{
		Chip:            device,
		Store:           st,
		Trust:           trust,
		Issuer:          iss,
		DefaultK:        opts.k,
		DefaultThresh:   opts.thresh,
		PowerUpAttempts: opts.attempts,
		Logger:          logger,
	})
	if err != nil {
		return err
	}
	var allowed []string
	if isLoopback(opts.listen) {
		allowed = httpapi.LoopbackHosts()
	} else {
		logger.Warn("layanan mendengar di luar loopback dan tidak punya autentikasi; pakai hanya di jaringan demo tertutup", "alamat", opts.listen)
	}
	limit := requestTimeout(info.PUFTimeout, opts.attempts)
	logger.Info("batas waktu", "perintah_puf", info.PUFTimeout.Round(time.Millisecond), "permintaan", limit.Round(time.Millisecond))
	handler := httpapi.NewHandler(httpapi.Options{Service: svc, Logger: logger, AllowedHosts: allowed, RequestTimeout: limit})
	srv := httpapi.NewServer(opts.listen, handler, limit)
	ln, err := net.Listen("tcp", opts.listen)
	if err != nil {
		return err
	}
	errc := make(chan error, 1)
	go func() {
		errc <- srv.Serve(ln)
	}()
	logger.Info("layanan berjalan", "alamat", "http://"+ln.Addr().String())
	select {
	case err := <-errc:
		if !errors.Is(err, http.ErrServerClosed) {
			return err
		}
		return nil
	case <-ctx.Done():
	}
	logger.Info("menghentikan layanan")
	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancelShutdown()
	return srv.Shutdown(shutdownCtx)
}

func main() {
	opts, err := parseFlags(os.Args[1:])
	if errors.Is(err, flag.ErrHelp) {
		return
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gembok-verifier:", err)
		os.Exit(2)
	}
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := run(ctx, opts, logger); err != nil {
		logger.Error("berhenti karena galat", "galat", err)
		stop()
		os.Exit(1)
	}
}
