package preprocessing

import "math"

type NoiseGate struct {
	sampleRate   int
	thresholdDB  float32
	hysteresisDB float32

	attackCoeff  float32
	releaseCoeff float32
	holdSamples  int

	envelope    float32
	gateOpen    bool
	holdCounter int
	currentGain float32
}

func NewNoiseGate(sampleRate int, thresholdDB, hysteresisDB float32) *NoiseGate {
	attackTime := 0.001
	releaseTime := 0.050
	holdTime := 0.010

	ng := &NoiseGate{
		sampleRate:   sampleRate,
		thresholdDB:  thresholdDB,
		hysteresisDB: hysteresisDB,
		envelope:     0.0,
		gateOpen:     false,
		holdCounter:  0,
		currentGain:  0.0,
		holdSamples:  int(holdTime * float64(sampleRate)),
	}

	ng.attackCoeff = float32(1.0 - math.Exp(-1.0/(attackTime*float64(sampleRate))))
	ng.releaseCoeff = float32(1.0 - math.Exp(-1.0/(releaseTime*float64(sampleRate))))

	return ng
}

func (ng *NoiseGate) Process(samples []float32) []float32 {
	output := make([]float32, len(samples))

	openThreshold := dbToLinear(ng.thresholdDB)
	closeThreshold := dbToLinear(ng.thresholdDB - ng.hysteresisDB)

	for i, sample := range samples {

		sampleLevel := abs32(sample)

		if sampleLevel > ng.envelope {
			ng.envelope += ng.attackCoeff * (sampleLevel - ng.envelope)
		} else {
			ng.envelope += ng.releaseCoeff * (sampleLevel - ng.envelope)
		}

		if ng.gateOpen {

			if ng.envelope < closeThreshold {
				if ng.holdCounter > 0 {
					ng.holdCounter--
				} else {
					ng.gateOpen = false
				}
			} else {

				ng.holdCounter = ng.holdSamples
			}
		} else {

			if ng.envelope > openThreshold {
				ng.gateOpen = true
				ng.holdCounter = ng.holdSamples
			}
		}

		targetGain := float32(0.0)
		if ng.gateOpen {
			targetGain = 1.0
		}

		if targetGain > ng.currentGain {
			ng.currentGain += ng.attackCoeff * (targetGain - ng.currentGain)
		} else {
			ng.currentGain += ng.releaseCoeff * (targetGain - ng.currentGain)
		}

		output[i] = sample * ng.currentGain
	}

	return output
}

func (ng *NoiseGate) Reset() {
	ng.envelope = 0.0
	ng.gateOpen = false
	ng.holdCounter = 0
	ng.currentGain = 0.0
}

func (ng *NoiseGate) SetThreshold(thresholdDB float32) {
	ng.thresholdDB = thresholdDB
}

func (ng *NoiseGate) SetHysteresis(hysteresisDB float32) {
	ng.hysteresisDB = hysteresisDB
}
