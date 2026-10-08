package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"gembok/verifier/internal/chip"
	"gembok/verifier/internal/issuer"
	"gembok/verifier/internal/lms"
	"gembok/verifier/internal/store"
	"gembok/verifier/internal/verify"
)

var (
	templateOnce sync.Once
	templateDir  string
	templateErr  error
)

func TestMain(m *testing.M) {
	code := m.Run()
	if templateDir != "" {
		os.RemoveAll(templateDir)
	}
	os.Exit(code)
}

func newService(t *testing.T) *verify.Service {
	t.Helper()
	templateOnce.Do(func() {
		templateDir, templateErr = os.MkdirTemp("", "gembok-penerbit-*")
		if templateErr != nil {
			return
		}
		s, err := store.Open(templateDir)
		if err != nil {
			templateErr = err
			return
		}
		defer s.Close()
		_, templateErr = s.LoadOrCreateIssuer(rand.Reader, nil)
	})
	if templateErr != nil {
		t.Fatal(templateErr)
	}
	dir := t.TempDir()
	os.MkdirAll(filepath.Join(dir, "penerbit"), 0o700)
	for _, name := range []string{"kunci-rahasia.json", "kunci-publik.json", "pohon.bin"} {
		raw, err := os.ReadFile(filepath.Join(templateDir, "penerbit", name))
		if err != nil {
			t.Fatal(err)
		}
		os.WriteFile(filepath.Join(dir, "penerbit", name), raw, 0o600)
	}
	st, err := store.Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	keys, err := st.LoadOrCreateIssuer(rand.Reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	iss, err := issuer.New(keys.Signer, keys.Counter, nil)
	if err != nil {
		t.Fatal(err)
	}
	trust, err := st.LoadIssuerPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	clock := chip.NewManualClock(100 * time.Nanosecond)
	sim, err := chip.NewSim(context.Background(), chip.SimConfig{Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	svc, err := verify.New(verify.Config{Chip: sim, Store: st, Trust: trust, Issuer: iss, DefaultK: 3, Now: clock.Now})
	if err != nil {
		t.Fatal(err)
	}
	return svc
}

type client struct {
	t       *testing.T
	handler http.Handler
}

type reply struct {
	status int
	header http.Header
	body   []byte
}

func (r reply) decode(t *testing.T, v any) {
	t.Helper()
	if err := json.Unmarshal(r.body, v); err != nil {
		t.Fatalf("jawaban bukan JSON (%d): %s", r.status, r.body)
	}
}

func (r reply) errorCode(t *testing.T) string {
	t.Helper()
	var e struct {
		Galat struct {
			Kode     string `json:"kode"`
			Pesan    string `json:"pesan"`
			KodeChip string `json:"kode_chip"`
		} `json:"galat"`
	}
	r.decode(t, &e)
	if e.Galat.Pesan == "" {
		t.Fatalf("galat tanpa pesan: %s", r.body)
	}
	return e.Galat.Kode
}

func (c client) do(method, target, body string, headers map[string]string) reply {
	c.t.Helper()
	var rd io.Reader
	if body != "" {
		rd = strings.NewReader(body)
	}
	req := httptest.NewRequest(method, target, rd)
	req.Host = "localhost:8080"
	if method == http.MethodPost {
		req.Header.Set("Content-Type", "application/json")
	}
	for k, v := range headers {
		if v == "" {
			req.Header.Del(k)
		} else {
			req.Header.Set(k, v)
		}
	}
	rec := httptest.NewRecorder()
	c.handler.ServeHTTP(rec, req)
	return reply{status: rec.Code, header: rec.Header(), body: rec.Body.Bytes()}
}

func (c client) post(target, body string) reply {
	c.t.Helper()
	return c.do(http.MethodPost, target, body, nil)
}

func newClient(t *testing.T, svc Service) client {
	t.Helper()
	return client{t: t, handler: NewHandler(Options{Service: svc, AllowedHosts: LoopbackHosts()})}
}

func TestDemoFlowOverHTTP(t *testing.T) {
	c := newClient(t, newService(t))
	status := c.do(http.MethodGet, "/api/status", "", nil)
	if status.status != http.StatusOK {
		t.Fatalf("status %d %s", status.status, status.body)
	}
	if status.header.Get("Cache-Control") != "no-store" || status.header.Get("X-Content-Type-Options") != "nosniff" || !strings.HasPrefix(status.header.Get("Content-Type"), "application/json") {
		t.Fatalf("header status %v", status.header)
	}
	var report verify.StatusReport
	status.decode(t, &report)
	if report.Backend != "sim" || report.Enrolled || report.Simulation == nil || report.Simulation.Active != "asli" {
		t.Fatalf("status awal %+v", report)
	}
	for _, field := range []string{`"ambang_bawaan":64`, `"coba_nyala":5`, `"peringatan":[]`, `"panjang_topeng":96`, `"jumlah_osilator":768`, `"jendela_log2":12`, `"batas_waktu_puf_ms":5000`} {
		if !strings.Contains(string(status.body), field) {
			t.Fatalf("status tanpa %s: %s", field, status.body)
		}
	}
	if r := c.post("/api/periksa", `{}`); r.status != http.StatusConflict || r.errorCode(t) != "BELUM_TERDAFTAR" {
		t.Fatalf("periksa sebelum daftar %d %s", r.status, r.body)
	}
	enroll := c.post("/api/daftar", `{"id_chip":"GEMBOK-HTTP","k":3}`)
	if enroll.status != http.StatusOK {
		t.Fatalf("daftar %d %s", enroll.status, enroll.body)
	}
	var enrolled map[string]any
	enroll.decode(t, &enrolled)
	if enrolled["sertifikat"].(map[string]any)["id_chip"] != "GEMBOK-HTTP" || enrolled["siklus_daftar"].(float64) != chip.SimCyclesEnroll768 {
		t.Fatalf("hasil daftar %s", enroll.body)
	}
	if helper := enrolled["data_bantu"].(map[string]any); helper["ambang"].(float64) != 64 || helper["panjang_topeng"].(float64) != 96 {
		t.Fatalf("data bantu %v", helper)
	}
	if r := c.post("/api/daftar", `{}`); r.status != http.StatusConflict || r.errorCode(t) != "SUDAH_TERDAFTAR" {
		t.Fatalf("daftar dua kali %d %s", r.status, r.body)
	}
	power := c.post("/api/nyalakan", `{}`)
	var powered verify.PowerUpResult
	power.decode(t, &powered)
	if power.status != http.StatusOK || !powered.Success || !powered.PUFReady || powered.Attempts != 1 || powered.MaxAttempts != 5 {
		t.Fatalf("nyalakan %d %s", power.status, power.body)
	}
	check := c.post("/api/periksa", `{"konteks":"scan-001"}`)
	var genuine verify.Result
	check.decode(t, &genuine)
	if check.status != http.StatusOK || genuine.Verdict != "ASLI" || genuine.Context != "scan-001" || genuine.ChipMicros != 908 {
		t.Fatalf("periksa chip asli %d %s", check.status, check.body)
	}
	sw := c.post("/api/simulasi/chip", `{"chip":"tiruan"}`)
	var switched verify.StatusReport
	sw.decode(t, &switched)
	if sw.status != http.StatusOK || switched.Simulation.Active != "tiruan" || switched.Chip.PUFReady {
		t.Fatalf("ganti chip %d %s", sw.status, sw.body)
	}
	var fake verify.Result
	fakeReply := c.post("/api/periksa", `{}`)
	fakeReply.decode(t, &fake)
	if fake.Verdict != "PALSU" || fake.Reason != verify.ReasonHelperRejected || !strings.Contains(string(fakeReply.body), `"percobaan_nyala":5`) {
		t.Fatalf("chip tiruan %s", fakeReply.body)
	}
	powerClone := c.post("/api/nyalakan", `{}`)
	var refused verify.PowerUpResult
	powerClone.decode(t, &refused)
	if powerClone.status != http.StatusOK || refused.Success || refused.ChipCode != "BAD_HELPER" || refused.Attempts != 5 {
		t.Fatalf("nyalakan chip tiruan %d %s", powerClone.status, powerClone.body)
	}
	c.post("/api/simulasi/chip", `{"chip":"tiruan","daftar_sendiri":true}`)
	c.post("/api/periksa", `{}`).decode(t, &fake)
	if fake.Verdict != "PALSU" || fake.Reason != verify.ReasonProofMismatch {
		t.Fatalf("chip tiruan yang mendaftar sendiri %+v", fake)
	}
	c.post("/api/simulasi/chip", `{"chip":"asli"}`)
	attack := c.post("/api/serang/baca-kunci", `{}`)
	var attacked verify.AttackResult
	attack.decode(t, &attacked)
	if attack.status != http.StatusOK || !attacked.SecretsAllZero || !attacked.ProofRan || attacked.Proof.Verdict != "ASLI" {
		t.Fatalf("baca kunci %d %s", attack.status, attack.body)
	}
	history := c.do(http.MethodGet, "/api/riwayat", "", nil)
	var h struct {
		Riwayat []verify.Result `json:"riwayat"`
		Jumlah  int             `json:"jumlah"`
	}
	history.decode(t, &h)
	if h.Jumlah != 4 || len(h.Riwayat) != 4 || h.Riwayat[0].Context != verify.AttackContext || h.Riwayat[3].Context != "scan-001" {
		t.Fatalf("riwayat %s", history.body)
	}
	c.do(http.MethodGet, "/api/riwayat?batas=1", "", nil).decode(t, &h)
	if h.Jumlah != 1 {
		t.Fatalf("riwayat terbatas %d", h.Jumlah)
	}
}

func TestRequestValidation(t *testing.T) {
	c := newClient(t, newService(t))
	if r := c.post("/api/daftar", `{"k":3}`); r.status != http.StatusOK {
		t.Fatalf("daftar %d %s", r.status, r.body)
	}
	cases := []struct {
		name, target, body string
		headers            map[string]string
		status             int
		code               string
	}{
		{"tanpa jenis isi", "/api/periksa", `{}`, map[string]string{"Content-Type": ""}, 415, "JENIS_ISI_DITOLAK"},
		{"jenis isi teks", "/api/periksa", `{}`, map[string]string{"Content-Type": "text/plain"}, 415, "JENIS_ISI_DITOLAK"},
		{"ruas asing", "/api/periksa", `{"konteks":"a","lain":1}`, nil, 400, "PERMINTAAN_TIDAK_SAH"},
		{"data sisa", "/api/periksa", `{"konteks":"a"} {}`, nil, 400, "PERMINTAAN_TIDAK_SAH"},
		{"larik", "/api/periksa", `[]`, nil, 400, "PERMINTAAN_TIDAK_SAH"},
		{"null", "/api/periksa", `null`, nil, 400, "PERMINTAAN_TIDAK_SAH"},
		{"teks", "/api/nyalakan", `"x"`, nil, 400, "PERMINTAAN_TIDAK_SAH"},
		{"konteks bukan teks", "/api/periksa", `{"konteks":5}`, nil, 400, "PERMINTAAN_TIDAK_SAH"},
		{"terlalu besar", "/api/periksa", `{"konteks":"` + strings.Repeat("a", 5000) + `"}`, nil, 400, "PERMINTAAN_TIDAK_SAH"},
		{"konteks 256 byte", "/api/periksa", `{"konteks":"` + strings.Repeat("a", 256) + `"}`, nil, 400, "PERMINTAAN_TIDAK_SAH"},
		{"k=2", "/api/daftar", `{"k":2,"ulang":true}`, nil, 400, "PERMINTAAN_TIDAK_SAH"},
		{"ambang 0", "/api/daftar", `{"ambang":0,"ulang":true}`, nil, 400, "PERMINTAAN_TIDAK_SAH"},
		{"ambang besar", "/api/daftar", `{"ambang":70000,"ulang":true}`, nil, 400, "PERMINTAAN_TIDAK_SAH"},
		{"id jalur", "/api/daftar", `{"id_chip":"../x","ulang":true}`, nil, 400, "PERMINTAAN_TIDAK_SAH"},
		{"chip kosong", "/api/simulasi/chip", `{}`, nil, 400, "PERMINTAAN_TIDAK_SAH"},
		{"chip asing", "/api/simulasi/chip", `{"chip":"lain"}`, nil, 400, "PERMINTAAN_TIDAK_SAH"},
		{"asal lain", "/api/periksa", `{}`, map[string]string{"Origin": "http://contoh.test"}, 403, "ASAL_DITOLAK"},
		{"asal null", "/api/periksa", `{}`, map[string]string{"Origin": "null"}, 403, "ASAL_DITOLAK"},
		{"lintas situs", "/api/periksa", `{}`, map[string]string{"Sec-Fetch-Site": "cross-site"}, 403, "LINTAS_SITUS_DITOLAK"},
		{"PUF gagal", "/api/daftar", `{"ambang":900,"ulang":true}`, nil, 409, "CHIP_MENOLAK"},
	}
	for _, tc := range cases {
		r := c.do(http.MethodPost, tc.target, tc.body, tc.headers)
		if r.status != tc.status || r.errorCode(t) != tc.code {
			t.Fatalf("%s: %d %s", tc.name, r.status, r.body)
		}
	}
	for _, q := range []string{"0", "51", "x", "-1"} {
		if r := c.do(http.MethodGet, "/api/riwayat?batas="+q, "", nil); r.status != http.StatusBadRequest {
			t.Fatalf("batas=%s: %d", q, r.status)
		}
	}
	var custom verify.EnrollResult
	if r := c.post("/api/daftar", `{"ambang":72,"ulang":true}`); r.status != http.StatusOK {
		t.Fatalf("daftar dengan ambang %d %s", r.status, r.body)
	} else if r.decode(t, &custom); custom.Helper.Thresh != 72 {
		t.Fatalf("ambang permintaan %d", custom.Helper.Thresh)
	}
	if r := c.post("/api/daftar", `{"ambang":null,"ulang":true}`); r.status != http.StatusOK {
		t.Fatalf("daftar dengan ambang null %d %s", r.status, r.body)
	} else if r.decode(t, &custom); custom.Helper.Thresh != 64 {
		t.Fatalf("ambang bawaan %d", custom.Helper.Thresh)
	}
	same := c.do(http.MethodPost, "/api/periksa", `{"konteks":"sama"}`, map[string]string{"Origin": "http://localhost:8080", "Sec-Fetch-Site": "same-origin"})
	if same.status != http.StatusOK {
		t.Fatalf("asal sama ditolak %d %s", same.status, same.body)
	}
	if r := c.do(http.MethodPost, "/api/periksa", "", nil); r.status != http.StatusOK {
		t.Fatalf("isi kosong %d %s", r.status, r.body)
	}
	if r := c.do(http.MethodPost, "/api/periksa", `{}`, map[string]string{"Content-Type": "application/json; charset=utf-8"}); r.status != http.StatusOK {
		t.Fatalf("jenis isi dengan charset %d %s", r.status, r.body)
	}
}

func TestHostAllowList(t *testing.T) {
	c := newClient(t, newService(t))
	for _, host := range []string{"localhost:8080", "127.0.0.1:8080", "[::1]:8080", "LOCALHOST", "127.0.0.1"} {
		req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		c.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("host %s: %d", host, rec.Code)
		}
	}
	for _, host := range []string{"contoh.test", "contoh.test:8080", "192.168.1.5:8080", "localhost.contoh.test"} {
		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.Host = host
		rec := httptest.NewRecorder()
		c.handler.ServeHTTP(rec, req)
		if rec.Code != http.StatusMisdirectedRequest {
			t.Fatalf("host %s: %d", host, rec.Code)
		}
	}
	open := NewHandler(Options{Service: newService(t)})
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	req.Host = "192.168.1.5:8080"
	rec := httptest.NewRecorder()
	open.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("tanpa daftar host: %d", rec.Code)
	}
}

func TestMethodsAndUnknownPaths(t *testing.T) {
	c := newClient(t, newService(t))
	r := c.do(http.MethodGet, "/api/periksa", "", nil)
	if r.status != http.StatusMethodNotAllowed || r.header.Get("Allow") != "POST" || r.errorCode(t) != "METODE_TIDAK_DIIZINKAN" {
		t.Fatalf("GET periksa %d %v %s", r.status, r.header, r.body)
	}
	r = c.do(http.MethodDelete, "/api/status", "", nil)
	if r.status != http.StatusMethodNotAllowed || r.header.Get("Allow") != "GET" {
		t.Fatalf("DELETE status %d", r.status)
	}
	r = c.do(http.MethodGet, "/api/tidak-ada", "", nil)
	if r.status != http.StatusNotFound || r.errorCode(t) != "TIDAK_DITEMUKAN" {
		t.Fatalf("api tidak dikenal %d", r.status)
	}
	if r := c.do(http.MethodHead, "/api/status", "", nil); r.status != http.StatusOK {
		t.Fatalf("HEAD status %d", r.status)
	}
}

func TestStaticFiles(t *testing.T) {
	c := newClient(t, newService(t))
	r := c.do(http.MethodGet, "/", "", nil)
	if r.status != http.StatusOK || !strings.HasPrefix(r.header.Get("Content-Type"), "text/html") || !strings.Contains(string(r.body), "<html") {
		t.Fatalf("halaman utama %d %v", r.status, r.header)
	}
	if !strings.Contains(r.header.Get("Content-Security-Policy"), "frame-ancestors 'none'") || r.header.Get("X-Frame-Options") != "DENY" || r.header.Get("Cache-Control") != "no-cache" {
		t.Fatalf("header keamanan %v", r.header)
	}
	for _, p := range []string{"/tidak-ada.js", "/_app/", "/_app", "/..%2f..%2fetc/passwd", "/%2e%2e/%2e%2e/etc/passwd", "/web/index.html"} {
		if r := c.do(http.MethodGet, p, "", nil); r.status != http.StatusNotFound {
			t.Fatalf("%s: %d", p, r.status)
		}
	}
	if r := c.do(http.MethodPost, "/", `{}`, nil); r.status != http.StatusMethodNotAllowed {
		t.Fatalf("POST halaman utama %d", r.status)
	}
	var immutable string
	fsysEntries, _ := webFiles.ReadDir("web/_app/immutable/entry")
	for _, e := range fsysEntries {
		immutable = "/_app/immutable/entry/" + e.Name()
		break
	}
	if immutable != "" {
		r := c.do(http.MethodGet, immutable, "", nil)
		if r.status != http.StatusOK || !strings.Contains(r.header.Get("Cache-Control"), "immutable") {
			t.Fatalf("berkas immutable %s: %d %v", immutable, r.status, r.header)
		}
	}
}

type stubService struct {
	err   error
	panic bool
}

func (s stubService) Status(context.Context) (verify.StatusReport, error) {
	if s.panic {
		panic("uji panik")
	}
	return verify.StatusReport{}, s.err
}

func (s stubService) Enroll(context.Context, verify.EnrollRequest) (verify.EnrollResult, error) {
	return verify.EnrollResult{}, s.err
}

func (s stubService) PowerUp(context.Context) (verify.PowerUpResult, error) {
	return verify.PowerUpResult{}, s.err
}

func (s stubService) Check(context.Context, string) (verify.Result, error) {
	return verify.Result{}, s.err
}

func (s stubService) AttackReadKey(context.Context) (verify.AttackResult, error) {
	return verify.AttackResult{}, s.err
}

func (s stubService) SwitchChip(context.Context, string, bool) (verify.StatusReport, error) {
	return verify.StatusReport{}, s.err
}

func (s stubService) History(int) []verify.Result {
	return nil
}

func TestErrorMapping(t *testing.T) {
	secret := errors.New("rincian internal /rahasia/jalur")
	cases := []struct {
		err      error
		status   int
		code     string
		chipCode string
	}{
		{&verify.ChipFailure{Op: "PROVE", Err: fmt.Errorf("x: %w", chip.ErrTimeout)}, 504, "CHIP_TIDAK_MENJAWAB", ""},
		{&verify.ChipFailure{Op: "PROVE", Err: fmt.Errorf("%w: %w", chip.ErrRateLimited, &chip.CommandError{Command: chip.CmdProve, Code: chip.ErrRate})}, 503, "PEMBATAS_LAJU", "RATE"},
		{&verify.ChipFailure{Op: "PUF_ENROLL", Err: &chip.CommandError{Command: chip.CmdPUFEnroll, Code: chip.ErrPUFFail}}, 409, "CHIP_MENOLAK", "PUF_FAIL"},
		{&verify.ChipFailure{Op: "membaca status", Err: secret}, 502, "CHIP_GAGAL", ""},
		{verify.ErrNotEnrolled, 409, "BELUM_TERDAFTAR", ""},
		{fmt.Errorf("menerbitkan: %w", lms.ErrExhausted), 503, "TANDA_TANGAN_HABIS", ""},
		{verify.ErrNoIssuer, 503, "PENERBIT_TIDAK_ADA", ""},
		{verify.ErrNotSimulation, 409, "BUKAN_SIMULASI", ""},
		{fmt.Errorf("%w: data bantu 95 byte, chip dengan 768 osilator butuh 96 byte", verify.ErrHelperMismatch), 409, "TOPENG_TIDAK_SESUAI", ""},
		{context.DeadlineExceeded, 503, "DIBATALKAN", ""},
		{secret, 500, "GALAT_INTERNAL", ""},
	}
	for _, tc := range cases {
		c := newClient(t, stubService{err: tc.err})
		r := c.post("/api/periksa", `{}`)
		var body struct {
			Galat struct {
				Kode     string `json:"kode"`
				Pesan    string `json:"pesan"`
				KodeChip string `json:"kode_chip"`
			} `json:"galat"`
		}
		r.decode(t, &body)
		if r.status != tc.status || body.Galat.Kode != tc.code || body.Galat.KodeChip != tc.chipCode {
			t.Fatalf("%v: %d %s", tc.err, r.status, r.body)
		}
		if strings.Contains(string(r.body), "rahasia") {
			t.Fatalf("rincian internal bocor: %s", r.body)
		}
		if tc.code == "PEMBATAS_LAJU" && r.header.Get("Retry-After") != "1" {
			t.Fatal("Retry-After tidak ada")
		}
	}
	c := newClient(t, stubService{panic: true})
	r := c.do(http.MethodGet, "/api/status", "", nil)
	if r.status != http.StatusInternalServerError || r.errorCode(t) != "GALAT_INTERNAL" {
		t.Fatalf("panik %d %s", r.status, r.body)
	}
}

func TestLoopbackHosts(t *testing.T) {
	cases := map[string]string{"localhost:80": "localhost", "[::1]:8080": "::1", "127.0.0.1": "127.0.0.1", "Contoh.Test:1": "contoh.test"}
	for in, want := range cases {
		if got := hostOnly(in); got != want {
			t.Fatalf("%s: %s", in, got)
		}
	}
	srv := NewServer("127.0.0.1:0", http.NotFoundHandler(), 0)
	if srv.ReadHeaderTimeout == 0 || srv.ReadTimeout == 0 || srv.WriteTimeout != 45*time.Second || srv.IdleTimeout == 0 || srv.MaxHeaderBytes == 0 {
		t.Fatalf("batas waktu server %+v", srv)
	}
	if slow := NewServer("127.0.0.1:0", http.NotFoundHandler(), 222*time.Second); slow.WriteTimeout != 247*time.Second {
		t.Fatalf("batas tulis untuk permintaan 222 s: %v", slow.WriteTimeout)
	}
}
