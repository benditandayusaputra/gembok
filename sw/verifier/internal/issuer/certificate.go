package issuer

import (
	"bytes"
	"crypto/mlkem"
	"crypto/sha3"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gembok/verifier/internal/chip"
	"gembok/verifier/internal/lms"
)

const (
	CertificateDomain  = "GEMBOK-v1/sertifikat"
	CertificateVersion = 1
	MaxChipIDLength    = 64
	maxFieldLength     = 1 << 16
)

var (
	ErrBadCertificate = errors.New("sertifikat: isi tidak sah")
	ErrBadSignature   = errors.New("sertifikat: tanda tangan penerbit tidak sah")
	ErrWrongIssuer    = errors.New("sertifikat: diterbitkan oleh kunci penerbit lain")
	ErrBadChipID      = errors.New("sertifikat: id chip harus 1 sampai 64 karakter huruf, angka, titik, garis bawah, atau tanda hubung, diawali huruf atau angka")
)

type Certificate struct {
	Version   int
	ChipID    string
	Parameter string
	EK        []byte
	IssuerID  [lms.IDSize]byte
	IssuedAt  time.Time
	Signature []byte
}

func ParameterName(k int) (string, bool) {
	level, ok := chip.LevelFor(k)
	if !ok {
		return "", false
	}
	return level.Name, true
}

func (c Certificate) Level() (chip.Level, bool) {
	for _, k := range []int{3, 4} {
		level, _ := chip.LevelFor(k)
		if level.Name == c.Parameter {
			return level, true
		}
	}
	return chip.Level{}, false
}

func ValidChipID(id string) bool {
	if len(id) == 0 || len(id) > MaxChipIDLength {
		return false
	}
	for i, r := range id {
		alnum := r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9'
		if !alnum && (i == 0 || !strings.ContainsRune("._-", r)) {
			return false
		}
	}
	return true
}

func (c Certificate) Validate() error {
	if c.Version != CertificateVersion {
		return fmt.Errorf("%w: versi %d", ErrBadCertificate, c.Version)
	}
	if !ValidChipID(c.ChipID) {
		return ErrBadChipID
	}
	level, ok := c.Level()
	if !ok {
		return fmt.Errorf("%w: parameter %q", ErrBadCertificate, c.Parameter)
	}
	if len(c.EK) != level.EKSize {
		return fmt.Errorf("%w: panjang ek %d untuk %s", ErrBadCertificate, len(c.EK), level.Name)
	}
	if err := checkEncapsulationKey(level.K, c.EK); err != nil {
		return fmt.Errorf("%w: ek gagal cek FIPS 203: %v", ErrBadCertificate, err)
	}
	if c.IssuedAt.IsZero() || c.IssuedAt.Unix() < 0 {
		return fmt.Errorf("%w: waktu terbit", ErrBadCertificate)
	}
	return nil
}

func checkEncapsulationKey(k int, ek []byte) error {
	switch k {
	case 3:
		_, err := mlkem.NewEncapsulationKey768(ek)
		return err
	case 4:
		_, err := mlkem.NewEncapsulationKey1024(ek)
		return err
	}
	return chip.ErrUnsupportedK
}

func appendField(out, field []byte) []byte {
	out = binary.BigEndian.AppendUint32(out, uint32(len(field)))
	return append(out, field...)
}

func (c Certificate) Body() []byte {
	var version, issued [8]byte
	binary.BigEndian.PutUint64(version[:], uint64(c.Version))
	binary.BigEndian.PutUint64(issued[:], uint64(c.IssuedAt.Unix()))
	out := make([]byte, 0, 64+len(c.ChipID)+len(c.Parameter)+len(c.EK))
	out = appendField(out, []byte(CertificateDomain))
	out = appendField(out, version[7:])
	out = appendField(out, []byte(c.ChipID))
	out = appendField(out, []byte(c.Parameter))
	out = appendField(out, c.EK)
	out = appendField(out, c.IssuerID[:])
	out = appendField(out, issued[:])
	return out
}

func ParseBody(body []byte) (Certificate, error) {
	var fields [][]byte
	rest := body
	for len(rest) > 0 {
		if len(rest) < 4 {
			return Certificate{}, fmt.Errorf("%w: kepala ruas terpotong", ErrBadCertificate)
		}
		n := binary.BigEndian.Uint32(rest)
		if n > maxFieldLength || int(n) > len(rest)-4 {
			return Certificate{}, fmt.Errorf("%w: panjang ruas", ErrBadCertificate)
		}
		fields = append(fields, rest[4:4+n])
		rest = rest[4+n:]
	}
	if len(fields) != 7 || string(fields[0]) != CertificateDomain || len(fields[1]) != 1 || len(fields[5]) != lms.IDSize || len(fields[6]) != 8 {
		return Certificate{}, fmt.Errorf("%w: susunan ruas", ErrBadCertificate)
	}
	c := Certificate{
		Version:   int(fields[1][0]),
		ChipID:    string(fields[2]),
		Parameter: string(fields[3]),
		EK:        bytes.Clone(fields[4]),
		IssuedAt:  time.Unix(int64(binary.BigEndian.Uint64(fields[6])), 0).UTC(),
	}
	copy(c.IssuerID[:], fields[5])
	return c, nil
}

func Verify(trust lms.PublicKey, c Certificate) error {
	if err := c.Validate(); err != nil {
		return err
	}
	if c.IssuerID != trust.ID {
		return ErrWrongIssuer
	}
	if !trust.Verify(c.Body(), c.Signature) {
		return ErrBadSignature
	}
	return nil
}

func Fingerprint(ek []byte) string {
	sum := sha3.Sum256(ek)
	return hex.EncodeToString(sum[:8])
}

func DefaultChipID(ek []byte) string {
	sum := sha3.Sum256(append([]byte("GEMBOK-v1/id"), ek...))
	return "GEMBOK-" + strings.ToUpper(hex.EncodeToString(sum[:6]))
}

type certificateJSON struct {
	Version   int    `json:"versi"`
	ChipID    string `json:"id_chip"`
	Parameter string `json:"parameter"`
	EK        string `json:"ek"`
	IssuerID  string `json:"penerbit"`
	IssuedAt  string `json:"diterbitkan"`
	Algorithm string `json:"algoritma_tanda_tangan"`
	Signature string `json:"tanda_tangan"`
}

func SignatureAlgorithm() string {
	return lms.LMS_SHA256_M32_H10.String() + "/" + lms.LMOTS_SHA256_N32_W8.String()
}

func (c Certificate) MarshalJSON() ([]byte, error) {
	return json.Marshal(certificateJSON{
		Version:   c.Version,
		ChipID:    c.ChipID,
		Parameter: c.Parameter,
		EK:        hex.EncodeToString(c.EK),
		IssuerID:  hex.EncodeToString(c.IssuerID[:]),
		IssuedAt:  c.IssuedAt.UTC().Format(time.RFC3339),
		Algorithm: SignatureAlgorithm(),
		Signature: hex.EncodeToString(c.Signature),
	})
}

func (c *Certificate) UnmarshalJSON(data []byte) error {
	var raw certificateJSON
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(&raw); err != nil {
		return fmt.Errorf("%w: %v", ErrBadCertificate, err)
	}
	if raw.Algorithm != SignatureAlgorithm() {
		return fmt.Errorf("%w: algoritma tanda tangan %q", ErrBadCertificate, raw.Algorithm)
	}
	ek, err := hex.DecodeString(raw.EK)
	if err != nil {
		return fmt.Errorf("%w: ek bukan hex", ErrBadCertificate)
	}
	issuerID, err := hex.DecodeString(raw.IssuerID)
	if err != nil || len(issuerID) != lms.IDSize {
		return fmt.Errorf("%w: pengenal penerbit", ErrBadCertificate)
	}
	sig, err := hex.DecodeString(raw.Signature)
	if err != nil {
		return fmt.Errorf("%w: tanda tangan bukan hex", ErrBadCertificate)
	}
	issued, err := time.Parse(time.RFC3339, raw.IssuedAt)
	if err != nil {
		return fmt.Errorf("%w: waktu terbit", ErrBadCertificate)
	}
	*c = Certificate{
		Version:   raw.Version,
		ChipID:    raw.ChipID,
		Parameter: raw.Parameter,
		EK:        ek,
		IssuedAt:  issued.UTC(),
		Signature: sig,
	}
	copy(c.IssuerID[:], issuerID)
	return nil
}
