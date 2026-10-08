package verify

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"gembok/verifier/internal/chip"
	"gembok/verifier/internal/issuer"
	"gembok/verifier/internal/store"
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

func issuerTemplate(t *testing.T) string {
	t.Helper()
	templateOnce.Do(func() {
		dir, err := os.MkdirTemp("", "gembok-penerbit-*")
		if err != nil {
			templateErr = err
			return
		}
		templateDir = dir
		s, err := store.Open(dir)
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
	return templateDir
}

type harness struct {
	svc   *Service
	sim   *chip.Sim
	store *store.Store
	clock *chip.ManualClock
	dir   string
}

func newHarness(t *testing.T, wrap func(chip.Chip) chip.Chip) *harness {
	t.Helper()
	return newHarnessWith(t, wrap, nil)
}

func newHarnessWith(t *testing.T, wrap func(chip.Chip) chip.Chip, tune func(*Config)) *harness {
	t.Helper()
	dir := t.TempDir()
	template := issuerTemplate(t)
	if err := os.MkdirAll(filepath.Join(dir, "penerbit"), 0o700); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"kunci-rahasia.json", "kunci-publik.json", "pohon.bin"} {
		raw, err := os.ReadFile(filepath.Join(template, "penerbit", name))
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(filepath.Join(dir, "penerbit", name), raw, 0o600); err != nil {
			t.Fatal(err)
		}
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
	if keys.Created || keys.TreeRebuilt {
		t.Fatal("templat kunci penerbit tidak terpakai")
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
	var c chip.Chip = sim
	if wrap != nil {
		c = wrap(sim)
	}
	cfg := Config{Chip: c, Store: st, Trust: trust, Issuer: iss, DefaultK: 3, Now: clock.Now}
	if tune != nil {
		tune(&cfg)
	}
	svc, err := New(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return &harness{svc: svc, sim: sim, store: st, clock: clock, dir: dir}
}

func (h *harness) enroll(t *testing.T, req EnrollRequest) EnrollResult {
	t.Helper()
	res, err := h.svc.Enroll(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func (h *harness) check(t *testing.T, proofContext string) Result {
	t.Helper()
	res, err := h.svc.Check(context.Background(), proofContext)
	if err != nil {
		t.Fatal(err)
	}
	return res
}

func (h *harness) proofs(t *testing.T) uint32 {
	t.Helper()
	st, err := h.sim.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	return st.Proofs
}

func TestGenuineChipIsASLI(t *testing.T) {
	h := newHarness(t, nil)
	enrolled := h.enroll(t, EnrollRequest{ChipID: "GEMBOK-DEMO-01"})
	if enrolled.Certificate.ChipID != "GEMBOK-DEMO-01" || enrolled.Helper.Pairs != chip.SimKeyBits || enrolled.EnrollCycles != chip.SimCyclesEnroll768 || !enrolled.SimulatedCycles {
		t.Fatalf("hasil daftar %+v", enrolled)
	}
	if enrolled.IssuerUsed != 1 || enrolled.IssuerCapacity != 1024 || enrolled.EnrollMicros != 362 {
		t.Fatalf("penerbit %d/%d, waktu %v", enrolled.IssuerUsed, enrolled.IssuerCapacity, enrolled.EnrollMicros)
	}
	res := h.check(t, "")
	if res.Verdict != VerdictGenuine || res.Reason != ReasonProofMatches {
		t.Fatalf("chip asli: %+v", res)
	}
	if res.ChipCycles != chip.SimCyclesProve768 || res.ChipMicros != 908 || !res.SimulatedCycles || res.AutoPowerUp || res.TotalMicros <= 0 {
		t.Fatalf("angka hasil %+v", res)
	}
	if res.Backend != chip.KindSim || res.SimChip != chip.PersonalityGenuine || res.ChipID != "GEMBOK-DEMO-01" || res.Parameter != "ML-KEM-768" || len(res.Tag) != 64 {
		t.Fatalf("keterangan hasil %+v", res)
	}
	second := h.check(t, "scan-001")
	if second.Verdict != VerdictGenuine || second.Context != "scan-001" || second.RateWaits != 1 {
		t.Fatalf("pemeriksaan kedua %+v", second)
	}
	history := h.svc.History(0)
	if len(history) != 2 || history[0].ID != second.ID || history[1].ID != res.ID || second.ID != res.ID+1 {
		t.Fatalf("riwayat %+v", history)
	}
	if h.proofs(t) != 2 {
		t.Fatalf("PROOFS %d", h.proofs(t))
	}
}

func TestLevelFourEndToEnd(t *testing.T) {
	h := newHarness(t, nil)
	h.enroll(t, EnrollRequest{K: 4})
	res := h.check(t, strings.Repeat("x", 255))
	if res.Verdict != VerdictGenuine || res.Parameter != "ML-KEM-1024" || res.ChipCycles != chip.SimCyclesProve1024 || res.ChipMicros != 1306 {
		t.Fatalf("k=4: %+v", res)
	}
}

func TestCloneWithCopiedHelperDataIsPALSU(t *testing.T) {
	h := newHarness(t, nil)
	h.enroll(t, EnrollRequest{})
	if _, err := h.svc.SwitchChip(context.Background(), chip.PersonalityClone, false); err != nil {
		t.Fatal(err)
	}
	res := h.check(t, "")
	if res.Verdict != VerdictFake || res.Reason != ReasonHelperRejected || res.ChipCode != "BAD_HELPER" || !res.AutoPowerUp {
		t.Fatalf("chip tiruan: %+v", res)
	}
	if res.PowerUpAttempts != DefaultPowerUpAttempts || !strings.Contains(res.Detail, "5 percobaan") {
		t.Fatalf("chip tiruan berhenti setelah %d percobaan: %s", res.PowerUpAttempts, res.Detail)
	}
	if res.SimChip != chip.PersonalityClone || res.ChipCycles != 0 {
		t.Fatalf("chip tiruan: %+v", res)
	}
	if h.proofs(t) != 0 {
		t.Fatal("PROVE dikirim ke chip yang gagal memulihkan kunci")
	}
}

func TestSelfEnrolledCloneIsPALSU(t *testing.T) {
	h := newHarness(t, nil)
	h.enroll(t, EnrollRequest{})
	status, err := h.svc.SwitchChip(context.Background(), chip.PersonalityClone, true)
	if err != nil {
		t.Fatal(err)
	}
	if status.Simulation == nil || status.Simulation.Active != chip.PersonalityClone || !status.Simulation.SelfEnrolled || !status.Chip.PUFReady {
		t.Fatalf("status setelah ganti %+v", status.Simulation)
	}
	res := h.check(t, "")
	if res.Verdict != VerdictFake || res.Reason != ReasonProofMismatch || res.AutoPowerUp || res.ChipCycles != chip.SimCyclesProve768 {
		t.Fatalf("chip tiruan yang mendaftar sendiri: %+v", res)
	}
	if _, err := h.svc.SwitchChip(context.Background(), chip.PersonalityGenuine, false); err != nil {
		t.Fatal(err)
	}
	back := h.check(t, "")
	if back.Verdict != VerdictGenuine || !back.AutoPowerUp {
		t.Fatalf("chip asli dipasang lagi: %+v", back)
	}
}

type replayChip struct {
	chip.Chip
	mu       sync.Mutex
	recorded *chip.Proof
}

func (r *replayChip) Prove(ctx context.Context, k int, ciphertext, proofContext []byte) (chip.Proof, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.recorded != nil {
		return *r.recorded, nil
	}
	proof, err := r.Chip.Prove(ctx, k, ciphertext, proofContext)
	if err == nil {
		r.recorded = &proof
	}
	return proof, err
}

func TestReplayedTagIsPALSU(t *testing.T) {
	h := newHarness(t, func(c chip.Chip) chip.Chip { return &replayChip{Chip: c} })
	h.enroll(t, EnrollRequest{})
	first := h.check(t, "gerbang-1")
	if first.Verdict != VerdictGenuine {
		t.Fatalf("pemeriksaan pertama %+v", first)
	}
	replayed := h.check(t, "gerbang-1")
	if replayed.Verdict != VerdictFake || replayed.Reason != ReasonProofMismatch || replayed.Tag != first.Tag {
		t.Fatalf("bukti lama diputar ulang: %+v", replayed)
	}
	if _, err := h.svc.SwitchChip(context.Background(), chip.PersonalityClone, false); !errors.Is(err, ErrNotSimulation) {
		t.Fatalf("chip tanpa kemampuan ganti: %v", err)
	}
}

func TestTamperedCertificateIsRejected(t *testing.T) {
	path := func(h *harness) string { return filepath.Join(h.dir, "chip", "sertifikat.json") }
	edit := func(t *testing.T, h *harness, change func(m map[string]any)) {
		raw, err := os.ReadFile(path(h))
		if err != nil {
			t.Fatal(err)
		}
		var m map[string]any
		if err := json.Unmarshal(raw, &m); err != nil {
			t.Fatal(err)
		}
		change(m)
		out, _ := json.Marshal(m)
		if err := os.WriteFile(path(h), out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	flipLastByte := func(s string) string {
		b, _ := hex.DecodeString(s)
		b[len(b)-1] ^= 1
		return hex.EncodeToString(b)
	}
	cases := map[string]func(m map[string]any){
		"id chip": func(m map[string]any) { m["id_chip"] = "GEMBOK-LAIN" },
		"rho ek":  func(m map[string]any) { m["ek"] = flipLastByte(m["ek"].(string)) },
		"waktu":   func(m map[string]any) { m["diterbitkan"] = "2030-01-01T00:00:00Z" },
		"tanda":   func(m map[string]any) { m["tanda_tangan"] = flipLastByte(m["tanda_tangan"].(string)) },
	}
	for name, change := range cases {
		t.Run(name, func(t *testing.T) {
			h := newHarness(t, nil)
			h.enroll(t, EnrollRequest{ChipID: "GEMBOK-ASLI"})
			edit(t, h, change)
			res := h.check(t, "")
			if res.Verdict != VerdictFake || res.Reason != ReasonBadCertificate {
				t.Fatalf("sertifikat diubah: %+v", res)
			}
			if h.proofs(t) != 0 {
				t.Fatal("chip ditanya walau sertifikat tidak sah")
			}
			status, err := h.svc.Status(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if status.Certificate == nil || status.Certificate.Valid || status.Certificate.Problem == "" {
				t.Fatalf("status sertifikat %+v", status.Certificate)
			}
		})
	}
	t.Run("ek chip tiruan", func(t *testing.T) {
		h := newHarness(t, nil)
		h.enroll(t, EnrollRequest{})
		if _, err := h.svc.SwitchChip(context.Background(), chip.PersonalityClone, true); err != nil {
			t.Fatal(err)
		}
		cloneEK, _, err := h.sim.Enroll(context.Background(), 3)
		if err != nil {
			t.Fatal(err)
		}
		edit(t, h, func(m map[string]any) { m["ek"] = hex.EncodeToString(cloneEK) })
		res := h.check(t, "")
		if res.Verdict != VerdictFake || res.Reason != ReasonBadCertificate {
			t.Fatalf("ek tiruan di sertifikat asli: %+v", res)
		}
	})
}

func TestNotEnrolled(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	if _, err := h.svc.Check(ctx, ""); !errors.Is(err, ErrNotEnrolled) {
		t.Fatalf("periksa sebelum daftar: %v", err)
	}
	if _, err := h.svc.PowerUp(ctx); !errors.Is(err, ErrNotEnrolled) {
		t.Fatalf("nyalakan sebelum daftar: %v", err)
	}
	attack, err := h.svc.AttackReadKey(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if attack.ProofRan || attack.Proof != nil || attack.Note == "" || !attack.SecretsAllZero {
		t.Fatalf("serang sebelum daftar %+v", attack)
	}
	status, err := h.svc.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if status.Enrolled || status.Certificate != nil || !status.Issuer.Available || status.Issuer.Used != 0 {
		t.Fatalf("status sebelum daftar %+v", status)
	}
}

func TestEnrollGuards(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	h.enroll(t, EnrollRequest{})
	if _, err := h.svc.Enroll(ctx, EnrollRequest{}); !errors.Is(err, ErrAlreadyEnrolled) {
		t.Fatalf("daftar dua kali: %v", err)
	}
	again := h.enroll(t, EnrollRequest{Overwrite: true, ChipID: "GEMBOK-ULANG"})
	if again.IssuerUsed != 2 {
		t.Fatalf("pemakaian penerbit %d", again.IssuerUsed)
	}
	bad := []EnrollRequest{{K: 2, Overwrite: true}, {K: 5, Overwrite: true}, {ChipID: "../x", Overwrite: true}, {ChipID: "a b", Overwrite: true}}
	for _, req := range bad {
		if _, err := h.svc.Enroll(ctx, req); !errors.Is(err, ErrBadRequest) {
			t.Fatalf("%+v: %v", req, err)
		}
	}
	if _, err := h.svc.Check(ctx, strings.Repeat("x", 256)); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("konteks 256 byte: %v", err)
	}
	if _, err := h.svc.Check(ctx, "\xff"); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("konteks bukan UTF-8: %v", err)
	}
	failing, err := New(Config{Chip: h.sim, Store: h.store, DefaultK: 3})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := failing.Enroll(ctx, EnrollRequest{Overwrite: true}); !errors.Is(err, ErrNoIssuer) {
		t.Fatalf("tanpa penerbit: %v", err)
	}
	if _, err := New(Config{Chip: h.sim, Store: h.store, DefaultK: 2}); err == nil {
		t.Fatal("k bawaan 2 diterima")
	}
}

func TestEnrollReportsPUFFailure(t *testing.T) {
	h := newHarness(t, nil)
	_, err := h.svc.Enroll(context.Background(), EnrollRequest{Thresh: 900})
	var cf *ChipFailure
	if !errors.As(err, &cf) {
		t.Fatalf("PUF gagal: %v", err)
	}
	if code, ok := chip.CodeOf(err); !ok || code != chip.ErrPUFFail {
		t.Fatalf("kode chip %v", err)
	}
	if _, err := h.store.LoadCertificate(); err == nil {
		t.Fatal("sertifikat tersimpan walau PUF gagal")
	}
}

func TestPowerUp(t *testing.T) {
	h := newHarness(t, nil)
	ctx := context.Background()
	h.enroll(t, EnrollRequest{})
	if _, err := h.svc.SwitchChip(ctx, chip.PersonalityGenuine, false); err != nil {
		t.Fatal(err)
	}
	ok, err := h.svc.PowerUp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !ok.Success || !ok.PUFReady || ok.ChipCode != "OK" || ok.Cycles == 0 || ok.Attempts != 1 || ok.MaxAttempts != DefaultPowerUpAttempts {
		t.Fatalf("nyalakan chip asli %+v", ok)
	}
	if _, err := h.svc.SwitchChip(ctx, chip.PersonalityClone, false); err != nil {
		t.Fatal(err)
	}
	bad, err := h.svc.PowerUp(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if bad.Success || bad.PUFReady || bad.ChipCode != "BAD_HELPER" || bad.Detail == "" || bad.Attempts != DefaultPowerUpAttempts || bad.MaxAttempts != DefaultPowerUpAttempts {
		t.Fatalf("nyalakan chip tiruan %+v", bad)
	}
}

type reconChip struct {
	chip.Chip
	mu      sync.Mutex
	calls   int
	corrupt int
	fail    error
}

func (r *reconChip) PUFRecon(ctx context.Context, helper chip.HelperData) (uint32, error) {
	r.mu.Lock()
	r.calls++
	corrupt := r.calls <= r.corrupt
	fail := r.fail
	r.mu.Unlock()
	if fail != nil {
		return 0, fail
	}
	if corrupt {
		helper.Check[0] ^= 0x01
	}
	return r.Chip.PUFRecon(ctx, helper)
}

func (r *reconChip) count() int {
	r.mu.Lock()
	defer r.mu.Unlock()
	return r.calls
}

func newReconHarness(t *testing.T, tune func(*Config)) (*harness, *reconChip) {
	t.Helper()
	var rc *reconChip
	h := newHarnessWith(t, func(c chip.Chip) chip.Chip {
		rc = &reconChip{Chip: c}
		return rc
	}, tune)
	return h, rc
}

func (h *harness) plug(t *testing.T, name string) {
	t.Helper()
	if err := h.sim.Switch(context.Background(), name, false); err != nil {
		t.Fatal(err)
	}
}

func TestGenuineChipThatFailsOnceIsASLI(t *testing.T) {
	h, rc := newReconHarness(t, nil)
	h.enroll(t, EnrollRequest{})
	h.plug(t, chip.PersonalityGenuine)
	rc.corrupt = 1
	res := h.check(t, "coba-ulang")
	if res.Verdict != VerdictGenuine || res.Reason != ReasonProofMatches || !res.AutoPowerUp || res.PowerUpAttempts != 2 || res.ChipCode != "" {
		t.Fatalf("chip asli yang gagal sekali: %+v", res)
	}
	if rc.count() != 2 || h.proofs(t) != 1 {
		t.Fatalf("PUF_RECON %d kali, PROVE %d kali", rc.count(), h.proofs(t))
	}
	again := h.check(t, "")
	if again.Verdict != VerdictGenuine || again.AutoPowerUp || again.PowerUpAttempts != 0 || rc.count() != 2 {
		t.Fatalf("pemeriksaan sesudah PUF siap: %+v", again)
	}
	h.plug(t, chip.PersonalityGenuine)
	rc.mu.Lock()
	rc.calls, rc.corrupt = 0, DefaultPowerUpAttempts-1
	rc.mu.Unlock()
	last := h.check(t, "")
	if last.Verdict != VerdictGenuine || last.PowerUpAttempts != DefaultPowerUpAttempts {
		t.Fatalf("pulih pada percobaan terakhir: %+v", last)
	}
}

func TestCloneFailsEveryAttempt(t *testing.T) {
	h, rc := newReconHarness(t, func(c *Config) { c.PowerUpAttempts = 3 })
	h.enroll(t, EnrollRequest{})
	h.plug(t, chip.PersonalityClone)
	res := h.check(t, "")
	if res.Verdict != VerdictFake || res.Reason != ReasonHelperRejected || res.PowerUpAttempts != 3 || res.ChipCode != "BAD_HELPER" {
		t.Fatalf("chip tiruan: %+v", res)
	}
	if rc.count() != 3 || h.proofs(t) != 0 {
		t.Fatalf("PUF_RECON %d kali, PROVE %d kali", rc.count(), h.proofs(t))
	}
	power, err := h.svc.PowerUp(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if power.Success || power.Attempts != 3 || power.MaxAttempts != 3 || rc.count() != 6 || !strings.Contains(power.Detail, "3 percobaan") {
		t.Fatalf("nyalakan chip tiruan: %+v setelah %d PUF_RECON", power, rc.count())
	}
	h.plug(t, chip.PersonalityGenuine)
	ok, err := h.svc.PowerUp(context.Background())
	if err != nil || !ok.Success || ok.Attempts != 1 || rc.count() != 7 {
		t.Fatalf("nyalakan chip asli: %+v (%v)", ok, err)
	}
}

func TestPowerUpStopsOnOtherErrors(t *testing.T) {
	h, rc := newReconHarness(t, nil)
	h.enroll(t, EnrollRequest{})
	h.plug(t, chip.PersonalityGenuine)
	rc.fail = &chip.CommandError{Command: chip.CmdPUFRecon, Code: chip.ErrBadCmd}
	res := h.check(t, "")
	if res.Verdict != VerdictFake || res.Reason != ReasonChipRefused || res.PowerUpAttempts != 1 || res.ChipCode != "BAD_CMD" || rc.count() != 1 {
		t.Fatalf("kode yang tidak diulang: %+v", res)
	}
	rc.fail = errors.New("bus terputus")
	_, err := h.svc.Check(context.Background(), "")
	var cf *ChipFailure
	if !errors.As(err, &cf) || cf.Op != "PUF_RECON" || rc.count() != 2 {
		t.Fatalf("galat bus: %v setelah %d PUF_RECON", err, rc.count())
	}
	if _, err := h.svc.PowerUp(context.Background()); !errors.As(err, &cf) || rc.count() != 3 {
		t.Fatalf("galat bus saat nyalakan: %v", err)
	}
}

func TestPowerUpAttemptsConfig(t *testing.T) {
	h := newHarness(t, nil)
	for _, n := range []int{-1, MaxPowerUpAttempts + 1} {
		if _, err := New(Config{Chip: h.sim, Store: h.store, DefaultK: 3, PowerUpAttempts: n}); err == nil {
			t.Fatalf("percobaan %d diterima", n)
		}
	}
	svc, err := New(Config{Chip: h.sim, Store: h.store, DefaultK: 3, PowerUpAttempts: MaxPowerUpAttempts})
	if err != nil || svc.attempts != MaxPowerUpAttempts {
		t.Fatalf("percobaan maksimum: %v", err)
	}
}

func TestHelperMaskMismatchIsRefused(t *testing.T) {
	h, rc := newReconHarness(t, nil)
	h.enroll(t, EnrollRequest{})
	cert, err := h.store.LoadCertificate()
	if err != nil {
		t.Fatal(err)
	}
	helper, err := h.store.LoadHelper()
	if err != nil {
		t.Fatal(err)
	}
	short := chip.HelperData{Thresh: helper.Thresh, Mask: helper.Mask[:len(helper.Mask)-1], Check: helper.Check}
	if err := h.store.SaveEnrollment(cert, short); err != nil {
		t.Fatal(err)
	}
	h.plug(t, chip.PersonalityGenuine)
	ctx := context.Background()
	if _, err := h.svc.Check(ctx, ""); !errors.Is(err, ErrHelperMismatch) || !strings.Contains(err.Error(), "95 byte") {
		t.Fatalf("periksa dengan topeng 95 byte: %v", err)
	}
	if _, err := h.svc.PowerUp(ctx); !errors.Is(err, ErrHelperMismatch) {
		t.Fatalf("nyalakan dengan topeng 95 byte: %v", err)
	}
	if rc.count() != 0 {
		t.Fatalf("PUF_RECON dikirim %d kali dengan topeng yang salah panjang", rc.count())
	}
}

func TestEnrollUsesDefaultThreshold(t *testing.T) {
	h := newHarnessWith(t, nil, func(c *Config) { c.DefaultThresh = 80 })
	enrolled := h.enroll(t, EnrollRequest{})
	if enrolled.Helper.Thresh != 80 || enrolled.Helper.MaskLen != 96 || len(enrolled.Helper.Mask) != 192 {
		t.Fatalf("data bantu %+v", enrolled.Helper)
	}
	helper, err := h.store.LoadHelper()
	if err != nil || helper.Thresh != 80 {
		t.Fatalf("ambang tersimpan %d (%v)", helper.Thresh, err)
	}
	explicit := h.enroll(t, EnrollRequest{Thresh: 70, Overwrite: true})
	if explicit.Helper.Thresh != 70 {
		t.Fatalf("ambang permintaan diabaikan: %d", explicit.Helper.Thresh)
	}
	status, err := h.svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.DefaultThresh != 80 || status.Chip.PUFThresh != 70 {
		t.Fatalf("ambang bawaan %d, ambang chip %d", status.DefaultThresh, status.Chip.PUFThresh)
	}
	plain := newHarness(t, nil)
	if got := plain.enroll(t, EnrollRequest{}); got.Helper.Thresh != chip.DefaultPUFThresh {
		t.Fatalf("ambang bawaan layanan %d", got.Helper.Thresh)
	}
}

type modeChip struct {
	chip.Chip
	kind  string
	mode  int
	debug bool
}

func (m *modeChip) Kind() string {
	return m.kind
}

func (m *modeChip) Info(ctx context.Context) (chip.Info, error) {
	info, err := m.Chip.Info(ctx)
	info.PUFMode = m.mode
	info.Debug = m.debug
	return info, err
}

func warningCodes(ws []Warning) []string {
	out := []string{}
	for _, w := range ws {
		if w.Message == "" {
			return nil
		}
		out = append(out, w.Code)
	}
	return out
}

func TestPUFModeWarnings(t *testing.T) {
	cases := []struct {
		kind  string
		mode  int
		debug bool
		want  []string
	}{
		{chip.KindMMIO, 0, false, []string{}},
		{chip.KindMMIO, 1, false, []string{WarningSimulatedPUF}},
		{chip.KindMMIO, 2, false, []string{WarningDevelopmentKey}},
		{chip.KindMMIO, 3, false, []string{WarningUnknownPUF}},
		{chip.KindMMIO, 0, true, []string{WarningDebugBuild}},
		{chip.KindMMIO, 2, true, []string{WarningDevelopmentKey, WarningDebugBuild}},
		{chip.KindSim, 1, false, []string{}},
		{chip.KindSim, 2, true, []string{}},
	}
	for _, c := range cases {
		h := newHarness(t, func(inner chip.Chip) chip.Chip {
			return &modeChip{Chip: inner, kind: c.kind, mode: c.mode, debug: c.debug}
		})
		status, err := h.svc.Status(context.Background())
		if err != nil {
			t.Fatal(err)
		}
		if got := warningCodes(status.Warnings); !slices.Equal(got, c.want) {
			t.Fatalf("%s mode %d debug %v: peringatan %v, harus %v", c.kind, c.mode, c.debug, got, c.want)
		}
		raw, _ := json.Marshal(status)
		if !bytes.Contains(raw, []byte(`"peringatan":[`)) {
			t.Fatalf("peringatan tidak berupa larik: %s", raw)
		}
	}
}

func TestAttackReadKeyShowsOnlyZeros(t *testing.T) {
	h := newHarness(t, nil)
	h.enroll(t, EnrollRequest{})
	attack, err := h.svc.AttackReadKey(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if !attack.ProofRan || attack.Proof == nil || attack.Proof.Verdict != VerdictGenuine || attack.Proof.Context != AttackContext {
		t.Fatalf("bukti sebelum membaca %+v", attack.Proof)
	}
	if len(attack.Regions) != 9 || !attack.SecretsAllZero || attack.SecretNonZero != 0 || attack.SecretBytes != 6*32+1152 {
		t.Fatalf("wilayah %+v", attack)
	}
	if attack.PublicBytesRead != 64 || attack.PublicNonZero == 0 || attack.Explanation == "" {
		t.Fatalf("wilayah publik tidak terbaca %+v", attack)
	}
	for _, r := range attack.Regions {
		data, _ := hex.DecodeString(r.Hex)
		if len(data) != r.Length {
			t.Fatalf("%s: %d byte hex untuk panjang %d", r.Name, len(data), r.Length)
		}
		if r.Secret && (!r.AllZero || !bytes.Equal(data, make([]byte, r.Length))) {
			t.Fatalf("%s tidak nol", r.Name)
		}
	}
	if r := attack.Regions[8]; r.Name != "TAG" || r.Address != "0x18E0" || r.Hex != attack.Proof.Tag {
		t.Fatalf("TAG yang terbaca %+v", r)
	}
	if history := h.svc.History(1); len(history) != 1 || history[0].Context != AttackContext {
		t.Fatalf("riwayat %+v", history)
	}
}

func TestStatusReport(t *testing.T) {
	h := newHarness(t, nil)
	enrolled := h.enroll(t, EnrollRequest{ChipID: "GEMBOK-STATUS"})
	h.check(t, "")
	status, err := h.svc.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if status.Backend != "sim" || !status.SimulatedCycles || status.ClockHz != 50_000_000 || status.DefaultLevel.Name != "ML-KEM-768" {
		t.Fatalf("status umum %+v", status)
	}
	if status.Chip.ID != "0x47454D42" || status.Chip.Version != "1.0" || status.Chip.PUFMode != 1 || !status.Chip.PUFReady || status.Chip.Proofs != 1 || !status.Chip.Booted {
		t.Fatalf("status chip %+v", status.Chip)
	}
	if !status.Enrolled || status.Certificate == nil || !status.Certificate.Valid || status.Certificate.ChipID != "GEMBOK-STATUS" || status.Certificate.EKFingerprint != enrolled.EKFingerprint {
		t.Fatalf("status sertifikat %+v", status.Certificate)
	}
	if status.Issuer.Used != 1 || status.Issuer.Capacity != 1024 || status.Issuer.Algorithm != "LMS_SHA256_M32_H10/LMOTS_SHA256_N32_W8" || len(status.Issuer.ID) != 32 {
		t.Fatalf("status penerbit %+v", status.Issuer)
	}
	if status.Simulation == nil || status.Simulation.Active != "asli" || len(status.Simulation.Options) != 2 {
		t.Fatalf("status simulasi %+v", status.Simulation)
	}
	if status.Chip.Oscillators != 768 || status.Chip.HelperMaskLen != 96 || status.DefaultThresh != chip.DefaultPUFThresh || status.PowerUpAttempts != DefaultPowerUpAttempts || len(status.Warnings) != 0 {
		t.Fatalf("osilator %d, topeng %d, ambang %d, percobaan %d, peringatan %v", status.Chip.Oscillators, status.Chip.HelperMaskLen, status.DefaultThresh, status.PowerUpAttempts, status.Warnings)
	}
	if status.Chip.WinLog2 != chip.SimWinLog2 || status.Chip.Votes != chip.SimVotes || status.Chip.PUFTimeoutMs != 5000 {
		t.Fatalf("WIN_LOG2 %d, suara %d, batas waktu PUF %d ms", status.Chip.WinLog2, status.Chip.Votes, status.Chip.PUFTimeoutMs)
	}
	if _, err := h.svc.SwitchChip(context.Background(), "lain", false); !errors.Is(err, ErrBadRequest) {
		t.Fatalf("nama chip tidak dikenal: %v", err)
	}
}

func TestHistoryKeepsNewestFifty(t *testing.T) {
	h := newHarness(t, nil)
	h.enroll(t, EnrollRequest{})
	var last Result
	for i := 0; i < HistorySize+5; i++ {
		last = h.check(t, "")
	}
	all := h.svc.History(0)
	if len(all) != HistorySize || all[0].ID != last.ID || all[HistorySize-1].ID != last.ID-HistorySize+1 {
		t.Fatalf("riwayat %d, id %d..%d", len(all), all[0].ID, all[len(all)-1].ID)
	}
	if few := h.svc.History(3); len(few) != 3 || few[0].ID != last.ID {
		t.Fatalf("riwayat terbatas %d", len(few))
	}
}
