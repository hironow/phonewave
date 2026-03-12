package domain

import (
	"encoding/binary"
	"errors"
	"math"
)

// BloomFilter is a probabilistic set membership structure.
// It guarantees zero false negatives: if MayContain returns false,
// the element was definitely never added.
type BloomFilter struct {
	bits    []uint64
	numHash uint
	m       uint // total bits
}

// NewBloomFilter creates a Bloom filter sized for expectedN elements
// at the given false-positive rate (e.g. 0.01 = 1%).
func NewBloomFilter(expectedN int, fpRate float64) *BloomFilter {
	if expectedN < 1 {
		expectedN = 1
	}
	if fpRate <= 0 || fpRate >= 1 {
		fpRate = 0.01
	}
	m := optimalM(expectedN, fpRate)
	k := optimalK(m, expectedN)
	words := (m + 63) / 64
	return &BloomFilter{
		bits:    make([]uint64, words),
		numHash: k,
		m:       m,
	}
}

// Add inserts an element into the filter.
func (bf *BloomFilter) Add(key string) {
	h1, h2 := hash([]byte(key))
	for i := uint(0); i < bf.numHash; i++ {
		pos := (h1 + i*h2) % bf.m
		bf.bits[pos/64] |= 1 << (pos % 64)
	}
}

// MayContain returns true if the element might be in the set,
// false if it is definitely not.
func (bf *BloomFilter) MayContain(key string) bool {
	h1, h2 := hash([]byte(key))
	for i := uint(0); i < bf.numHash; i++ {
		pos := (h1 + i*h2) % bf.m
		if bf.bits[pos/64]&(1<<(pos%64)) == 0 {
			return false
		}
	}
	return true
}

// hash produces two independent hash values using FNV-inspired mixing.
func hash(data []byte) (uint, uint) {
	// FNV-1a 64-bit
	const (
		offset64 = 14695981039346656037
		prime64  = 1099511628211
	)
	h1 := uint64(offset64)
	for _, b := range data {
		h1 ^= uint64(b)
		h1 *= prime64
	}
	// second hash: rotate + mix
	h2 := h1
	h2 = (h2 >> 33) ^ h2
	h2 *= 0xff51afd7ed558ccd
	h2 = (h2 >> 33) ^ h2
	h2 *= 0xc4ceb9fe1a85ec53
	h2 = (h2 >> 33) ^ h2
	return uint(h1), uint(h2)
}

func optimalM(n int, fp float64) uint {
	m := -float64(n) * math.Log(fp) / (math.Log(2) * math.Log(2))
	return uint(math.Ceil(m))
}

func optimalK(m uint, n int) uint {
	k := float64(m) / float64(n) * math.Log(2)
	return uint(math.Ceil(k))
}

// MarshalBinary encodes the filter to bytes for persistence.
func (bf *BloomFilter) MarshalBinary() []byte {
	// header: m (8 bytes) + numHash (8 bytes) + bit data
	buf := make([]byte, 16+len(bf.bits)*8)
	binary.LittleEndian.PutUint64(buf[0:8], uint64(bf.m))
	binary.LittleEndian.PutUint64(buf[8:16], uint64(bf.numHash))
	for i, w := range bf.bits {
		binary.LittleEndian.PutUint64(buf[16+i*8:], w)
	}
	return buf
}

// UnmarshalBloomFilter restores a filter from bytes produced by MarshalBinary.
func UnmarshalBloomFilter(data []byte) (*BloomFilter, error) {
	if len(data) < 16 {
		return nil, errBloomCorrupt
	}
	m := uint(binary.LittleEndian.Uint64(data[0:8]))
	k := uint(binary.LittleEndian.Uint64(data[8:16]))
	words := (m + 63) / 64
	if uint(len(data)) < 16+words*8 {
		return nil, errBloomCorrupt
	}
	bits := make([]uint64, words)
	for i := range bits {
		bits[i] = binary.LittleEndian.Uint64(data[16+i*8:])
	}
	return &BloomFilter{bits: bits, numHash: k, m: m}, nil
}

var errBloomCorrupt = errors.New("bloom filter: corrupt data")
