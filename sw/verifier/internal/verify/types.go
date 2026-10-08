package verify

import (
	"errors"
	"time"

	"gembok/verifier/internal/chip"
	"gembok/verifier/internal/issuer"
)

const (
	VerdictGenuine = "ASLI"
	VerdictFake    = "PALSU"

	ReasonProofMatches   = "BUKTI_COCOK"
	ReasonProofMismatch  = "BUKTI_TIDAK_COCOK"
	ReasonBadCertificate = "SERTIFIKAT_TIDAK_SAH"
	ReasonHelperRejected = "DATA_BANTU_DITOLAK"
	ReasonNoKey          = "CHIP_TANPA_KUNCI"
	ReasonChipRefused    = "CHIP_MENOLAK"

	HistorySize            = 50
	AttackContext          = "serang/baca-kunci"
	DefaultPowerUpAttempts = 5
	MaxPowerUpAttempts     = 20

	WarningDevelopmentKey = "PUF_KUNCI_PENGEMBANGAN"
	WarningSimulatedPUF   = "PUF_MODEL_SIMULASI"
	WarningUnknownPUF     = "PUF_MODE_TIDAK_DIKENAL"
	WarningDebugBuild     = "PUF_BANGUNAN_DEBUG"
)

var (
	ErrNotEnrolled     = errors.New("chip belum didaftarkan")
	ErrAlreadyEnrolled = errors.New("chip sudah didaftarkan, kirim \"ulang\": true untuk mendaftar ulang")
	ErrNoIssuer        = errors.New("kunci penerbit tidak tersedia")
	ErrNotSimulation   = errors.New("hanya tersedia pada backend sim")
	ErrBadRequest      = errors.New("permintaan tidak sah")
	ErrHelperMismatch  = errors.New("panjang topeng data bantu tidak sesuai dengan jumlah osilator chip")
)

type ChipFailure struct {
	Op  string
	Err error
}

func (e *ChipFailure) Error() string {
	return "chip gagal saat " + e.Op + ": " + e.Err.Error()
}

func (e *ChipFailure) Unwrap() error {
	return e.Err
}

type Store interface {
	LoadCertificate() (issuer.Certificate, error)
	LoadHelper() (chip.HelperData, error)
	SaveEnrollment(cert issuer.Certificate, helper chip.HelperData) error
}

type Result struct {
	ID              int       `json:"id"`
	Time            time.Time `json:"waktu"`
	Verdict         string    `json:"hasil"`
	Reason          string    `json:"alasan"`
	Detail          string    `json:"keterangan"`
	ChipID          string    `json:"id_chip"`
	Parameter       string    `json:"parameter"`
	Context         string    `json:"konteks"`
	ChipCycles      uint32    `json:"siklus_chip"`
	ChipMicros      float64   `json:"waktu_chip_us"`
	TotalMicros     int64     `json:"waktu_total_us"`
	SimulatedCycles bool      `json:"siklus_simulasi"`
	AutoPowerUp     bool      `json:"nyala_otomatis"`
	PowerUpAttempts int       `json:"percobaan_nyala"`
	RateWaits       int       `json:"tunggu_pembatas"`
	ChipCode        string    `json:"kode_chip,omitempty"`
	Tag             string    `json:"tag_chip,omitempty"`
	Backend         string    `json:"backend"`
	SimChip         string    `json:"chip_simulasi,omitempty"`
}

type HelperView struct {
	Thresh  uint16 `json:"ambang"`
	MaskLen int    `json:"panjang_topeng"`
	Mask    string `json:"topeng"`
	Check   string `json:"cek"`
	Pairs   int    `json:"jumlah_pasangan"`
}

type EnrollRequest struct {
	ChipID    string
	K         int
	Thresh    uint16
	Overwrite bool
}

type EnrollResult struct {
	Certificate     issuer.Certificate `json:"sertifikat"`
	Helper          HelperView         `json:"data_bantu"`
	EKFingerprint   string             `json:"sidik_ek"`
	PUFCycles       uint32             `json:"siklus_puf"`
	EnrollCycles    uint32             `json:"siklus_daftar"`
	EnrollMicros    float64            `json:"waktu_daftar_chip_us"`
	SimulatedCycles bool               `json:"siklus_simulasi"`
	IssuerUsed      uint32             `json:"tanda_tangan_terpakai"`
	IssuerCapacity  uint32             `json:"tanda_tangan_maksimum"`
	TotalMicros     int64              `json:"waktu_total_us"`
}

type PowerUpResult struct {
	Success         bool    `json:"berhasil"`
	PUFReady        bool    `json:"puf_siap"`
	ChipCode        string  `json:"kode_chip"`
	Detail          string  `json:"keterangan"`
	Attempts        int     `json:"percobaan"`
	MaxAttempts     int     `json:"percobaan_maksimum"`
	Cycles          uint32  `json:"siklus"`
	ChipMicros      float64 `json:"waktu_chip_us"`
	SimulatedCycles bool    `json:"siklus_simulasi"`
}

type MemoryRegion struct {
	Name    string `json:"nama"`
	Address string `json:"alamat"`
	Length  int    `json:"panjang"`
	Secret  bool   `json:"rahasia"`
	AllZero bool   `json:"semua_nol"`
	NonZero int    `json:"byte_bukan_nol"`
	Hex     string `json:"isi_hex"`
}

type AttackResult struct {
	Time            time.Time      `json:"waktu"`
	ProofRan        bool           `json:"bukti_dijalankan"`
	Proof           *Result        `json:"bukti"`
	Note            string         `json:"catatan,omitempty"`
	Regions         []MemoryRegion `json:"wilayah"`
	SecretBytes     int            `json:"jumlah_byte_rahasia"`
	SecretNonZero   int            `json:"byte_rahasia_bukan_nol"`
	SecretsAllZero  bool           `json:"rahasia_semua_nol"`
	PublicBytesRead int            `json:"jumlah_byte_publik"`
	PublicNonZero   int            `json:"byte_publik_bukan_nol"`
	Explanation     string         `json:"penjelasan"`
}

type ChipReport struct {
	ID             string `json:"id"`
	Version        string `json:"versi"`
	PUFMode        int    `json:"mode_puf"`
	PUFModeName    string `json:"nama_mode_puf"`
	Debug          bool   `json:"bangunan_debug"`
	WinLog2        int    `json:"jendela_log2"`
	Votes          int    `json:"jumlah_suara"`
	Oscillators    int    `json:"jumlah_osilator"`
	HelperMaskLen  int    `json:"panjang_topeng"`
	PUFTimeoutMs   int64  `json:"batas_waktu_puf_ms"`
	Booted         bool   `json:"sudah_boot"`
	Busy           bool   `json:"sibuk"`
	PUFReady       bool   `json:"puf_siap"`
	Cooling        bool   `json:"pendinginan"`
	CooldownCycles uint32 `json:"sisa_pendinginan_siklus"`
	Proofs         uint32 `json:"jumlah_bukti"`
	LastError      string `json:"galat_terakhir"`
	LastCycles     uint32 `json:"siklus_terakhir"`
	PUFThresh      uint16 `json:"ambang_puf"`
}

type CertificateSummary struct {
	ChipID        string `json:"id_chip"`
	Parameter     string `json:"parameter"`
	IssuedAt      string `json:"diterbitkan"`
	EKFingerprint string `json:"sidik_ek"`
	LeafIndex     uint32 `json:"indeks_tanda_tangan"`
	Valid         bool   `json:"sah"`
	Problem       string `json:"masalah,omitempty"`
}

type IssuerSummary struct {
	Available bool   `json:"tersedia"`
	Algorithm string `json:"algoritma"`
	ID        string `json:"id"`
	Used      uint32 `json:"tanda_tangan_terpakai"`
	Capacity  uint32 `json:"tanda_tangan_maksimum"`
}

type LevelReport struct {
	K    int    `json:"k"`
	Name string `json:"nama"`
}

type SimulationReport struct {
	Active       string   `json:"chip"`
	SelfEnrolled bool     `json:"daftar_sendiri"`
	Options      []string `json:"pilihan"`
}

type Warning struct {
	Code    string `json:"kode"`
	Message string `json:"pesan"`
}

type StatusReport struct {
	Backend         string              `json:"backend"`
	SimulatedCycles bool                `json:"siklus_simulasi"`
	ClockHz         int                 `json:"clock_hz"`
	DefaultLevel    LevelReport         `json:"parameter_bawaan"`
	DefaultThresh   uint16              `json:"ambang_bawaan"`
	PowerUpAttempts int                 `json:"coba_nyala"`
	Warnings        []Warning           `json:"peringatan"`
	Chip            ChipReport          `json:"chip"`
	Enrolled        bool                `json:"terdaftar"`
	Certificate     *CertificateSummary `json:"sertifikat"`
	Issuer          IssuerSummary       `json:"penerbit"`
	Simulation      *SimulationReport   `json:"simulasi"`
	Time            time.Time           `json:"waktu"`
}
