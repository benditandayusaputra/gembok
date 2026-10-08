package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"net"
	"net/http"
	"net/url"
	"runtime/debug"
	"strconv"
	"strings"
	"time"

	"gembok/verifier/internal/chip"
	"gembok/verifier/internal/lms"
	"gembok/verifier/internal/verify"
)

const (
	maxBodyBytes          = 4 << 10
	DefaultRequestTimeout = 20 * time.Second
	writeTimeoutMargin    = 25 * time.Second
)

type Service interface {
	Status(ctx context.Context) (verify.StatusReport, error)
	Enroll(ctx context.Context, req verify.EnrollRequest) (verify.EnrollResult, error)
	PowerUp(ctx context.Context) (verify.PowerUpResult, error)
	Check(ctx context.Context, proofContext string) (verify.Result, error)
	AttackReadKey(ctx context.Context) (verify.AttackResult, error)
	SwitchChip(ctx context.Context, name string, selfEnroll bool) (verify.StatusReport, error)
	History(limit int) []verify.Result
}

type Options struct {
	Service        Service
	Logger         *slog.Logger
	AllowedHosts   []string
	RequestTimeout time.Duration
}

type api struct {
	svc          Service
	log          *slog.Logger
	allowedHosts map[string]bool
	timeout      time.Duration
}

var endpoints = map[string]string{
	"/api/status":            http.MethodGet,
	"/api/riwayat":           http.MethodGet,
	"/api/daftar":            http.MethodPost,
	"/api/nyalakan":          http.MethodPost,
	"/api/periksa":           http.MethodPost,
	"/api/serang/baca-kunci": http.MethodPost,
	"/api/simulasi/chip":     http.MethodPost,
}

func NewHandler(opts Options) http.Handler {
	a := &api{svc: opts.Service, log: opts.Logger, timeout: opts.RequestTimeout}
	if a.log == nil {
		a.log = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	if a.timeout <= 0 {
		a.timeout = DefaultRequestTimeout
	}
	if len(opts.AllowedHosts) > 0 {
		a.allowedHosts = map[string]bool{}
		for _, h := range opts.AllowedHosts {
			a.allowedHosts[strings.ToLower(h)] = true
		}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/status", a.status)
	mux.HandleFunc("GET /api/riwayat", a.history)
	mux.HandleFunc("POST /api/daftar", a.enroll)
	mux.HandleFunc("POST /api/nyalakan", a.powerUp)
	mux.HandleFunc("POST /api/periksa", a.check)
	mux.HandleFunc("POST /api/serang/baca-kunci", a.attack)
	mux.HandleFunc("POST /api/simulasi/chip", a.switchChip)
	mux.HandleFunc("/api/", a.unknown)
	mux.Handle("/", staticHandler())
	return a.middleware(mux)
}

func NewServer(addr string, handler http.Handler, requestTimeout time.Duration) *http.Server {
	return &http.Server{
		Addr:              addr,
		Handler:           handler,
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       10 * time.Second,
		WriteTimeout:      max(requestTimeout, DefaultRequestTimeout) + writeTimeoutMargin,
		IdleTimeout:       60 * time.Second,
		MaxHeaderBytes:    16 << 10,
	}
}

func LoopbackHosts() []string {
	return []string{"localhost", "127.0.0.1", "::1"}
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

func (a *api) middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
		h := rec.Header()
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Frame-Options", "DENY")
		h.Set("Cross-Origin-Opener-Policy", "same-origin")
		h.Set("Cross-Origin-Resource-Policy", "same-origin")
		h.Set("Permissions-Policy", "camera=(), microphone=(), geolocation=()")
		h.Set("Content-Security-Policy", "frame-ancestors 'none'; base-uri 'self'; form-action 'self'; object-src 'none'")
		defer func() {
			if p := recover(); p != nil {
				a.log.Error("panik saat melayani permintaan", "jalur", r.URL.Path, "panik", fmt.Sprint(p), "jejak", string(debug.Stack()))
				writeJSONError(rec, http.StatusInternalServerError, "GALAT_INTERNAL", "Terjadi galat di layanan.", "")
			}
			a.log.Info("permintaan", "metode", r.Method, "jalur", r.URL.Path, "status", rec.status, "durasi", time.Since(start).Round(time.Microsecond))
		}()
		if a.allowedHosts != nil && !a.allowedHosts[hostOnly(r.Host)] {
			writeJSONError(rec, http.StatusMisdirectedRequest, "HOST_DITOLAK", "Nama host tidak diizinkan untuk layanan yang hanya mendengar di loopback.", "")
			return
		}
		if strings.HasPrefix(r.URL.Path, "/api/") {
			h.Set("Cache-Control", "no-store")
			if r.Method == http.MethodPost {
				if status, code, msg := checkWriteRequest(r); status != 0 {
					writeJSONError(rec, status, code, msg, "")
					return
				}
			}
		}
		next.ServeHTTP(rec, r)
	})
}

func hostOnly(hostport string) string {
	host := hostport
	if h, _, err := net.SplitHostPort(hostport); err == nil {
		host = h
	}
	return strings.ToLower(strings.Trim(host, "[]"))
}

func checkWriteRequest(r *http.Request) (int, string, string) {
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		return http.StatusUnsupportedMediaType, "JENIS_ISI_DITOLAK", "Permintaan POST harus memakai Content-Type: application/json."
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site != "" && site != "same-origin" && site != "none" {
		return http.StatusForbidden, "LINTAS_SITUS_DITOLAK", "Permintaan dari situs lain ditolak."
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || u.Host == "" || !strings.EqualFold(u.Host, r.Host) {
			return http.StatusForbidden, "ASAL_DITOLAK", "Asal permintaan tidak sama dengan layanan."
		}
	}
	return 0, "", ""
}

type errorBody struct {
	Code     string `json:"kode"`
	Message  string `json:"pesan"`
	ChipCode string `json:"kode_chip,omitempty"`
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(true)
	if err := enc.Encode(v); err != nil {
		status = http.StatusInternalServerError
		buf.Reset()
		buf.WriteString(`{"galat":{"kode":"GALAT_INTERNAL","pesan":"Gagal menyusun jawaban."}}` + "\n")
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Content-Length", strconv.Itoa(buf.Len()))
	w.WriteHeader(status)
	w.Write(buf.Bytes())
}

func writeJSONError(w http.ResponseWriter, status int, code, msg, chipCode string) {
	writeJSON(w, status, map[string]errorBody{"galat": {Code: code, Message: msg, ChipCode: chipCode}})
}

func (a *api) fail(w http.ResponseWriter, r *http.Request, err error) {
	status, code, msg, chipCode := classify(err)
	if status >= 500 {
		a.log.Error("permintaan gagal", "jalur", r.URL.Path, "galat", err)
	} else {
		a.log.Info("permintaan ditolak", "jalur", r.URL.Path, "galat", err)
	}
	if status == http.StatusServiceUnavailable && code == "PEMBATAS_LAJU" {
		w.Header().Set("Retry-After", "1")
	}
	writeJSONError(w, status, code, msg, chipCode)
}

func classify(err error) (int, string, string, string) {
	switch {
	case errors.Is(err, verify.ErrBadRequest):
		return http.StatusBadRequest, "PERMINTAAN_TIDAK_SAH", err.Error(), ""
	case errors.Is(err, verify.ErrNotEnrolled):
		return http.StatusConflict, "BELUM_TERDAFTAR", "Chip belum didaftarkan. Jalankan POST /api/daftar lebih dulu.", ""
	case errors.Is(err, verify.ErrAlreadyEnrolled):
		return http.StatusConflict, "SUDAH_TERDAFTAR", "Chip sudah didaftarkan. Kirim \"ulang\": true untuk mendaftar ulang (memakai satu tanda tangan penerbit).", ""
	case errors.Is(err, verify.ErrNotSimulation):
		return http.StatusConflict, "BUKAN_SIMULASI", "Penggantian chip hanya tersedia pada backend sim.", ""
	case errors.Is(err, verify.ErrHelperMismatch):
		return http.StatusConflict, "TOPENG_TIDAK_SESUAI", "Data bantu terdaftar tidak cocok dengan chip ini (" + err.Error() + "). PUF_RECON tidak dikirim. Daftarkan ulang chip.", ""
	case errors.Is(err, verify.ErrNoIssuer):
		return http.StatusServiceUnavailable, "PENERBIT_TIDAK_ADA", "Kunci penerbit tidak tersedia.", ""
	case errors.Is(err, lms.ErrExhausted):
		return http.StatusServiceUnavailable, "TANDA_TANGAN_HABIS", "Semua 1024 tanda tangan penerbit sudah terpakai. Buat kunci penerbit baru.", ""
	}
	var cf *verify.ChipFailure
	if errors.As(err, &cf) {
		switch {
		case errors.Is(err, chip.ErrRateLimited):
			return http.StatusServiceUnavailable, "PEMBATAS_LAJU", "Pembatas laju chip masih aktif. Coba lagi sebentar.", chip.ErrRate.String()
		case chip.IsTimeout(err):
			return http.StatusGatewayTimeout, "CHIP_TIDAK_MENJAWAB", "Chip tidak menjawab dalam batas waktu.", ""
		}
		if code, ok := chip.CodeOf(err); ok {
			return http.StatusConflict, "CHIP_MENOLAK", "Chip menolak perintah " + cf.Op + ": " + code.Meaning() + ".", code.String()
		}
		return http.StatusBadGateway, "CHIP_GAGAL", "Chip gagal saat " + cf.Op + ".", ""
	}
	if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return http.StatusServiceUnavailable, "DIBATALKAN", "Permintaan dibatalkan atau melewati batas waktu.", ""
	}
	return http.StatusInternalServerError, "GALAT_INTERNAL", "Terjadi galat di layanan.", ""
}

func decodeBody(w http.ResponseWriter, r *http.Request, dst any) error {
	body := http.MaxBytesReader(w, r.Body, maxBodyBytes)
	raw, err := io.ReadAll(body)
	if err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return fmt.Errorf("%w: isi permintaan lebih dari %d byte", verify.ErrBadRequest, maxBodyBytes)
		}
		return fmt.Errorf("%w: isi permintaan tidak terbaca", verify.ErrBadRequest)
	}
	if len(bytes.TrimSpace(raw)) == 0 {
		return nil
	}
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(dst); err != nil {
		return fmt.Errorf("%w: JSON tidak sah: %v", verify.ErrBadRequest, err)
	}
	if dec.More() {
		return fmt.Errorf("%w: ada data sesudah objek JSON", verify.ErrBadRequest)
	}
	if bytes.TrimSpace(raw)[0] != '{' {
		return fmt.Errorf("%w: isi harus objek JSON", verify.ErrBadRequest)
	}
	return nil
}

func (a *api) withTimeout(r *http.Request) (context.Context, context.CancelFunc) {
	return context.WithTimeout(r.Context(), a.timeout)
}

func (a *api) status(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := a.withTimeout(r)
	defer cancel()
	report, err := a.svc.Status(ctx)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, report)
}

func (a *api) history(w http.ResponseWriter, r *http.Request) {
	limit := verify.HistorySize
	if raw := r.URL.Query().Get("batas"); raw != "" {
		n, err := strconv.Atoi(raw)
		if err != nil || n < 1 || n > verify.HistorySize {
			a.fail(w, r, fmt.Errorf("%w: batas harus 1 sampai %d", verify.ErrBadRequest, verify.HistorySize))
			return
		}
		limit = n
	}
	items := a.svc.History(limit)
	writeJSON(w, http.StatusOK, map[string]any{"riwayat": items, "jumlah": len(items)})
}

type enrollBody struct {
	ChipID    string `json:"id_chip"`
	K         *int   `json:"k"`
	Thresh    *int   `json:"ambang"`
	Overwrite bool   `json:"ulang"`
}

func (a *api) enroll(w http.ResponseWriter, r *http.Request) {
	var body enrollBody
	if err := decodeBody(w, r, &body); err != nil {
		a.fail(w, r, err)
		return
	}
	req := verify.EnrollRequest{ChipID: body.ChipID, Overwrite: body.Overwrite}
	if body.K != nil {
		if *body.K != 3 && *body.K != 4 {
			a.fail(w, r, fmt.Errorf("%w: k harus 3 atau 4", verify.ErrBadRequest))
			return
		}
		req.K = *body.K
	}
	if body.Thresh != nil {
		if *body.Thresh < 1 || *body.Thresh > 0xFFFF {
			a.fail(w, r, fmt.Errorf("%w: ambang harus 1 sampai 65535", verify.ErrBadRequest))
			return
		}
		req.Thresh = uint16(*body.Thresh)
	}
	ctx, cancel := a.withTimeout(r)
	defer cancel()
	res, err := a.svc.Enroll(ctx, req)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type emptyBody struct{}

func (a *api) powerUp(w http.ResponseWriter, r *http.Request) {
	if err := decodeBody(w, r, &emptyBody{}); err != nil {
		a.fail(w, r, err)
		return
	}
	ctx, cancel := a.withTimeout(r)
	defer cancel()
	res, err := a.svc.PowerUp(ctx)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type checkBody struct {
	Context string `json:"konteks"`
}

func (a *api) check(w http.ResponseWriter, r *http.Request) {
	var body checkBody
	if err := decodeBody(w, r, &body); err != nil {
		a.fail(w, r, err)
		return
	}
	ctx, cancel := a.withTimeout(r)
	defer cancel()
	res, err := a.svc.Check(ctx, body.Context)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *api) attack(w http.ResponseWriter, r *http.Request) {
	if err := decodeBody(w, r, &emptyBody{}); err != nil {
		a.fail(w, r, err)
		return
	}
	ctx, cancel := a.withTimeout(r)
	defer cancel()
	res, err := a.svc.AttackReadKey(ctx)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

type switchBody struct {
	Chip       string `json:"chip"`
	SelfEnroll bool   `json:"daftar_sendiri"`
}

func (a *api) switchChip(w http.ResponseWriter, r *http.Request) {
	var body switchBody
	if err := decodeBody(w, r, &body); err != nil {
		a.fail(w, r, err)
		return
	}
	if body.Chip == "" {
		a.fail(w, r, fmt.Errorf("%w: ruas chip wajib diisi", verify.ErrBadRequest))
		return
	}
	ctx, cancel := a.withTimeout(r)
	defer cancel()
	res, err := a.svc.SwitchChip(ctx, body.Chip, body.SelfEnroll)
	if err != nil {
		a.fail(w, r, err)
		return
	}
	writeJSON(w, http.StatusOK, res)
}

func (a *api) unknown(w http.ResponseWriter, r *http.Request) {
	if method, ok := endpoints[r.URL.Path]; ok {
		w.Header().Set("Allow", method)
		writeJSONError(w, http.StatusMethodNotAllowed, "METODE_TIDAK_DIIZINKAN", "Pakai metode "+method+" untuk "+r.URL.Path+".", "")
		return
	}
	writeJSONError(w, http.StatusNotFound, "TIDAK_DITEMUKAN", "Titik akhir API tidak dikenal.", "")
}
