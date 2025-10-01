package main

import (
	"sync"
	"time"
)

type AudioPacket struct {
	samples   []float32
	timestamp time.Time
	sequence  uint32
}

type JitterBuffer struct {
	packets       []AudioPacket
	mu            sync.Mutex
	playoutDelay  time.Duration
	baseTimestamp time.Time
	nextSequence  uint32
	lastPlayout   time.Time
	adaptiveMin   time.Duration
	adaptiveMax   time.Duration
	latePackets   int
	totalPackets  int
}

func NewJitterBuffer() *JitterBuffer {
	return &JitterBuffer{
		packets:      make([]AudioPacket, 0, 50),
		playoutDelay: 60 * time.Millisecond,
		adaptiveMin:  20 * time.Millisecond,
		adaptiveMax:  200 * time.Millisecond,
		nextSequence: 0,
	}
}

func (jb *JitterBuffer) Add(samples []float32) {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	now := time.Now()

	if jb.baseTimestamp.IsZero() {
		jb.baseTimestamp = now
		jb.lastPlayout = now
	}

	packet := AudioPacket{
		samples:   samples,
		timestamp: now,
		sequence:  jb.nextSequence,
	}

	jb.nextSequence++
	jb.totalPackets++

	inserted := false
	for i := 0; i < len(jb.packets); i++ {
		if packet.sequence < jb.packets[i].sequence {
			jb.packets = append(jb.packets[:i], append([]AudioPacket{packet}, jb.packets[i:]...)...)
			inserted = true
			break
		}
	}

	if !inserted {
		jb.packets = append(jb.packets, packet)
	}

	if len(jb.packets) > 10 {

		jb.playoutDelay = min(jb.playoutDelay+5*time.Millisecond, jb.adaptiveMax)
	} else if len(jb.packets) < 3 && jb.playoutDelay > jb.adaptiveMin {

		jb.playoutDelay = max(jb.playoutDelay-2*time.Millisecond, jb.adaptiveMin)
	}

	if len(jb.packets) > 30 {
		jb.packets = jb.packets[1:]
	}
}

func (jb *JitterBuffer) Get(targetSize int) []float32 {
	jb.mu.Lock()
	defer jb.mu.Unlock()

	now := time.Now()

	if now.Sub(jb.baseTimestamp) < jb.playoutDelay {
		return nil
	}

	if len(jb.packets) == 0 {
		return nil
	}

	packet := jb.packets[0]

	if now.Sub(packet.timestamp) < jb.playoutDelay {

		return nil
	}

	jb.packets = jb.packets[1:]
	jb.lastPlayout = now

	if len(packet.samples) < targetSize {
		padded := make([]float32, targetSize)
		copy(padded, packet.samples)
		return padded
	}

	if len(packet.samples) > targetSize {
		return packet.samples[:targetSize]
	}

	return packet.samples
}

func (jb *JitterBuffer) Conceal(targetSize int, lastSamples []float32) []float32 {
	concealed := make([]float32, targetSize)

	if len(lastSamples) == 0 {
		return concealed
	}

	fadeLen := minInt(len(lastSamples), targetSize)
	for i := range fadeLen {
		fade := float32(fadeLen-i) / float32(fadeLen) * 0.7
		concealed[i] = lastSamples[minInt(i, len(lastSamples)-1)] * fade
	}

	return concealed
}
