package preprocessing

import (
	"math"
	"math/cmplx"
)

// AudioProcessor handles all audio preprocessing for voice optimization
type AudioProcessor struct {
	sampleRate int

	// Noise gate
	gate *NoiseGate

	// Filters
	highPass *BiquadFilter
	lowPass  *BiquadFilter

	// Noise suppression
	// noiseSuppressor *NoiseSuppressor

	// Dynamics processing
	compressor *Compressor
	deEsser    *DeEsser

	// Voice EQ
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

	// Spectral noise suppression
	// ap.noiseSuppressor = NewNoiseSuppressor(sampleRate)

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

	// 2. Noise suppression (spectral subtraction)
	// processed = ap.noiseSuppressor.Process(processed)

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
	// ap.noiseSuppressor.Reset()
	ap.compressor.Reset()
	ap.deEsser.Reset()
	for _, filter := range ap.eqFilters {
		filter.Reset()
	}
}

// UpdateNoiseProfile updates the noise suppressor's noise floor estimate
// Call this during silence periods
func (ap *AudioProcessor) UpdateNoiseProfile(samples []float32) {
	// ap.noiseSuppressor.UpdateNoiseProfile(samples)
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

func clamp(value, min, max float32) float32 {
	if value < min {
		return min
	}
	if value > max {
		return max
	}
	return value
}

func abs32(x float32) float32 {
	if x < 0 {
		return -x
	}
	return x
}

// FFT helper for noise suppression
func fft(input []complex128) []complex128 {
	n := len(input)
	if n <= 1 {
		return input
	}

	// Cooley-Tukey FFT algorithm
	if n&(n-1) != 0 {
		// Not a power of 2, use DFT
		return dft(input)
	}

	// Split into even and odd
	even := make([]complex128, n/2)
	odd := make([]complex128, n/2)
	for i := 0; i < n/2; i++ {
		even[i] = input[2*i]
		odd[i] = input[2*i+1]
	}

	// Recursive FFT
	fftEven := fft(even)
	fftOdd := fft(odd)

	output := make([]complex128, n)
	for k := 0; k < n/2; k++ {
		t := cmplx.Exp(complex(0, -2*math.Pi*float64(k)/float64(n))) * fftOdd[k]
		output[k] = fftEven[k] + t
		output[k+n/2] = fftEven[k] - t
	}

	return output
}

func ifft(input []complex128) []complex128 {
	n := len(input)
	
	// Conjugate
	conj := make([]complex128, n)
	for i := range input {
		conj[i] = cmplx.Conj(input[i])
	}

	// Forward FFT
	output := fft(conj)

	// Conjugate and scale
	for i := range output {
		output[i] = cmplx.Conj(output[i]) / complex(float64(n), 0)
	}

	return output
}

// DFT for non-power-of-2 sizes
func dft(input []complex128) []complex128 {
	n := len(input)
	output := make([]complex128, n)

	for k := 0; k < n; k++ {
		sum := complex(0, 0)
		for t := 0; t < n; t++ {
			angle := -2 * math.Pi * float64(t) * float64(k) / float64(n)
			sum += input[t] * cmplx.Exp(complex(0, angle))
		}
		output[k] = sum
	}

	return output
}

// nextPowerOf2 finds the next power of 2 greater than or equal to n
func nextPowerOf2(n int) int {
	if n <= 0 {
		return 1
	}
	n--
	n |= n >> 1
	n |= n >> 2
	n |= n >> 4
	n |= n >> 8
	n |= n >> 16
	n++
	return n
}

