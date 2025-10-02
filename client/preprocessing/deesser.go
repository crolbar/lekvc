package preprocessing

import "math"

type DeEsser struct {
	sampleRate  int
	frequency   float32
	thresholdDB float32
	ratio       float32

	sidechainFilter *BiquadFilter

	compressor *Compressor

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

	de.sidechainFilter = NewBandPassFilter(sampleRate, float64(frequency), 2.0)

	attackMs := float32(1.0)
	releaseMs := float32(50.0)

	attackSec := float64(attackMs) / 1000.0
	releaseSec := float64(releaseMs) / 1000.0

	de.attackCoeff = float32(1.0 - math.Exp(-1.0/(attackSec*float64(sampleRate))))
	de.releaseCoeff = float32(1.0 - math.Exp(-1.0/(releaseSec*float64(sampleRate))))

	return de
}

func (de *DeEsser) Process(samples []float32) []float32 {
	output := make([]float32, len(samples))

	sidechain := make([]float32, len(samples))
	copy(sidechain, samples)
	de.sidechainFilter.Process(sidechain)

	threshold := dbToLinear(de.thresholdDB)

	for i, sample := range samples {

		sidechainLevel := abs32(sidechain[i])

		if sidechainLevel > de.envelope {
			de.envelope += de.attackCoeff * (sidechainLevel - de.envelope)
		} else {
			de.envelope += de.releaseCoeff * (sidechainLevel - de.envelope)
		}

		gain := float32(1.0)
		if de.envelope > threshold {

			envelopeDB := linearToDb(de.envelope)
			thresholdDB := de.thresholdDB

			gainReductionDB := (thresholdDB - envelopeDB) + (envelopeDB-thresholdDB)/de.ratio
			gain = dbToLinear(gainReductionDB)

			if gain < 0.3 {
				gain = 0.3
			}
		}

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

	de.sidechainFilter = NewBandPassFilter(de.sampleRate, float64(frequency), 2.0)
}
