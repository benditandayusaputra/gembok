//go:build js && wasm

package main

import (
	"context"
	"crypto/mlkem"
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"syscall/js"
	"time"

	"gembok/verifier/internal/chip"
	"gembok/verifier/internal/issuer"
	"gembok/verifier/internal/lms"
)

type counter struct {
	mu   sync.Mutex
	next uint32
	cap  uint32
}

func (c *counter) Reserve() (uint32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.next >= c.cap {
		return 0, errors.New("tanda tangan penerbit habis")
	}
	q := c.next
	c.next++
	return q, nil
}

func (c *counter) Used() uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.next
}

func (c *counter) Capacity() uint32 {
	return c.cap
}

type document struct {
	sim     *chip.Sim
	helper  chip.HelperData
	cert    issuer.Certificate
	lastTag []byte
}

var (
	mu    sync.Mutex
	iss   *issuer.Issuer
	trust lms.PublicKey
	docs  = map[string]*document{}
)

const level = 3

func ms(t time.Time) float64 {
	return float64(time.Since(t).Microseconds()) / 1000
}

func head(b []byte, n int) string {
	if len(b) < n {
		n = len(b)
	}
	return hex.EncodeToString(b[:n])
}

func promise(work func() (map[string]any, error)) js.Value {
	handler := js.FuncOf(func(this js.Value, args []js.Value) any {
		resolve, reject := args[0], args[1]
		go func() {
			out, err := work()
			if err != nil {
				reject.Invoke(js.Global().Get("Error").New(err.Error()))
				return
			}
			resolve.Invoke(js.ValueOf(out))
		}()
		return nil
	})
	return js.Global().Get("Promise").New(handler)
}

func setup() (map[string]any, error) {
	mu.Lock()
	defer mu.Unlock()
	if iss != nil {
		return map[string]any{"ms": 0, "kunci_publik": head(trust.Bytes(), 12)}, nil
	}
	t := time.Now()
	key, err := lms.GenerateKey(rand.Reader, lms.LMS_SHA256_M32_H10, lms.LMOTS_SHA256_N32_W8)
	if err != nil {
		return nil, err
	}
	tree, err := key.BuildTree()
	if err != nil {
		return nil, err
	}
	c := &counter{cap: 1 << 10}
	signer, err := lms.NewSigner(key, tree, c, rand.Reader)
	if err != nil {
		return nil, err
	}
	iss, err = issuer.New(signer, c, nil)
	if err != nil {
		return nil, err
	}
	trust = signer.PublicKey()
	return map[string]any{"ms": ms(t), "kunci_publik": head(trust.Bytes(), 12)}, nil
}

func personalize(name, chipID string, seed int) (map[string]any, error) {
	mu.Lock()
	defer mu.Unlock()
	if iss == nil {
		return nil, errors.New("penerbit belum disiapkan")
	}
	ctx := context.Background()
	genuine, clone := chip.ModelPUF(uint32(seed)), chip.ModelPUF(uint32(seed+9000))
	sim, err := chip.NewSim(ctx, chip.SimConfig{Genuine: &genuine, Clone: &clone})
	if err != nil {
		return nil, err
	}
	t := time.Now()
	helper, pufCycles, err := sim.PUFEnroll(ctx, chip.DefaultPUFThresh)
	if err != nil {
		return nil, err
	}
	ek, enrollCycles, err := sim.Enroll(ctx, level)
	if err != nil {
		return nil, err
	}
	cert, err := iss.Issue(chipID, level, ek)
	if err != nil {
		return nil, err
	}
	docs[name] = &document{sim: sim, helper: helper, cert: cert}
	return map[string]any{
		"id_chip":       cert.ChipID,
		"panjang_ek":    len(ek),
		"ek_awal":       head(ek, 16),
		"siklus_puf":    int(pufCycles),
		"siklus_daftar": int(enrollCycles),
		"us_daftar":     float64(chip.CyclesToDuration(enrollCycles).Microseconds()),
		"panjang_ttd":   len(cert.Signature),
		"ms":            ms(t),
	}, nil
}

func verify(name, mode, proofContext string) (map[string]any, error) {
	mu.Lock()
	defer mu.Unlock()
	d, ok := docs[name]
	if !ok {
		return nil, errors.New("dokumen belum dipersonalisasi")
	}
	if len(proofContext) > 255 {
		return nil, errors.New("konteks terlalu panjang")
	}
	if mode == "ulang" && d.lastTag == nil {
		return nil, errors.New("belum ada bukti lama untuk diputar ulang")
	}
	ctx := context.Background()
	start := time.Now()
	steps := []any{}
	add := func(key string, ok bool, took float64, detail string) {
		steps = append(steps, map[string]any{"langkah": key, "ok": ok, "ms": took, "rincian": detail})
	}
	out := map[string]any{"langkah": nil}
	finish := func(verdict, reason string) (map[string]any, error) {
		out["langkah"] = steps
		out["putusan"] = verdict
		out["alasan"] = reason
		out["ms_total"] = ms(start)
		return out, nil
	}

	t := time.Now()
	certErr := issuer.Verify(trust, d.cert)
	add("sertifikat", certErr == nil, ms(t), fmt.Sprintf("tanda tangan LMS %d byte, id %s", len(d.cert.Signature), d.cert.ChipID))
	if certErr != nil {
		return finish("PALSU", "sertifikat tidak sah")
	}

	t = time.Now()
	personality, self := chip.PersonalityGenuine, false
	if mode == "klon" || mode == "klon_daftar" {
		personality = chip.PersonalityClone
		self = mode == "klon_daftar"
	}
	if err := d.sim.Switch(ctx, personality, self); err != nil {
		return nil, err
	}
	attempts := 0
	if !self {
		var lastErr error
		for attempts = 1; attempts <= 5; attempts++ {
			_, lastErr = d.sim.PUFRecon(ctx, d.helper)
			if lastErr == nil {
				break
			}
			code, isChip := chip.CodeOf(lastErr)
			if !isChip || (code != chip.ErrBadHelper && code != chip.ErrPUFFail) {
				return nil, lastErr
			}
		}
		if lastErr != nil {
			attempts = 5
			add("nyala", false, ms(t), "kunci PUF tidak pulih dalam 5 percobaan: data bantu ditolak")
			out["percobaan"] = attempts
			return finish("PALSU", "PUF chip ini bukan PUF yang didaftarkan")
		}
		add("nyala", true, ms(t), fmt.Sprintf("kunci PUF pulih pada percobaan ke-%d", attempts))
	} else {
		add("nyala", true, ms(t), "chip tiruan memakai PUF-nya sendiri")
	}
	out["percobaan"] = attempts

	t = time.Now()
	ek, err := mlkem.NewEncapsulationKey768(d.cert.EK)
	if err != nil {
		return nil, err
	}
	shared, ciphertext := ek.Encapsulate()
	add("tantangan", true, ms(t), fmt.Sprintf("Encaps ML-KEM-768: sandi %d byte, rahasia 32 byte", len(ciphertext)))
	out["sandi_awal"] = head(ciphertext, 16)
	out["panjang_sandi"] = len(ciphertext)

	t = time.Now()
	proof, err := d.sim.Prove(ctx, level, ciphertext, []byte(proofContext))
	if err != nil {
		return nil, err
	}
	tag := proof.Tag[:]
	detail := "chip membuka sandi dan menghitung bukti 32 byte"
	if mode == "ulang" {
		tag = d.lastTag
		detail = "pemalsu mengirim bukti dari pemeriksaan sebelumnya"
	}
	add("bukti", true, ms(t), detail)
	out["siklus"] = int(proof.Cycles)
	out["us_chip"] = float64(chip.CyclesToDuration(proof.Cycles).Microseconds())
	out["bukti"] = hex.EncodeToString(tag)

	t = time.Now()
	expected := chip.ComputeTag(shared, []byte(proofContext))
	match := subtle.ConstantTimeCompare(expected[:], tag) == 1
	add("cocokkan", match, ms(t), "SHA3-256 atas label, rahasia, dan konteks dibandingkan dalam waktu konstan")
	out["harapan"] = hex.EncodeToString(expected[:])
	if mode == "asli" && match {
		d.lastTag = append([]byte(nil), tag...)
	}
	if !match {
		if mode == "ulang" {
			return finish("PALSU", "bukti lama tidak berlaku untuk tantangan baru")
		}
		return finish("PALSU", "bukti tidak cocok dengan kunci publik di sertifikat")
	}
	return finish("ASLI", "bukti cocok dengan kunci publik di sertifikat")
}

func main() {
	api := js.Global().Get("Object").New()
	api.Set("siapkan", js.FuncOf(func(this js.Value, args []js.Value) any {
		return promise(setup)
	}))
	api.Set("personalisasi", js.FuncOf(func(this js.Value, args []js.Value) any {
		name, id, seed := args[0].String(), args[1].String(), args[2].Int()
		return promise(func() (map[string]any, error) { return personalize(name, id, seed) })
	}))
	api.Set("periksa", js.FuncOf(func(this js.Value, args []js.Value) any {
		name, mode, ctx := args[0].String(), args[1].String(), args[2].String()
		return promise(func() (map[string]any, error) { return verify(name, mode, ctx) })
	}))
	js.Global().Set("gembok", api)
	js.Global().Call("dispatchEvent", js.Global().Get("Event").New("gembok-siap"))
	select {}
}
