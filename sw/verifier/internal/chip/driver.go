package chip

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"time"
)

const (
	defaultCommandTimeout = 5 * time.Second
	defaultSpinFor        = 3 * time.Millisecond
	defaultPollInterval   = 200 * time.Microsecond
	defaultRateRetries    = 3
	rateWaitMargin        = 100 * time.Microsecond
)

type DriverOptions struct {
	Kind            string
	SimulatedCycles bool
	Clock           Clock
	CommandTimeout  time.Duration
	SpinFor         time.Duration
	PollInterval    time.Duration
	RateRetries     int
}

type Driver struct {
	mu   sync.Mutex
	bus  Bus
	opts DriverOptions
}

func NewDriver(bus Bus, opts DriverOptions) *Driver {
	if opts.Clock == nil {
		opts.Clock = SystemClock()
	}
	if opts.CommandTimeout <= 0 {
		opts.CommandTimeout = defaultCommandTimeout
	}
	if opts.SpinFor <= 0 {
		opts.SpinFor = defaultSpinFor
	}
	if opts.PollInterval <= 0 {
		opts.PollInterval = defaultPollInterval
	}
	if opts.RateRetries <= 0 {
		opts.RateRetries = defaultRateRetries
	}
	return &Driver{bus: bus, opts: opts}
}

func (d *Driver) Kind() string {
	return d.opts.Kind
}

func (d *Driver) SimulatedCycles() bool {
	return d.opts.SimulatedCycles
}

func (d *Driver) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.bus.Close()
}

func (d *Driver) Probe(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if id := d.bus.Read32(RegID); id != IDValue {
		return fmt.Errorf("%w: terbaca 0x%08X, seharusnya 0x%08X", ErrNotGembok, id, uint32(IDValue))
	}
	if version := d.bus.Read32(RegVersion); version>>16 != VersionMajor {
		return fmt.Errorf("%w: terbaca 0x%08X", ErrVersion, version)
	}
	if _, err := d.readCaps(); err != nil {
		return err
	}
	return d.waitIdle(ctx)
}

func (d *Driver) readCaps() (Caps, error) {
	caps := DecodeCaps(d.bus.Read32(RegCaps))
	return caps, caps.Validate()
}

func (d *Driver) pufTimeout(caps Caps) time.Duration {
	return caps.PUFTimeout(d.opts.CommandTimeout)
}

func (d *Driver) idleTimeout() time.Duration {
	caps, err := d.readCaps()
	if err != nil {
		return d.opts.CommandTimeout
	}
	return d.pufTimeout(caps)
}

func (d *Driver) Info(ctx context.Context) (Info, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Info{}, err
	}
	caps := DecodeCaps(d.bus.Read32(RegCaps))
	return Info{
		ID:            d.bus.Read32(RegID),
		Version:       d.bus.Read32(RegVersion),
		PUFMode:       caps.PUFMode,
		Debug:         caps.Debug,
		WinLog2:       caps.WinLog2,
		Votes:         caps.Votes,
		Oscillators:   caps.Oscillators,
		HelperMaskLen: caps.MaskLen(),
		PUFTimeout:    d.pufTimeout(caps),
	}, nil
}

func (d *Driver) Status(ctx context.Context) (Status, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := ctx.Err(); err != nil {
		return Status{}, err
	}
	st := d.bus.Read32(RegStatus)
	return Status{
		Busy:           st&StatusBusy != 0,
		Done:           st&StatusDone != 0,
		Error:          st&StatusError != 0,
		Err:            statusErr(st),
		PUFReady:       st&StatusPUFReady != 0,
		Cooling:        st&StatusCooldown != 0,
		Booted:         st&StatusBooted != 0,
		CooldownCycles: d.bus.Read32(RegCooldown),
		Proofs:         d.bus.Read32(RegProofs),
		LastCycles:     d.bus.Read32(RegCycles),
		K:              int(d.bus.Read32(RegParam) & 7),
		PUFThresh:      uint16(d.bus.Read32(RegPUFThresh)),
	}, nil
}

func (d *Driver) PUFEnroll(ctx context.Context, thresh uint16) (HelperData, uint32, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	caps, err := d.readCaps()
	if err != nil {
		return HelperData{}, 0, err
	}
	if err := d.waitIdle(ctx); err != nil {
		return HelperData{}, 0, err
	}
	d.bus.Write32(RegPUFThresh, uint32(thresh))
	cycles, err := d.run(ctx, CmdPUFEnroll, d.pufTimeout(caps))
	if err != nil {
		return HelperData{}, cycles, err
	}
	helper := HelperData{Thresh: thresh, Mask: d.readBytes(AddrHelperMask, caps.MaskLen())}
	copy(helper.Check[:], d.readBytes(AddrHelperChk, HelperChkSize))
	return helper, cycles, nil
}

func (d *Driver) PUFRecon(ctx context.Context, helper HelperData) (uint32, error) {
	d.mu.Lock()
	defer d.mu.Unlock()
	caps, err := d.readCaps()
	if err != nil {
		return 0, err
	}
	if len(helper.Mask) != caps.MaskLen() {
		return 0, fmt.Errorf("%w: data bantu %d byte, chip %d byte", ErrHelperLength, len(helper.Mask), caps.MaskLen())
	}
	if err := d.waitIdle(ctx); err != nil {
		return 0, err
	}
	d.bus.Write32(RegPUFThresh, uint32(helper.Thresh))
	d.writeBytes(AddrHelperMask, helper.Mask)
	d.writeBytes(AddrHelperChk, helper.Check[:])
	return d.run(ctx, CmdPUFRecon, d.pufTimeout(caps))
}

func (d *Driver) Enroll(ctx context.Context, k int) ([]byte, uint32, error) {
	level, ok := LevelFor(k)
	if !ok {
		return nil, 0, ErrUnsupportedK
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.waitIdle(ctx); err != nil {
		return nil, 0, err
	}
	d.bus.Write32(RegParam, uint32(k))
	cycles, err := d.run(ctx, CmdEnroll, d.opts.CommandTimeout)
	if err != nil {
		return nil, cycles, err
	}
	return d.readBytes(AddrEK, level.EKSize), cycles, nil
}

func (d *Driver) Prove(ctx context.Context, k int, ciphertext, proofContext []byte) (Proof, error) {
	level, ok := LevelFor(k)
	if !ok {
		return Proof{}, ErrUnsupportedK
	}
	if len(ciphertext) != level.CTSize {
		return Proof{}, ErrBadCiphertext
	}
	if len(proofContext) > MaxContext {
		return Proof{}, ErrContextTooLong
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	var proof Proof
	for {
		if err := d.waitIdle(ctx); err != nil {
			return proof, err
		}
		d.bus.Write32(RegParam, uint32(k))
		d.bus.Write32(RegCtxLen, uint32(len(proofContext)))
		d.writeBytes(AddrCT, ciphertext)
		d.writeBytes(AddrCtx, proofContext)
		cycles, err := d.run(ctx, CmdProve, d.opts.CommandTimeout)
		if err == nil {
			proof.Cycles = cycles
			copy(proof.Tag[:], d.readBytes(AddrTag, TagSize))
			return proof, nil
		}
		if code, _ := CodeOf(err); code != ErrRate {
			return proof, err
		}
		if proof.RateWaits >= d.opts.RateRetries {
			return proof, fmt.Errorf("%w: %w", ErrRateLimited, err)
		}
		wait := CyclesToDuration(d.bus.Read32(RegCooldown)) + rateWaitMargin
		if deadline, ok := ctx.Deadline(); ok && time.Until(deadline) < wait {
			return proof, fmt.Errorf("%w: %w", context.DeadlineExceeded, err)
		}
		d.opts.Clock.Sleep(wait)
		proof.RateWaits++
		proof.RateWaited += wait
	}
}

func (d *Driver) ReadMem(ctx context.Context, addr, n int) ([]byte, error) {
	if addr < 0 || n < 0 || addr > MemSize || n > MemSize-addr {
		return nil, ErrOutOfRange
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.waitIdle(ctx); err != nil {
		return nil, err
	}
	return d.readBytes(addr, n), nil
}

func (d *Driver) Wipe(ctx context.Context) error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if err := d.waitIdle(ctx); err != nil {
		return err
	}
	_, err := d.run(ctx, CmdWipe, d.opts.CommandTimeout)
	return err
}

func (d *Driver) run(ctx context.Context, cmd Command, limit time.Duration) (uint32, error) {
	d.bus.Write32(RegCtrl, uint32(cmd))
	d.letCommandLatch()
	st, err := d.waitStatus(ctx, limit, func(st uint32) bool {
		return st&StatusBusy == 0 && st&StatusDone != 0
	})
	if err != nil {
		return 0, fmt.Errorf("menunggu %s selesai: %w", cmd, err)
	}
	cycles := d.bus.Read32(RegCycles)
	if code := statusErr(st); code != ErrOK {
		return cycles, &CommandError{Command: cmd, Code: code}
	}
	return cycles, nil
}

func (d *Driver) letCommandLatch() {
	d.bus.Read32(RegID)
}

func (d *Driver) waitIdle(ctx context.Context) error {
	_, err := d.waitStatus(ctx, d.idleTimeout(), func(st uint32) bool {
		return st&StatusBusy == 0 && st&StatusBooted != 0
	})
	if err != nil {
		return fmt.Errorf("menunggu chip siap: %w", err)
	}
	return nil
}

func (d *Driver) waitStatus(ctx context.Context, limit time.Duration, ready func(st uint32) bool) (uint32, error) {
	clock := d.opts.Clock
	start := clock.Now()
	for {
		st := d.bus.Read32(RegStatus)
		if ready(st) {
			return st, nil
		}
		if err := ctx.Err(); err != nil {
			return st, err
		}
		elapsed := clock.Now().Sub(start)
		if elapsed > limit {
			return st, ErrTimeout
		}
		if elapsed > d.opts.SpinFor {
			clock.Sleep(d.opts.PollInterval)
		}
	}
}

func (d *Driver) writeBytes(addr int, data []byte) {
	for i, b := range data {
		d.bus.Write32(MemBase+4*uint32(addr+i), uint32(b))
	}
}

func (d *Driver) readBytes(addr, n int) []byte {
	out := make([]byte, n)
	for i := range out {
		out[i] = byte(d.bus.Read32(MemBase + 4*uint32(addr+i)))
	}
	return out
}

func statusErr(st uint32) ErrCode {
	return ErrCode(st >> StatusErrShift & StatusErrMask)
}

func IsTimeout(err error) bool {
	return errors.Is(err, ErrTimeout) || errors.Is(err, context.DeadlineExceeded)
}
