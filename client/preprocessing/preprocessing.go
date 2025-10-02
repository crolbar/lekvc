package preprocessing

import "math"

type AudioProcessor struct {
	sampleRate int

	gate *NoiseGate

	highPass *BiquadFilter
	lowPass  *BiquadFilter

	compressor *Compressor
	deEsser    *DeEsser

	eqFilters []*BiquadFilter
}

// NewAudioProcessor creates a new audio processor optimized for voice
func NewAudioProcessor(sampleRate int) *AudioProcessor {
	ap := &AudioProcessor{
		sampleRate: sampleRate,
	}

	// Advanced noise gate with smooth attack/release
	ap.gate = NewNoiseGate(sampleRate, -40.0, 10.0)

	// High-pass filter at 80Hz to remove rumble and pops
	ap.highPass = NewHighPassFilter(sampleRate, 80.0, 0.707)

	// Low-pass filter at 8kHz (voice range upper limit)
	ap.lowPass = NewLowPassFilter(sampleRate, 8000.0, 0.707)

	// Compressor for evening out dynamics (-20dB threshold, 3:1 ratio)
	ap.compressor = NewCompressor(sampleRate, -20.0, 3.0, 10.0, 50.0)

	// De-esser to tame harsh sibilants around 6-8kHz
	ap.deEsser = NewDeEsser(sampleRate, 6500.0, -12.0, 2.0)

	// Voice-optimized EQ
	ap.eqFilters = []*BiquadFilter{
		// Boost presence around 3kHz for clarity
		NewPeakingFilter(sampleRate, 3000.0, 2.0, 1.2),
		// Slight cut around 250Hz to reduce muddiness
		NewPeakingFilter(sampleRate, 250.0, -1.5, 1.5),
		// Boost high-mids for intelligibility
		NewPeakingFilter(sampleRate, 5000.0, 1.5, 1.0),
	}

	return ap
}

// Process applies all preprocessing to the audio samples
func (ap *AudioProcessor) Process(samples []float32) []float32 {
	if len(samples) == 0 {
		return samples
	}

	// Make a copy to avoid modifying the original
	processed := make([]float32, len(samples))
	copy(processed, samples)

	// 1. High-pass filter (remove low-frequency rumble)
	ap.highPass.Process(processed)

	// 3. Low-pass filter (remove high-frequency noise above voice range)
	ap.lowPass.Process(processed)

	// 4. Voice EQ
	for _, filter := range ap.eqFilters {
		filter.Process(processed)
	}

	// 5. De-esser (reduce harsh sibilants)
	processed = ap.deEsser.Process(processed)

	// 6. Compressor (even out dynamics)
	processed = ap.compressor.Process(processed)

	// 7. Noise gate (final cleanup with smooth envelope)
	processed = ap.gate.Process(processed)

	return processed
}

// Reset resets all stateful processors (useful when connection drops)
func (ap *AudioProcessor) Reset() {
	ap.gate.Reset()
	ap.highPass.Reset()
	ap.lowPass.Reset()
	ap.compressor.Reset()
	ap.deEsser.Reset()
	for _, filter := range ap.eqFilters {
		filter.Reset()
	}
}

// Helper functions

func linearToDb(linear float32) float32 {
	if linear <= 0.0 {
		return -100.0
	}
	return 20.0 * float32(math.Log10(float64(linear)))
}

func dbToLinear(db float32) float32 {
	return float32(math.Pow(10.0, float64(db)/20.0))
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}
