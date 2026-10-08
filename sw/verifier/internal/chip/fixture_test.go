package chip

import (
	"bytes"
	"context"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"
)

type fixtureProof struct {
	Ciphertext string `json:"c"`
	Context    string `json:"konteks"`
	KVerifier  string `json:"k_pemeriksa"`
	Tag        string `json:"tag"`
	Match      bool   `json:"cocok"`
}

type fixtureLevel struct {
	K      int            `json:"k"`
	Name   string         `json:"nama"`
	EK     string         `json:"ek"`
	HEK    string         `json:"h_ek"`
	CTLen  int            `json:"panjang_ct"`
	Proofs []fixtureProof `json:"bukti"`
}

type fixtureFixed struct {
	Name   string         `json:"nama"`
	PUFKey string         `json:"kunci_puf"`
	D      string         `json:"benih_d"`
	Z      string         `json:"benih_z"`
	Mask   string         `json:"topeng"`
	Check  string         `json:"cek"`
	Levels []fixtureLevel `json:"tingkat"`
}

type fixtureModel struct {
	ChipSeed    uint32         `json:"benih_chip"`
	Thresh      uint16         `json:"ambang"`
	Oscillators int            `json:"jumlah_osilator"`
	Freq        []uint32       `json:"frekuensi_awal"`
	Selected    int            `json:"terpilih"`
	Mask        string         `json:"topeng"`
	PUFKey      string         `json:"kunci_puf"`
	Check       string         `json:"cek"`
	Levels      []fixtureLevel `json:"tingkat"`
}

type fixture struct {
	Labels struct {
		Seed  string `json:"benih"`
		Tag   string `json:"bukti"`
		Check string `json:"cek"`
	} `json:"label"`
	Fixed []fixtureFixed `json:"kunci_tetap"`
	Model []fixtureModel `json:"model_puf"`
}

func loadFixture(t *testing.T) fixture {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", "fixture_pemeriksa.json"))
	if err != nil {
		t.Fatal(err)
	}
	var fx fixture
	dec := json.NewDecoder(bytes.NewReader(raw))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&fx); err != nil {
		t.Fatal(err)
	}
	if len(fx.Fixed) == 0 || len(fx.Model) == 0 {
		t.Fatal("fixture kosong")
	}
	return fx
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("hex tidak sah: %v", err)
	}
	return b
}

func unhexKey(t *testing.T, s string) [PUFKeySize]byte {
	t.Helper()
	b := unhex(t, s)
	if len(b) != PUFKeySize {
		t.Fatalf("panjang kunci PUF %d", len(b))
	}
	return [PUFKeySize]byte(b)
}

func newTestDevice(t *testing.T, spec PUFSpec) (*Driver, *simBus, *ManualClock) {
	t.Helper()
	return newTestDeviceN(t, spec, DefaultOscillators)
}

func newTestDeviceN(t *testing.T, spec PUFSpec, oscillators int) (*Driver, *simBus, *ManualClock) {
	t.Helper()
	clock := NewManualClock(100 * time.Nanosecond)
	bus := &simBus{dev: newSimDevice(clock, spec, SimDefaultCooldown, oscillators)}
	d := NewDriver(bus, DriverOptions{Kind: KindSim, SimulatedCycles: true, Clock: clock})
	if err := d.Probe(context.Background()); err != nil {
		t.Fatal(err)
	}
	return d, bus, clock
}

func hostWrite(d *Driver, addr int, data []byte) {
	d.mu.Lock()
	defer d.mu.Unlock()
	d.writeBytes(addr, data)
}

func checkLevels(t *testing.T, d *Driver, clock *ManualClock, levels []fixtureLevel) {
	t.Helper()
	ctx := context.Background()
	for _, lv := range levels {
		level, ok := LevelFor(lv.K)
		if !ok || level.Name != lv.Name || level.CTSize != lv.CTLen || level.EKSize*2 != len(lv.EK) {
			t.Fatalf("k=%d: tabel tingkat berbeda dari model", lv.K)
		}
		ek, cycles, err := d.Enroll(ctx, lv.K)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(ek, unhex(t, lv.EK)) {
			t.Fatalf("k=%d: ek dari simulasi berbeda dengan gembok.Chip.enroll()", lv.K)
		}
		if cycles != simEnrollCycles(lv.K) {
			t.Fatalf("k=%d: siklus DAFTAR %d", lv.K, cycles)
		}
		h, err := d.ReadMem(ctx, AddrH, SlotSize)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(h, unhex(t, lv.HEK)) {
			t.Fatalf("k=%d: slot H bukan SHA3-256(ek)", lv.K)
		}
		for i, p := range lv.Proofs {
			clock.Advance(CyclesToDuration(SimDefaultCooldown))
			proofContext := unhex(t, p.Context)
			proof, err := d.Prove(ctx, lv.K, unhex(t, p.Ciphertext), proofContext)
			if err != nil {
				t.Fatalf("k=%d bukti %d: %v", lv.K, i, err)
			}
			if !bytes.Equal(proof.Tag[:], unhex(t, p.Tag)) {
				t.Fatalf("k=%d bukti %d: tag berbeda dengan gembok.Chip.prove()", lv.K, i)
			}
			if proof.Cycles != simProveCycles(lv.K) || proof.RateWaits != 0 {
				t.Fatalf("k=%d bukti %d: siklus %d, tunggu %d", lv.K, i, proof.Cycles, proof.RateWaits)
			}
			expected := ComputeTag(unhex(t, p.KVerifier), proofContext)
			if (expected == proof.Tag) != p.Match {
				t.Fatalf("k=%d bukti %d: hasil pemeriksaan tag berbeda dengan gembok.Verifier.check()", lv.K, i)
			}
			secrets, err := d.ReadMem(ctx, SecretSlotsAddr, SecretSlotsSize)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(secrets, make([]byte, SecretSlotsSize)) {
				t.Fatalf("k=%d bukti %d: slot rahasia tidak nol", lv.K, i)
			}
		}
	}
}

func TestFixtureLabels(t *testing.T) {
	fx := loadFixture(t)
	if fx.Labels.Seed != LabelSeed || fx.Labels.Tag != LabelTag || fx.Labels.Check != LabelCheck {
		t.Fatalf("label berbeda dengan model: %+v", fx.Labels)
	}
}

func TestFixtureFixedKeysMatchReferenceModel(t *testing.T) {
	fx := loadFixture(t)
	for _, e := range fx.Fixed {
		t.Run(e.Name, func(t *testing.T) {
			key := unhexKey(t, e.PUFKey)
			d, z := DeriveSeeds(key)
			if !bytes.Equal(d[:], unhex(t, e.D)) || !bytes.Equal(z[:], unhex(t, e.Z)) {
				t.Fatal("benih (d, z) berbeda dengan gembok.derive_seeds()")
			}
			mask := unhex(t, e.Mask)
			check := HelperCheck(key, mask)
			if !bytes.Equal(check[:], unhex(t, e.Check)) {
				t.Fatal("nilai cek berbeda dengan gembok.helper_check()")
			}
			drv, _, clock := newTestDevice(t, FixedKeyPUF(key))
			hostWrite(drv, AddrHelperMask, mask)
			helper, _, err := drv.PUFEnroll(context.Background(), DefaultPUFThresh)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(helper.Mask, mask) || helper.Check != check {
				t.Fatal("data bantu dari PUF_ENROLL berbeda dengan model")
			}
			checkLevels(t, drv, clock, e.Levels)

			fresh, _, freshClock := newTestDevice(t, FixedKeyPUF(key))
			if _, err := fresh.PUFRecon(context.Background(), helper); err != nil {
				t.Fatalf("PUF_RECON dengan data bantu sendiri: %v", err)
			}
			checkLevels(t, fresh, freshClock, e.Levels[:1])

			other := key
			other[0] ^= 1
			stranger, _, _ := newTestDevice(t, FixedKeyPUF(other))
			if _, err := stranger.PUFRecon(context.Background(), helper); !isCode(err, ErrBadHelper) {
				t.Fatalf("chip dengan kunci lain: %v", err)
			}
		})
	}
}

func TestFixtureModelPUFMatchesReferenceModel(t *testing.T) {
	fx := loadFixture(t)
	for _, e := range fx.Model {
		e := e
		t.Run(fmt.Sprintf("benih_%d_ambang_%d_osilator_%d", e.ChipSeed, e.Thresh, e.Oscillators), func(t *testing.T) {
			maskLen, ok := HelperMaskLen(e.Oscillators)
			if !ok || len(e.Mask) != 2*maskLen {
				t.Fatalf("jumlah osilator %d, topeng %d hex", e.Oscillators, len(e.Mask))
			}
			for i, f := range e.Freq {
				if got := RingOscillatorCount(e.ChipSeed, i); got != f {
					t.Fatalf("hitungan osilator %d: %d, harap %d", i, got, f)
				}
			}
			puf := modelPUF{seed: e.ChipSeed, candidates: e.Oscillators - 1}
			mask := make([]byte, maskLen)
			key, ok := puf.enroll(e.Thresh, mask)
			if ok != (e.Selected == SimKeyBits) {
				t.Fatalf("hasil pendaftaran %v, model memilih %d pasangan", ok, e.Selected)
			}
			if !bytes.Equal(mask, unhex(t, e.Mask)) {
				t.Fatal("topeng berbeda dengan puf_model.enroll()")
			}
			drv, _, clock := newTestDeviceN(t, ModelPUF(e.ChipSeed), e.Oscillators)
			info, err := drv.Info(context.Background())
			if err != nil || info.Oscillators != e.Oscillators || info.HelperMaskLen != maskLen {
				t.Fatalf("CAPS simulasi %+v", info)
			}
			helper, _, err := drv.PUFEnroll(context.Background(), e.Thresh)
			if !ok {
				if !isCode(err, ErrPUFFail) {
					t.Fatalf("PUF_ENROLL dengan ambang %d: %v", e.Thresh, err)
				}
				st, _ := drv.Status(context.Background())
				if st.PUFReady {
					t.Fatal("PUF siap walau pendaftaran gagal")
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(key[:], unhex(t, e.PUFKey)) {
				t.Fatal("kunci PUF berbeda dengan puf_model.enroll()")
			}
			again, ok := puf.reconstruct(mask)
			if !ok || again != key {
				t.Fatal("pemulihan dari topeng sendiri memberi kunci lain")
			}
			if !bytes.Equal(helper.Mask, mask) || !bytes.Equal(helper.Check[:], unhex(t, e.Check)) {
				t.Fatal("data bantu dari PUF_ENROLL berbeda dengan model")
			}
			checkLevels(t, drv, clock, e.Levels)
		})
	}
}

func isCode(err error, want ErrCode) bool {
	code, ok := CodeOf(err)
	return ok && code == want
}
