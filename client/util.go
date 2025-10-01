package main

import (
	"encoding/binary"
	"math"
	"time"
)

type RingBuffer struct {
	buf   []float32
	size  int
	read  int
	write int
}

func NewRingBuffer(size int) *RingBuffer {
	return &RingBuffer{
		buf:  make([]float32, size),
		size: size,
	}
}

func (r *RingBuffer) Write(samples []float32) int {
	n := 0
	for _, s := range samples {
		next := (r.write + 1) % r.size
		if next == r.read {
			break // buffer full
		}
		r.buf[r.write] = s
		r.write = next
		n++
	}
	return n
}

func (r *RingBuffer) Read(out []float32) int {
	n := 0
	for i := range out {
		if r.read == r.write {
			break // buffer empty
		}
		out[i] = r.buf[r.read]
		r.read = (r.read + 1) % r.size
		n++
	}
	return n
}

func bytesToFloat32(b []byte) []float32 {
	size := len(b) / 4
	out := make([]float32, size)

	for i := range out {
		out[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}

	return out
}

func float32ToBytes(f []float32) []byte {
	out := make([]byte, len(f)*4)
	for i, v := range f {
		binary.LittleEndian.PutUint32(out[i*4:], math.Float32bits(v))
	}
	return out
}

func min(a, b time.Duration) time.Duration {
	if a < b {
		return a
	}
	return b
}

func max(a, b time.Duration) time.Duration {
	if a > b {
		return a
	}
	return b
}

func minInt(a, b int) int {
	if a < b {
		return a
	}
	return b
}

// float32ToInt16 converts float32 samples (-1.0 to 1.0) to int16 samples
func float32ToInt16(samples []float32) []int16 {
	out := make([]int16, len(samples))
	for i, s := range samples {
		// Clamp to valid range
		if s > 1.0 {
			s = 1.0
		} else if s < -1.0 {
			s = -1.0
		}
		// Convert to int16 range (-32768 to 32767)
		out[i] = int16(s * 32767.0)
	}
	return out
}

// int16ToFloat32 converts int16 samples to float32 samples (-1.0 to 1.0)
func int16ToFloat32(samples []int16) []float32 {
	out := make([]float32, len(samples))
	for i, s := range samples {
		out[i] = float32(s) / 32768.0
	}
	return out
}
