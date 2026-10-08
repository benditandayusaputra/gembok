package store

import (
	"errors"
	"fmt"
	"io"
	"syscall"
)

const errSharingViolation = syscall.Errno(32)

type handleLock struct {
	handle syscall.Handle
}

func (l handleLock) Close() error {
	return syscall.CloseHandle(l.handle)
}

func init() {
	lockDir = lockExclusiveHandle
}

func lockExclusiveHandle(path string) (io.Closer, error) {
	name, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return nil, err
	}
	h, err := syscall.CreateFile(name, syscall.GENERIC_READ|syscall.GENERIC_WRITE, 0, nil, syscall.OPEN_ALWAYS, syscall.FILE_ATTRIBUTE_NORMAL, 0)
	if err != nil {
		if errors.Is(err, errSharingViolation) {
			return nil, fmt.Errorf("%w: %s", ErrLocked, path)
		}
		return nil, err
	}
	return handleLock{handle: h}, nil
}
