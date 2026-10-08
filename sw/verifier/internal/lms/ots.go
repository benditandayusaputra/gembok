package lms

import (
	"crypto/sha256"
	"encoding/binary"
	"hash"
)

const (
	chainHeadSize = IDSize + 4 + 2 + 1
	chainBufSize  = chainHeadSize + HashSize
)

type chain struct {
	buf [chainBufSize]byte
}

func newChain(id *[IDSize]byte, q uint32) *chain {
	c := &chain{}
	copy(c.buf[:IDSize], id[:])
	binary.BigEndian.PutUint32(c.buf[IDSize:], q)
	return c
}

func (c *chain) secret(i int, seed *[SeedSize]byte) [HashSize]byte {
	binary.BigEndian.PutUint16(c.buf[IDSize+4:], uint16(i))
	c.buf[IDSize+6] = domainSeed
	copy(c.buf[chainHeadSize:], seed[:])
	return sha256.Sum256(c.buf[:])
}

func (c *chain) walk(i int, value [HashSize]byte, from, to int) [HashSize]byte {
	binary.BigEndian.PutUint16(c.buf[IDSize+4:], uint16(i))
	for j := from; j < to; j++ {
		c.buf[IDSize+6] = byte(j)
		copy(c.buf[chainHeadSize:], value[:])
		value = sha256.Sum256(c.buf[:])
	}
	return value
}

func (c *chain) clear() {
	clear(c.buf[:])
}

func coef(s []byte, i, w int) int {
	shift := 8 - (w*(i%(8/w)) + w)
	return int(s[i*w/8]>>shift) & (1<<w - 1)
}

func checksum(q []byte, op otsParams) uint16 {
	sum := 0
	for i := 0; i < HashSize*8/op.w; i++ {
		sum += (1<<op.w - 1) - coef(q, i, op.w)
	}
	return uint16(sum << op.ls)
}

func prefixedHasher(id *[IDSize]byte, q uint32, domain uint16) hash.Hash {
	var head [IDSize + 6]byte
	copy(head[:IDSize], id[:])
	binary.BigEndian.PutUint32(head[IDSize:], q)
	binary.BigEndian.PutUint16(head[IDSize+4:], domain)
	h := sha256.New()
	h.Write(head[:])
	return h
}

func messageDigits(id *[IDSize]byte, q uint32, op otsParams, randomizer, msg []byte) []int {
	h := prefixedHasher(id, q, domainMessage)
	h.Write(randomizer)
	h.Write(msg)
	var qc [HashSize + 2]byte
	h.Sum(qc[:0])
	binary.BigEndian.PutUint16(qc[HashSize:], checksum(qc[:HashSize], op))
	digits := make([]int, op.p)
	for i := range digits {
		digits[i] = coef(qc[:], i, op.w)
	}
	return digits
}

func otsPublicKey(id *[IDSize]byte, q uint32, op otsParams, seed *[SeedSize]byte) [HashSize]byte {
	c := newChain(id, q)
	defer c.clear()
	h := prefixedHasher(id, q, domainPublic)
	top := 1<<op.w - 1
	for i := 0; i < op.p; i++ {
		y := c.walk(i, c.secret(i, seed), 0, top)
		h.Write(y[:])
	}
	var k [HashSize]byte
	h.Sum(k[:0])
	return k
}

func otsSign(id *[IDSize]byte, q uint32, ots OTSType, op otsParams, seed *[SeedSize]byte, randomizer *[HashSize]byte, msg []byte) []byte {
	digits := messageDigits(id, q, op, randomizer[:], msg)
	c := newChain(id, q)
	defer c.clear()
	out := make([]byte, 0, op.signatureSize())
	out = binary.BigEndian.AppendUint32(out, uint32(ots))
	out = append(out, randomizer[:]...)
	for i, a := range digits {
		y := c.walk(i, c.secret(i, seed), 0, a)
		out = append(out, y[:]...)
	}
	return out
}

func otsCandidate(id *[IDSize]byte, q uint32, op otsParams, body, msg []byte) [HashSize]byte {
	digits := messageDigits(id, q, op, body[:HashSize], msg)
	c := newChain(id, q)
	h := prefixedHasher(id, q, domainPublic)
	top := 1<<op.w - 1
	for i, a := range digits {
		var y [HashSize]byte
		copy(y[:], body[HashSize*(i+1):])
		z := c.walk(i, y, a, top)
		h.Write(z[:])
	}
	var k [HashSize]byte
	h.Sum(k[:0])
	return k
}

func leafNode(id *[IDSize]byte, r uint32, k *[HashSize]byte) [HashSize]byte {
	var buf [IDSize + 6 + HashSize]byte
	copy(buf[:IDSize], id[:])
	binary.BigEndian.PutUint32(buf[IDSize:], r)
	binary.BigEndian.PutUint16(buf[IDSize+4:], domainLeaf)
	copy(buf[IDSize+6:], k[:])
	return sha256.Sum256(buf[:])
}

func interiorNode(id *[IDSize]byte, r uint32, left, right []byte) [HashSize]byte {
	var buf [IDSize + 6 + 2*HashSize]byte
	copy(buf[:IDSize], id[:])
	binary.BigEndian.PutUint32(buf[IDSize:], r)
	binary.BigEndian.PutUint16(buf[IDSize+4:], domainInterior)
	copy(buf[IDSize+6:], left)
	copy(buf[IDSize+6+HashSize:], right)
	return sha256.Sum256(buf[:])
}
