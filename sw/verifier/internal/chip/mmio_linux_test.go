package chip

import (
	"context"
	"encoding/binary"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestMMIOBusOverFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "jendela")
	if err := os.WriteFile(path, make([]byte, Span), 0o600); err != nil {
		t.Fatal(err)
	}
	bus, err := openMMIO(path, 0)
	if err != nil {
		t.Fatal(err)
	}
	bus.Write32(RegParam, 0x11223344)
	bus.Write32(MemBase+4*0x18E0, 0xAB)
	bus.Write32(Span+RegCtxLen, 7)
	if got := bus.Read32(RegParam); got != 0x11223344 {
		t.Fatalf("PARAM %#x", got)
	}
	if got := bus.Read32(RegCtxLen + 2); got != 7 {
		t.Fatalf("offset tidak sejajar tidak dipotong ke kata: %#x", got)
	}
	d := NewDriver(bus, DriverOptions{Clock: NewManualClock(time.Microsecond)})
	if err := d.Probe(context.Background()); !errors.Is(err, ErrNotGembok) {
		t.Fatalf("berkas biasa dikira GEMBOK: %v", err)
	}
	if err := bus.Close(); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if binary.LittleEndian.Uint32(raw[RegParam:]) != 0x11223344 || raw[MemBase+4*0x18E0] != 0xAB || raw[RegCtxLen] != 7 {
		t.Fatal("isi jendela tidak sesuai tulisan")
	}
	if _, err := openMMIO(path, 100); err == nil {
		t.Fatal("alamat dasar tidak sejajar halaman diterima")
	}
	if _, err := openMMIO(path, 1<<32); err == nil {
		t.Fatal("alamat di luar 32 bit diterima")
	}
	if _, err := openMMIO(filepath.Join(t.TempDir(), "tidak-ada"), 0); err == nil {
		t.Fatal("berkas yang tidak ada diterima")
	}
}
