package lms

import (
	"crypto/subtle"
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"runtime"
	"sync"
)

var (
	ErrUnknownParameters = errors.New("lms: tipe parameter tidak dikenal")
	ErrBadPublicKey      = errors.New("lms: kunci publik tidak sah")
	ErrBadTree           = errors.New("lms: pohon tidak cocok dengan kunci")
	ErrExhausted         = errors.New("lms: semua tanda tangan sekali pakai sudah habis")
	ErrSelfCheck         = errors.New("lms: tanda tangan gagal diperiksa ulang sebelum dilepas")
)

type PublicKey struct {
	Tree TreeType
	OTS  OTSType
	ID   [IDSize]byte
	Root [HashSize]byte
}

func ParsePublicKey(b []byte) (PublicKey, error) {
	if len(b) != PublicKeySize {
		return PublicKey{}, ErrBadPublicKey
	}
	k := PublicKey{
		Tree: TreeType(binary.BigEndian.Uint32(b)),
		OTS:  OTSType(binary.BigEndian.Uint32(b[4:])),
	}
	if _, ok := k.Tree.height(); !ok {
		return PublicKey{}, ErrBadPublicKey
	}
	if _, ok := k.OTS.params(); !ok {
		return PublicKey{}, ErrBadPublicKey
	}
	copy(k.ID[:], b[8:])
	copy(k.Root[:], b[8+IDSize:])
	return k, nil
}

func (k PublicKey) Bytes() []byte {
	out := make([]byte, 0, PublicKeySize)
	out = binary.BigEndian.AppendUint32(out, uint32(k.Tree))
	out = binary.BigEndian.AppendUint32(out, uint32(k.OTS))
	out = append(out, k.ID[:]...)
	out = append(out, k.Root[:]...)
	return out
}

func (k PublicKey) Verify(msg, sig []byte) bool {
	h, okTree := k.Tree.height()
	op, okOTS := k.OTS.params()
	if !okTree || !okOTS {
		return false
	}
	if len(sig) < 8 {
		return false
	}
	q := binary.BigEndian.Uint32(sig)
	if OTSType(binary.BigEndian.Uint32(sig[4:])) != k.OTS {
		return false
	}
	otsSize := op.signatureSize()
	if len(sig) < 8+otsSize {
		return false
	}
	if TreeType(binary.BigEndian.Uint32(sig[4+otsSize:])) != k.Tree {
		return false
	}
	if uint64(q) >= uint64(1)<<h {
		return false
	}
	if len(sig) != 8+otsSize+HashSize*h {
		return false
	}
	candidate := otsCandidate(&k.ID, q, op, sig[8:4+otsSize], msg)
	node := uint32(1)<<h + q
	tmp := leafNode(&k.ID, node, &candidate)
	path := sig[8+otsSize:]
	for i := 0; node > 1; i++ {
		sibling := path[HashSize*i : HashSize*(i+1)]
		if node&1 == 1 {
			tmp = interiorNode(&k.ID, node/2, sibling, tmp[:])
		} else {
			tmp = interiorNode(&k.ID, node/2, tmp[:], sibling)
		}
		node /= 2
	}
	return subtle.ConstantTimeCompare(tmp[:], k.Root[:]) == 1
}

func LeafIndex(sig []byte) (uint32, bool) {
	if len(sig) < 4 {
		return 0, false
	}
	return binary.BigEndian.Uint32(sig), true
}

type PrivateKey struct {
	Tree TreeType
	OTS  OTSType
	ID   [IDSize]byte
	Seed [SeedSize]byte
}

func GenerateKey(random io.Reader, tree TreeType, ots OTSType) (PrivateKey, error) {
	k := PrivateKey{Tree: tree, OTS: ots}
	if err := k.validate(); err != nil {
		return PrivateKey{}, err
	}
	if _, err := io.ReadFull(random, k.ID[:]); err != nil {
		return PrivateKey{}, fmt.Errorf("lms: membaca acak untuk pengenal: %w", err)
	}
	if _, err := io.ReadFull(random, k.Seed[:]); err != nil {
		return PrivateKey{}, fmt.Errorf("lms: membaca acak untuk benih: %w", err)
	}
	return k, nil
}

func (k PrivateKey) validate() error {
	h, okTree := k.Tree.height()
	_, okOTS := k.OTS.params()
	if !okTree || !okOTS || h > maxSigningHeight {
		return ErrUnknownParameters
	}
	return nil
}

type Tree struct {
	height int
	nodes  [][HashSize]byte
}

func (k PrivateKey) BuildTree() (*Tree, error) {
	if err := k.validate(); err != nil {
		return nil, err
	}
	h, _ := k.Tree.height()
	op, _ := k.OTS.params()
	leaves := 1 << h
	t := &Tree{height: h, nodes: make([][HashSize]byte, 2*leaves)}
	workers := runtime.GOMAXPROCS(0)
	if workers > leaves {
		workers = leaves
	}
	var wg sync.WaitGroup
	for w := 0; w < workers; w++ {
		wg.Add(1)
		go func(first int) {
			defer wg.Done()
			for q := first; q < leaves; q += workers {
				pub := otsPublicKey(&k.ID, uint32(q), op, &k.Seed)
				t.nodes[leaves+q] = leafNode(&k.ID, uint32(leaves+q), &pub)
			}
		}(w)
	}
	wg.Wait()
	t.fillInterior(&k.ID)
	return t, nil
}

func TreeFromLeaves(tree TreeType, id [IDSize]byte, leaves []byte) (*Tree, error) {
	h, ok := tree.height()
	if !ok || h > maxSigningHeight {
		return nil, ErrUnknownParameters
	}
	count := 1 << h
	if len(leaves) != count*HashSize {
		return nil, ErrBadTree
	}
	t := &Tree{height: h, nodes: make([][HashSize]byte, 2*count)}
	for q := 0; q < count; q++ {
		copy(t.nodes[count+q][:], leaves[q*HashSize:])
	}
	t.fillInterior(&id)
	return t, nil
}

func (t *Tree) fillInterior(id *[IDSize]byte) {
	for r := 1<<t.height - 1; r >= 1; r-- {
		t.nodes[r] = interiorNode(id, uint32(r), t.nodes[2*r][:], t.nodes[2*r+1][:])
	}
}

func (t *Tree) Root() [HashSize]byte {
	return t.nodes[1]
}

func (t *Tree) Leaves() []byte {
	count := 1 << t.height
	out := make([]byte, 0, count*HashSize)
	for q := 0; q < count; q++ {
		out = append(out, t.nodes[count+q][:]...)
	}
	return out
}

func (k PrivateKey) PublicKey(t *Tree) PublicKey {
	return PublicKey{Tree: k.Tree, OTS: k.OTS, ID: k.ID, Root: t.Root()}
}

func (k PrivateKey) signLeaf(t *Tree, q uint32, randomizer *[HashSize]byte, msg []byte) ([]byte, error) {
	if err := k.validate(); err != nil {
		return nil, err
	}
	h, _ := k.Tree.height()
	op, _ := k.OTS.params()
	if t == nil || t.height != h {
		return nil, ErrBadTree
	}
	if uint64(q) >= uint64(1)<<h {
		return nil, ErrExhausted
	}
	sig := make([]byte, 0, 8+op.signatureSize()+HashSize*h)
	sig = binary.BigEndian.AppendUint32(sig, q)
	sig = append(sig, otsSign(&k.ID, q, k.OTS, op, &k.Seed, randomizer, msg)...)
	sig = binary.BigEndian.AppendUint32(sig, uint32(k.Tree))
	node := uint32(1)<<h + q
	for node > 1 {
		sig = append(sig, t.nodes[node^1][:]...)
		node /= 2
	}
	return sig, nil
}

type Counter interface {
	Reserve() (uint32, error)
}

type Signer struct {
	mu      sync.Mutex
	key     PrivateKey
	tree    *Tree
	pub     PublicKey
	counter Counter
	random  io.Reader
}

func NewSigner(key PrivateKey, tree *Tree, counter Counter, random io.Reader) (*Signer, error) {
	if err := key.validate(); err != nil {
		return nil, err
	}
	h, _ := key.Tree.height()
	if tree == nil || tree.height != h {
		return nil, ErrBadTree
	}
	if counter == nil || random == nil {
		return nil, errors.New("lms: pencacah dan sumber acak wajib ada")
	}
	return &Signer{key: key, tree: tree, pub: key.PublicKey(tree), counter: counter, random: random}, nil
}

func (s *Signer) PublicKey() PublicKey {
	return s.pub
}

func (s *Signer) Sign(msg []byte) ([]byte, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	q, err := s.counter.Reserve()
	if err != nil {
		return nil, err
	}
	var randomizer [HashSize]byte
	if _, err := io.ReadFull(s.random, randomizer[:]); err != nil {
		return nil, fmt.Errorf("lms: membaca acak untuk tanda tangan: %w", err)
	}
	sig, err := s.key.signLeaf(s.tree, q, &randomizer, msg)
	if err != nil {
		return nil, err
	}
	if !s.pub.Verify(msg, sig) {
		return nil, ErrSelfCheck
	}
	return sig, nil
}
