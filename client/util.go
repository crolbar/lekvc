package main

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"

	p "github.com/crolbar/lekvc/lekvcs/protocol"
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

func noiseGate(samples []float32, thresholdDB float64) []float32 {
	threshold := float32(1.0 * pow10(thresholdDB/20.0)) // convert dB to linear
	output := make([]float32, len(samples))
	for i, s := range samples {
		if abs(s) < threshold {
			output[i] = 0
		} else {
			output[i] = s
		}
	}
	return output
}

func pow10(x float64) float64 {
	return math.Pow(10, x)
}

func abs(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

func applyLowPass(samples []float32) {
	for i := 1; i < len(samples)-1; i++ {
		samples[i] = (samples[i-1] + samples[i] + samples[i+1]) / 3.0
	}
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

func ChatPrintClient(text string) {
	ChatPrint("\x1b[38;5;188mCLIENT\x1b[m", text)
}

func ChatPrintServer(text string) {
	ChatPrint("\x1b[38;5;141mSERVER\x1b[m", text)
}

func Prompt() {
	now := time.Now()
	fmtTime := now.Format("15:04:05")
	fmt.Printf("\x1b[38;5;238m[%s]\x1b[m => ", fmtTime)
}

func ChatPrint(sender string, text string) {
	now := time.Now()
	fmtTime := now.Format("15:04:05")
	fmt.Printf("\r\x1b[38;5;238m[%s]\x1b[m %s => %s\n",
		fmtTime,
		sender,
		text)

	Prompt()
}

func generateClientColorFromID(id uint8) int {
	return (int(id)*98+21)%255
}

func ChatPrintMsg(sender string, msg *p.Msg) {
	now := time.Now()
	fmtTime := now.Format("15:04:05")
	fmt.Printf("\r\x1b[38;5;238m[%s]\x1b[38;5;%dm %s\x1b[m => %s\n",
		fmtTime,
		generateClientColorFromID(msg.ID),
		sender,
		string(msg.Payload))

	Prompt()
}
