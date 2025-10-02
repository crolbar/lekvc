package preprocessing

import "math"

// DeEsser reduces harsh sibilant sounds (S, T, SH sounds) in the 4-10kHz range
type DeEsser struct {
	sampleRate  int
	frequency   float32
	thresholdDB float32
	ratio       float32
	
	// Sidechain filter to detect sibilants
	sidechainFilter *BiquadFilter
	
	// Dynamics
	compressor *Compressor
	
	// Envelope detection
	envelope     float32
	attackCoeff  float32
	releaseCoeff float32
}

func NewDeEsser(sampleRate int, frequency, thresholdDB, ratio float32) *DeEsser {
	de := &DeEsser{
		sampleRate:  sampleRate,
		frequency:   frequency,
		thresholdDB: thresholdDB,
		ratio:       ratio,
		envelope:    0.0,
	}

	// Create a band-pass filter centered on sibilant frequencies
	// Q factor of 2.0 gives a reasonably narrow band
	de.sidechainFilter = NewBandPassFilter(sampleRate, float64(frequency), 2.0)

	// Fast attack, medium release for sibilant control
	attackMs := float32(1.0)   // 1ms attack
	releaseMs := float32(50.0) // 50ms release

	attackSec := float64(attackMs) / 1000.0
	releaseSec := float64(releaseMs) / 1000.0
	
	de.attackCoeff = float32(1.0 - math.Exp(-1.0/(attackSec*float64(sampleRate))))
	de.releaseCoeff = float32(1.0 - math.Exp(-1.0/(releaseSec*float64(sampleRate))))

	return de
}

func (de *DeEsser) Process(samples []float32) []float32 {
	output := make([]float32, len(samples))
	
	// Create sidechain signal (detect sibilants)
	sidechain := make([]float32, len(samples))
	copy(sidechain, samples)
	de.sidechainFilter.Process(sidechain)

	threshold := dbToLinear(de.thresholdDB)

	for i, sample := range samples {
		// Detect sibilant energy
		sidechainLevel := abs32(sidechain[i])

		// Envelope follower on sidechain
		if sidechainLevel > de.envelope {
			de.envelope += de.attackCoeff * (sidechainLevel - de.envelope)
		} else {
			de.envelope += de.releaseCoeff * (sidechainLevel - de.envelope)
		}

		// Calculate gain reduction
		gain := float32(1.0)
		if de.envelope > threshold {
			// Sibilant detected - apply reduction
			envelopeDB := linearToDb(de.envelope)
			thresholdDB := de.thresholdDB
			
			// Calculate gain reduction in dB (similar to compressor)
			gainReductionDB := (thresholdDB - envelopeDB) + (envelopeDB-thresholdDB)/de.ratio
			gain = dbToLinear(gainReductionDB)

			// Ensure gain doesn't go below 0.3 (-10dB) to avoid over-processing
			if gain < 0.3 {
				gain = 0.3
			}
		}

		// Apply gain only to high frequencies (split-band de-essing)
		// For simplicity, we apply to the whole signal, but in a production
		// environment, you'd split the signal into bands
		output[i] = sample * gain
	}

	return output
}

func (de *DeEsser) Reset() {
	de.envelope = 0.0
	de.sidechainFilter.Reset()
}

func (de *DeEsser) SetThreshold(thresholdDB float32) {
	de.thresholdDB = thresholdDB
}

func (de *DeEsser) SetFrequency(frequency float32) {
	de.frequency = frequency
	// Recreate filter with new frequency
	de.sidechainFilter = NewBandPassFilter(de.sampleRate, float64(frequency), 2.0)
}

