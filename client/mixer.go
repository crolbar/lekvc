package main

import "time"

func mixer() {
	// in ms
	var frameTime float32 = (float32(targetFramesize) / float32(sampleRate)) * 1000
	ticker := time.NewTicker(time.Millisecond * time.Duration(frameTime))
	defer ticker.Stop()

	for range ticker.C {
		if len(clients) == 0 {
			continue
		}

		summed := make([]float32, targetFramesize)
		active := 0

		for _, c := range clients {
			if c.jitterBuffer == nil {
				continue
			}

			// Try to get samples from jitter buffer
			samples := c.jitterBuffer.Get(targetFramesize)

			// If no samples available, use packet loss concealment
			if samples == nil {
				samples = c.jitterBuffer.Conceal(targetFramesize, c.lastSamples)
				// Don't count concealed packets as active to reduce their impact
				if len(samples) > 0 {
					// Mix in concealed audio at reduced level
					for i := range samples {
						if i < len(summed) {
							summed[i] += samples[i] * 0.3 // reduce volume of concealed audio
						}
					}
				}
				continue
			}

			// Store last valid samples for future concealment
			if len(samples) > 0 {
				c.lastSamples = make([]float32, len(samples))
				copy(c.lastSamples, samples)
			}

			// Mix in the samples
			for i, s := range samples {
				if i < len(summed) {
					summed[i] += s
				}
			}
			active++
		}

		if active == 0 {
			continue
		}

		// Normalize and apply soft clipping
		for i := range summed {
			summed[i] /= float32(active)

			// Soft clipping to prevent harsh distortion
			if summed[i] > 1 {
				summed[i] = 1
			} else if summed[i] < -1 {
				summed[i] = -1
			}
		}

		ringMu.Lock()
		ring.Write(summed)
		ringMu.Unlock()
	}
}
