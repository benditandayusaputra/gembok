package store

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"sync"

	"gembok/verifier/internal/lms"
)

const (
	issuerTree = lms.LMS_SHA256_M32_H10
	issuerOTS  = lms.LMOTS_SHA256_N32_W8
	keyVersion = 1
)

type issuerPrivateFile struct {
	Version  int    `json:"versi"`
	TreeType string `json:"tipe_lms"`
	OTSType  string `json:"tipe_lmots"`
	ID       string `json:"id"`
	Seed     string `json:"benih"`
	Next     uint32 `json:"indeks_berikut"`
}

type issuerPublicFile struct {
	Version   int    `json:"versi"`
	Algorithm string `json:"algoritma"`
	ID        string `json:"id"`
	PublicKey string `json:"kunci_publik"`
	Capacity  uint32 `json:"kapasitas"`
}

func algorithmName() string {
	return issuerTree.String() + "/" + issuerOTS.String()
}

func (f issuerPrivateFile) key() (lms.PrivateKey, error) {
	if f.Version != keyVersion || f.TreeType != issuerTree.String() || f.OTSType != issuerOTS.String() {
		return lms.PrivateKey{}, fmt.Errorf("%w: parameter kunci penerbit", ErrCorrupt)
	}
	id, err := hex.DecodeString(f.ID)
	if err != nil || len(id) != lms.IDSize {
		return lms.PrivateKey{}, fmt.Errorf("%w: pengenal kunci penerbit", ErrCorrupt)
	}
	seed, err := hex.DecodeString(f.Seed)
	if err != nil || len(seed) != lms.SeedSize {
		return lms.PrivateKey{}, fmt.Errorf("%w: benih kunci penerbit", ErrCorrupt)
	}
	k := lms.PrivateKey{Tree: issuerTree, OTS: issuerOTS}
	copy(k.ID[:], id)
	copy(k.Seed[:], seed)
	clear(seed)
	return k, nil
}

type FileCounter struct {
	mu       sync.Mutex
	store    *Store
	state    issuerPrivateFile
	capacity uint32
}

func (c *FileCounter) Reserve() (uint32, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	q := c.state.Next
	if q >= c.capacity {
		return 0, lms.ErrExhausted
	}
	next := c.state
	next.Next = q + 1
	if err := c.store.writeJSON(fileIssuerPrivate, next, 0o600); err != nil {
		return 0, fmt.Errorf("penyimpanan: mencatat indeks daun %d: %w", q, err)
	}
	c.state = next
	return q, nil
}

func (c *FileCounter) Used() uint32 {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.state.Next
}

func (c *FileCounter) Capacity() uint32 {
	return c.capacity
}

type IssuerKeys struct {
	Signer      *lms.Signer
	Counter     *FileCounter
	Created     bool
	TreeRebuilt bool
}

func (s *Store) LoadIssuerPublicKey() (lms.PublicKey, error) {
	var f issuerPublicFile
	if err := s.readJSON(fileIssuerPublic, &f); err != nil {
		return lms.PublicKey{}, err
	}
	raw, err := hex.DecodeString(f.PublicKey)
	if err != nil {
		return lms.PublicKey{}, fmt.Errorf("%w: kunci publik penerbit", ErrCorrupt)
	}
	pub, err := lms.ParsePublicKey(raw)
	if err != nil || f.Algorithm != algorithmName() || pub.Tree != issuerTree || pub.OTS != issuerOTS || f.ID != hex.EncodeToString(pub.ID[:]) {
		return lms.PublicKey{}, fmt.Errorf("%w: kunci publik penerbit", ErrCorrupt)
	}
	return pub, nil
}

func (s *Store) LoadOrCreateIssuer(random io.Reader, progress func(string)) (*IssuerKeys, error) {
	if progress == nil {
		progress = func(string) {}
	}
	capacity, _ := lms.Capacity(issuerTree)
	var priv issuerPrivateFile
	err := s.readJSON(fileIssuerPrivate, &priv)
	if errors.Is(err, ErrNotFound) {
		return s.createIssuer(random, progress, capacity)
	}
	if err != nil {
		return nil, err
	}
	key, err := priv.key()
	if err != nil {
		return nil, err
	}
	if priv.Next > capacity {
		return nil, fmt.Errorf("%w: indeks daun %d melebihi kapasitas", ErrCorrupt, priv.Next)
	}
	pub, pubErr := s.LoadIssuerPublicKey()
	if pubErr != nil && !errors.Is(pubErr, ErrNotFound) {
		return nil, pubErr
	}
	tree, rebuilt, err := s.loadTree(key, pub, pubErr == nil, progress)
	if err != nil {
		return nil, err
	}
	derived := key.PublicKey(tree)
	if pubErr == nil && !bytes.Equal(derived.Bytes(), pub.Bytes()) {
		return nil, fmt.Errorf("%w: kunci publik penerbit tidak cocok dengan kunci rahasia", ErrCorrupt)
	}
	if pubErr != nil {
		if err := s.writePublic(derived, capacity); err != nil {
			return nil, err
		}
	}
	return s.issuerKeys(key, tree, priv, capacity, random, false, rebuilt)
}

func (s *Store) loadTree(key lms.PrivateKey, pub lms.PublicKey, havePub bool, progress func(string)) (*lms.Tree, bool, error) {
	if leaves, err := s.readFile(fileIssuerTree); err == nil && havePub {
		tree, err := lms.TreeFromLeaves(key.Tree, key.ID, leaves)
		if err == nil && tree.Root() == pub.Root {
			return tree, false, nil
		}
		progress("cache pohon penerbit tidak cocok, pohon dibangun ulang dari benih")
	}
	progress("membangun pohon Merkle penerbit (1024 daun), harap tunggu")
	tree, err := key.BuildTree()
	if err != nil {
		return nil, false, err
	}
	if err := s.writeFile(fileIssuerTree, tree.Leaves(), 0o644); err != nil {
		return nil, false, err
	}
	return tree, true, nil
}

func (s *Store) writePublic(pub lms.PublicKey, capacity uint32) error {
	return s.writeJSON(fileIssuerPublic, issuerPublicFile{
		Version:   keyVersion,
		Algorithm: algorithmName(),
		ID:        hex.EncodeToString(pub.ID[:]),
		PublicKey: hex.EncodeToString(pub.Bytes()),
		Capacity:  capacity,
	}, 0o644)
}

func (s *Store) createIssuer(random io.Reader, progress func(string), capacity uint32) (*IssuerKeys, error) {
	progress("membangkitkan kunci penerbit LMS baru (sekali saja), harap tunggu")
	key, err := lms.GenerateKey(random, issuerTree, issuerOTS)
	if err != nil {
		return nil, err
	}
	tree, err := key.BuildTree()
	if err != nil {
		return nil, err
	}
	if err := s.writeFile(fileIssuerTree, tree.Leaves(), 0o644); err != nil {
		return nil, err
	}
	if err := s.writePublic(key.PublicKey(tree), capacity); err != nil {
		return nil, err
	}
	priv := issuerPrivateFile{
		Version:  keyVersion,
		TreeType: issuerTree.String(),
		OTSType:  issuerOTS.String(),
		ID:       hex.EncodeToString(key.ID[:]),
		Seed:     hex.EncodeToString(key.Seed[:]),
		Next:     0,
	}
	if err := s.writeJSON(fileIssuerPrivate, priv, 0o600); err != nil {
		return nil, err
	}
	return s.issuerKeys(key, tree, priv, capacity, random, true, true)
}

func (s *Store) issuerKeys(key lms.PrivateKey, tree *lms.Tree, priv issuerPrivateFile, capacity uint32, random io.Reader, created, rebuilt bool) (*IssuerKeys, error) {
	counter := &FileCounter{store: s, state: priv, capacity: capacity}
	signer, err := lms.NewSigner(key, tree, counter, random)
	if err != nil {
		return nil, err
	}
	return &IssuerKeys{Signer: signer, Counter: counter, Created: created, TreeRebuilt: rebuilt}, nil
}
