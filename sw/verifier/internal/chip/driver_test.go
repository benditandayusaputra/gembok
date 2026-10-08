package chip

import (
	"context"
	"errors"
	"testing"
	"time"
)

type scriptedBus struct {
	id       uint32
	version  uint32
	caps     uint32
	status   func() uint32
	cooldown uint32
	writes   []uint32
	onCtrl   func(value uint32)
}

var (
	boardCaps   = Caps{WinLog2: 12, Votes: 5, Oscillators: 768}
	slowestCaps = Caps{WinLog2: 16, Votes: 15, Oscillators: 1025}
)

func (b *scriptedBus) Read32(offset uint32) uint32 {
	switch offset {
	case RegID:
		return b.id
	case RegVersion:
		return b.version
	case RegStatus:
		return b.status()
	case RegCooldown:
		return b.cooldown
	case RegCaps:
		return b.caps
	}
	return 0
}

func (b *scriptedBus) Write32(offset, value uint32) {
	if offset == RegCtrl {
		b.writes = append(b.writes, value)
		if b.onCtrl != nil {
			b.onCtrl(value)
		}
	}
}

func (b *scriptedBus) Close() error {
	return nil
}

func newScripted(status func() uint32) *scriptedBus {
	return &scriptedBus{id: IDValue, version: VersionValue, caps: boardCaps.Encode(), status: status}
}

func TestProbeRejectsForeignDevice(t *testing.T) {
	ctx := context.Background()
	idle := func() uint32 { return StatusBooted }
	clock := NewManualClock(time.Microsecond)
	foreign := newScripted(idle)
	foreign.id = 0xDEADBEEF
	if err := NewDriver(foreign, DriverOptions{Clock: clock}).Probe(ctx); !errors.Is(err, ErrNotGembok) {
		t.Fatalf("ID asing: %v", err)
	}
	future := newScripted(idle)
	future.version = 0x00020000
	if err := NewDriver(future, DriverOptions{Clock: clock}).Probe(ctx); !errors.Is(err, ErrVersion) {
		t.Fatalf("versi 2: %v", err)
	}
	for _, oscillators := range []int{0, 1, 1026, 4095} {
		odd := newScripted(idle)
		odd.caps = Caps{WinLog2: 12, Votes: 5, Oscillators: oscillators}.Encode()
		if err := NewDriver(odd, DriverOptions{Clock: clock}).Probe(ctx); !errors.Is(err, ErrBadCaps) {
			t.Fatalf("CAPS dengan %d osilator: %v", oscillators, err)
		}
	}
	for _, winLog2 := range []int{0, 21, 31} {
		odd := newScripted(idle)
		odd.caps = Caps{WinLog2: winLog2, Votes: 5, Oscillators: 768}.Encode()
		if err := NewDriver(odd, DriverOptions{Clock: clock}).Probe(ctx); !errors.Is(err, ErrBadCaps) {
			t.Fatalf("CAPS dengan WIN_LOG2 %d: %v", winLog2, err)
		}
	}
	for _, winLog2 := range []int{1, 20} {
		edge := newScripted(idle)
		edge.caps = Caps{WinLog2: winLog2, Votes: 5, Oscillators: 768}.Encode()
		if err := NewDriver(edge, DriverOptions{Clock: clock}).Probe(ctx); err != nil {
			t.Fatalf("CAPS dengan WIN_LOG2 %d ditolak: %v", winLog2, err)
		}
	}
	if err := NewDriver(newScripted(idle), DriverOptions{Clock: clock}).Probe(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestHelperMaskLength(t *testing.T) {
	cases := map[int]int{2: 1, 9: 1, 10: 2, 768: 96, 769: 96, 770: 97, 1024: 128, 1025: 128}
	for oscillators, want := range cases {
		if got, ok := HelperMaskLen(oscillators); !ok || got != want {
			t.Fatalf("%d osilator: %d byte, harap %d", oscillators, got, want)
		}
	}
	for _, oscillators := range []int{0, 1, 1026} {
		if _, ok := HelperMaskLen(oscillators); ok {
			t.Fatalf("%d osilator diterima", oscillators)
		}
	}
	if HelperMaskMaxSize != 128 || AddrHelperMask+HelperMaskMaxSize != AddrHelperChk {
		t.Fatal("topeng terpanjang tidak pas sebelum HELPER_CHK")
	}
	clock := NewManualClock(time.Microsecond)
	bus := newScripted(func() uint32 { return StatusBooted })
	d := NewDriver(bus, DriverOptions{Clock: clock})
	for _, n := range []int{0, 95, 97, 128} {
		_, err := d.PUFRecon(context.Background(), HelperData{Mask: make([]byte, n)})
		if !errors.Is(err, ErrHelperLength) {
			t.Fatalf("topeng %d byte: %v", n, err)
		}
	}
	if len(bus.writes) != 0 {
		t.Fatalf("PUF_RECON dikirim dengan topeng salah panjang: %v", bus.writes)
	}
}

func TestDriverTimesOutOnStuckChip(t *testing.T) {
	clock := NewManualClock(time.Microsecond)
	stuck := newScripted(func() uint32 { return StatusBooted | StatusBusy })
	d := NewDriver(stuck, DriverOptions{Clock: clock, CommandTimeout: 50 * time.Millisecond})
	start := clock.Peek()
	_, _, err := d.Enroll(context.Background(), 3)
	if !errors.Is(err, ErrTimeout) || !IsTimeout(err) {
		t.Fatalf("chip macet: %v", err)
	}
	if waited := clock.Peek().Sub(start); waited < 50*time.Millisecond {
		t.Fatalf("hanya menunggu %v", waited)
	}
	never := newScripted(func() uint32 { return StatusBooted })
	d = NewDriver(never, DriverOptions{Clock: clock, CommandTimeout: 20 * time.Millisecond})
	if _, err := d.PUFRecon(context.Background(), HelperData{Mask: make([]byte, 96)}); !errors.Is(err, ErrTimeout) {
		t.Fatalf("DONE tidak pernah naik: %v", err)
	}
	if len(never.writes) != 1 || never.writes[0] != uint32(CmdPUFRecon) {
		t.Fatalf("perintah tertulis %v", never.writes)
	}
}

func TestDriverHonoursContext(t *testing.T) {
	clock := NewManualClock(time.Microsecond)
	stuck := newScripted(func() uint32 { return StatusBooted | StatusBusy })
	d := NewDriver(stuck, DriverOptions{Clock: clock})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, _, err := d.Enroll(ctx, 3); !errors.Is(err, context.Canceled) {
		t.Fatalf("konteks dibatalkan: %v", err)
	}
	if _, err := d.Status(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("status dengan konteks dibatalkan: %v", err)
	}
}

func TestDriverGivesUpOnPersistentRateLimit(t *testing.T) {
	clock := NewManualClock(time.Microsecond)
	rate := uint32(StatusBooted | StatusDone | StatusError | uint32(ErrRate)<<StatusErrShift)
	limited := newScripted(func() uint32 { return rate })
	limited.cooldown = 50_000
	d := NewDriver(limited, DriverOptions{Clock: clock, RateRetries: 2})
	ct := make([]byte, 1088)
	proof, err := d.Prove(context.Background(), 3, ct, nil)
	if !errors.Is(err, ErrRateLimited) || !isCode(err, ErrRate) {
		t.Fatalf("pembatas laju terus aktif: %v", err)
	}
	if proof.RateWaits != 2 || len(limited.writes) != 3 {
		t.Fatalf("menunggu %d kali, %d perintah", proof.RateWaits, len(limited.writes))
	}
	if want := 2 * (CyclesToDuration(50_000) + rateWaitMargin); proof.RateWaited != want {
		t.Fatalf("total tunggu %v, harap %v", proof.RateWaited, want)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Microsecond)
	defer cancel()
	limited.writes = nil
	if _, err := d.Prove(ctx, 3, ct, nil); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("tenggat lebih pendek dari COOLDOWN: %v", err)
	}
	if len(limited.writes) != 1 {
		t.Fatalf("%d perintah dikirim walau tenggat tidak cukup", len(limited.writes))
	}
}

func TestDriverValidatesInputsBeforeTouchingChip(t *testing.T) {
	clock := NewManualClock(time.Microsecond)
	bus := newScripted(func() uint32 { return StatusBooted })
	d := NewDriver(bus, DriverOptions{Clock: clock})
	ctx := context.Background()
	if _, err := d.Prove(ctx, 2, make([]byte, 768), nil); !errors.Is(err, ErrUnsupportedK) {
		t.Fatalf("k=2: %v", err)
	}
	if _, err := d.Prove(ctx, 3, make([]byte, 1087), nil); !errors.Is(err, ErrBadCiphertext) {
		t.Fatalf("ciphertext pendek: %v", err)
	}
	if _, err := d.Prove(ctx, 4, make([]byte, 1088), nil); !errors.Is(err, ErrBadCiphertext) {
		t.Fatalf("ciphertext k=3 untuk k=4: %v", err)
	}
	if _, err := d.Prove(ctx, 3, make([]byte, 1088), make([]byte, 256)); !errors.Is(err, ErrContextTooLong) {
		t.Fatalf("konteks 256 byte: %v", err)
	}
	if _, _, err := d.Enroll(ctx, 5); !errors.Is(err, ErrUnsupportedK) {
		t.Fatalf("k=5: %v", err)
	}
	for _, r := range [][2]int{{-1, 1}, {0, -1}, {8190, 3}, {MemSize + 1, 0}, {0, MemSize + 1}} {
		if _, err := d.ReadMem(ctx, r[0], r[1]); !errors.Is(err, ErrOutOfRange) {
			t.Fatalf("baca %v: %v", r, err)
		}
	}
	if len(bus.writes) != 0 {
		t.Fatalf("perintah terkirim untuk masukan salah: %v", bus.writes)
	}
	for _, r := range [][2]int{{0, MemSize}, {MemSize - 1, 1}, {MemSize, 0}} {
		if _, err := d.ReadMem(ctx, r[0], r[1]); err != nil {
			t.Fatalf("baca %v: %v", r, err)
		}
	}
}

func TestCommandErrorText(t *testing.T) {
	err := error(&CommandError{Command: CmdPUFRecon, Code: ErrBadHelper})
	if code, ok := CodeOf(err); !ok || code != ErrBadHelper {
		t.Fatal("kode galat tidak terbaca")
	}
	if _, ok := CodeOf(errors.New("lain")); ok {
		t.Fatal("galat biasa dianggap galat chip")
	}
	if err.Error() == "" || ErrCode(42).String() != "ERR(42)" || Command(13).String() != "CMD(13)" {
		t.Fatal("teks galat")
	}
	for code := ErrOK; code <= ErrPUFFail; code++ {
		if code.Meaning() == "kode galat tidak dikenal" {
			t.Fatalf("kode %d tanpa arti", code)
		}
	}
}

func TestDecodeCaps(t *testing.T) {
	board := DecodeCaps(0x03000560)
	if board != boardCaps || board.Encode() != 0x03000560 || board.Validate() != nil || board.MaskLen() != 96 {
		t.Fatalf("CAPS bawaan papan terbaca %+v", board)
	}
	if got := DecodeCaps(0xF0000000 | board.Encode()); got != board {
		t.Fatalf("bit 31..28 ikut terbaca: %+v", got)
	}
	cases := []Caps{
		{PUFMode: 2, Debug: true, WinLog2: 4, Votes: 1, Oscillators: 600},
		{PUFMode: 1, WinLog2: 16, Votes: 15, Oscillators: 1025},
		{PUFMode: 0, WinLog2: 1, Votes: 3, Oscillators: 2},
		{PUFMode: 3, Debug: true, WinLog2: 20, Votes: 255, Oscillators: 801},
	}
	for _, c := range cases {
		if got := DecodeCaps(c.Encode()); got != c || got.Validate() != nil {
			t.Fatalf("%+v terbaca %+v", c, got)
		}
	}
	for _, winLog2 := range []int{4, 12, 16} {
		bus := newScripted(func() uint32 { return StatusBooted })
		bus.caps = Caps{PUFMode: 2, WinLog2: winLog2, Votes: 7, Oscillators: 640}.Encode()
		info, err := NewDriver(bus, DriverOptions{Clock: NewManualClock(time.Microsecond)}).Info(context.Background())
		if err != nil || info.WinLog2 != winLog2 || info.Votes != 7 || info.Oscillators != 640 || info.PUFMode != 2 || info.HelperMaskLen != 80 {
			t.Fatalf("WIN_LOG2 %d: info %+v (%v)", winLog2, info, err)
		}
		if want := PUFTimeout(640, 7, winLog2, 5*time.Second); info.PUFTimeout != want {
			t.Fatalf("WIN_LOG2 %d: batas waktu %v, harap %v", winLog2, info.PUFTimeout, want)
		}
	}
	for _, winLog2 := range []int{0, 21, 31} {
		c := DecodeCaps(Caps{WinLog2: winLog2, Votes: 5, Oscillators: 768}.Encode())
		if err := c.Validate(); !errors.Is(err, ErrBadCaps) {
			t.Fatalf("WIN_LOG2 %d diterima: %v", winLog2, err)
		}
	}
}

func TestPUFTimeout(t *testing.T) {
	if got := PUFBudgetCycles(768, 5, 12); got != 15_943_152 {
		t.Fatalf("anggaran siklus bawaan %d", got)
	}
	if got := PUFTimeout(768, 5, 12, 0); got != 1_637_726_080*time.Nanosecond {
		t.Fatalf("batas waktu bawaan tanpa lantai %v", got)
	}
	if got := boardCaps.PUFTimeout(5 * time.Second); got != 5*time.Second {
		t.Fatalf("batas waktu bawaan %v, harus 5 s", got)
	}
	if got := PUFBudgetCycles(1025, 15, 16); got != 1_007_240_864 {
		t.Fatalf("anggaran siklus (1025, 15, 16) %d", got)
	}
	if got := slowestCaps.PUFTimeout(5 * time.Second); got != 41_289_634_560*time.Nanosecond {
		t.Fatalf("batas waktu (1025, 15, 16) %v", got)
	}
	if got := PUFTimeout(768, 5, 16, 5*time.Second); got != 11_062_622_080*time.Nanosecond {
		t.Fatalf("batas waktu (768, 5, 16) %v", got)
	}
	if got := PUFTimeout(1025, 255, MaxWinLog2, 0); got <= 0 || got < 10_000*time.Second {
		t.Fatalf("batas waktu terbesar meluap: %v", got)
	}
}

func timedBus(clock *ManualClock, caps Caps, busyFor time.Duration) *scriptedBus {
	var started time.Time
	running := false
	bus := newScripted(nil)
	bus.caps = caps.Encode()
	bus.onCtrl = func(uint32) {
		started = clock.Peek()
		running = true
	}
	bus.status = func() uint32 {
		switch {
		case !running:
			return StatusBooted
		case clock.Peek().Sub(started) < busyFor:
			return StatusBooted | StatusBusy
		}
		return StatusBooted | StatusDone
	}
	return bus
}

func TestSlowPUFCommandsAreNotCutOff(t *testing.T) {
	ctx := context.Background()
	timed := func(caps Caps, busyFor time.Duration, command func(d *Driver) error) (time.Duration, error) {
		clock := NewManualClock(time.Microsecond)
		d := NewDriver(timedBus(clock, caps, busyFor), DriverOptions{Clock: clock})
		start := clock.Peek()
		err := command(d)
		return clock.Peek().Sub(start), err
	}
	recon := func(caps Caps) func(d *Driver) error {
		return func(d *Driver) error {
			_, err := d.PUFRecon(ctx, HelperData{Mask: make([]byte, caps.MaskLen())})
			return err
		}
	}
	pufEnroll := func(d *Driver) error {
		_, _, err := d.PUFEnroll(ctx, DefaultPUFThresh)
		return err
	}
	kemEnroll := func(d *Driver) error {
		_, _, err := d.Enroll(ctx, 3)
		return err
	}
	slowLimit := slowestCaps.PUFTimeout(5 * time.Second)

	if waited, err := timed(slowestCaps, 6*time.Second, recon(slowestCaps)); err != nil || waited < 6*time.Second {
		t.Fatalf("PUF_RECON 6 s dengan (1025, 15, 16): %v setelah %v", err, waited)
	}
	if waited, err := timed(slowestCaps, 20*time.Second, pufEnroll); err != nil || waited < 20*time.Second {
		t.Fatalf("PUF_ENROLL 20 s dengan (1025, 15, 16): %v setelah %v", err, waited)
	}
	if waited, err := timed(slowestCaps, 45*time.Second, recon(slowestCaps)); !errors.Is(err, ErrTimeout) || waited < slowLimit || waited > slowLimit+time.Second {
		t.Fatalf("PUF_RECON 45 s: %v setelah %v, batas %v", err, waited, slowLimit)
	}
	if waited, err := timed(boardCaps, 6*time.Second, recon(boardCaps)); !errors.Is(err, ErrTimeout) || waited < 5*time.Second || waited > 6*time.Second {
		t.Fatalf("PUF_RECON 6 s dengan parameter bawaan: %v setelah %v", err, waited)
	}
	if waited, err := timed(boardCaps, 4*time.Second, recon(boardCaps)); err != nil || waited < 4*time.Second {
		t.Fatalf("PUF_RECON 4 s dengan parameter bawaan: %v setelah %v", err, waited)
	}
	if waited, err := timed(slowestCaps, 6*time.Second, kemEnroll); !errors.Is(err, ErrTimeout) || waited > 6*time.Second {
		t.Fatalf("ENROLL 6 s tetap dibatasi 5 s: %v setelah %v", err, waited)
	}
}

func TestIdleWaitCoversSlowPUFCommand(t *testing.T) {
	clock := NewManualClock(time.Microsecond)
	busyUntil := clock.Peek().Add(30 * time.Second)
	bus := newScripted(func() uint32 {
		if clock.Peek().Before(busyUntil) {
			return StatusBooted | StatusBusy
		}
		return StatusBooted
	})
	bus.caps = slowestCaps.Encode()
	d := NewDriver(bus, DriverOptions{Clock: clock})
	if _, err := d.ReadMem(context.Background(), AddrH, SlotSize); err != nil {
		t.Fatalf("menunggu PUF_ENROLL lama yang masih berjalan: %v", err)
	}
	if waited := clock.Peek().Sub(busyUntil); waited < 0 {
		t.Fatalf("membaca sebelum chip selesai")
	}
}
