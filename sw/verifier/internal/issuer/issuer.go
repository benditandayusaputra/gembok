package issuer

import (
	"errors"
	"time"

	"gembok/verifier/internal/lms"
)

type Signer interface {
	PublicKey() lms.PublicKey
	Sign(msg []byte) ([]byte, error)
}

type Usage interface {
	Used() uint32
	Capacity() uint32
}

type Issuer struct {
	signer Signer
	usage  Usage
	now    func() time.Time
}

func New(signer Signer, usage Usage, now func() time.Time) (*Issuer, error) {
	if signer == nil || usage == nil {
		return nil, errors.New("penerbit: penanda tangan dan pencatat pemakaian wajib ada")
	}
	pub := signer.PublicKey()
	if pub.Tree != lms.LMS_SHA256_M32_H10 || pub.OTS != lms.LMOTS_SHA256_N32_W8 {
		return nil, errors.New("penerbit: kunci harus LMS_SHA256_M32_H10 dengan LMOTS_SHA256_N32_W8")
	}
	if now == nil {
		now = time.Now
	}
	return &Issuer{signer: signer, usage: usage, now: now}, nil
}

func (i *Issuer) PublicKey() lms.PublicKey {
	return i.signer.PublicKey()
}

func (i *Issuer) Used() uint32 {
	return i.usage.Used()
}

func (i *Issuer) Capacity() uint32 {
	return i.usage.Capacity()
}

func (i *Issuer) Issue(chipID string, k int, ek []byte) (Certificate, error) {
	name, ok := ParameterName(k)
	if !ok {
		return Certificate{}, ErrBadCertificate
	}
	if chipID == "" {
		chipID = DefaultChipID(ek)
	}
	c := Certificate{
		Version:   CertificateVersion,
		ChipID:    chipID,
		Parameter: name,
		EK:        append([]byte(nil), ek...),
		IssuerID:  i.PublicKey().ID,
		IssuedAt:  i.now().UTC().Truncate(time.Second),
	}
	if err := c.Validate(); err != nil {
		return Certificate{}, err
	}
	sig, err := i.signer.Sign(c.Body())
	if err != nil {
		return Certificate{}, err
	}
	c.Signature = sig
	return c, nil
}
