package issuer

import (
	"bytes"
	"crypto/mlkem"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"gembok/verifier/internal/lms"
)

type testCounter struct {
	mu   sync.Mutex
	next uint32
}

func (c *testCounter) Reserve() (uint32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	q := c.next
	c.next++
	return q, nil
}

func (c *testCounter) Used() uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.next
}

func (c *testCounter) Capacity() uint32 {
	return 1024
}

var (
	issuerOnce sync.Once
	issuerKey  lms.PrivateKey
	issuerTree *lms.Tree
)

func newTestIssuer(t *testing.T, now func() time.Time) (*Issuer, *testCounter) {
	t.Helper()
	issuerOnce.Do(func() {
		key, err := lms.GenerateKey(rand.Reader, lms.LMS_SHA256_M32_H10, lms.LMOTS_SHA256_N32_W8)
		if err != nil {
			panic(err)
		}
		tree, err := key.BuildTree()
		if err != nil {
			panic(err)
		}
		issuerKey, issuerTree = key, tree
	})
	counter := &testCounter{}
	signer, err := lms.NewSigner(issuerKey, issuerTree, counter, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	iss, err := New(signer, counter, now)
	if err != nil {
		t.Fatal(err)
	}
	return iss, counter
}

func fixedEK(t *testing.T, k int, fill byte) []byte {
	t.Helper()
	seed := bytes.Repeat([]byte{fill}, 64)
	switch k {
	case 3:
		dk, err := mlkem.NewDecapsulationKey768(seed)
		if err != nil {
			t.Fatal(err)
		}
		return dk.EncapsulationKey().Bytes()
	case 4:
		dk, err := mlkem.NewDecapsulationKey1024(seed)
		if err != nil {
			t.Fatal(err)
		}
		return dk.EncapsulationKey().Bytes()
	}
	t.Fatalf("k=%d", k)
	return nil
}

func unhex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatal(err)
	}
	return b
}

func TestCertificateBodyLayout(t *testing.T) {
	ek := fixedEK(t, 3, 0x42)
	var id [lms.IDSize]byte
	copy(id[:], unhex(t, "00112233445566778899aabbccddeeff"))
	c := Certificate{
		Version:   1,
		ChipID:    "GEMBOK-UJI",
		Parameter: "ML-KEM-768",
		EK:        ek,
		IssuerID:  id,
		IssuedAt:  time.Unix(0x68E61A00, 0).UTC(),
	}
	var want []byte
	want = append(want, unhex(t, "00000014")...)
	want = append(want, "GEMBOK-v1/sertifikat"...)
	want = append(want, unhex(t, "0000000101")...)
	want = append(want, unhex(t, "0000000a")...)
	want = append(want, "GEMBOK-UJI"...)
	want = append(want, unhex(t, "0000000a")...)
	want = append(want, "ML-KEM-768"...)
	want = append(want, unhex(t, "000004a0")...)
	want = append(want, ek...)
	want = append(want, unhex(t, "00000010")...)
	want = append(want, id[:]...)
	want = append(want, unhex(t, "000000080000000068e61a00")...)
	body := c.Body()
	if !bytes.Equal(body, want) {
		t.Fatalf("susunan badan sertifikat berbeda\n dapat %x\nharap %x", body[:64], want[:64])
	}
	parsed, err := ParseBody(body)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(parsed.Body(), body) || parsed.ChipID != c.ChipID || parsed.Parameter != c.Parameter || !parsed.IssuedAt.Equal(c.IssuedAt) || parsed.IssuerID != id {
		t.Fatal("badan sertifikat tidak kembali utuh")
	}
}

func TestParseBodyRejectsMalformed(t *testing.T) {
	c := Certificate{Version: 1, ChipID: "A", Parameter: "ML-KEM-768", EK: fixedEK(t, 3, 1), IssuedAt: time.Unix(1, 0)}
	body := c.Body()
	wrongDomain := bytes.Clone(body)
	wrongDomain[4] = 'X'
	cases := map[string][]byte{
		"kosong":           nil,
		"terpotong":        body[:len(body)-1],
		"byte sisa":        append(bytes.Clone(body), 0),
		"ruas tambahan":    appendField(bytes.Clone(body), []byte("x")),
		"domain lain":      wrongDomain,
		"panjang berlebih": append([]byte{0xff, 0xff, 0xff, 0xff}, body...),
	}
	for name, b := range cases {
		if _, err := ParseBody(b); !errors.Is(err, ErrBadCertificate) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestLengthPrefixesPreventFieldShifting(t *testing.T) {
	ek := fixedEK(t, 3, 1)
	a := Certificate{Version: 1, ChipID: "AB", Parameter: "ML-KEM-768", EK: ek, IssuedAt: time.Unix(5, 0)}
	b := a
	b.ChipID = "A"
	b.Parameter = "BML-KEM-768"
	if bytes.Equal(a.Body(), b.Body()) {
		t.Fatal("dua sertifikat berbeda memberi badan yang sama")
	}
	if !strings.Contains(string(a.Body()[:30]), "sertifikat") {
		t.Fatal("pemisah domain tidak ada di awal badan")
	}
}

func TestIssueAndVerify(t *testing.T) {
	now := time.Date(2026, 10, 8, 9, 30, 15, 999, time.UTC)
	iss, counter := newTestIssuer(t, func() time.Time { return now })
	for _, k := range []int{3, 4} {
		ek := fixedEK(t, k, byte(k))
		c, err := iss.Issue("", k, ek)
		if err != nil {
			t.Fatal(err)
		}
		if c.ChipID != DefaultChipID(ek) || !strings.HasPrefix(c.ChipID, "GEMBOK-") || !ValidChipID(c.ChipID) {
			t.Fatalf("id chip bawaan %q", c.ChipID)
		}
		if !c.IssuedAt.Equal(now.Truncate(time.Second)) {
			t.Fatalf("waktu terbit %v", c.IssuedAt)
		}
		if err := Verify(iss.PublicKey(), c); err != nil {
			t.Fatalf("k=%d: sertifikat sah ditolak: %v", k, err)
		}
		size, _ := lms.SignatureSize(lms.LMS_SHA256_M32_H10, lms.LMOTS_SHA256_N32_W8)
		if len(c.Signature) != size {
			t.Fatalf("panjang tanda tangan %d", len(c.Signature))
		}
	}
	if counter.Used() != 2 || iss.Used() != 2 || iss.Capacity() != 1024 {
		t.Fatalf("pemakaian %d", counter.Used())
	}
	named, err := iss.Issue("Papan-DE10.01", 3, fixedEK(t, 3, 9))
	if err != nil || named.ChipID != "Papan-DE10.01" {
		t.Fatalf("id chip pilihan: %v %q", err, named.ChipID)
	}
}

func TestIssueRejectsBadInputWithoutSpendingLeaf(t *testing.T) {
	iss, counter := newTestIssuer(t, nil)
	badEK := bytes.Repeat([]byte{0xff}, 1184)
	cases := []struct {
		id string
		k  int
		ek []byte
	}{
		{"", 2, make([]byte, 800)},
		{"", 3, badEK},
		{"", 3, fixedEK(t, 4, 1)},
		{"-awal-salah", 3, fixedEK(t, 3, 1)},
		{"spasi tidak boleh", 3, fixedEK(t, 3, 1)},
		{"../keluar", 3, fixedEK(t, 3, 1)},
		{strings.Repeat("a", 65), 3, fixedEK(t, 3, 1)},
		{"üñî", 3, fixedEK(t, 3, 1)},
	}
	for _, c := range cases {
		if _, err := iss.Issue(c.id, c.k, c.ek); err == nil {
			t.Fatalf("diterbitkan untuk id %q k=%d", c.id, c.k)
		}
	}
	if counter.Used() != 0 {
		t.Fatalf("%d daun terpakai untuk masukan salah", counter.Used())
	}
}

func TestTamperedCertificatesAreRejected(t *testing.T) {
	iss, _ := newTestIssuer(t, nil)
	c, err := iss.Issue("GEMBOK-ASLI", 3, fixedEK(t, 3, 7))
	if err != nil {
		t.Fatal(err)
	}
	cloneEK := fixedEK(t, 3, 8)
	mutations := map[string]struct {
		mutate func(c *Certificate)
		want   error
	}{
		"versi":         {func(c *Certificate) { c.Version = 2 }, ErrBadCertificate},
		"id chip":       {func(c *Certificate) { c.ChipID = "GEMBOK-PALSU" }, ErrBadSignature},
		"parameter":     {func(c *Certificate) { c.Parameter = "ML-KEM-1024" }, ErrBadCertificate},
		"ek tiruan":     {func(c *Certificate) { c.EK = cloneEK }, ErrBadSignature},
		"rho ek":        {func(c *Certificate) { c.EK = bytes.Clone(c.EK); c.EK[len(c.EK)-1] ^= 1 }, ErrBadSignature},
		"ek rusak":      {func(c *Certificate) { c.EK = bytes.Clone(c.EK); c.EK[0], c.EK[1] = 0xff, 0xff }, ErrBadCertificate},
		"waktu":         {func(c *Certificate) { c.IssuedAt = c.IssuedAt.Add(time.Second) }, ErrBadSignature},
		"penerbit":      {func(c *Certificate) { c.IssuerID[0] ^= 1 }, ErrWrongIssuer},
		"tanda tangan":  {func(c *Certificate) { c.Signature = bytes.Clone(c.Signature); c.Signature[600] ^= 1 }, ErrBadSignature},
		"tanda kosong":  {func(c *Certificate) { c.Signature = nil }, ErrBadSignature},
		"tanda pendek":  {func(c *Certificate) { c.Signature = c.Signature[:len(c.Signature)-32] }, ErrBadSignature},
		"tanda penuh 0": {func(c *Certificate) { c.Signature = make([]byte, len(c.Signature)) }, ErrBadSignature},
	}
	for name, m := range mutations {
		bad := c
		m.mutate(&bad)
		if err := Verify(iss.PublicKey(), bad); !errors.Is(err, m.want) {
			t.Fatalf("%s: %v, harap %v", name, err, m.want)
		}
	}
	otherID := iss.PublicKey()
	otherID.ID[0] ^= 1
	if err := Verify(otherID, c); !errors.Is(err, ErrWrongIssuer) {
		t.Fatalf("kunci penerbit dengan pengenal lain: %v", err)
	}
	otherRoot := iss.PublicKey()
	otherRoot.Root[0] ^= 1
	if err := Verify(otherRoot, c); !errors.Is(err, ErrBadSignature) {
		t.Fatalf("kunci penerbit dengan akar lain: %v", err)
	}
	if err := Verify(iss.PublicKey(), c); err != nil {
		t.Fatalf("sertifikat asli ikut berubah: %v", err)
	}
}

func TestCertificateJSON(t *testing.T) {
	iss, _ := newTestIssuer(t, nil)
	c, err := iss.Issue("GEMBOK-JSON", 4, fixedEK(t, 4, 3))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]any
	if err := json.Unmarshal(raw, &fields); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"versi", "id_chip", "parameter", "ek", "penerbit", "diterbitkan", "algoritma_tanda_tangan", "tanda_tangan"} {
		if _, ok := fields[key]; !ok {
			t.Fatalf("ruas %s tidak ada", key)
		}
	}
	var back Certificate
	if err := json.Unmarshal(raw, &back); err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(back.Body(), c.Body()) || !bytes.Equal(back.Signature, c.Signature) {
		t.Fatal("sertifikat berubah setelah JSON")
	}
	if err := Verify(iss.PublicKey(), back); err != nil {
		t.Fatal(err)
	}
	broken := map[string]string{
		"ruas asing":    strings.Replace(string(raw), `"versi"`, `"asing":1,"versi"`, 1),
		"algoritma":     strings.Replace(string(raw), "LMOTS_SHA256_N32_W8", "LMOTS_SHA256_N32_W4", 1),
		"ek bukan hex":  strings.Replace(string(raw), `"ek":"`, `"ek":"zz`, 1),
		"waktu":         strings.Replace(string(raw), `"diterbitkan":"`, `"diterbitkan":"kemarin`, 1),
		"penerbit":      strings.Replace(string(raw), `"penerbit":"`, `"penerbit":"00`, 1),
		"tanda bukan x": strings.Replace(string(raw), `"tanda_tangan":"`, `"tanda_tangan":"q`, 1),
	}
	for name, s := range broken {
		var c Certificate
		if err := json.Unmarshal([]byte(s), &c); !errors.Is(err, ErrBadCertificate) {
			t.Fatalf("%s: %v", name, err)
		}
	}
}

func TestChipIDRules(t *testing.T) {
	good := []string{"A", "GEMBOK-0001", "papan_de10.nano-2", strings.Repeat("x", 64)}
	bad := []string{"", "-a", ".a", "_a", "a b", "a/b", "a\\b", "..", "a\x00", strings.Repeat("x", 65), "é"}
	for _, id := range good {
		if !ValidChipID(id) {
			t.Fatalf("%q ditolak", id)
		}
	}
	for _, id := range bad {
		if ValidChipID(id) {
			t.Fatalf("%q diterima", id)
		}
	}
}

func TestNewRequiresIssuerParameters(t *testing.T) {
	key, err := lms.GenerateKey(rand.Reader, lms.LMS_SHA256_M32_H5, lms.LMOTS_SHA256_N32_W8)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := key.BuildTree()
	if err != nil {
		t.Fatal(err)
	}
	counter := &testCounter{}
	signer, err := lms.NewSigner(key, tree, counter, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := New(signer, counter, nil); err == nil {
		t.Fatal("kunci H5 diterima sebagai kunci penerbit")
	}
	if _, err := New(nil, counter, nil); err == nil {
		t.Fatal("penanda tangan kosong diterima")
	}
}
