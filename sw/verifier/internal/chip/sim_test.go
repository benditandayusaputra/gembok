package chip

import (
	"bytes"
	"context"
	"crypto/mlkem"
	"errors"
	"math/rand/v2"
	"testing"
	"time"
)

func rawCommand(t *testing.T, d *Driver, k int, cmd Command) ErrCode {
	t.Helper()
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bus.Write32(RegParam, uint32(k))
	_, err := d.run(context.Background(), cmd, d.idleTimeout())
	if err == nil {
		return ErrOK
	}
	code, ok := CodeOf(err)
	if !ok {
		t.Fatalf("galat bukan dari chip: %v", err)
	}
	return code
}

func readReg(d *Driver, off uint32) uint32 {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.bus.Read32(off)
}

func writeReg(d *Driver, off, value uint32) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.bus.Write32(off, value)
}

func encapsulate(t *testing.T, k int, ek []byte) (shared, ciphertext []byte) {
	t.Helper()
	switch k {
	case 3:
		key, err := mlkem.NewEncapsulationKey768(ek)
		if err != nil {
			t.Fatal(err)
		}
		shared, ciphertext = key.Encapsulate()
	case 4:
		key, err := mlkem.NewEncapsulationKey1024(ek)
		if err != nil {
			t.Fatal(err)
		}
		shared, ciphertext = key.Encapsulate()
	default:
		t.Fatalf("k=%d", k)
	}
	return shared, ciphertext
}

func enrolledDevice(t *testing.T, spec PUFSpec, k int) (*Driver, *simBus, *ManualClock, HelperData, []byte) {
	t.Helper()
	d, bus, clock := newTestDevice(t, spec)
	helper, _, err := d.PUFEnroll(context.Background(), DefaultPUFThresh)
	if err != nil {
		t.Fatal(err)
	}
	ek, _, err := d.Enroll(context.Background(), k)
	if err != nil {
		t.Fatal(err)
	}
	return d, bus, clock, helper, ek
}

func TestRegistersAfterBoot(t *testing.T) {
	d, _, _ := newTestDevice(t, ModelPUF(1))
	ctx := context.Background()
	info, err := d.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != IDValue || info.Version != VersionValue || info.PUFMode != 1 || info.Debug || info.Votes != SimVotes || info.Oscillators != DefaultOscillators || info.HelperMaskLen != 96 {
		t.Fatalf("info %+v", info)
	}
	if caps := readReg(d, RegCaps); caps != 0x03000561 || info.WinLog2 != SimWinLog2 || info.PUFTimeout != 5*time.Second {
		t.Fatalf("CAPS 0x%08X, WIN_LOG2 %d, batas waktu PUF %v", caps, info.WinLog2, info.PUFTimeout)
	}
	st, err := d.Status(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if !st.Booted || st.Busy || st.PUFReady || st.Done || st.Cooling || st.Proofs != 0 || st.K != 3 || st.PUFThresh != DefaultPUFThresh {
		t.Fatalf("status setelah boot %+v", st)
	}
	mem, err := d.ReadMem(ctx, 0, MemSize)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(mem, make([]byte, MemSize)) {
		t.Fatal("memori tidak kosong setelah boot")
	}
	writeReg(d, RegPUFThresh, 123)
	if got := readReg(d, RegPUFThresh); got != 123 {
		t.Fatalf("PUF_THRESH %d", got)
	}
	writeReg(d, RegCtxLen, 0x1FF)
	if got := readReg(d, RegCtxLen); got != 0xFF {
		t.Fatalf("CTXLEN %#x", got)
	}
	for _, off := range []uint32{0x34, 0xFC, RegSpan, 0x7FFC} {
		if got := readReg(d, off); got != 0 {
			t.Fatalf("offset %#x terbaca %#x", off, got)
		}
	}
	fixed, _, _ := newTestDevice(t, FixedKeyPUF([PUFKeySize]byte{1}))
	if info, _ := fixed.Info(ctx); info.PUFMode != 2 {
		t.Fatalf("mode PUF kunci tetap %d", info.PUFMode)
	}
}

func TestCommandErrors(t *testing.T) {
	d, _, _ := newTestDevice(t, ModelPUF(1))
	cases := []struct {
		k    int
		cmd  Command
		want ErrCode
	}{
		{5, CmdEnroll, ErrBadParam},
		{0, CmdProve, ErrBadParam},
		{2, CmdEnroll, ErrBadParam},
		{3, CmdEnroll, ErrNoKey},
		{3, CmdProve, ErrNoKey},
		{3, CmdPUFMeasure, ErrBadCmd},
		{3, Command(12), ErrBadCmd},
		{3, CmdKeygen, ErrBadCmd},
		{3, CmdEncaps, ErrBadCmd},
		{3, CmdDecaps, ErrBadCmd},
		{3, CmdCheckEK, ErrBadCmd},
		{3, CmdCheckDK, ErrBadCmd},
	}
	for _, c := range cases {
		if got := rawCommand(t, d, c.k, c.cmd); got != c.want {
			t.Fatalf("%s k=%d: %s, harap %s", c.cmd, c.k, got, c.want)
		}
		if cycles := readReg(d, RegCycles); cycles != 1 {
			t.Fatalf("%s k=%d: CYCLES %d untuk perintah yang langsung ditolak", c.cmd, c.k, cycles)
		}
		st := readReg(d, RegStatus)
		if st&StatusError == 0 || st&StatusDone == 0 || statusErr(st) != c.want {
			t.Fatalf("%s: STATUS %#x", c.cmd, st)
		}
	}
	if got := rawCommand(t, d, 3, CmdPUFEnroll); got != ErrOK {
		t.Fatalf("PUF_ENROLL sesudah galat: %s", got)
	}
	if st := readReg(d, RegStatus); st&StatusError != 0 || st&StatusPUFReady == 0 {
		t.Fatalf("STATUS sesudah perintah berhasil %#x", st)
	}
	if _, _, err := d.Enroll(context.Background(), 3); err != nil {
		t.Fatal(err)
	}
}

func TestAccessGuardDuringProve(t *testing.T) {
	const k = 3
	key := [PUFKeySize]byte{7, 7, 7}
	d, bus, _, _, ek := enrolledDevice(t, FixedKeyPUF(key), k)
	shared, ciphertext := encapsulate(t, k, ek)
	proofContext := []byte("uji-penjaga")
	hostWrite(d, AddrCT, ciphertext)
	hostWrite(d, AddrCtx, proofContext)
	writeReg(d, RegCtxLen, uint32(len(proofContext)))
	writeReg(d, RegParam, k)
	writeReg(d, RegCtrl, uint32(CmdProve))
	if readReg(d, RegStatus)&StatusBusy == 0 {
		t.Fatal("chip tidak sibuk setelah PROVE")
	}
	rng := rand.New(rand.NewPCG(1, 2))
	var seen []byte
	probes := 0
	for readReg(d, RegStatus)&StatusBusy != 0 {
		var addr int
		switch rng.IntN(3) {
		case 0:
			addr = SecretSlotsAddr + rng.IntN(256)
		case 1:
			addr = AddrDK + rng.IntN(64)
		default:
			addr = AddrEK + rng.IntN(64)
		}
		seen = append(seen, byte(readReg(d, MemBase+4*uint32(addr))))
		probes++
		if probes%50 == 0 && readReg(d, RegCycles) < SimCyclesProve768-1000 {
			writeReg(d, MemBase+4*uint32(AddrCT+rng.IntN(1088)), uint32(rng.IntN(256)))
			hostWrite(d, AddrTag, bytes.Repeat([]byte{0xAA}, TagSize))
			writeReg(d, RegCtrl, uint32(CmdWipe))
		}
	}
	if probes < 100 {
		t.Fatalf("hanya %d percobaan baca selama sibuk", probes)
	}
	if !bytes.Equal(seen, make([]byte, len(seen))) {
		t.Fatal("host bisa membaca memori saat brankas bekerja")
	}
	st := readReg(d, RegStatus)
	if st&StatusDone == 0 || statusErr(st) != ErrOK || st&StatusPUFReady == 0 {
		t.Fatalf("STATUS sesudah PROVE %#x", st)
	}
	tag, _ := d.ReadMem(context.Background(), AddrTag, TagSize)
	if want := ComputeTag(shared, proofContext); !bytes.Equal(tag, want[:]) {
		t.Fatal("tulisan host saat sibuk memengaruhi hasil")
	}
	mem, _ := d.ReadMem(context.Background(), 0, MemSize)
	if !bytes.Equal(mem[SecretSlotsAddr:SecretSlotsAddr+SecretSlotsSize], make([]byte, SecretSlotsSize)) {
		t.Fatal("slot rahasia tidak terhapus")
	}
	if !bytes.Equal(mem[AddrDK:AddrDK+DKRegionSize], make([]byte, DKRegionSize)) {
		t.Fatal("wilayah DK terisi pada mode brankas")
	}
	if !bytes.Equal(mem[AddrEK:AddrEK+len(ek)], ek) {
		t.Fatal("EK tidak ada di memori")
	}
	dSeed, zSeed := DeriveSeeds(key)
	for name, secret := range map[string][]byte{"kunci PUF": key[:], "d": dSeed[:], "z": zSeed[:], "K": shared} {
		for off := 0; off+8 <= len(secret); off += 8 {
			piece := secret[off : off+8]
			if !bytes.Equal(piece, make([]byte, 8)) && bytes.Contains(mem, piece) {
				t.Fatalf("rahasia %s terbaca host", name)
			}
		}
	}
	bus.mu.Lock()
	leftover := bus.dev.key
	bus.mu.Unlock()
	if leftover != key {
		t.Fatal("brankas kehilangan kunci PUF")
	}
}

func TestRateLimiterRegisters(t *testing.T) {
	const k = 3
	d, _, clock, _, ek := enrolledDevice(t, ModelPUF(1), k)
	ctx := context.Background()
	if readReg(d, RegProofs) != 0 {
		t.Fatal("PROOFS bukan nol")
	}
	shared, ciphertext := encapsulate(t, k, ek)
	proof, err := d.Prove(ctx, k, ciphertext, nil)
	if err != nil {
		t.Fatal(err)
	}
	if proof.Tag != ComputeTag(shared, nil) {
		t.Fatal("tag pertama salah")
	}
	if readReg(d, RegProofs) != 1 {
		t.Fatal("PROOFS bukan satu")
	}
	if readReg(d, RegStatus)&StatusCooldown == 0 {
		t.Fatal("pembatas laju tidak aktif setelah bukti")
	}
	if got := rawCommand(t, d, k, CmdProve); got != ErrRate {
		t.Fatalf("PROVE langsung sesudahnya: %s", got)
	}
	if readReg(d, RegProofs) != 1 {
		t.Fatal("PROVE yang ditolak ikut dihitung")
	}
	tag, _ := d.ReadMem(ctx, AddrTag, TagSize)
	if !bytes.Equal(tag, proof.Tag[:]) {
		t.Fatal("tag lama berubah oleh PROVE yang ditolak")
	}
	left := readReg(d, RegCooldown)
	if left == 0 || left > SimDefaultCooldown {
		t.Fatalf("COOLDOWN %d", left)
	}
	if got := rawCommand(t, d, k, CmdEnroll); got != ErrOK {
		t.Fatalf("DAFTAR ikut dibatasi: %s", got)
	}
	clock.Advance(CyclesToDuration(SimDefaultCooldown))
	if readReg(d, RegStatus)&StatusCooldown != 0 || readReg(d, RegCooldown) != 0 {
		t.Fatal("pembatas laju tidak berhenti")
	}
	shared2, ciphertext2 := encapsulate(t, k, ek)
	proof2, err := d.Prove(ctx, k, ciphertext2, nil)
	if err != nil {
		t.Fatal(err)
	}
	if proof2.RateWaits != 0 || proof2.Tag != ComputeTag(shared2, nil) {
		t.Fatalf("bukti kedua %+v", proof2)
	}
	if readReg(d, RegProofs) != 2 {
		t.Fatal("PROOFS bukan dua")
	}
}

func TestDriverWaitsOutRateLimiter(t *testing.T) {
	const k = 4
	d, _, clock, _, ek := enrolledDevice(t, ModelPUF(2), k)
	ctx := context.Background()
	_, first := encapsulate(t, k, ek)
	if _, err := d.Prove(ctx, k, first, []byte("a")); err != nil {
		t.Fatal(err)
	}
	before := clock.Peek()
	shared, second := encapsulate(t, k, ek)
	proof, err := d.Prove(ctx, k, second, []byte("b"))
	if err != nil {
		t.Fatal(err)
	}
	if proof.RateWaits != 1 {
		t.Fatalf("menunggu %d kali, harap 1", proof.RateWaits)
	}
	if elapsed := clock.Peek().Sub(before); elapsed < CyclesToDuration(SimDefaultCooldown)-time.Millisecond {
		t.Fatalf("pengemudi hanya menunggu %v", elapsed)
	}
	if proof.RateWaited <= 0 || proof.Tag != ComputeTag(shared, []byte("b")) || proof.Cycles != SimCyclesProve1024 {
		t.Fatalf("bukti setelah menunggu %+v", proof)
	}
	if readReg(d, RegProofs) != 2 {
		t.Fatal("PROOFS bukan dua")
	}
}

func TestWipeForgetsKey(t *testing.T) {
	d, _, _, _, _ := enrolledDevice(t, ModelPUF(1), 3)
	ctx := context.Background()
	if err := d.Wipe(ctx); err != nil {
		t.Fatal(err)
	}
	st, _ := d.Status(ctx)
	if st.PUFReady {
		t.Fatal("kunci PUF masih siap setelah WIPE")
	}
	mem, _ := d.ReadMem(ctx, 0, MemSize)
	if !bytes.Equal(mem, make([]byte, MemSize)) {
		t.Fatal("memori tidak kosong setelah WIPE")
	}
	if _, _, err := d.Enroll(ctx, 3); !isCode(err, ErrNoKey) {
		t.Fatalf("DAFTAR sesudah WIPE: %v", err)
	}
}

func TestCloneChip(t *testing.T) {
	const k = 3
	ctx := context.Background()
	_, _, _, helper, ek := enrolledDevice(t, ModelPUF(1), k)
	clone, _, _ := newTestDevice(t, ModelPUF(2))
	if _, err := clone.PUFRecon(ctx, helper); !isCode(err, ErrBadHelper) {
		t.Fatalf("PUF_RECON chip tiruan dengan data bantu asli: %v", err)
	}
	_, ciphertext := encapsulate(t, k, ek)
	if _, err := clone.Prove(ctx, k, ciphertext, nil); !isCode(err, ErrNoKey) {
		t.Fatalf("PROVE chip tiruan tanpa kunci: %v", err)
	}
	if _, _, err := clone.PUFEnroll(ctx, DefaultPUFThresh); err != nil {
		t.Fatal(err)
	}
	cloneEK, _, err := clone.Enroll(ctx, k)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(cloneEK, ek) {
		t.Fatal("chip tiruan punya kunci publik yang sama")
	}
	shared, ciphertext := encapsulate(t, k, ek)
	proof, err := clone.Prove(ctx, k, ciphertext, nil)
	if err != nil {
		t.Fatal(err)
	}
	if proof.Tag == ComputeTag(shared, nil) {
		t.Fatal("chip tiruan lolos pemeriksaan")
	}
}

func flipBits(mask []byte, positions ...int) []byte {
	out := bytes.Clone(mask)
	for _, c := range positions {
		out[c>>3] ^= 1 << (c & 7)
	}
	return out
}

func TestTamperedHelperData(t *testing.T) {
	const k = 3
	ctx := context.Background()
	_, _, _, helper, ek := enrolledDevice(t, ModelPUF(1), k)
	var ones, zeros []int
	for c := 0; c < DefaultOscillators-1; c++ {
		if helper.Mask[c>>3]>>(c&7)&1 == 1 {
			ones = append(ones, c)
		} else {
			zeros = append(zeros, c)
		}
	}
	if len(ones) != SimKeyBits {
		t.Fatalf("topeng berisi %d pasangan", len(ones))
	}
	changedCheck := helper.Check
	changedCheck[0] ^= 1
	full := bytes.Repeat([]byte{0xFF}, len(helper.Mask))
	cases := []struct {
		name  string
		mask  []byte
		check [HelperChkSize]byte
		want  ErrCode
	}{
		{"satu pasangan dibuang", flipBits(helper.Mask, ones[10]), helper.Check, ErrPUFFail},
		{"satu pasangan ditambah", flipBits(helper.Mask, zeros[5]), helper.Check, ErrPUFFail},
		{"satu pasangan ditukar", flipBits(helper.Mask, ones[40], zeros[40]), helper.Check, ErrBadHelper},
		{"nilai cek diubah", helper.Mask, changedCheck, ErrBadHelper},
		{"topeng kosong", make([]byte, len(helper.Mask)), helper.Check, ErrPUFFail},
		{"topeng penuh", full, helper.Check, ErrPUFFail},
	}
	d, _, _ := newTestDevice(t, ModelPUF(1))
	for _, c := range cases {
		bad := HelperData{Thresh: helper.Thresh, Mask: c.mask, Check: c.check}
		if _, err := d.PUFRecon(ctx, bad); !isCode(err, c.want) {
			t.Fatalf("%s: %v, harap %s", c.name, err, c.want)
		}
		if st, _ := d.Status(ctx); st.PUFReady {
			t.Fatalf("%s: kunci tetap siap", c.name)
		}
		if _, _, err := d.Enroll(ctx, k); !isCode(err, ErrNoKey) {
			t.Fatalf("%s: DAFTAR %v", c.name, err)
		}
		if got := rawCommand(t, d, k, CmdProve); got != ErrNoKey {
			t.Fatalf("%s: PROVE %s", c.name, got)
		}
	}
	if _, err := d.PUFRecon(ctx, helper); err != nil {
		t.Fatal(err)
	}
	again, _, err := d.Enroll(ctx, k)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, ek) {
		t.Fatal("kunci publik berubah setelah data bantu yang benar dipakai lagi")
	}
}

func TestSimSwitchPersonality(t *testing.T) {
	ctx := context.Background()
	clock := NewManualClock(100 * time.Nanosecond)
	sim, err := NewSim(ctx, SimConfig{Clock: clock})
	if err != nil {
		t.Fatal(err)
	}
	if name, self := sim.Active(); name != PersonalityGenuine || self {
		t.Fatalf("chip awal %s %v", name, self)
	}
	if sim.Kind() != KindSim || !sim.SimulatedCycles() {
		t.Fatal("jenis backend salah")
	}
	helper, _, err := sim.PUFEnroll(ctx, DefaultPUFThresh)
	if err != nil {
		t.Fatal(err)
	}
	ek, _, err := sim.Enroll(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	if err := sim.Switch(ctx, PersonalityClone, false); err != nil {
		t.Fatal(err)
	}
	st, _ := sim.Status(ctx)
	if st.PUFReady || st.Proofs != 0 {
		t.Fatalf("chip tiruan tidak mulai dari keadaan reset: %+v", st)
	}
	if mem, _ := sim.ReadMem(ctx, 0, MemSize); !bytes.Equal(mem, make([]byte, MemSize)) {
		t.Fatal("memori chip tiruan tidak kosong")
	}
	if _, err := sim.PUFRecon(ctx, helper); !isCode(err, ErrBadHelper) {
		t.Fatalf("chip tiruan menerima data bantu asli: %v", err)
	}
	if err := sim.Switch(ctx, PersonalityClone, true); err != nil {
		t.Fatal(err)
	}
	if name, self := sim.Active(); name != PersonalityClone || !self {
		t.Fatalf("chip aktif %s %v", name, self)
	}
	cloneEK, _, err := sim.Enroll(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(cloneEK, ek) {
		t.Fatal("chip tiruan punya kunci publik yang sama")
	}
	if err := sim.Switch(ctx, PersonalityGenuine, false); err != nil {
		t.Fatal(err)
	}
	if _, err := sim.PUFRecon(ctx, helper); err != nil {
		t.Fatal(err)
	}
	again, _, err := sim.Enroll(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(again, ek) {
		t.Fatal("chip asli berganti kunci setelah dicabut dan dipasang lagi")
	}
	if err := sim.Switch(ctx, "lain", false); !errors.Is(err, ErrUnknownChip) {
		t.Fatalf("nama chip tidak dikenal: %v", err)
	}
	if got := sim.Personalities(); len(got) != 2 || got[0] != PersonalityGenuine || got[1] != PersonalityClone {
		t.Fatalf("daftar chip %v", got)
	}
}

func TestOpenBackends(t *testing.T) {
	ctx := context.Background()
	c, err := Open(ctx, Config{Backend: KindSim, Clock: NewManualClock(time.Microsecond)})
	if err != nil {
		t.Fatal(err)
	}
	if c.Kind() != KindSim {
		t.Fatalf("jenis %s", c.Kind())
	}
	if _, ok := c.(Switchable); !ok {
		t.Fatal("backend sim tidak bisa berganti chip")
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := Open(ctx, Config{Backend: "fpga"}); !errors.Is(err, ErrUnknownBackend) {
		t.Fatalf("backend tidak dikenal: %v", err)
	}
}

func TestRejectedCommandReportsBusyFirst(t *testing.T) {
	clock := NewManualClock(5 * time.Nanosecond)
	bus := &simBus{dev: newSimDevice(clock, ModelPUF(1), SimDefaultCooldown, DefaultOscillators)}
	d := NewDriver(bus, DriverOptions{Clock: clock})
	if err := d.Probe(context.Background()); err != nil {
		t.Fatal(err)
	}
	writeReg(d, RegParam, 3)
	writeReg(d, RegCtrl, uint32(CmdProve))
	writeReg(d, MemBase+4*AddrCT, 0x5A)
	st := readReg(d, RegStatus)
	if st&StatusBusy == 0 || st&StatusDone != 0 || st&StatusError != 0 {
		t.Fatalf("STATUS tepat sesudah CTRL %#x", st)
	}
	clock.Advance(CyclesToDuration(simCyclesReject))
	st = readReg(d, RegStatus)
	if st&StatusBusy != 0 || st&StatusDone == 0 || statusErr(st) != ErrNoKey {
		t.Fatalf("STATUS sesudah penolakan %#x", st)
	}
	if cycles := readReg(d, RegCycles); cycles != 1 {
		t.Fatalf("CYCLES %d", cycles)
	}
	if got := readReg(d, MemBase+4*AddrCT); got != 0 {
		t.Fatalf("tulisan memori tepat sesudah CTRL diterima: %#x", got)
	}
}

func TestRegisterWritesIgnoredWhileBusy(t *testing.T) {
	const k = 3
	d, _, _, _, ek := enrolledDevice(t, ModelPUF(1), k)
	shared, ciphertext := encapsulate(t, k, ek)
	proofContext := []byte("terkunci")
	hostWrite(d, AddrCT, ciphertext)
	hostWrite(d, AddrCtx, proofContext)
	writeReg(d, RegCtxLen, uint32(len(proofContext)))
	writeReg(d, RegPUFThresh, 64)
	writeReg(d, RegParam, k)
	writeReg(d, RegCtrl, uint32(CmdProve))
	writeReg(d, RegParam, 4)
	writeReg(d, RegCtxLen, 2)
	writeReg(d, RegPUFThresh, 99)
	writeReg(d, RegPUFIdx, 5)
	for readReg(d, RegStatus)&StatusBusy != 0 {
	}
	if statusErr(readReg(d, RegStatus)) != ErrOK {
		t.Fatal("PROVE gagal")
	}
	if readReg(d, RegParam) != k || readReg(d, RegCtxLen) != uint32(len(proofContext)) || readReg(d, RegPUFThresh) != 64 || readReg(d, RegPUFIdx) != 0 {
		t.Fatal("register berubah oleh tulisan saat sibuk")
	}
	tag, _ := d.ReadMem(context.Background(), AddrTag, TagSize)
	if want := ComputeTag(shared, proofContext); !bytes.Equal(tag, want[:]) {
		t.Fatal("konteks yang dikunci saat PROVE diterima ikut berubah")
	}
	if readReg(d, RegPUFDbg) != 0 || readReg(d, RegPUFDbg1) != 0 {
		t.Fatal("PUF_DBG terisi pada bangunan bukan debug")
	}
}

func TestLongestHelperMask(t *testing.T) {
	ctx := context.Background()
	d, _, _ := newTestDeviceN(t, ModelPUF(7), MaxOscillators)
	info, err := d.Info(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if info.Oscillators != MaxOscillators || info.HelperMaskLen != HelperMaskMaxSize {
		t.Fatalf("info %+v", info)
	}
	helper, _, err := d.PUFEnroll(ctx, DefaultPUFThresh)
	if err != nil {
		t.Fatal(err)
	}
	if len(helper.Mask) != HelperMaskMaxSize {
		t.Fatalf("topeng %d byte", len(helper.Mask))
	}
	ek, _, err := d.Enroll(ctx, 3)
	if err != nil {
		t.Fatal(err)
	}
	fresh, _, _ := newTestDeviceN(t, ModelPUF(7), MaxOscillators)
	if _, err := fresh.PUFRecon(ctx, helper); err != nil {
		t.Fatalf("PUF_RECON dengan topeng 128 byte: %v", err)
	}
	again, _, err := fresh.Enroll(ctx, 3)
	if err != nil || !bytes.Equal(again, ek) {
		t.Fatalf("kunci berbeda setelah pemulihan: %v", err)
	}
	short, _, _ := newTestDevice(t, ModelPUF(7))
	if _, err := short.PUFRecon(ctx, helper); !errors.Is(err, ErrHelperLength) {
		t.Fatalf("topeng 128 byte pada chip 768 osilator: %v", err)
	}
	if _, err := NewSim(ctx, SimConfig{Clock: NewManualClock(time.Microsecond), Oscillators: 2000}); !errors.Is(err, ErrBadCaps) {
		t.Fatalf("simulasi dengan 2000 osilator: %v", err)
	}
}
