package lms

import (
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"errors"
	"fmt"
	"sync"
	"testing"
)

type memoryCounter struct {
	mu   sync.Mutex
	next uint32
	fail error
}

func (c *memoryCounter) Reserve() (uint32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.fail != nil {
		return 0, c.fail
	}
	q := c.next
	c.next++
	return q, nil
}

func newTestSigner(t *testing.T, tree TreeType, ots OTSType) (*Signer, *memoryCounter) {
	t.Helper()
	key, err := GenerateKey(rand.Reader, tree, ots)
	if err != nil {
		t.Fatal(err)
	}
	built, err := key.BuildTree()
	if err != nil {
		t.Fatal(err)
	}
	counter := &memoryCounter{}
	signer, err := NewSigner(key, built, counter, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	return signer, counter
}

func TestSignVerifyRoundTrip(t *testing.T) {
	combos := []struct {
		tree TreeType
		ots  OTSType
	}{
		{LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W1},
		{LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W2},
		{LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W4},
		{LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8},
		{LMS_SHA256_M32_H10, LMOTS_SHA256_N32_W8},
	}
	for _, combo := range combos {
		t.Run(fmt.Sprintf("%v_%v", combo.tree, combo.ots), func(t *testing.T) {
			signer, _ := newTestSigner(t, combo.tree, combo.ots)
			pub := signer.PublicKey()
			parsed, err := ParsePublicKey(pub.Bytes())
			if err != nil {
				t.Fatal(err)
			}
			if parsed != pub {
				t.Fatal("kunci publik berubah setelah dikemas dan dibaca ulang")
			}
			size, _ := SignatureSize(combo.tree, combo.ots)
			other, _ := newTestSigner(t, LMS_SHA256_M32_H5, combo.ots)
			for i := 0; i < 3; i++ {
				msg := []byte(fmt.Sprintf("pesan uji %d", i))
				sig, err := signer.Sign(msg)
				if err != nil {
					t.Fatal(err)
				}
				if len(sig) != size {
					t.Fatalf("panjang tanda tangan %d, harap %d", len(sig), size)
				}
				if q, _ := LeafIndex(sig); q != uint32(i) {
					t.Fatalf("indeks daun %d, harap %d", q, i)
				}
				if !parsed.Verify(msg, sig) {
					t.Fatal("tanda tangan sah ditolak")
				}
				if parsed.Verify(append(msg, '!'), sig) {
					t.Fatal("pesan lain diterima")
				}
				if other.PublicKey().Verify(msg, sig) {
					t.Fatal("kunci publik lain menerima tanda tangan")
				}
				bad := bytes.Clone(sig)
				bad[len(bad)/2] ^= 4
				if parsed.Verify(msg, bad) {
					t.Fatal("tanda tangan diubah diterima")
				}
			}
		})
	}
}

func TestSignerUsesEveryLeafOnceThenStops(t *testing.T) {
	signer, counter := newTestSigner(t, LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8)
	capacity, _ := Capacity(LMS_SHA256_M32_H5)
	seen := map[uint32]bool{}
	msg := []byte("sertifikat")
	for i := uint32(0); i < capacity; i++ {
		sig, err := signer.Sign(msg)
		if err != nil {
			t.Fatalf("tanda tangan ke-%d: %v", i, err)
		}
		q, _ := LeafIndex(sig)
		if seen[q] {
			t.Fatalf("indeks daun %d dipakai dua kali", q)
		}
		seen[q] = true
		if !signer.PublicKey().Verify(msg, sig) {
			t.Fatalf("tanda tangan ke-%d ditolak", i)
		}
	}
	if len(seen) != int(capacity) {
		t.Fatalf("%d daun terpakai, harap %d", len(seen), capacity)
	}
	if _, err := signer.Sign(msg); !errors.Is(err, ErrExhausted) {
		t.Fatalf("galat setelah daun habis: %v", err)
	}
	if counter.next != capacity+1 {
		t.Fatalf("pencacah %d", counter.next)
	}
}

func TestSignerReleasesNothingWhenCounterFails(t *testing.T) {
	signer, counter := newTestSigner(t, LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8)
	counter.fail = errors.New("cakram penuh")
	sig, err := signer.Sign([]byte("x"))
	if err == nil || sig != nil {
		t.Fatalf("tanda tangan dilepas walau pencacah gagal: %v", err)
	}
	counter.fail = nil
	sig, err = signer.Sign([]byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if q, _ := LeafIndex(sig); q != 0 {
		t.Fatalf("indeks daun %d setelah kegagalan, harap 0", q)
	}
}

type failingReader struct{}

func (failingReader) Read([]byte) (int, error) {
	return 0, errors.New("sumber acak mati")
}

func TestSignerFailsClosedWithoutRandomness(t *testing.T) {
	key, err := GenerateKey(rand.Reader, LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := key.BuildTree()
	if err != nil {
		t.Fatal(err)
	}
	counter := &memoryCounter{}
	signer, err := NewSigner(key, tree, counter, failingReader{})
	if err != nil {
		t.Fatal(err)
	}
	if sig, err := signer.Sign([]byte("x")); err == nil || sig != nil {
		t.Fatal("tanda tangan dilepas tanpa sumber acak")
	}
	if counter.next != 1 {
		t.Fatalf("daun yang gagal harus tetap hangus, pencacah %d", counter.next)
	}
	if _, err := GenerateKey(failingReader{}, LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8); err == nil {
		t.Fatal("kunci dibangkitkan tanpa sumber acak")
	}
}

func TestSignerConcurrentUseKeepsLeavesDistinct(t *testing.T) {
	signer, _ := newTestSigner(t, LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8)
	const workers = 16
	const perWorker = 2
	results := make(chan uint32, workers*perWorker)
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < perWorker; i++ {
				sig, err := signer.Sign([]byte("serentak"))
				if err != nil {
					t.Error(err)
					return
				}
				q, _ := LeafIndex(sig)
				results <- q
			}
		}()
	}
	wg.Wait()
	close(results)
	seen := map[uint32]bool{}
	for q := range results {
		if seen[q] {
			t.Fatalf("indeks daun %d dipakai dua kali", q)
		}
		seen[q] = true
	}
	if len(seen) != workers*perWorker {
		t.Fatalf("%d tanda tangan, harap %d", len(seen), workers*perWorker)
	}
}

func TestTreeFromLeaves(t *testing.T) {
	key, err := GenerateKey(rand.Reader, LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := key.BuildTree()
	if err != nil {
		t.Fatal(err)
	}
	leaves := tree.Leaves()
	again, err := TreeFromLeaves(key.Tree, key.ID, leaves)
	if err != nil {
		t.Fatal(err)
	}
	if again.Root() != tree.Root() {
		t.Fatal("akar dari daun tersimpan berbeda")
	}
	leaves[100] ^= 1
	bad, err := TreeFromLeaves(key.Tree, key.ID, leaves)
	if err != nil {
		t.Fatal(err)
	}
	if bad.Root() == tree.Root() {
		t.Fatal("daun yang rusak tidak mengubah akar")
	}
	if _, err := TreeFromLeaves(key.Tree, key.ID, leaves[:len(leaves)-1]); !errors.Is(err, ErrBadTree) {
		t.Fatalf("panjang daun salah: %v", err)
	}
	if _, err := TreeFromLeaves(LMS_SHA256_M32_H20, key.ID, leaves); !errors.Is(err, ErrUnknownParameters) {
		t.Fatalf("tinggi tidak didukung: %v", err)
	}
}

func TestSignerRejectsMismatchedTree(t *testing.T) {
	small, err := GenerateKey(rand.Reader, LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := small.BuildTree()
	if err != nil {
		t.Fatal(err)
	}
	big := small
	big.Tree = LMS_SHA256_M32_H10
	if _, err := NewSigner(big, tree, &memoryCounter{}, rand.Reader); !errors.Is(err, ErrBadTree) {
		t.Fatalf("pohon salah tinggi diterima: %v", err)
	}
	if _, err := NewSigner(small, nil, &memoryCounter{}, rand.Reader); !errors.Is(err, ErrBadTree) {
		t.Fatalf("pohon kosong diterima: %v", err)
	}
	if _, err := NewSigner(small, tree, nil, rand.Reader); err == nil {
		t.Fatal("pencacah kosong diterima")
	}
	huge := small
	huge.Tree = LMS_SHA256_M32_H20
	if _, err := huge.BuildTree(); !errors.Is(err, ErrUnknownParameters) {
		t.Fatalf("pohon terlalu tinggi dibangun: %v", err)
	}
}

func TestSelfCheckCatchesCorruptedTree(t *testing.T) {
	key, err := GenerateKey(rand.Reader, LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8)
	if err != nil {
		t.Fatal(err)
	}
	tree, err := key.BuildTree()
	if err != nil {
		t.Fatal(err)
	}
	signer, err := NewSigner(key, tree, &memoryCounter{}, rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tree.nodes[1<<5+1][0] ^= 1
	if sig, err := signer.Sign([]byte("x")); !errors.Is(err, ErrSelfCheck) || sig != nil {
		t.Fatalf("tanda tangan dari pohon rusak dilepas: %v", err)
	}
}

func TestParsePublicKeyRejectsMalformed(t *testing.T) {
	signer, _ := newTestSigner(t, LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8)
	good := signer.PublicKey().Bytes()
	if len(good) != PublicKeySize {
		t.Fatalf("panjang kunci publik %d", len(good))
	}
	for _, n := range []int{0, 1, PublicKeySize - 1, PublicKeySize + 1} {
		if _, err := ParsePublicKey(make([]byte, n)); !errors.Is(err, ErrBadPublicKey) {
			t.Fatalf("panjang %d diterima", n)
		}
	}
	badTree := bytes.Clone(good)
	binary.BigEndian.PutUint32(badTree, 4)
	if _, err := ParsePublicKey(badTree); !errors.Is(err, ErrBadPublicKey) {
		t.Fatal("tipe pohon tidak dikenal diterima")
	}
	badOTS := bytes.Clone(good)
	binary.BigEndian.PutUint32(badOTS[4:], 5)
	if _, err := ParsePublicKey(badOTS); !errors.Is(err, ErrBadPublicKey) {
		t.Fatal("tipe LM-OTS tidak dikenal diterima")
	}
}

func TestVerifyRejectsMalformedSignatures(t *testing.T) {
	signer, _ := newTestSigner(t, LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8)
	pub := signer.PublicKey()
	msg := []byte("pesan")
	sig, err := signer.Sign(msg)
	if err != nil {
		t.Fatal(err)
	}
	op, _ := pub.OTS.params()
	otsSize := op.signatureSize()
	mutate := func(f func(b []byte) []byte) []byte {
		return f(bytes.Clone(sig))
	}
	cases := map[string][]byte{
		"kosong":            nil,
		"tujuh byte":        sig[:7],
		"hanya kepala":      sig[:8],
		"tanpa jalur":       sig[:8+otsSize],
		"jalur kurang satu": sig[:len(sig)-HashSize],
		"indeks di luar pohon": mutate(func(b []byte) []byte {
			binary.BigEndian.PutUint32(b, 32)
			return b
		}),
		"indeks lain": mutate(func(b []byte) []byte {
			binary.BigEndian.PutUint32(b, 1)
			return b
		}),
		"tipe LM-OTS lain": mutate(func(b []byte) []byte {
			binary.BigEndian.PutUint32(b[4:], uint32(LMOTS_SHA256_N32_W4))
			return b
		}),
		"tipe pohon lain": mutate(func(b []byte) []byte {
			binary.BigEndian.PutUint32(b[4+otsSize:], uint32(LMS_SHA256_M32_H10))
			return b
		}),
		"jalur berlebih": append(bytes.Clone(sig), make([]byte, HashSize)...),
	}
	for name, bad := range cases {
		if pub.Verify(msg, bad) {
			t.Fatalf("%s: diterima", name)
		}
	}
	unknown := pub
	unknown.Tree = 99
	if unknown.Verify(msg, sig) {
		t.Fatal("kunci dengan tipe pohon tidak dikenal menerima tanda tangan")
	}
	unknown = pub
	unknown.OTS = 0
	if unknown.Verify(msg, sig) {
		t.Fatal("kunci dengan tipe LM-OTS tidak dikenal menerima tanda tangan")
	}
	if !pub.Verify(msg, sig) {
		t.Fatal("tanda tangan asli ditolak")
	}
}

func TestParameterTables(t *testing.T) {
	sizes := []struct {
		tree TreeType
		ots  OTSType
		want int
	}{
		{LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8, 1292},
		{LMS_SHA256_M32_H10, LMOTS_SHA256_N32_W8, 1452},
		{LMS_SHA256_M32_H10, LMOTS_SHA256_N32_W4, 2508},
		{LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W1, 8684},
		{LMS_SHA256_M32_H25, LMOTS_SHA256_N32_W2, 5100},
	}
	for _, s := range sizes {
		got, ok := SignatureSize(s.tree, s.ots)
		if !ok || got != s.want {
			t.Fatalf("%v %v: ukuran %d, harap %d", s.tree, s.ots, got, s.want)
		}
	}
	if _, ok := SignatureSize(0, LMOTS_SHA256_N32_W8); ok {
		t.Fatal("tipe pohon 0 diterima")
	}
	if c, ok := Capacity(LMS_SHA256_M32_H10); !ok || c != 1024 {
		t.Fatalf("kapasitas H10 %d", c)
	}
	if LMS_SHA256_M32_H10.String() != "LMS_SHA256_M32_H10" || LMOTS_SHA256_N32_W8.String() != "LMOTS_SHA256_N32_W8" {
		t.Fatal("nama parameter salah")
	}
}
