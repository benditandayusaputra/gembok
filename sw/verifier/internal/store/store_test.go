package store

import (
	"bytes"
	"crypto/mlkem"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"gembok/verifier/internal/chip"
	"gembok/verifier/internal/issuer"
	"gembok/verifier/internal/lms"
)

func openTemp(t *testing.T, dir string) *Store {
	t.Helper()
	s, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	return s
}

func TestOpenCreatesPrivateDirectoriesAndLocks(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "data")
	s := openTemp(t, dir)
	for _, sub := range []string{"", "penerbit", "chip"} {
		info, err := os.Stat(filepath.Join(dir, sub))
		if err != nil {
			t.Fatal(err)
		}
		if runtime.GOOS != "windows" && info.Mode().Perm() != 0o700 {
			t.Fatalf("izin %s %v", sub, info.Mode().Perm())
		}
	}
	if _, err := Open(dir); !errors.Is(err, ErrLocked) {
		t.Fatalf("direktori dipakai dua proses: %v", err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := Open(dir)
	if err != nil {
		t.Fatalf("membuka lagi sesudah ditutup: %v", err)
	}
	again.Close()
}

func TestMarkerLockFallback(t *testing.T) {
	path := filepath.Join(t.TempDir(), "proses.lock")
	first, err := lockWithMarkerFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := lockWithMarkerFile(path); !errors.Is(err, ErrLocked) {
		t.Fatalf("kunci kedua: %v", err)
	}
	if err := first.Close(); err != nil {
		t.Fatal(err)
	}
	second, err := lockWithMarkerFile(path)
	if err != nil {
		t.Fatal(err)
	}
	second.Close()
}

func TestPathRejectsTraversal(t *testing.T) {
	s := openTemp(t, t.TempDir())
	bad := []string{"", "/etc/passwd", "../luar.json", "chip/../../x", "chip/./x", "a/b/c", ".tersembunyi", "chip/", "Besar.json", "chip\\x", "c:x", "chip/x y"}
	for _, name := range bad {
		if _, err := s.path(name); !errors.Is(err, ErrBadName) {
			t.Fatalf("%q diterima", name)
		}
		if err := s.writeFile(name, []byte("x"), 0o600); !errors.Is(err, ErrBadName) {
			t.Fatalf("%q bisa ditulis: %v", name, err)
		}
	}
	for _, name := range []string{fileIssuerPrivate, fileIssuerPublic, fileIssuerTree, fileCertificate, fileHelper, "a.b-c_d"} {
		path, err := s.path(name)
		if err != nil {
			t.Fatalf("%q ditolak: %v", name, err)
		}
		if !strings.HasPrefix(path, s.Dir()+string(filepath.Separator)) {
			t.Fatalf("%q keluar dari direktori data: %s", name, path)
		}
	}
}

func TestAtomicWriteAndStrictRead(t *testing.T) {
	s := openTemp(t, t.TempDir())
	if err := s.writeJSON("chip/uji.json", map[string]int{"a": 1}, 0o600); err != nil {
		t.Fatal(err)
	}
	entries, _ := os.ReadDir(filepath.Join(s.Dir(), "chip"))
	if len(entries) != 1 || entries[0].Name() != "uji.json" {
		t.Fatalf("berkas sementara tertinggal: %v", entries)
	}
	info, _ := os.Stat(filepath.Join(s.Dir(), "chip", "uji.json"))
	if runtime.GOOS != "windows" && info.Mode().Perm() != 0o600 {
		t.Fatalf("izin %v", info.Mode().Perm())
	}
	var v struct {
		A int `json:"a"`
	}
	if err := s.readJSON("chip/uji.json", &v); err != nil || v.A != 1 {
		t.Fatalf("baca: %v %d", err, v.A)
	}
	if err := s.readJSON("chip/tidak-ada.json", &v); !errors.Is(err, ErrNotFound) {
		t.Fatalf("berkas tidak ada: %v", err)
	}
	cases := map[string]string{
		"asing.json": `{"a":1,"b":2}`,
		"sisa.json":  `{"a":1} {"a":2}`,
		"rusak.json": `{"a":`,
	}
	for name, content := range cases {
		if err := s.writeFile("chip/"+name, []byte(content), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := s.readJSON("chip/"+name, &v); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("%s: %v", name, err)
		}
	}
	big := bytes.Repeat([]byte(" "), maxFileSize+1)
	if err := s.writeFile("chip/besar.json", big, 0o600); err != nil {
		t.Fatal(err)
	}
	if _, err := s.readFile("chip/besar.json"); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("berkas besar: %v", err)
	}
}

func TestIssuerKeyLifecycle(t *testing.T) {
	dir := t.TempDir()
	s := openTemp(t, dir)
	var messages []string
	keys, err := s.LoadOrCreateIssuer(rand.Reader, func(m string) { messages = append(messages, m) })
	if err != nil {
		t.Fatal(err)
	}
	if !keys.Created || len(messages) == 0 {
		t.Fatalf("kunci baru tidak dilaporkan: %v %v", keys.Created, messages)
	}
	pub, err := s.LoadIssuerPublicKey()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(pub.Bytes(), keys.Signer.PublicKey().Bytes()) {
		t.Fatal("berkas kunci publik berbeda dengan penanda tangan")
	}
	if runtime.GOOS != "windows" {
		info, _ := os.Stat(filepath.Join(dir, "penerbit", "kunci-rahasia.json"))
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("izin kunci rahasia %v", info.Mode().Perm())
		}
	}
	var used []uint32
	for i := 0; i < 3; i++ {
		sig, err := keys.Signer.Sign([]byte{byte(i)})
		if err != nil {
			t.Fatal(err)
		}
		q, _ := lms.LeafIndex(sig)
		used = append(used, q)
		var onDisk issuerPrivateFile
		if err := s.readJSON(fileIssuerPrivate, &onDisk); err != nil {
			t.Fatal(err)
		}
		if onDisk.Next != q+1 {
			t.Fatalf("indeks di cakram %d sesudah memakai daun %d", onDisk.Next, q)
		}
	}
	if keys.Counter.Used() != 3 || keys.Counter.Capacity() != 1024 {
		t.Fatalf("pemakaian %d/%d", keys.Counter.Used(), keys.Counter.Capacity())
	}
	s.Close()

	s2 := openTemp(t, dir)
	reloaded, err := s2.LoadOrCreateIssuer(rand.Reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if reloaded.Created || reloaded.TreeRebuilt {
		t.Fatal("kunci atau pohon dibangun ulang padahal cache utuh")
	}
	if !bytes.Equal(reloaded.Signer.PublicKey().Bytes(), pub.Bytes()) {
		t.Fatal("kunci publik berubah setelah dibuka ulang")
	}
	sig, err := reloaded.Signer.Sign([]byte("lanjut"))
	if err != nil {
		t.Fatal(err)
	}
	if q, _ := lms.LeafIndex(sig); q != 3 {
		t.Fatalf("daun %d dipakai setelah dibuka ulang, harap 3 (sudah dipakai %v)", q, used)
	}
	s2.Close()

	if err := os.Remove(filepath.Join(dir, "penerbit", "pohon.bin")); err != nil {
		t.Fatal(err)
	}
	s3 := openTemp(t, dir)
	rebuilt, err := s3.LoadOrCreateIssuer(rand.Reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !rebuilt.TreeRebuilt || !bytes.Equal(rebuilt.Signer.PublicKey().Bytes(), pub.Bytes()) || rebuilt.Counter.Used() != 4 {
		t.Fatal("pohon tidak dibangun ulang dengan benar")
	}
	s3.Close()

	leaves, _ := os.ReadFile(filepath.Join(dir, "penerbit", "pohon.bin"))
	leaves[77] ^= 1
	os.WriteFile(filepath.Join(dir, "penerbit", "pohon.bin"), leaves, 0o644)
	s4 := openTemp(t, dir)
	repaired, err := s4.LoadOrCreateIssuer(rand.Reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	if !repaired.TreeRebuilt {
		t.Fatal("cache pohon rusak dipakai")
	}
	if sig, err := repaired.Signer.Sign([]byte("x")); err != nil || !pub.Verify([]byte("x"), sig) {
		t.Fatalf("tanda tangan setelah perbaikan: %v", err)
	}
	s4.Close()

	var pubFile issuerPublicFile
	raw, _ := os.ReadFile(filepath.Join(dir, "penerbit", "kunci-publik.json"))
	json.Unmarshal(raw, &pubFile)
	other, _ := lms.GenerateKey(rand.Reader, lms.LMS_SHA256_M32_H5, lms.LMOTS_SHA256_N32_W8)
	otherTree, _ := other.BuildTree()
	forged := pub
	forged.Root = otherTree.Root()
	pubFile.PublicKey = hex.EncodeToString(forged.Bytes())
	tampered, _ := json.Marshal(pubFile)
	os.WriteFile(filepath.Join(dir, "penerbit", "kunci-publik.json"), tampered, 0o644)
	s5 := openTemp(t, dir)
	if _, err := s5.LoadOrCreateIssuer(rand.Reader, nil); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("kunci publik palsu diterima: %v", err)
	}
}

func TestCounterFailureReleasesNoSignature(t *testing.T) {
	dir := t.TempDir()
	s := openTemp(t, dir)
	keys, err := s.LoadOrCreateIssuer(rand.Reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	private := filepath.Join(dir, "penerbit", "kunci-rahasia.json")
	saved, _ := os.ReadFile(private)
	if err := os.Remove(private); err != nil {
		t.Fatal(err)
	}
	if err := os.Mkdir(private, 0o700); err != nil {
		t.Fatal(err)
	}
	if sig, err := keys.Signer.Sign([]byte("x")); err == nil || sig != nil {
		t.Fatal("tanda tangan dilepas walau indeks gagal disimpan")
	}
	if keys.Counter.Used() != 0 {
		t.Fatalf("pencacah maju walau gagal disimpan: %d", keys.Counter.Used())
	}
	os.Remove(private)
	os.WriteFile(private, saved, 0o600)
	sig, err := keys.Signer.Sign([]byte("x"))
	if err != nil {
		t.Fatal(err)
	}
	if q, _ := lms.LeafIndex(sig); q != 0 {
		t.Fatalf("daun %d", q)
	}
}

func TestCounterStopsAtCapacity(t *testing.T) {
	dir := t.TempDir()
	s := openTemp(t, dir)
	if _, err := s.LoadOrCreateIssuer(rand.Reader, nil); err != nil {
		t.Fatal(err)
	}
	var priv issuerPrivateFile
	if err := s.readJSON(fileIssuerPrivate, &priv); err != nil {
		t.Fatal(err)
	}
	priv.Next = 1023
	if err := s.writeJSON(fileIssuerPrivate, priv, 0o600); err != nil {
		t.Fatal(err)
	}
	s.Close()
	s2 := openTemp(t, dir)
	keys, err := s2.LoadOrCreateIssuer(rand.Reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	sig, err := keys.Signer.Sign([]byte("terakhir"))
	if err != nil {
		t.Fatal(err)
	}
	if q, _ := lms.LeafIndex(sig); q != 1023 {
		t.Fatalf("daun terakhir %d", q)
	}
	if _, err := keys.Signer.Sign([]byte("lebih")); !errors.Is(err, lms.ErrExhausted) {
		t.Fatalf("setelah habis: %v", err)
	}
	if keys.Counter.Used() != 1024 {
		t.Fatalf("pemakaian %d", keys.Counter.Used())
	}
	priv.Next = 1025
	s2.writeJSON(fileIssuerPrivate, priv, 0o600)
	s2.Close()
	s3 := openTemp(t, dir)
	if _, err := s3.LoadOrCreateIssuer(rand.Reader, nil); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("indeks melebihi kapasitas diterima: %v", err)
	}
}

func TestEnrollmentRoundTrip(t *testing.T) {
	s := openTemp(t, t.TempDir())
	if _, err := s.LoadCertificate(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sertifikat sebelum daftar: %v", err)
	}
	if _, err := s.LoadHelper(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("data bantu sebelum daftar: %v", err)
	}
	keys, err := s.LoadOrCreateIssuer(rand.Reader, nil)
	if err != nil {
		t.Fatal(err)
	}
	iss, err := issuer.New(keys.Signer, keys.Counter, func() time.Time { return time.Date(2026, 10, 8, 10, 0, 0, 0, time.UTC) })
	if err != nil {
		t.Fatal(err)
	}
	dk, _ := mlkem.GenerateKey768()
	cert, err := iss.Issue("GEMBOK-SIMPAN", 3, dk.EncapsulationKey().Bytes())
	if err != nil {
		t.Fatal(err)
	}
	helper := chip.HelperData{Thresh: 70, Mask: make([]byte, 101)}
	helper.Mask[3] = 0x5A
	helper.Check[15] = 0xC3
	if err := s.SaveEnrollment(cert, helper); err != nil {
		t.Fatal(err)
	}
	gotCert, err := s.LoadCertificate()
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(gotCert.Body(), cert.Body()) || !bytes.Equal(gotCert.Signature, cert.Signature) {
		t.Fatal("sertifikat berubah setelah disimpan")
	}
	pub, _ := s.LoadIssuerPublicKey()
	if err := issuer.Verify(pub, gotCert); err != nil {
		t.Fatal(err)
	}
	gotHelper, err := s.LoadHelper()
	if err != nil || !sameHelper(gotHelper, helper) {
		t.Fatalf("data bantu berubah: %v", err)
	}
	path := s.CertificatePath()
	raw, _ := os.ReadFile(path)
	os.WriteFile(path, bytes.Replace(raw, []byte(`"versi"`), []byte(`"tambahan": 1, "versi"`), 1), 0o644)
	if _, err := s.LoadCertificate(); !errors.Is(err, ErrCorrupt) {
		t.Fatalf("sertifikat dengan ruas asing: %v", err)
	}
	if err := s.ForgetEnrollment(); err != nil {
		t.Fatal(err)
	}
	if _, err := s.LoadCertificate(); !errors.Is(err, ErrNotFound) {
		t.Fatalf("sertifikat setelah dihapus: %v", err)
	}
	if err := s.ForgetEnrollment(); err != nil {
		t.Fatalf("menghapus dua kali: %v", err)
	}
}

func sameHelper(a, b chip.HelperData) bool {
	return a.Thresh == b.Thresh && a.Check == b.Check && bytes.Equal(a.Mask, b.Mask)
}

func writeHelperFile(t *testing.T, s *Store, f helperFile) {
	t.Helper()
	raw, err := json.Marshal(f)
	if err != nil {
		t.Fatal(err)
	}
	if err := s.writeFile(fileHelper, raw, 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestHelperMaskLength(t *testing.T) {
	s := openTemp(t, t.TempDir())
	check := strings.Repeat("ab", chip.HelperChkSize)
	for _, n := range []int{1, 96, chip.HelperMaskMaxSize} {
		helper := chip.HelperData{Thresh: 64, Mask: bytes.Repeat([]byte{0xA5}, n)}
		if err := s.SaveEnrollment(issuer.Certificate{}, helper); err != nil {
			t.Fatalf("topeng %d byte: %v", n, err)
		}
		got, err := s.LoadHelper()
		if err != nil || !sameHelper(got, helper) {
			t.Fatalf("topeng %d byte setelah dibaca: %v", n, err)
		}
		var f helperFile
		if err := s.readJSON(fileHelper, &f); err != nil || f.Version != helperVersion || f.MaskLen != n {
			t.Fatalf("berkas data bantu %d byte: versi %d panjang %d (%v)", n, f.Version, f.MaskLen, err)
		}
	}
	for _, n := range []int{0, chip.HelperMaskMaxSize + 1} {
		if err := s.SaveEnrollment(issuer.Certificate{}, chip.HelperData{Mask: make([]byte, n)}); err == nil {
			t.Fatalf("topeng %d byte disimpan", n)
		}
	}
	legacy := helperFile{Version: legacyHelperVersion, Thresh: 64, Mask: strings.Repeat("0f", legacyMaskLen), Check: check}
	writeHelperFile(t, s, legacy)
	got, err := s.LoadHelper()
	if err != nil || len(got.Mask) != legacyMaskLen || got.Mask[0] != 0x0F {
		t.Fatalf("data bantu versi 1: %v", err)
	}
	bad := []helperFile{
		{Version: helperVersion, MaskLen: 101, Mask: strings.Repeat("00", 100), Check: check},
		{Version: helperVersion, MaskLen: 0, Mask: "", Check: check},
		{Version: helperVersion, MaskLen: chip.HelperMaskMaxSize + 1, Mask: strings.Repeat("00", chip.HelperMaskMaxSize+1), Check: check},
		{Version: helperVersion, MaskLen: 2, Mask: "zz00", Check: check},
		{Version: helperVersion, MaskLen: 2, Mask: "0000", Check: "00"},
		{Version: legacyHelperVersion, Mask: strings.Repeat("00", 101), Check: check},
		{Version: legacyHelperVersion, MaskLen: legacyMaskLen, Mask: strings.Repeat("00", legacyMaskLen), Check: check},
		{Version: 3, MaskLen: 2, Mask: "0000", Check: check},
	}
	for i, f := range bad {
		writeHelperFile(t, s, f)
		if _, err := s.LoadHelper(); !errors.Is(err, ErrCorrupt) {
			t.Fatalf("data bantu rusak %d diterima: %v", i, err)
		}
	}
}
