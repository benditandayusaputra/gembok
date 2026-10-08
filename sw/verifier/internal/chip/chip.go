package chip

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	KindSim  = "sim"
	KindMMIO = "mmio"

	PersonalityGenuine = "asli"
	PersonalityClone   = "tiruan"
)

var (
	ErrTimeout        = errors.New("chip: batas waktu menunggu chip habis")
	ErrNotGembok      = errors.New("chip: register ID bukan milik GEMBOK")
	ErrVersion        = errors.New("chip: versi IP tidak didukung")
	ErrUnsupportedK   = errors.New("chip: nilai k harus 3 atau 4")
	ErrBadCiphertext  = errors.New("chip: panjang ciphertext tidak sesuai tingkat")
	ErrContextTooLong = errors.New("chip: konteks lebih dari 255 byte")
	ErrOutOfRange     = errors.New("chip: alamat memori byte di luar jangkauan")
	ErrUnknownBackend = errors.New("chip: backend tidak dikenal")
	ErrNoMMIO         = errors.New("chip: backend mmio hanya tersedia di Linux")
	ErrUnknownChip    = errors.New("chip: nama chip simulasi tidak dikenal")
	ErrRateLimited    = errors.New("chip: pembatas laju tetap aktif setelah beberapa kali menunggu")
	ErrBadCaps        = errors.New("chip: isi CAPS di luar rentang yang didukung")
	ErrHelperLength   = errors.New("chip: panjang topeng data bantu tidak sesuai dengan jumlah osilator chip")
)

type CommandError struct {
	Command Command
	Code    ErrCode
}

func (e *CommandError) Error() string {
	return fmt.Sprintf("chip: perintah %s gagal dengan %s (%s)", e.Command, e.Code, e.Code.Meaning())
}

func CodeOf(err error) (ErrCode, bool) {
	var ce *CommandError
	if errors.As(err, &ce) {
		return ce.Code, true
	}
	return ErrOK, false
}

type Info struct {
	ID            uint32
	Version       uint32
	PUFMode       int
	Debug         bool
	WinLog2       int
	Votes         int
	Oscillators   int
	HelperMaskLen int
	PUFTimeout    time.Duration
}

type Status struct {
	Busy           bool
	Done           bool
	Error          bool
	Err            ErrCode
	PUFReady       bool
	Cooling        bool
	Booted         bool
	CooldownCycles uint32
	Proofs         uint32
	LastCycles     uint32
	K              int
	PUFThresh      uint16
}

type HelperData struct {
	Thresh uint16
	Mask   []byte
	Check  [HelperChkSize]byte
}

type Proof struct {
	Tag        [TagSize]byte
	Cycles     uint32
	RateWaits  int
	RateWaited time.Duration
}

type Chip interface {
	Kind() string
	SimulatedCycles() bool
	Info(ctx context.Context) (Info, error)
	Status(ctx context.Context) (Status, error)
	PUFEnroll(ctx context.Context, thresh uint16) (HelperData, uint32, error)
	PUFRecon(ctx context.Context, helper HelperData) (uint32, error)
	Enroll(ctx context.Context, k int) ([]byte, uint32, error)
	Prove(ctx context.Context, k int, ciphertext, proofContext []byte) (Proof, error)
	ReadMem(ctx context.Context, addr, n int) ([]byte, error)
	Wipe(ctx context.Context) error
	Close() error
}

type Switchable interface {
	Personalities() []string
	Active() (name string, selfEnrolled bool)
	Switch(ctx context.Context, name string, selfEnroll bool) error
}

type Bus interface {
	Read32(offset uint32) uint32
	Write32(offset uint32, value uint32)
	Close() error
}

type Clock interface {
	Now() time.Time
	Sleep(d time.Duration)
}

type systemClock struct{}

func (systemClock) Now() time.Time {
	return time.Now()
}

func (systemClock) Sleep(d time.Duration) {
	time.Sleep(d)
}

func SystemClock() Clock {
	return systemClock{}
}

type ManualClock struct {
	mu   sync.Mutex
	now  time.Time
	step time.Duration
}

func NewManualClock(step time.Duration) *ManualClock {
	return &ManualClock{now: time.Unix(1_800_000_000, 0), step: step}
}

func (c *ManualClock) Now() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	now := c.now
	c.now = c.now.Add(c.step)
	return now
}

func (c *ManualClock) Sleep(d time.Duration) {
	c.Advance(d)
}

func (c *ManualClock) Advance(d time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if d > 0 {
		c.now = c.now.Add(d)
	}
}

func (c *ManualClock) Peek() time.Time {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.now
}

type Config struct {
	Backend string
	Base    uint64
	Clock   Clock
	Sim     SimConfig
}

var mmioOpener func(base uint64) (Bus, error)

func Open(ctx context.Context, cfg Config) (Chip, error) {
	clock := cfg.Clock
	if clock == nil {
		clock = SystemClock()
	}
	switch cfg.Backend {
	case KindSim:
		sim := cfg.Sim
		if sim.Clock == nil {
			sim.Clock = clock
		}
		return NewSim(ctx, sim)
	case KindMMIO:
		if mmioOpener == nil {
			return nil, ErrNoMMIO
		}
		bus, err := mmioOpener(cfg.Base)
		if err != nil {
			return nil, err
		}
		driver := NewDriver(bus, DriverOptions{Kind: KindMMIO, Clock: clock})
		if err := driver.Probe(ctx); err != nil {
			bus.Close()
			return nil, err
		}
		return driver, nil
	}
	return nil, fmt.Errorf("%w: %q", ErrUnknownBackend, cfg.Backend)
}
