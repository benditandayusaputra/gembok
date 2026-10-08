package lms

import "fmt"

type OTSType uint32

type TreeType uint32

const (
	LMOTS_SHA256_N32_W1 OTSType = 1
	LMOTS_SHA256_N32_W2 OTSType = 2
	LMOTS_SHA256_N32_W4 OTSType = 3
	LMOTS_SHA256_N32_W8 OTSType = 4
)

const (
	LMS_SHA256_M32_H5  TreeType = 5
	LMS_SHA256_M32_H10 TreeType = 6
	LMS_SHA256_M32_H15 TreeType = 7
	LMS_SHA256_M32_H20 TreeType = 8
	LMS_SHA256_M32_H25 TreeType = 9
)

const (
	HashSize      = 32
	IDSize        = 16
	SeedSize      = 32
	PublicKeySize = 8 + IDSize + HashSize

	maxSigningHeight = 15

	domainPublic   = 0x8080
	domainMessage  = 0x8181
	domainLeaf     = 0x8282
	domainInterior = 0x8383
	domainSeed     = 0xff
)

type otsParams struct {
	w  int
	p  int
	ls int
}

func (t OTSType) params() (otsParams, bool) {
	switch t {
	case LMOTS_SHA256_N32_W1:
		return otsParams{w: 1, p: 265, ls: 7}, true
	case LMOTS_SHA256_N32_W2:
		return otsParams{w: 2, p: 133, ls: 6}, true
	case LMOTS_SHA256_N32_W4:
		return otsParams{w: 4, p: 67, ls: 4}, true
	case LMOTS_SHA256_N32_W8:
		return otsParams{w: 8, p: 34, ls: 0}, true
	}
	return otsParams{}, false
}

func (t OTSType) String() string {
	switch t {
	case LMOTS_SHA256_N32_W1:
		return "LMOTS_SHA256_N32_W1"
	case LMOTS_SHA256_N32_W2:
		return "LMOTS_SHA256_N32_W2"
	case LMOTS_SHA256_N32_W4:
		return "LMOTS_SHA256_N32_W4"
	case LMOTS_SHA256_N32_W8:
		return "LMOTS_SHA256_N32_W8"
	}
	return fmt.Sprintf("LMOTS(%d)", uint32(t))
}

func (t TreeType) height() (int, bool) {
	switch t {
	case LMS_SHA256_M32_H5:
		return 5, true
	case LMS_SHA256_M32_H10:
		return 10, true
	case LMS_SHA256_M32_H15:
		return 15, true
	case LMS_SHA256_M32_H20:
		return 20, true
	case LMS_SHA256_M32_H25:
		return 25, true
	}
	return 0, false
}

func (t TreeType) String() string {
	switch t {
	case LMS_SHA256_M32_H5:
		return "LMS_SHA256_M32_H5"
	case LMS_SHA256_M32_H10:
		return "LMS_SHA256_M32_H10"
	case LMS_SHA256_M32_H15:
		return "LMS_SHA256_M32_H15"
	case LMS_SHA256_M32_H20:
		return "LMS_SHA256_M32_H20"
	case LMS_SHA256_M32_H25:
		return "LMS_SHA256_M32_H25"
	}
	return fmt.Sprintf("LMS(%d)", uint32(t))
}

func (o otsParams) signatureSize() int {
	return 4 + HashSize*(o.p+1)
}

func SignatureSize(tree TreeType, ots OTSType) (int, bool) {
	h, okTree := tree.height()
	op, okOTS := ots.params()
	if !okTree || !okOTS {
		return 0, false
	}
	return 8 + op.signatureSize() + HashSize*h, true
}

func Capacity(tree TreeType) (uint32, bool) {
	h, ok := tree.height()
	if !ok {
		return 0, false
	}
	return uint32(1) << h, true
}
