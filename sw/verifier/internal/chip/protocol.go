package chip

import "crypto/sha3"

const (
	LabelSeed  = "GEMBOK-v1/benih"
	LabelTag   = "GEMBOK-v1/bukti"
	LabelCheck = "GEMBOK-v1/cek"
)

func DeriveSeeds(pufKey [PUFKeySize]byte) (d, z [32]byte) {
	input := make([]byte, 0, len(LabelSeed)+PUFKeySize)
	input = append(input, LabelSeed...)
	input = append(input, pufKey[:]...)
	out := sha3.SumSHAKE256(input, 64)
	copy(d[:], out[:32])
	copy(z[:], out[32:])
	clear(input)
	clear(out)
	return d, z
}

func HelperCheck(pufKey [PUFKeySize]byte, mask []byte) [HelperChkSize]byte {
	input := make([]byte, 0, len(LabelCheck)+PUFKeySize+len(mask))
	input = append(input, LabelCheck...)
	input = append(input, pufKey[:]...)
	input = append(input, mask...)
	var out [HelperChkSize]byte
	copy(out[:], sha3.SumSHAKE256(input, HelperChkSize))
	clear(input)
	return out
}

func ComputeTag(shared []byte, proofContext []byte) [TagSize]byte {
	h := sha3.New256()
	h.Write([]byte(LabelTag))
	h.Write(shared)
	h.Write([]byte{byte(len(proofContext))})
	h.Write(proofContext)
	var tag [TagSize]byte
	h.Sum(tag[:0])
	return tag
}
