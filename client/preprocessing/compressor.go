package preprocessing

import "math"

type Compressor struct {
	sampleRate   int
	thresholdDB  float32
	ratio        float32
	attackCoeff  float32
	releaseCoeff float32

	envelope   float32
	makeupGain float32
}

func NewCompressor(sampleRate int, thresholdDB, ratio, attackMs, releaseMs float32) *Compressor {
	c := &Compressor{
		sampleRate:  sampleRate,
		thresholdDB: thresholdDB,
		ratio:       ratio,
		envelope:    0.0,
		makeupGain:  1.0,
	}

	attackSec := float64(attackMs) / 1000.0
	releaseSec := float64(releaseMs) / 1000.0

	c.attackCoeff = float32(1.0 - math.Exp(-1.0/(attackSec*float64(sampleRate))))
	c.releaseCoeff = float32(1.0 - math.Exp(-1.0/(releaseSec*float64(sampleRate))))

	c.makeupGain = float32(math.Pow(10.0, float64(-thresholdDB*(1.0-1.0/ratio))/40.0))

	return c
}

func (c *Compressor) Process(samples []float32) []float32 {
	output := make([]float32, len(samples))
	threshold := dbToLinear(c.thresholdDB)

	for i, sample := range samples {

		inputLevel := abs32(sample)

		if inputLevel > c.envelope {
			c.envelope += c.attackCoeff * (inputLevel - c.envelope)
		} else {
			c.envelope += c.releaseCoeff * (inputLevel - c.envelope)
		}

		gain := float32(1.0)
		if c.envelope > threshold {

			envelopeDB := linearToDb(c.envelope)
			thresholdDB := c.thresholdDB

			gainReductionDB := (thresholdDB - envelopeDB) + (envelopeDB-thresholdDB)/c.ratio
			gain = dbToLinear(gainReductionDB)
		}

		output[i] = sample * gain * c.makeupGain

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

	c.makeupGain = float32(math.Pow(10.0, float64(-thresholdDB*(1.0-1.0/c.ratio))/40.0))
}

func (c *Compressor) SetRatio(ratio float32) {
	c.ratio = ratio

	c.makeupGain = float32(math.Pow(10.0, float64(-c.thresholdDB*(1.0-1.0/ratio))/40.0))
}
