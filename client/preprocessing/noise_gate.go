package preprocessing

import "math"

// NoiseGate implements a professional noise gate with hysteresis, attack, and release
type NoiseGate struct {
	sampleRate     int
	thresholdDB    float32
	hysteresisDB   float32 // Hysteresis to prevent chattering
	
	// Envelope smoothing
	attackCoeff    float32
	releaseCoeff   float32
	holdSamples    int
	
	// State
	envelope       float32
	gateOpen       bool
	holdCounter    int
	currentGain    float32
}

func NewNoiseGate(sampleRate int, thresholdDB, hysteresisDB float32) *NoiseGate {
	attackTime := 0.001  // 1ms attack (fast response)
	releaseTime := 0.050 // 50ms release (smooth tail)
	holdTime := 0.010    // 10ms hold time

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

	// Calculate time constants for attack and release
	ng.attackCoeff = float32(1.0 - math.Exp(-1.0/(attackTime*float64(sampleRate))))
	ng.releaseCoeff = float32(1.0 - math.Exp(-1.0/(releaseTime*float64(sampleRate))))

	return ng
}

func (ng *NoiseGate) Process(samples []float32) []float32 {
	output := make([]float32, len(samples))
	
	openThreshold := dbToLinear(ng.thresholdDB)
	closeThreshold := dbToLinear(ng.thresholdDB - ng.hysteresisDB)

	for i, sample := range samples {
		// Calculate envelope (RMS with smoothing)
		sampleLevel := abs32(sample)
		
		// Attack/release envelope follower
		if sampleLevel > ng.envelope {
			ng.envelope += ng.attackCoeff * (sampleLevel - ng.envelope)
		} else {
			ng.envelope += ng.releaseCoeff * (sampleLevel - ng.envelope)
		}

		// Gate logic with hysteresis
		if ng.gateOpen {
			// Gate is open - check if we should close
			if ng.envelope < closeThreshold {
				if ng.holdCounter > 0 {
					ng.holdCounter--
				} else {
					ng.gateOpen = false
				}
			} else {
				// Reset hold counter while signal is above close threshold
				ng.holdCounter = ng.holdSamples
			}
		} else {
			// Gate is closed - check if we should open
			if ng.envelope > openThreshold {
				ng.gateOpen = true
				ng.holdCounter = ng.holdSamples
			}
		}

		// Calculate target gain
		targetGain := float32(0.0)
		if ng.gateOpen {
			targetGain = 1.0
		}

		// Smooth gain transitions to avoid clicks
		if targetGain > ng.currentGain {
			ng.currentGain += ng.attackCoeff * (targetGain - ng.currentGain)
		} else {
			ng.currentGain += ng.releaseCoeff * (targetGain - ng.currentGain)
		}

		// Apply gain
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

