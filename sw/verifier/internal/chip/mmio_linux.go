package chip

import (
	"fmt"
	"os"
	"syscall"
	"unsafe"
)

const devMemPath = "/dev/mem"

type mmioBus struct {
	file *os.File
	mem  []byte
}

func init() {
	mmioOpener = func(base uint64) (Bus, error) {
		return openMMIO(devMemPath, base)
	}
}

func openMMIO(path string, base uint64) (*mmioBus, error) {
	page := uint64(os.Getpagesize())
	if base%page != 0 {
		return nil, fmt.Errorf("chip: alamat dasar 0x%X harus kelipatan %d", base, page)
	}
	if base > 1<<32-Span {
		return nil, fmt.Errorf("chip: alamat dasar 0x%X di luar ruang alamat 32 bit", base)
	}
	file, err := os.OpenFile(path, os.O_RDWR|os.O_SYNC, 0)
	if err != nil {
		return nil, fmt.Errorf("chip: membuka %s: %w", path, err)
	}
	mem, err := syscall.Mmap(int(file.Fd()), int64(base), Span, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		file.Close()
		return nil, fmt.Errorf("chip: memetakan %s pada 0x%X: %w", path, base, err)
	}
	return &mmioBus{file: file, mem: mem}, nil
}

func (b *mmioBus) word(offset uint32) *uint32 {
	return (*uint32)(unsafe.Pointer(&b.mem[offset&(Span-4)]))
}

func (b *mmioBus) Read32(offset uint32) uint32 {
	return *b.word(offset)
}

func (b *mmioBus) Write32(offset, value uint32) {
	*b.word(offset) = value
}

func (b *mmioBus) Close() error {
	err := syscall.Munmap(b.mem)
	if cerr := b.file.Close(); err == nil {
		err = cerr
	}
	return err
}
