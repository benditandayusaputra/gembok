package verify

import (
	"context"
	"crypto/mlkem"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"math/bits"
	"slices"
	"sync"
	"time"
	"unicode/utf8"

	"gembok/verifier/internal/chip"
	"gembok/verifier/internal/issuer"
	"gembok/verifier/internal/lms"
)

type Config struct {
	Chip            chip.Chip
	Store           Store
	Trust           lms.PublicKey
	Issuer          *issuer.Issuer
	DefaultK        int
	DefaultThresh   uint16
	PowerUpAttempts int
	Now             func() time.Time
	Logger          *slog.Logger
}

type Service struct {
	mu            sync.Mutex
	chip          chip.Chip
	store         Store
	trust         lms.PublicKey
	issuer        *issuer.Issuer
	defaultK      int
	defaultThresh uint16
	attempts      int
	now           func() time.Time
	log           *slog.Logger
	history       []Result
	nextID        int
}

func New(cfg Config) (*Service, error) {
	if cfg.Chip == nil || cfg.Store == nil {
		return nil, errors.New("layanan: chip dan penyimpanan wajib ada")
	}
	if _, ok := chip.LevelFor(cfg.DefaultK); !ok {
		return nil, chip.ErrUnsupportedK
	}
	if cfg.DefaultThresh == 0 {
		cfg.DefaultThresh = chip.DefaultPUFThresh
	}
	if cfg.PowerUpAttempts == 0 {
		cfg.PowerUpAttempts = DefaultPowerUpAttempts
	}
	if cfg.PowerUpAttempts < 1 || cfg.PowerUpAttempts > MaxPowerUpAttempts {
		return nil, fmt.Errorf("layanan: percobaan PUF_RECON harus 1 sampai %d", MaxPowerUpAttempts)
	}
	if cfg.Now == nil {
		cfg.Now = time.Now
	}
	if cfg.Logger == nil {
		cfg.Logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return &Service{
		chip:          cfg.Chip,
		store:         cfg.Store,
		trust:         cfg.Trust,
		issuer:        cfg.Issuer,
		defaultK:      cfg.DefaultK,
		defaultThresh: cfg.DefaultThresh,
		attempts:      cfg.PowerUpAttempts,
		now:           cfg.Now,
		log:           cfg.Logger,
		nextID:        1,
	}, nil
}

type powerUpOutcome struct {
	attempts int
	code     chip.ErrCode
	cycles   uint32
}

func retryablePowerUp(code chip.ErrCode) bool {
	return code == chip.ErrBadHelper || code == chip.ErrPUFFail
}

func (s *Service) powerUpLocked(ctx context.Context, helper chip.HelperData) (powerUpOutcome, error) {
	info, err := s.chip.Info(ctx)
	if err != nil {
		return powerUpOutcome{}, failure("membaca info", err)
	}
	if len(helper.Mask) != info.HelperMaskLen {
		return powerUpOutcome{}, fmt.Errorf("%w: data bantu %d byte, chip dengan %d osilator butuh %d byte", ErrHelperMismatch, len(helper.Mask), info.Oscillators, info.HelperMaskLen)
	}
	var out powerUpOutcome
	for out.attempts < s.attempts {
		out.attempts++
		cycles, err := s.chip.PUFRecon(ctx, helper)
		out.cycles = cycles
		if err == nil {
			out.code = chip.ErrOK
			return out, nil
		}
		code, ok := chip.CodeOf(err)
		if !ok {
			return out, failure("PUF_RECON", err)
		}
		out.code = code
		if !retryablePowerUp(code) {
			return out, nil
		}
		if out.attempts < s.attempts {
			s.log.Info("PUF_RECON ditolak, diulang", "percobaan", out.attempts, "maksimum", s.attempts, "kode", code.String())
		}
	}
	s.log.Warn("PUF_RECON ditolak di semua percobaan", "percobaan", out.attempts, "kode", out.code.String())
	return out, nil
}

func chipMicros(cycles uint32) float64 {
	return float64(cycles) / (chip.ClockHz / 1e6)
}

func failure(op string, err error) error {
	return &ChipFailure{Op: op, Err: err}
}

func isNotFound(err error) bool {
	return errors.Is(err, fs.ErrNotExist)
}

func encapsulate(k int, ek []byte) (shared, ciphertext []byte, err error) {
	switch k {
	case 3:
		key, err := mlkem.NewEncapsulationKey768(ek)
		if err != nil {
			return nil, nil, err
		}
		shared, ciphertext = key.Encapsulate()
		return shared, ciphertext, nil
	case 4:
		key, err := mlkem.NewEncapsulationKey1024(ek)
		if err != nil {
			return nil, nil, err
		}
		shared, ciphertext = key.Encapsulate()
		return shared, ciphertext, nil
	}
	return nil, nil, chip.ErrUnsupportedK
}

func (s *Service) loadEnrollment() (issuer.Certificate, chip.HelperData, error) {
	cert, err := s.store.LoadCertificate()
	if isNotFound(err) {
		return issuer.Certificate{}, chip.HelperData{}, ErrNotEnrolled
	}
	if err != nil {
		return issuer.Certificate{}, chip.HelperData{}, err
	}
	helper, err := s.store.LoadHelper()
	if isNotFound(err) {
		return issuer.Certificate{}, chip.HelperData{}, ErrNotEnrolled
	}
	if err != nil {
		return issuer.Certificate{}, chip.HelperData{}, err
	}
	return cert, helper, nil
}

func (s *Service) simChip() string {
	if sw, ok := s.chip.(chip.Switchable); ok {
		name, _ := sw.Active()
		return name
	}
	return ""
}

func (s *Service) record(res Result, start time.Time, verdict, reason, detail string) Result {
	res.Verdict = verdict
	res.Reason = reason
	res.Detail = detail
	res.TotalMicros = s.now().Sub(start).Microseconds()
	res.ID = s.nextID
	s.nextID++
	s.history = append(s.history, res)
	if len(s.history) > HistorySize {
		s.history = slices.Clone(s.history[len(s.history)-HistorySize:])
	}
	s.log.Info("pemeriksaan", "id", res.ID, "hasil", verdict, "alasan", reason, "chip", res.ChipID, "siklus", res.ChipCycles, "simulasi", res.SimChip)
	return res
}

func (s *Service) Check(ctx context.Context, proofContext string) (Result, error) {
	if len(proofContext) > chip.MaxContext {
		return Result{}, fmt.Errorf("%w: konteks lebih dari %d byte", ErrBadRequest, chip.MaxContext)
	}
	if !utf8.ValidString(proofContext) {
		return Result{}, fmt.Errorf("%w: konteks bukan UTF-8", ErrBadRequest)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.checkLocked(ctx, proofContext)
}

func (s *Service) checkLocked(ctx context.Context, proofContext string) (Result, error) {
	start := s.now()
	cert, helper, err := s.loadEnrollment()
	if err != nil {
		return Result{}, err
	}
	res := Result{
		Time:            start.UTC().Round(time.Millisecond),
		ChipID:          cert.ChipID,
		Parameter:       cert.Parameter,
		Context:         proofContext,
		SimulatedCycles: s.chip.SimulatedCycles(),
		Backend:         s.chip.Kind(),
		SimChip:         s.simChip(),
	}
	if err := issuer.Verify(s.trust, cert); err != nil {
		return s.record(res, start, VerdictFake, ReasonBadCertificate, "Sertifikat ditolak sebelum chip ditanya: "+err.Error()), nil
	}
	level, _ := cert.Level()
	shared, ciphertext, err := encapsulate(level.K, cert.EK)
	if err != nil {
		return s.record(res, start, VerdictFake, ReasonBadCertificate, "Kunci publik di sertifikat tidak bisa dipakai: "+err.Error()), nil
	}
	defer clear(shared)
	st, err := s.chip.Status(ctx)
	if err != nil {
		return Result{}, failure("membaca status", err)
	}
	if !st.PUFReady {
		res.AutoPowerUp = true
		out, err := s.powerUpLocked(ctx, helper)
		res.PowerUpAttempts = out.attempts
		if err != nil {
			return Result{}, err
		}
		if out.code != chip.ErrOK {
			res.ChipCode = out.code.String()
			if retryablePowerUp(out.code) {
				return s.record(res, start, VerdictFake, ReasonHelperRejected, fmt.Sprintf("Chip tidak bisa memulihkan kunci dari data bantu terdaftar dalam %d percobaan (%s): PUF chip ini bukan PUF chip yang didaftarkan.", out.attempts, out.code)), nil
			}
			return s.record(res, start, VerdictFake, ReasonChipRefused, "Chip menolak PUF_RECON: "+out.code.Meaning()), nil
		}
	}
	proof, err := s.chip.Prove(ctx, level.K, ciphertext, []byte(proofContext))
	res.RateWaits = proof.RateWaits
	if err != nil {
		code, ok := chip.CodeOf(err)
		if !ok || code == chip.ErrRate {
			return Result{}, failure("PROVE", err)
		}
		res.ChipCode = code.String()
		if code == chip.ErrNoKey {
			return s.record(res, start, VerdictFake, ReasonNoKey, "Chip tidak memegang kunci PUF, jadi tidak bisa membuka tantangan."), nil
		}
		return s.record(res, start, VerdictFake, ReasonChipRefused, "Chip menolak PROVE: "+code.Meaning()), nil
	}
	res.ChipCycles = proof.Cycles
	res.ChipMicros = chipMicros(proof.Cycles)
	res.Tag = hex.EncodeToString(proof.Tag[:])
	expected := chip.ComputeTag(shared, []byte(proofContext))
	if subtle.ConstantTimeCompare(expected[:], proof.Tag[:]) == 1 {
		return s.record(res, start, VerdictGenuine, ReasonProofMatches, "Bukti dari chip sama dengan SHA3-256 atas kunci bersama yang hanya diketahui pemeriksa dan pemegang kunci rahasia."), nil
	}
	return s.record(res, start, VerdictFake, ReasonProofMismatch, "Bukti dari chip tidak cocok: chip ini tidak memegang kunci rahasia pasangan kunci publik di sertifikat."), nil
}

func helperView(h chip.HelperData) HelperView {
	pairs := 0
	for _, b := range h.Mask {
		pairs += bits.OnesCount8(b)
	}
	return HelperView{Thresh: h.Thresh, MaskLen: len(h.Mask), Mask: hex.EncodeToString(h.Mask), Check: hex.EncodeToString(h.Check[:]), Pairs: pairs}
}

func (s *Service) Enroll(ctx context.Context, req EnrollRequest) (EnrollResult, error) {
	if s.issuer == nil {
		return EnrollResult{}, ErrNoIssuer
	}
	k := req.K
	if k == 0 {
		k = s.defaultK
	}
	if _, ok := chip.LevelFor(k); !ok {
		return EnrollResult{}, fmt.Errorf("%w: k harus 3 atau 4", ErrBadRequest)
	}
	thresh := req.Thresh
	if thresh == 0 {
		thresh = s.defaultThresh
	}
	if req.ChipID != "" && !issuer.ValidChipID(req.ChipID) {
		return EnrollResult{}, fmt.Errorf("%w: %v", ErrBadRequest, issuer.ErrBadChipID)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	start := s.now()
	if !req.Overwrite {
		if _, err := s.store.LoadCertificate(); !isNotFound(err) {
			return EnrollResult{}, ErrAlreadyEnrolled
		}
	}
	helper, pufCycles, err := s.chip.PUFEnroll(ctx, thresh)
	if err != nil {
		return EnrollResult{}, failure("PUF_ENROLL", err)
	}
	ek, enrollCycles, err := s.chip.Enroll(ctx, k)
	if err != nil {
		return EnrollResult{}, failure("ENROLL", err)
	}
	cert, err := s.issuer.Issue(req.ChipID, k, ek)
	if err != nil {
		return EnrollResult{}, fmt.Errorf("menerbitkan sertifikat: %w", err)
	}
	if err := s.store.SaveEnrollment(cert, helper); err != nil {
		return EnrollResult{}, err
	}
	s.log.Info("pendaftaran", "chip", cert.ChipID, "parameter", cert.Parameter, "sidik_ek", issuer.Fingerprint(ek), "tanda_tangan_terpakai", s.issuer.Used())
	return EnrollResult{
		Certificate:     cert,
		Helper:          helperView(helper),
		EKFingerprint:   issuer.Fingerprint(ek),
		PUFCycles:       pufCycles,
		EnrollCycles:    enrollCycles,
		EnrollMicros:    chipMicros(enrollCycles),
		SimulatedCycles: s.chip.SimulatedCycles(),
		IssuerUsed:      s.issuer.Used(),
		IssuerCapacity:  s.issuer.Capacity(),
		TotalMicros:     s.now().Sub(start).Microseconds(),
	}, nil
}

func (s *Service) PowerUp(ctx context.Context) (PowerUpResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	helper, err := s.store.LoadHelper()
	if isNotFound(err) {
		return PowerUpResult{}, ErrNotEnrolled
	}
	if err != nil {
		return PowerUpResult{}, err
	}
	out, err := s.powerUpLocked(ctx, helper)
	if err != nil {
		return PowerUpResult{}, err
	}
	res := PowerUpResult{
		SimulatedCycles: s.chip.SimulatedCycles(),
		ChipCode:        out.code.String(),
		Attempts:        out.attempts,
		MaxAttempts:     s.attempts,
		Cycles:          out.cycles,
		ChipMicros:      chipMicros(out.cycles),
	}
	if out.code != chip.ErrOK {
		res.Detail = fmt.Sprintf("Chip menolak data bantu dalam %d percobaan: %s.", out.attempts, out.code.Meaning())
		return res, nil
	}
	res.Success = true
	res.PUFReady = true
	res.Detail = fmt.Sprintf("Kunci PUF dipulihkan pada percobaan ke-%d dan cocok dengan nilai cek data bantu.", out.attempts)
	return res, nil
}

func PUFWarnings(kind string, info chip.Info) []Warning {
	out := []Warning{}
	if kind != chip.KindMMIO {
		return out
	}
	switch info.PUFMode {
	case 1:
		out = append(out, Warning{Code: WarningSimulatedPUF, Message: "Bitstream memakai model PUF simulasi (PUF_MODE 1). Kunci dihitung dari rumus yang diketahui umum, bukan dari fisik chip."})
	case 2:
		out = append(out, Warning{Code: WarningDevelopmentKey, Message: "Bitstream memakai kunci pengembangan tetap (PUF_MODE 2). Kunci ini publik, jadi hasil ASLI tidak membuktikan keaslian chip."})
	case 3:
		out = append(out, Warning{Code: WarningUnknownPUF, Message: "CAPS melaporkan mode PUF yang tidak dikenal. Anggap kunci chip tidak rahasia."})
	}
	if info.Debug {
		out = append(out, Warning{Code: WarningDebugBuild, Message: "Bangunan debug membuka PUF_MEASURE dan hitungan osilator mentah, jadi kunci bisa dibaca dari luar chip."})
	}
	return out
}

type regionSpec struct {
	name   string
	addr   int
	length int
	secret bool
}

func (s *Service) AttackReadKey(ctx context.Context) (AttackResult, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := AttackResult{Time: s.now().UTC().Round(time.Millisecond)}
	k := s.defaultK
	cert, _, err := s.loadEnrollment()
	switch {
	case err == nil:
		proof, err := s.checkLocked(ctx, AttackContext)
		if err != nil {
			return AttackResult{}, err
		}
		res.Proof = &proof
		res.ProofRan = proof.ChipCycles > 0
		if level, ok := cert.Level(); ok {
			k = level.K
		}
		if !res.ProofRan {
			res.Note = "PROVE tidak dijalankan karena pemeriksaan berhenti lebih awal (" + proof.Reason + ")."
		}
	case errors.Is(err, ErrNotEnrolled):
		res.Note = "Chip belum didaftarkan, jadi memori dibaca tanpa bukti sebelumnya."
	default:
		return AttackResult{}, err
	}
	level, _ := chip.LevelFor(k)
	specs := []regionSpec{
		{"D", chip.AddrD, chip.SlotSize, true},
		{"Z", chip.AddrZ, chip.SlotSize, true},
		{"M", chip.AddrM, chip.SlotSize, true},
		{"K", chip.AddrK, chip.SlotSize, true},
		{"SIGMA", chip.AddrSigma, chip.SlotSize, true},
		{"KBAR", chip.AddrKBar, chip.SlotSize, true},
		{"DK", chip.AddrDK, level.DKSize, true},
		{"H", chip.AddrH, chip.SlotSize, false},
		{"TAG", chip.AddrTag, chip.TagSize, false},
	}
	for _, spec := range specs {
		data, err := s.chip.ReadMem(ctx, spec.addr, spec.length)
		if err != nil {
			return AttackResult{}, failure("membaca memori", err)
		}
		nonZero := 0
		for _, b := range data {
			if b != 0 {
				nonZero++
			}
		}
		res.Regions = append(res.Regions, MemoryRegion{
			Name:    spec.name,
			Address: fmt.Sprintf("0x%04X", spec.addr),
			Length:  spec.length,
			Secret:  spec.secret,
			AllZero: nonZero == 0,
			NonZero: nonZero,
			Hex:     hex.EncodeToString(data),
		})
		if spec.secret {
			res.SecretBytes += spec.length
			res.SecretNonZero += nonZero
		} else {
			res.PublicBytesRead += spec.length
			res.PublicNonZero += nonZero
		}
	}
	res.SecretsAllZero = res.SecretNonZero == 0
	if res.SecretsAllZero {
		res.Explanation = fmt.Sprintf("Host membaca %d byte wilayah rahasia dan semuanya nol. Brankas menghapus slot D, Z, M, K, SIGMA, KBAR sebelum BUSY turun, dan kunci dekapsulasi hanya hidup di memori polinomial internal yang tidak punya alamat di bus. Wilayah publik (H, TAG) tetap terbaca, jadi jalur baca memang bekerja.", res.SecretBytes)
	} else {
		res.Explanation = fmt.Sprintf("Peringatan: %d byte wilayah rahasia tidak nol. Ini menyalahi jaminan peta register.", res.SecretNonZero)
		s.log.Warn("wilayah rahasia tidak nol", "byte", res.SecretNonZero)
	}
	return res, nil
}

func (s *Service) SwitchChip(ctx context.Context, name string, selfEnroll bool) (StatusReport, error) {
	sw, ok := s.chip.(chip.Switchable)
	if !ok {
		return StatusReport{}, ErrNotSimulation
	}
	if !slices.Contains(sw.Personalities(), name) {
		return StatusReport{}, fmt.Errorf("%w: chip harus salah satu dari %v", ErrBadRequest, sw.Personalities())
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if err := sw.Switch(ctx, name, selfEnroll); err != nil {
		return StatusReport{}, failure("mengganti chip", err)
	}
	s.log.Info("chip simulasi diganti", "chip", name, "daftar_sendiri", selfEnroll)
	return s.statusLocked(ctx)
}

func (s *Service) History(limit int) []Result {
	s.mu.Lock()
	defer s.mu.Unlock()
	if limit <= 0 || limit > len(s.history) {
		limit = len(s.history)
	}
	out := make([]Result, 0, limit)
	for i := len(s.history) - 1; i >= 0 && len(out) < limit; i-- {
		out = append(out, s.history[i])
	}
	return out
}

func (s *Service) Status(ctx context.Context) (StatusReport, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.statusLocked(ctx)
}

func pufModeName(mode int) string {
	switch mode {
	case 0:
		return "osilator cincin"
	case 1:
		return "model simulasi"
	case 2:
		return "kunci pengembangan tetap (tidak aman)"
	}
	return "tidak dikenal"
}

func (s *Service) statusLocked(ctx context.Context) (StatusReport, error) {
	info, err := s.chip.Info(ctx)
	if err != nil {
		return StatusReport{}, failure("membaca info", err)
	}
	st, err := s.chip.Status(ctx)
	if err != nil {
		return StatusReport{}, failure("membaca status", err)
	}
	level, _ := chip.LevelFor(s.defaultK)
	report := StatusReport{
		Backend:         s.chip.Kind(),
		SimulatedCycles: s.chip.SimulatedCycles(),
		ClockHz:         chip.ClockHz,
		DefaultLevel:    LevelReport{K: level.K, Name: level.Name},
		DefaultThresh:   s.defaultThresh,
		PowerUpAttempts: s.attempts,
		Warnings:        PUFWarnings(s.chip.Kind(), info),
		Time:            s.now().UTC().Round(time.Millisecond),
		Chip: ChipReport{
			ID:             fmt.Sprintf("0x%08X", info.ID),
			Version:        fmt.Sprintf("%d.%d", info.Version>>16, info.Version&0xFFFF),
			PUFMode:        info.PUFMode,
			PUFModeName:    pufModeName(info.PUFMode),
			Debug:          info.Debug,
			WinLog2:        info.WinLog2,
			Votes:          info.Votes,
			Oscillators:    info.Oscillators,
			HelperMaskLen:  info.HelperMaskLen,
			PUFTimeoutMs:   info.PUFTimeout.Milliseconds(),
			Booted:         st.Booted,
			Busy:           st.Busy,
			PUFReady:       st.PUFReady,
			Cooling:        st.Cooling,
			CooldownCycles: st.CooldownCycles,
			Proofs:         st.Proofs,
			LastError:      st.Err.String(),
			LastCycles:     st.LastCycles,
			PUFThresh:      st.PUFThresh,
		},
		Issuer: IssuerSummary{
			Available: s.issuer != nil,
			Algorithm: issuer.SignatureAlgorithm(),
			ID:        hex.EncodeToString(s.trust.ID[:]),
		},
	}
	if s.issuer != nil {
		report.Issuer.Used = s.issuer.Used()
		report.Issuer.Capacity = s.issuer.Capacity()
	}
	cert, err := s.store.LoadCertificate()
	switch {
	case err == nil:
		report.Enrolled = true
		summary := &CertificateSummary{
			ChipID:        cert.ChipID,
			Parameter:     cert.Parameter,
			IssuedAt:      cert.IssuedAt.UTC().Format(time.RFC3339),
			EKFingerprint: issuer.Fingerprint(cert.EK),
		}
		summary.LeafIndex, _ = lms.LeafIndex(cert.Signature)
		if verr := issuer.Verify(s.trust, cert); verr != nil {
			summary.Problem = verr.Error()
		} else {
			summary.Valid = true
		}
		report.Certificate = summary
	case !isNotFound(err):
		report.Enrolled = true
		report.Certificate = &CertificateSummary{Problem: err.Error()}
	}
	if sw, ok := s.chip.(chip.Switchable); ok {
		name, self := sw.Active()
		report.Simulation = &SimulationReport{Active: name, SelfEnrolled: self, Options: sw.Personalities()}
	}
	return report, nil
}
