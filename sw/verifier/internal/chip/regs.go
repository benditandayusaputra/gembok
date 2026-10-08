package chip

import (
	"fmt"
	"time"
)

const (
	RegID        = 0x0000
	RegVersion   = 0x0004
	RegCtrl      = 0x0008
	RegStatus    = 0x000C
	RegParam     = 0x0010
	RegCtxLen    = 0x0014
	RegCycles    = 0x0018
	RegCaps      = 0x001C
	RegCooldown  = 0x0020
	RegProofs    = 0x0024
	RegPUFThresh = 0x0028
	RegPUFIdx    = 0x002C
	RegPUFDbg    = 0x0030
	RegPUFDbg1   = 0x0034

	RegSpan = 0x0100
	MemBase = 0x8000
	MemSize = 8192
	Span    = 0x10000

	IDValue      = 0x47454D42
	VersionValue = 0x00010000
	VersionMajor = 1
)

const (
	StatusBusy     = 1 << 0
	StatusDone     = 1 << 1
	StatusError    = 1 << 2
	StatusErrShift = 4
	StatusErrMask  = 0xF
	StatusPUFReady = 1 << 8
	StatusCooldown = 1 << 9
	StatusBooted   = 1 << 10
)

const (
	AddrEK         = 0x0000
	AddrCT         = 0x0800
	AddrDK         = 0x1000
	AddrD          = 0x1800
	AddrZ          = 0x1820
	AddrM          = 0x1840
	AddrK          = 0x1860
	AddrSigma      = 0x1880
	AddrKBar       = 0x18A0
	AddrH          = 0x18C0
	AddrTag        = 0x18E0
	AddrCtx        = 0x1900
	AddrHelperMask = 0x1A00
	AddrHelperChk  = 0x1A80

	SlotSize           = 32
	TagSize            = 32
	MaxContext         = 255
	HelperMaskMaxSize  = AddrHelperChk - AddrHelperMask
	HelperChkSize      = 16
	PUFKeySize         = 32
	SecretSlotsAddr    = AddrD
	SecretSlotsSize    = 6 * SlotSize
	DKRegionSize       = AddrD - AddrDK
	DefaultOscillators = 768
	MinOscillators     = 2
	MaxOscillators     = 1025

	DefaultPUFThresh = 64
	ClockHz          = 50_000_000
	CyclePeriod      = time.Second / ClockHz
)

type Command uint32

const (
	CmdKeygen     Command = 1
	CmdEncaps     Command = 2
	CmdDecaps     Command = 3
	CmdCheckEK    Command = 4
	CmdCheckDK    Command = 5
	CmdEnroll     Command = 6
	CmdProve      Command = 7
	CmdPUFEnroll  Command = 8
	CmdPUFRecon   Command = 9
	CmdPUFMeasure Command = 10
	CmdWipe       Command = 15
)

func (c Command) String() string {
	switch c {
	case CmdKeygen:
		return "KEYGEN"
	case CmdEncaps:
		return "ENCAPS"
	case CmdDecaps:
		return "DECAPS"
	case CmdCheckEK:
		return "CHECK_EK"
	case CmdCheckDK:
		return "CHECK_DK"
	case CmdEnroll:
		return "ENROLL"
	case CmdProve:
		return "PROVE"
	case CmdPUFEnroll:
		return "PUF_ENROLL"
	case CmdPUFRecon:
		return "PUF_RECON"
	case CmdPUFMeasure:
		return "PUF_MEASURE"
	case CmdWipe:
		return "WIPE"
	}
	return fmt.Sprintf("CMD(%d)", uint32(c))
}

type ErrCode uint8

const (
	ErrOK        ErrCode = 0
	ErrBadEK     ErrCode = 1
	ErrBadDK     ErrCode = 2
	ErrBadHelper ErrCode = 3
	ErrBadParam  ErrCode = 4
	ErrNoKey     ErrCode = 5
	ErrRate      ErrCode = 6
	ErrBadCmd    ErrCode = 7
	ErrPUFFail   ErrCode = 8
)

func (e ErrCode) String() string {
	switch e {
	case ErrOK:
		return "OK"
	case ErrBadEK:
		return "BAD_EK"
	case ErrBadDK:
		return "BAD_DK"
	case ErrBadHelper:
		return "BAD_HELPER"
	case ErrBadParam:
		return "BAD_PARAM"
	case ErrNoKey:
		return "NO_KEY"
	case ErrRate:
		return "RATE"
	case ErrBadCmd:
		return "BAD_CMD"
	case ErrPUFFail:
		return "PUF_FAIL"
	}
	return fmt.Sprintf("ERR(%d)", uint8(e))
}

func (e ErrCode) Meaning() string {
	switch e {
	case ErrOK:
		return "berhasil"
	case ErrBadEK:
		return "kunci enkapsulasi gagal cek modulus"
	case ErrBadDK:
		return "kunci dekapsulasi gagal cek hash"
	case ErrBadHelper:
		return "data bantu PUF tidak cocok dengan kunci yang dipulihkan"
	case ErrBadParam:
		return "nilai k tidak didukung"
	case ErrNoKey:
		return "kunci PUF belum siap"
	case ErrRate:
		return "pembatas laju masih aktif"
	case ErrBadCmd:
		return "perintah tidak dikenal atau tidak tersedia di bangunan ini"
	case ErrPUFFail:
		return "PUF tidak memberi tepat 256 bit"
	}
	return "kode galat tidak dikenal"
}

type Level struct {
	K      int
	Name   string
	EKSize int
	CTSize int
	DKSize int
}

func LevelFor(k int) (Level, bool) {
	switch k {
	case 3:
		return Level{K: 3, Name: "ML-KEM-768", EKSize: 1184, CTSize: 1088, DKSize: 1152}, true
	case 4:
		return Level{K: 4, Name: "ML-KEM-1024", EKSize: 1568, CTSize: 1568, DKSize: 1536}, true
	}
	return Level{}, false
}

func CyclesToDuration(cycles uint32) time.Duration {
	return time.Duration(cycles) * CyclePeriod
}

func HelperMaskLen(oscillators int) (int, bool) {
	if oscillators < MinOscillators || oscillators > MaxOscillators {
		return 0, false
	}
	return (oscillators + 6) / 8, true
}
