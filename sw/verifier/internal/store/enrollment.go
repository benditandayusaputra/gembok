package store

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

	"gembok/verifier/internal/chip"
	"gembok/verifier/internal/issuer"
)

const (
	helperVersion       = 2
	legacyHelperVersion = 1
	legacyMaskLen       = 96
)

type helperFile struct {
	Version int    `json:"versi"`
	Thresh  uint16 `json:"ambang"`
	MaskLen int    `json:"panjang_topeng,omitempty"`
	Mask    string `json:"topeng"`
	Check   string `json:"cek"`
	Saved   string `json:"disimpan"`
}

func validMaskLen(n int) bool {
	return n >= 1 && n <= chip.HelperMaskMaxSize
}

func (s *Store) SaveEnrollment(cert issuer.Certificate, helper chip.HelperData) error {
	if !validMaskLen(len(helper.Mask)) {
		return fmt.Errorf("penyimpanan: topeng data bantu %d byte di luar 1 sampai %d", len(helper.Mask), chip.HelperMaskMaxSize)
	}
	if err := s.writeJSON(fileHelper, helperFile{
		Version: helperVersion,
		Thresh:  helper.Thresh,
		MaskLen: len(helper.Mask),
		Mask:    hex.EncodeToString(helper.Mask),
		Check:   hex.EncodeToString(helper.Check[:]),
		Saved:   time.Now().UTC().Format(time.RFC3339),
	}, 0o644); err != nil {
		return fmt.Errorf("penyimpanan: data bantu: %w", err)
	}
	raw, err := json.MarshalIndent(cert, "", "  ")
	if err != nil {
		return err
	}
	if err := s.writeFile(fileCertificate, append(raw, '\n'), 0o644); err != nil {
		return fmt.Errorf("penyimpanan: sertifikat: %w", err)
	}
	return nil
}

func (s *Store) LoadCertificate() (issuer.Certificate, error) {
	var cert issuer.Certificate
	if err := s.readJSON(fileCertificate, &cert); err != nil {
		return issuer.Certificate{}, err
	}
	return cert, nil
}

func (s *Store) LoadHelper() (chip.HelperData, error) {
	var f helperFile
	if err := s.readJSON(fileHelper, &f); err != nil {
		return chip.HelperData{}, err
	}
	switch f.Version {
	case helperVersion:
	case legacyHelperVersion:
		if f.MaskLen != 0 {
			return chip.HelperData{}, fmt.Errorf("%w: data bantu versi 1 tidak punya panjang topeng", ErrCorrupt)
		}
		f.MaskLen = legacyMaskLen
	default:
		return chip.HelperData{}, fmt.Errorf("%w: versi data bantu %d tidak dikenal", ErrCorrupt, f.Version)
	}
	if !validMaskLen(f.MaskLen) {
		return chip.HelperData{}, fmt.Errorf("%w: panjang topeng data bantu %d di luar 1 sampai %d", ErrCorrupt, f.MaskLen, chip.HelperMaskMaxSize)
	}
	mask, err := hex.DecodeString(f.Mask)
	if err != nil || len(mask) != f.MaskLen {
		return chip.HelperData{}, fmt.Errorf("%w: topeng data bantu tidak sama dengan panjang tercatat %d byte", ErrCorrupt, f.MaskLen)
	}
	check, err := hex.DecodeString(f.Check)
	if err != nil || len(check) != chip.HelperChkSize {
		return chip.HelperData{}, fmt.Errorf("%w: nilai cek data bantu", ErrCorrupt)
	}
	h := chip.HelperData{Thresh: f.Thresh, Mask: mask}
	copy(h.Check[:], check)
	return h, nil
}

func (s *Store) ForgetEnrollment() error {
	if err := s.remove(fileCertificate); err != nil {
		return err
	}
	return s.remove(fileHelper)
}

func (s *Store) CertificatePath() string {
	path, _ := s.path(fileCertificate)
	return path
}
