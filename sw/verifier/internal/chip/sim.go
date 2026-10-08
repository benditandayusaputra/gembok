package chip

import (
	"context"
	"crypto/mlkem"
	"crypto/sha3"
	"fmt"
	"sync"
	"time"
)

const (
	SimCyclesProve768       = 45_400
	SimCyclesProve1024      = 65_300
	SimCyclesEnroll768      = 18_100
	SimCyclesEnroll1024     = 26_900
	SimDefaultCooldown      = 1 << 20
	simCyclesBoot           = 8_200
	simCyclesReject         = 2
	simCyclesWipe           = 8_200
	simCyclesPUFFixed       = 300
	simCyclesPUFEnrollModel = 6_000_000
	simCyclesPUFReconModel  = 5_300_000
)

type vaultKey struct {
	ek          []byte
	decapsulate func([]byte) ([]byte, error)
}

func expandVaultKey(k int, pufKey [PUFKeySize]byte) (vaultKey, error) {
	d, z := DeriveSeeds(pufKey)
	seed := append(d[:], z[:]...)
	defer clear(seed)
	switch k {
	case 3:
		dk, err := mlkem.NewDecapsulationKey768(seed)
		if err != nil {
			return vaultKey{}, err
		}
		return vaultKey{ek: dk.EncapsulationKey().Bytes(), decapsulate: dk.Decapsulate}, nil
	case 4:
		dk, err := mlkem.NewDecapsulationKey1024(seed)
		if err != nil {
			return vaultKey{}, err
		}
		return vaultKey{ek: dk.EncapsulationKey().Bytes(), decapsulate: dk.Decapsulate}, nil
	}
	return vaultKey{}, ErrUnsupportedK
}

func simProveCycles(k int) uint32 {
	if k == 4 {
		return SimCyclesProve1024
	}
	return SimCyclesProve768
}

func simEnrollCycles(k int) uint32 {
	if k == 4 {
		return SimCyclesEnroll1024
	}
	return SimCyclesEnroll768
}

type simDevice struct {
	clock          Clock
	puf            pufSource
	cooldownCycles uint32
	oscillators    int
	maskLen        int

	mem       [MemSize]byte
	param     uint32
	ctxLen    uint32
	thresh    uint32
	pufIdx    uint32
	cmdK      int
	cmdCtxLen int
	key       [PUFKeySize]byte
	pufReady  bool
	done      bool
	err       ErrCode
	cycles    uint32
	proofs    uint32
	booted    bool

	busy          bool
	busyStart     time.Time
	busyUntil     time.Time
	busyCycles    uint32
	finish        func()
	cooldownUntil time.Time
}

func newSimDevice(clock Clock, spec PUFSpec, cooldownCycles uint32, oscillators int) *simDevice {
	maskLen, ok := HelperMaskLen(oscillators)
	if !ok {
		oscillators = DefaultOscillators
		maskLen, _ = HelperMaskLen(oscillators)
	}
	s := &simDevice{
		clock:          clock,
		puf:            spec.source(oscillators),
		cooldownCycles: cooldownCycles,
		oscillators:    oscillators,
		maskLen:        maskLen,
		param:          3,
		thresh:         DefaultPUFThresh,
	}
	s.begin(clock.Now(), simCyclesBoot, simCyclesBoot, func() {
		s.booted = true
		s.done = false
	})
	return s
}

func (s *simDevice) advance() time.Time {
	now := s.clock.Now()
	if s.busy && !now.Before(s.busyUntil) {
		s.busy = false
		s.cycles = s.busyCycles
		finish := s.finish
		s.finish = nil
		if finish != nil {
			finish()
		}
	}
	return now
}

func (s *simDevice) begin(now time.Time, duration, reported uint32, finish func()) {
	s.busy = true
	s.busyStart = now
	s.busyUntil = now.Add(CyclesToDuration(duration))
	s.busyCycles = reported
	s.finish = func() {
		s.done = true
		finish()
	}
}

func (s *simDevice) reject(now time.Time, code ErrCode) {
	s.begin(now, simCyclesReject, 1, func() { s.err = code })
}

func (s *simDevice) read32(offset uint32) uint32 {
	now := s.advance()
	offset &= Span - 4
	if offset >= MemBase {
		if s.busy {
			return 0
		}
		return uint32(s.mem[(offset-MemBase)/4%MemSize])
	}
	if offset >= RegSpan {
		return 0
	}
	switch offset {
	case RegID:
		return IDValue
	case RegVersion:
		return VersionValue
	case RegStatus:
		return s.status(now)
	case RegParam:
		return s.param
	case RegCtxLen:
		return s.ctxLen
	case RegCycles:
		if s.busy {
			return uint32(now.Sub(s.busyStart)/CyclePeriod) + 1
		}
		return s.cycles
	case RegCaps:
		return Caps{PUFMode: s.puf.mode(), WinLog2: SimWinLog2, Votes: SimVotes, Oscillators: s.oscillators}.Encode()
	case RegCooldown:
		return s.cooldownLeft(now)
	case RegProofs:
		return s.proofs
	case RegPUFThresh:
		return s.thresh
	case RegPUFIdx:
		return s.pufIdx
	}
	return 0
}

func (s *simDevice) write32(offset, value uint32) {
	now := s.advance()
	offset &= Span - 4
	if s.busy {
		return
	}
	if offset >= MemBase {
		s.mem[(offset-MemBase)/4%MemSize] = byte(value)
		return
	}
	switch offset {
	case RegCtrl:
		s.dispatch(now, Command(value&0xF))
	case RegParam:
		s.param = value & 7
	case RegCtxLen:
		s.ctxLen = value & 0xFF
	case RegPUFThresh:
		s.thresh = value & 0xFFFF
	case RegPUFIdx:
		s.pufIdx = value & 0x3FF
	}
}

func (s *simDevice) status(now time.Time) uint32 {
	var st uint32
	if s.busy {
		st |= StatusBusy
	}
	if s.done {
		st |= StatusDone
	}
	if s.err != ErrOK {
		st |= StatusError
	}
	st |= uint32(s.err) << StatusErrShift
	if s.pufReady {
		st |= StatusPUFReady
	}
	if now.Before(s.cooldownUntil) {
		st |= StatusCooldown
	}
	if s.booted {
		st |= StatusBooted
	}
	return st
}

func (s *simDevice) cooldownLeft(now time.Time) uint32 {
	if !now.Before(s.cooldownUntil) {
		return 0
	}
	left := (s.cooldownUntil.Sub(now) + CyclePeriod - 1) / CyclePeriod
	if left > time.Duration(s.cooldownCycles) {
		left = time.Duration(s.cooldownCycles)
	}
	return uint32(left)
}

func (s *simDevice) dispatch(now time.Time, cmd Command) {
	s.done = false
	s.err = ErrOK
	s.cycles = 1
	s.cmdK = int(s.param)
	s.cmdCtxLen = int(s.ctxLen)
	k := s.cmdK
	switch cmd {
	case CmdEnroll, CmdProve:
		level, ok := LevelFor(k)
		switch {
		case !ok:
			s.reject(now, ErrBadParam)
		case !s.pufReady:
			s.reject(now, ErrNoKey)
		case cmd == CmdProve && now.Before(s.cooldownUntil):
			s.reject(now, ErrRate)
		case cmd == CmdEnroll:
			s.begin(now, simEnrollCycles(k), simEnrollCycles(k), func() { s.finishEnroll(level) })
		default:
			s.begin(now, simProveCycles(k), simProveCycles(k), func() { s.finishProve(level) })
		}
	case CmdPUFEnroll:
		s.forgetKey()
		s.begin(now, s.puf.enrollCycles(), s.puf.enrollCycles(), s.finishPUFEnroll)
	case CmdPUFRecon:
		s.forgetKey()
		s.begin(now, s.puf.reconCycles(), s.puf.reconCycles(), s.finishPUFRecon)
	case CmdWipe:
		s.forgetKey()
		s.begin(now, simCyclesWipe, simCyclesWipe, func() { clear(s.mem[:]) })
	default:
		s.reject(now, ErrBadCmd)
	}
}

func (s *simDevice) forgetKey() {
	clear(s.key[:])
	s.pufReady = false
}

func (s *simDevice) helperMask() []byte {
	return s.mem[AddrHelperMask : AddrHelperMask+s.maskLen]
}

func (s *simDevice) finishPUFEnroll() {
	key, ok := s.puf.enroll(uint16(s.thresh), s.helperMask())
	if !ok {
		s.err = ErrPUFFail
		return
	}
	check := HelperCheck(key, s.helperMask())
	copy(s.mem[AddrHelperChk:], check[:])
	s.key = key
	s.pufReady = true
}

func (s *simDevice) finishPUFRecon() {
	key, ok := s.puf.reconstruct(s.helperMask())
	if !ok {
		s.err = ErrPUFFail
		return
	}
	check := HelperCheck(key, s.helperMask())
	if check != [HelperChkSize]byte(s.mem[AddrHelperChk:AddrHelperChk+HelperChkSize]) {
		clear(key[:])
		s.err = ErrBadHelper
		return
	}
	s.key = key
	s.pufReady = true
}

func (s *simDevice) publishKey(level Level) (vaultKey, bool) {
	vk, err := expandVaultKey(level.K, s.key)
	if err != nil {
		s.err = ErrBadParam
		return vaultKey{}, false
	}
	copy(s.mem[AddrEK:], vk.ek)
	h := sha3.Sum256(vk.ek)
	copy(s.mem[AddrH:], h[:])
	return vk, true
}

func (s *simDevice) wipeSecretSlots() {
	clear(s.mem[SecretSlotsAddr : SecretSlotsAddr+SecretSlotsSize])
}

func (s *simDevice) finishEnroll(level Level) {
	s.publishKey(level)
	s.wipeSecretSlots()
}

func (s *simDevice) finishProve(level Level) {
	s.cooldownUntil = s.busyUntil.Add(CyclesToDuration(s.cooldownCycles))
	s.proofs++
	vk, ok := s.publishKey(level)
	if !ok {
		return
	}
	ciphertext := s.mem[AddrCT : AddrCT+level.CTSize]
	proofContext := s.mem[AddrCtx : AddrCtx+s.cmdCtxLen]
	shared, err := vk.decapsulate(ciphertext)
	if err != nil {
		s.err = ErrBadParam
		return
	}
	tag := ComputeTag(shared, proofContext)
	clear(shared)
	copy(s.mem[AddrTag:], tag[:])
	s.wipeSecretSlots()
}

type simBus struct {
	mu  sync.Mutex
	dev *simDevice
}

func (b *simBus) Read32(offset uint32) uint32 {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.dev.read32(offset)
}

func (b *simBus) Write32(offset, value uint32) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dev.write32(offset, value)
}

func (b *simBus) Close() error {
	return nil
}

func (b *simBus) plug(dev *simDevice) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.dev = dev
}

type SimConfig struct {
	Clock          Clock
	CooldownCycles uint32
	Oscillators    int
	Genuine        *PUFSpec
	Clone          *PUFSpec
	Driver         DriverOptions
}

type Sim struct {
	*Driver
	bus          *simBus
	clock        Clock
	cooldown     uint32
	oscillators  int
	specs        map[string]PUFSpec
	mu           sync.Mutex
	active       string
	selfEnrolled bool
}

func NewSim(ctx context.Context, cfg SimConfig) (*Sim, error) {
	if cfg.Clock == nil {
		cfg.Clock = SystemClock()
	}
	if cfg.CooldownCycles == 0 {
		cfg.CooldownCycles = SimDefaultCooldown
	}
	if cfg.Oscillators == 0 {
		cfg.Oscillators = DefaultOscillators
	}
	if err := (Caps{WinLog2: SimWinLog2, Votes: SimVotes, Oscillators: cfg.Oscillators}).Validate(); err != nil {
		return nil, err
	}
	genuine := ModelPUF(1)
	if cfg.Genuine != nil {
		genuine = *cfg.Genuine
	}
	clone := ModelPUF(2)
	if cfg.Clone != nil {
		clone = *cfg.Clone
	}
	opts := cfg.Driver
	opts.Kind = KindSim
	opts.SimulatedCycles = true
	opts.Clock = cfg.Clock
	s := &Sim{
		bus:         &simBus{},
		clock:       cfg.Clock,
		cooldown:    cfg.CooldownCycles,
		oscillators: cfg.Oscillators,
		specs:       map[string]PUFSpec{PersonalityGenuine: genuine, PersonalityClone: clone},
	}
	s.Driver = NewDriver(s.bus, opts)
	if err := s.Switch(ctx, PersonalityGenuine, false); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Sim) Personalities() []string {
	return []string{PersonalityGenuine, PersonalityClone}
}

func (s *Sim) Active() (string, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.active, s.selfEnrolled
}

func (s *Sim) Switch(ctx context.Context, name string, selfEnroll bool) error {
	spec, ok := s.specs[name]
	if !ok {
		return fmt.Errorf("%w: %q", ErrUnknownChip, name)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.Driver.mu.Lock()
	s.bus.plug(newSimDevice(s.clock, spec, s.cooldown, s.oscillators))
	s.Driver.mu.Unlock()
	s.active = name
	s.selfEnrolled = false
	if err := s.Driver.Probe(ctx); err != nil {
		return err
	}
	if selfEnroll {
		if _, _, err := s.Driver.PUFEnroll(ctx, DefaultPUFThresh); err != nil {
			return err
		}
		s.selfEnrolled = true
	}
	return nil
}
