package preprocessing

import "math"

// BiquadFilter implements a second-order IIR filter
// Used for EQ, high-pass, low-pass, band-pass, etc.
type BiquadFilter struct {
	// Coefficients
	a0, a1, a2 float64
	b0, b1, b2 float64

	// State variables (Direct Form I)
	x1, x2 float64 // Input history
	y1, y2 float64 // Output history
}

// NewLowPassFilter creates a low-pass filter
func NewLowPassFilter(sampleRate int, cutoffFreq, q float64) *BiquadFilter {
	filter := &BiquadFilter{}
	filter.setLowPass(sampleRate, cutoffFreq, q)
	return filter
}

// NewHighPassFilter creates a high-pass filter
func NewHighPassFilter(sampleRate int, cutoffFreq, q float64) *BiquadFilter {
	filter := &BiquadFilter{}
	filter.setHighPass(sampleRate, cutoffFreq, q)
	return filter
}

// NewPeakingFilter creates a peaking EQ filter
func NewPeakingFilter(sampleRate int, centerFreq, gainDB, q float64) *BiquadFilter {
	filter := &BiquadFilter{}
	filter.setPeakingEQ(sampleRate, centerFreq, gainDB, q)
	return filter
}

// NewBandPassFilter creates a band-pass filter
func NewBandPassFilter(sampleRate int, centerFreq, q float64) *BiquadFilter {
	filter := &BiquadFilter{}
	filter.setBandPass(sampleRate, centerFreq, q)
	return filter
}

func (f *BiquadFilter) setLowPass(sampleRate int, cutoffFreq, q float64) {
	w0 := 2.0 * math.Pi * cutoffFreq / float64(sampleRate)
	alpha := math.Sin(w0) / (2.0 * q)
	cosw0 := math.Cos(w0)

	f.b0 = (1.0 - cosw0) / 2.0
	f.b1 = 1.0 - cosw0
	f.b2 = (1.0 - cosw0) / 2.0
	f.a0 = 1.0 + alpha
	f.a1 = -2.0 * cosw0
	f.a2 = 1.0 - alpha

	// Normalize
	f.b0 /= f.a0
	f.b1 /= f.a0
	f.b2 /= f.a0
	f.a1 /= f.a0
	f.a2 /= f.a0
	f.a0 = 1.0
}

func (f *BiquadFilter) setHighPass(sampleRate int, cutoffFreq, q float64) {
	w0 := 2.0 * math.Pi * cutoffFreq / float64(sampleRate)
	alpha := math.Sin(w0) / (2.0 * q)
	cosw0 := math.Cos(w0)

	f.b0 = (1.0 + cosw0) / 2.0
	f.b1 = -(1.0 + cosw0)
	f.b2 = (1.0 + cosw0) / 2.0
	f.a0 = 1.0 + alpha
	f.a1 = -2.0 * cosw0
	f.a2 = 1.0 - alpha

	// Normalize
	f.b0 /= f.a0
	f.b1 /= f.a0
	f.b2 /= f.a0
	f.a1 /= f.a0
	f.a2 /= f.a0
	f.a0 = 1.0
}

func (f *BiquadFilter) setPeakingEQ(sampleRate int, centerFreq, gainDB, q float64) {
	w0 := 2.0 * math.Pi * centerFreq / float64(sampleRate)
	alpha := math.Sin(w0) / (2.0 * q)
	A := math.Pow(10.0, gainDB/40.0)
	cosw0 := math.Cos(w0)

	f.b0 = 1.0 + alpha*A
	f.b1 = -2.0 * cosw0
	f.b2 = 1.0 - alpha*A
	f.a0 = 1.0 + alpha/A
	f.a1 = -2.0 * cosw0
	f.a2 = 1.0 - alpha/A

	// Normalize
	f.b0 /= f.a0
	f.b1 /= f.a0
	f.b2 /= f.a0
	f.a1 /= f.a0
	f.a2 /= f.a0
	f.a0 = 1.0
}

func (f *BiquadFilter) setBandPass(sampleRate int, centerFreq, q float64) {
	w0 := 2.0 * math.Pi * centerFreq / float64(sampleRate)
	alpha := math.Sin(w0) / (2.0 * q)
	cosw0 := math.Cos(w0)

	f.b0 = alpha
	f.b1 = 0.0
	f.b2 = -alpha
	f.a0 = 1.0 + alpha
	f.a1 = -2.0 * cosw0
	f.a2 = 1.0 - alpha

	// Normalize
	f.b0 /= f.a0
	f.b1 /= f.a0
	f.b2 /= f.a0
	f.a1 /= f.a0
	f.a2 /= f.a0
	f.a0 = 1.0
}

// Process applies the filter to a buffer in-place
func (f *BiquadFilter) Process(samples []float32) {
	for i := range samples {
		// Direct Form I implementation
		x := float64(samples[i])
		
		y := f.b0*x + f.b1*f.x1 + f.b2*f.x2 - f.a1*f.y1 - f.a2*f.y2

		// Update state
		f.x2 = f.x1
		f.x1 = x
		f.y2 = f.y1
		f.y1 = y

		samples[i] = float32(y)
	}
}

// ProcessSample processes a single sample
func (f *BiquadFilter) ProcessSample(sample float32) float32 {
	x := float64(sample)
	y := f.b0*x + f.b1*f.x1 + f.b2*f.x2 - f.a1*f.y1 - f.a2*f.y2

	f.x2 = f.x1
	f.x1 = x
	f.y2 = f.y1
	f.y1 = y

	return float32(y)
}

// Reset clears the filter state
func (f *BiquadFilter) Reset() {
	f.x1 = 0.0
	f.x2 = 0.0
	f.y1 = 0.0
	f.y2 = 0.0
}

