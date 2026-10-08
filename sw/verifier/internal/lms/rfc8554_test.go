package lms

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

type rfcCase struct {
	Source    string `json:"sumber"`
	PublicKey string `json:"kunci_publik_hss"`
	Message   string `json:"pesan"`
	Signature string `json:"tanda_tangan_hss"`
	Private   []struct {
		Seed string `json:"benih"`
		ID   string `json:"id"`
	} `json:"kunci_rahasia"`
}

type hssLink struct {
	key PublicKey
	msg []byte
	sig []byte
}

func mustHex(t *testing.T, s string) []byte {
	t.Helper()
	b, err := hex.DecodeString(s)
	if err != nil {
		t.Fatalf("hex tidak sah: %v", err)
	}
	return b
}

func loadRFCCase(t *testing.T, name string) (rfcCase, []hssLink) {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	var c rfcCase
	if err := json.Unmarshal(raw, &c); err != nil {
		t.Fatal(err)
	}
	return c, splitHSS(t, mustHex(t, c.PublicKey), mustHex(t, c.Message), mustHex(t, c.Signature))
}

func splitHSS(t *testing.T, pub, msg, sig []byte) []hssLink {
	t.Helper()
	if len(pub) != 4+PublicKeySize {
		t.Fatalf("panjang kunci publik HSS %d", len(pub))
	}
	levels := binary.BigEndian.Uint32(pub)
	key, err := ParsePublicKey(pub[4:])
	if err != nil {
		t.Fatal(err)
	}
	if len(sig) < 4 {
		t.Fatal("tanda tangan HSS terlalu pendek")
	}
	signedKeys := binary.BigEndian.Uint32(sig)
	if signedKeys+1 != levels {
		t.Fatalf("Nspk %d tidak cocok dengan L %d", signedKeys, levels)
	}
	rest := sig[4:]
	var links []hssLink
	for i := uint32(0); i < signedKeys; i++ {
		size, ok := SignatureSize(key.Tree, key.OTS)
		if !ok || len(rest) < size+PublicKeySize {
			t.Fatalf("tanda tangan tingkat %d terpotong", i)
		}
		next := rest[size : size+PublicKeySize]
		links = append(links, hssLink{key: key, msg: next, sig: rest[:size]})
		key, err = ParsePublicKey(next)
		if err != nil {
			t.Fatal(err)
		}
		rest = rest[size+PublicKeySize:]
	}
	return append(links, hssLink{key: key, msg: msg, sig: rest})
}

func TestRFC8554Vectors(t *testing.T) {
	expect := map[string][]struct {
		tree TreeType
		ots  OTSType
		q    uint32
	}{
		"rfc8554_kasus1.json": {
			{LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8, 5},
			{LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8, 10},
		},
		"rfc8554_kasus2.json": {
			{LMS_SHA256_M32_H10, LMOTS_SHA256_N32_W4, 3},
			{LMS_SHA256_M32_H5, LMOTS_SHA256_N32_W8, 4},
		},
	}
	for name, want := range expect {
		t.Run(name, func(t *testing.T) {
			_, links := loadRFCCase(t, name)
			if len(links) != len(want) {
				t.Fatalf("jumlah tingkat %d, harap %d", len(links), len(want))
			}
			for i, link := range links {
				if link.key.Tree != want[i].tree || link.key.OTS != want[i].ots {
					t.Fatalf("tingkat %d: parameter %v %v", i, link.key.Tree, link.key.OTS)
				}
				q, ok := LeafIndex(link.sig)
				if !ok || q != want[i].q {
					t.Fatalf("tingkat %d: q = %d, harap %d", i, q, want[i].q)
				}
				size, _ := SignatureSize(link.key.Tree, link.key.OTS)
				if len(link.sig) != size {
					t.Fatalf("tingkat %d: panjang tanda tangan %d, harap %d", i, len(link.sig), size)
				}
				if !link.key.Verify(link.msg, link.sig) {
					t.Fatalf("tingkat %d: vektor RFC 8554 ditolak", i)
				}
			}
		})
	}
}

func TestRFC8554RejectsTampering(t *testing.T) {
	for _, name := range []string{"rfc8554_kasus1.json", "rfc8554_kasus2.json"} {
		_, links := loadRFCCase(t, name)
		for level, link := range links {
			for pos := 0; pos < len(link.sig); pos++ {
				bad := bytes.Clone(link.sig)
				bad[pos] ^= 1 << (pos % 8)
				if link.key.Verify(link.msg, bad) {
					t.Fatalf("%s tingkat %d: tanda tangan dengan byte %d diubah diterima", name, level, pos)
				}
			}
			for pos := 0; pos < len(link.msg); pos++ {
				bad := bytes.Clone(link.msg)
				bad[pos] ^= 0x80
				if link.key.Verify(bad, link.sig) {
					t.Fatalf("%s tingkat %d: pesan dengan byte %d diubah diterima", name, level, pos)
				}
			}
			for pos := 0; pos < IDSize; pos++ {
				bad := link.key
				bad.ID[pos] ^= 1
				if bad.Verify(link.msg, link.sig) {
					t.Fatalf("%s tingkat %d: pengenal diubah diterima", name, level)
				}
			}
			for pos := 0; pos < HashSize; pos++ {
				bad := link.key
				bad.Root[pos] ^= 1
				if bad.Verify(link.msg, link.sig) {
					t.Fatalf("%s tingkat %d: akar diubah diterima", name, level)
				}
			}
			if link.key.Verify(link.msg, link.sig[:len(link.sig)-1]) {
				t.Fatalf("%s tingkat %d: tanda tangan terpotong diterima", name, level)
			}
			if link.key.Verify(link.msg, append(bytes.Clone(link.sig), 0)) {
				t.Fatalf("%s tingkat %d: tanda tangan dengan byte tambahan diterima", name, level)
			}
			if link.key.Verify(append(bytes.Clone(link.msg), 0), link.sig) {
				t.Fatalf("%s tingkat %d: pesan dengan byte tambahan diterima", name, level)
			}
		}
	}
}

func rfcPrivateKeys(t *testing.T) (rfcCase, []hssLink, []PrivateKey) {
	t.Helper()
	c, links := loadRFCCase(t, "rfc8554_kasus2.json")
	if len(c.Private) != len(links) {
		t.Fatalf("jumlah kunci rahasia %d, tingkat %d", len(c.Private), len(links))
	}
	keys := make([]PrivateKey, len(links))
	for i, link := range links {
		keys[i] = PrivateKey{Tree: link.key.Tree, OTS: link.key.OTS}
		copy(keys[i].ID[:], mustHex(t, c.Private[i].ID))
		copy(keys[i].Seed[:], mustHex(t, c.Private[i].Seed))
	}
	return c, links, keys
}

func TestRFC8554KeyGeneration(t *testing.T) {
	_, links, keys := rfcPrivateKeys(t)
	for i, key := range keys {
		tree, err := key.BuildTree()
		if err != nil {
			t.Fatal(err)
		}
		pub := key.PublicKey(tree)
		if !bytes.Equal(pub.Bytes(), links[i].key.Bytes()) {
			t.Fatalf("tingkat %d: kunci publik dari benih %x, vektor RFC %x", i, pub.Bytes(), links[i].key.Bytes())
		}
	}
}

func TestRFC8554SignatureReproduction(t *testing.T) {
	_, links, keys := rfcPrivateKeys(t)
	for i, key := range keys {
		tree, err := key.BuildTree()
		if err != nil {
			t.Fatal(err)
		}
		q, _ := LeafIndex(links[i].sig)
		var randomizer [HashSize]byte
		copy(randomizer[:], links[i].sig[8:8+HashSize])
		sig, err := key.signLeaf(tree, q, &randomizer, links[i].msg)
		if err != nil {
			t.Fatal(err)
		}
		if !bytes.Equal(sig, links[i].sig) {
			t.Fatalf("tingkat %d: tanda tangan yang dibuat ulang berbeda dari vektor RFC", i)
		}
	}
}

func TestCoefExampleFromRFC(t *testing.T) {
	s := []byte{0x12, 0x34}
	if got := coef(s, 7, 1); got != 0 {
		t.Fatalf("coef(S, 7, 1) = %d", got)
	}
	if got := coef(s, 0, 4); got != 1 {
		t.Fatalf("coef(S, 0, 4) = %d", got)
	}
	wantNibbles := []int{1, 2, 3, 4}
	for i, want := range wantNibbles {
		if got := coef(s, i, 4); got != want {
			t.Fatalf("coef(S, %d, 4) = %d, harap %d", i, got, want)
		}
	}
	wantPairs := []int{0, 1, 0, 2, 0, 3, 1, 0}
	for i, want := range wantPairs {
		if got := coef(s, i, 2); got != want {
			t.Fatalf("coef(S, %d, 2) = %d, harap %d", i, got, want)
		}
	}
	if got := coef(s, 1, 8); got != 0x34 {
		t.Fatalf("coef(S, 1, 8) = %#x", got)
	}
}

func TestChecksumBounds(t *testing.T) {
	zero := make([]byte, HashSize)
	full := bytes.Repeat([]byte{0xff}, HashSize)
	cases := []struct {
		ots  OTSType
		want uint16
	}{
		{LMOTS_SHA256_N32_W1, 256 << 7},
		{LMOTS_SHA256_N32_W2, 128 * 3 << 6},
		{LMOTS_SHA256_N32_W4, 64 * 15 << 4},
		{LMOTS_SHA256_N32_W8, 32 * 255},
	}
	for _, c := range cases {
		op, _ := c.ots.params()
		if got := checksum(zero, op); got != c.want {
			t.Fatalf("%v: checksum pesan nol %d, harap %d", c.ots, got, c.want)
		}
		if got := checksum(full, op); got != 0 {
			t.Fatalf("%v: checksum pesan penuh %d, harap 0", c.ots, got)
		}
	}
}
