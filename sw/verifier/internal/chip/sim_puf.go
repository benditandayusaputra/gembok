package chip

const (
	SimVotes   = 5
	SimWinLog2 = 12
	SimKeyBits = 256
)

type pufSource interface {
	mode() int
	enroll(thresh uint16, mask []byte) ([PUFKeySize]byte, bool)
	reconstruct(mask []byte) ([PUFKeySize]byte, bool)
	enrollCycles() uint32
	reconCycles() uint32
}

type PUFSpec struct {
	ChipSeed uint32
	FixedKey *[PUFKeySize]byte
}

func ModelPUF(chipSeed uint32) PUFSpec {
	return PUFSpec{ChipSeed: chipSeed}
}

func FixedKeyPUF(key [PUFKeySize]byte) PUFSpec {
	return PUFSpec{FixedKey: &key}
}

func (p PUFSpec) source(oscillators int) pufSource {
	if p.FixedKey != nil {
		return fixedKeyPUF{key: *p.FixedKey}
	}
	return modelPUF{seed: p.ChipSeed, candidates: oscillators - 1}
}

type fixedKeyPUF struct {
	key [PUFKeySize]byte
}

func (f fixedKeyPUF) mode() int {
	return 2
}

func (f fixedKeyPUF) enroll(uint16, []byte) ([PUFKeySize]byte, bool) {
	return f.key, true
}

func (f fixedKeyPUF) reconstruct([]byte) ([PUFKeySize]byte, bool) {
	return f.key, true
}

func (f fixedKeyPUF) enrollCycles() uint32 {
	return simCyclesPUFFixed
}

func (f fixedKeyPUF) reconCycles() uint32 {
	return simCyclesPUFFixed
}

type modelPUF struct {
	seed       uint32
	candidates int
}

func RingOscillatorCount(chipSeed uint32, index int) uint32 {
	x := chipSeed ^ uint32(index)*0x9E3779B1
	x ^= x >> 16
	x *= 0x85EBCA6B
	x ^= x >> 13
	x *= 0xC2B2AE35
	x ^= x >> 16
	return 20000 + x&0x3FF
}

func (m modelPUF) mode() int {
	return 1
}

func (m modelPUF) pairBit(c int) (bit bool, distance int) {
	diff := int(RingOscillatorCount(m.seed, c)) - int(RingOscillatorCount(m.seed, c+1))
	if diff < 0 {
		return false, -diff
	}
	return diff > 0, diff
}

func (m modelPUF) enroll(thresh uint16, mask []byte) ([PUFKeySize]byte, bool) {
	var key [PUFKeySize]byte
	clear(mask)
	selected := 0
	skip := false
	for c := 0; c < m.candidates; c++ {
		if skip || selected == SimKeyBits {
			skip = false
			continue
		}
		bit, distance := m.pairBit(c)
		if distance < int(thresh) {
			continue
		}
		mask[c>>3] |= 1 << (c & 7)
		if bit {
			key[selected>>3] |= 1 << (selected & 7)
		}
		selected++
		skip = true
	}
	if selected != SimKeyBits {
		return [PUFKeySize]byte{}, false
	}
	return key, true
}

func (m modelPUF) reconstruct(mask []byte) ([PUFKeySize]byte, bool) {
	var key [PUFKeySize]byte
	selected := 0
	for c := 0; c < m.candidates; c++ {
		if mask[c>>3]>>(c&7)&1 == 0 {
			continue
		}
		if bit, _ := m.pairBit(c); bit && selected < SimKeyBits {
			key[selected>>3] |= 1 << (selected & 7)
		}
		selected++
	}
	if selected != SimKeyBits {
		return [PUFKeySize]byte{}, false
	}
	return key, true
}

func (m modelPUF) enrollCycles() uint32 {
	return simCyclesPUFEnrollModel
}

func (m modelPUF) reconCycles() uint32 {
	return simCyclesPUFReconModel
}
