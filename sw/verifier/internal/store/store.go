package store

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const (
	fileIssuerPrivate = "penerbit/kunci-rahasia.json"
	fileIssuerPublic  = "penerbit/kunci-publik.json"
	fileIssuerTree    = "penerbit/pohon.bin"
	fileCertificate   = "chip/sertifikat.json"
	fileHelper        = "chip/data-bantu.json"
	fileLock          = "proses.lock"

	maxFileSize = 1 << 20
)

var (
	ErrNotFound = errors.New("penyimpanan: data belum ada")
	ErrLocked   = errors.New("penyimpanan: direktori data sedang dipakai proses lain")
	ErrBadName  = errors.New("penyimpanan: nama berkas tidak sah")
	ErrCorrupt  = errors.New("penyimpanan: isi berkas rusak")
)

var lockDir = lockWithMarkerFile

type Store struct {
	dir  string
	lock io.Closer
}

func Open(dir string) (*Store, error) {
	abs, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	for _, sub := range []string{"", "penerbit", "chip"} {
		if err := os.MkdirAll(filepath.Join(abs, sub), 0o700); err != nil {
			return nil, fmt.Errorf("penyimpanan: membuat direktori: %w", err)
		}
	}
	lock, err := lockDir(filepath.Join(abs, fileLock))
	if err != nil {
		return nil, err
	}
	return &Store{dir: abs, lock: lock}, nil
}

func (s *Store) Dir() string {
	return s.dir
}

func (s *Store) Close() error {
	if s.lock == nil {
		return nil
	}
	err := s.lock.Close()
	s.lock = nil
	return err
}

func validSegment(seg string) bool {
	if seg == "" || seg == "." || seg == ".." {
		return false
	}
	for i, r := range seg {
		ok := r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || (i > 0 && strings.ContainsRune("._-", r))
		if !ok {
			return false
		}
	}
	return true
}

func (s *Store) path(name string) (string, error) {
	parts := strings.Split(name, "/")
	if len(parts) == 0 || len(parts) > 2 {
		return "", fmt.Errorf("%w: %q", ErrBadName, name)
	}
	for _, p := range parts {
		if !validSegment(p) {
			return "", fmt.Errorf("%w: %q", ErrBadName, name)
		}
	}
	return filepath.Join(append([]string{s.dir}, parts...)...), nil
}

func (s *Store) readFile(name string) ([]byte, error) {
	path, err := s.path(name)
	if err != nil {
		return nil, err
	}
	f, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s: %w", ErrNotFound, name, fs.ErrNotExist)
	}
	if err != nil {
		return nil, err
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, maxFileSize+1))
	if err != nil {
		return nil, err
	}
	if len(data) > maxFileSize {
		return nil, fmt.Errorf("%w: %s terlalu besar", ErrCorrupt, name)
	}
	return data, nil
}

func (s *Store) readJSON(name string, v any) error {
	data, err := s.readFile(name)
	if err != nil {
		return err
	}
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.DisallowUnknownFields()
	if err := dec.Decode(v); err != nil {
		return fmt.Errorf("%w: %s: %v", ErrCorrupt, name, err)
	}
	if dec.More() {
		return fmt.Errorf("%w: %s: data sisa", ErrCorrupt, name)
	}
	return nil
}

func (s *Store) writeJSON(name string, v any, perm os.FileMode) error {
	data, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return s.writeFile(name, append(data, '\n'), perm)
}

func (s *Store) writeFile(name string, data []byte, perm os.FileMode) (err error) {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".sementara-"+filepath.Base(path)+"-*")
	if err != nil {
		return err
	}
	defer func() {
		if err != nil {
			tmp.Close()
			os.Remove(tmp.Name())
		}
	}()
	if err = tmp.Chmod(perm); err != nil && runtime.GOOS != "windows" {
		return err
	}
	if _, err = tmp.Write(data); err != nil {
		return err
	}
	if err = tmp.Sync(); err != nil {
		return err
	}
	if err = tmp.Close(); err != nil {
		return err
	}
	if err = os.Rename(tmp.Name(), path); err != nil {
		return err
	}
	return syncDir(dir)
}

func syncDir(dir string) error {
	if runtime.GOOS == "windows" {
		return nil
	}
	d, err := os.Open(dir)
	if err != nil {
		return err
	}
	defer d.Close()
	return d.Sync()
}

func (s *Store) remove(name string) error {
	path, err := s.path(name)
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return err
	}
	return nil
}

type markerLock struct {
	file *os.File
	path string
}

func (l *markerLock) Close() error {
	err := l.file.Close()
	if rerr := os.Remove(l.path); err == nil {
		err = rerr
	}
	return err
}

func lockWithMarkerFile(path string) (io.Closer, error) {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_EXCL|os.O_WRONLY, 0o600)
	if errors.Is(err, os.ErrExist) {
		return nil, fmt.Errorf("%w (hapus %s bila yakin tidak ada proses lain)", ErrLocked, path)
	}
	if err != nil {
		return nil, err
	}
	return &markerLock{file: f, path: path}, nil
}
