package preprocessing

import "math"

// Compressor implements a dynamic range compressor
type Compressor struct {
	sampleRate  int
	thresholdDB float32
	ratio       float32
	attackCoeff float32
	releaseCoeff float32
	
	// State
	envelope    float32
	makeupGain  float32
}

func NewCompressor(sampleRate int, thresholdDB, ratio, attackMs, releaseMs float32) *Compressor {
	c := &Compressor{
		sampleRate:  sampleRate,
		thresholdDB: thresholdDB,
		ratio:       ratio,
		envelope:    0.0,
		makeupGain:  1.0,
	}

	// Calculate attack and release coefficients
	attackSec := float64(attackMs) / 1000.0
	releaseSec := float64(releaseMs) / 1000.0
	
	c.attackCoeff = float32(1.0 - math.Exp(-1.0/(attackSec*float64(sampleRate))))
	c.releaseCoeff = float32(1.0 - math.Exp(-1.0/(releaseSec*float64(sampleRate))))

	// Calculate makeup gain to compensate for compression
	// Approximate gain reduction at threshold
	c.makeupGain = float32(math.Pow(10.0, float64(-thresholdDB*(1.0-1.0/ratio))/40.0))

	return c
}

func (c *Compressor) Process(samples []float32) []float32 {
	output := make([]float32, len(samples))
	threshold := dbToLinear(c.thresholdDB)

	for i, sample := range samples {
		// Calculate input level
		inputLevel := abs32(sample)

		// Envelope follower with attack/release
		if inputLevel > c.envelope {
			c.envelope += c.attackCoeff * (inputLevel - c.envelope)
		} else {
			c.envelope += c.releaseCoeff * (inputLevel - c.envelope)
		}

		// Calculate gain reduction
		gain := float32(1.0)
		if c.envelope > threshold {
			// Above threshold - apply compression
			envelopeDB := linearToDb(c.envelope)
			thresholdDB := c.thresholdDB
			
			// Calculate gain reduction in dB
			gainReductionDB := (thresholdDB - envelopeDB) + (envelopeDB-thresholdDB)/c.ratio
			gain = dbToLinear(gainReductionDB)
		}

		// Apply gain with makeup gain
		output[i] = sample * gain * c.makeupGain

		// Soft clipping to prevent overshooting
		if output[i] > 1.0 {
			output[i] = 1.0
		} else if output[i] < -1.0 {
			output[i] = -1.0
		}
	}

	return output
}

func (c *Compressor) Reset() {
	c.envelope = 0.0
}

func (c *Compressor) SetThreshold(thresholdDB float32) {
	c.thresholdDB = thresholdDB
	// Recalculate makeup gain
	c.makeupGain = float32(math.Pow(10.0, float64(-thresholdDB*(1.0-1.0/c.ratio))/40.0))
}

func (c *Compressor) SetRatio(ratio float32) {
	c.ratio = ratio
	// Recalculate makeup gain
	c.makeupGain = float32(math.Pow(10.0, float64(-c.thresholdDB*(1.0-1.0/ratio))/40.0))
}

