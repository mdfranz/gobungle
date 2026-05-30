package game

import (
	"log/slog"
	"math"
	"math/rand"
	"sync"
	"time"

	"github.com/gopxl/beep"
	"github.com/gopxl/beep/speaker"
)

var (
	soundEnabled bool
	soundMu      sync.Mutex
	lastPlayTime = make(map[string]time.Time)
)

// InitSound initializes the audio speaker.
// If it fails, it will gracefully fallback with sound disabled.
func InitSound() {
	soundMu.Lock()
	defer soundMu.Unlock()

	sr := beep.SampleRate(44100)
	// Initialize speaker with a buffer size of 1/10th of a second
	err := speaker.Init(sr, sr.N(time.Second/10))
	if err != nil {
		slog.Warn("Failed to initialize audio speaker (running in silent mode)", "error", err)
		soundEnabled = false
		return
	}
	soundEnabled = true
	slog.Info("Audio system successfully initialized using Beep")
}

// SynthSound holds state for a synthesized sound effect.
type SynthSound struct {
	sampleRate beep.SampleRate
	duration   time.Duration
	time       float64
	soundType  string
	volume     float64
}

func (s *SynthSound) Stream(samples [][2]float64) (n int, ok bool) {
	totalSamples := int(float64(s.sampleRate) * s.duration.Seconds())
	currentSample := int(s.time * float64(s.sampleRate))

	if currentSample >= totalSamples {
		return 0, false
	}

	for i := range samples {
		if currentSample >= totalSamples {
			return i, true
		}

		progress := float64(currentSample) / float64(totalSamples)
		var val float64

		switch s.soundType {
		case "warning":
			// Pulsating dual tone alarm
			freq := 880.0
			if int(s.time*8)%2 == 0 {
				freq = 660.0
			}
			val = math.Sin(2 * math.Pi * freq * s.time)
			if progress > 0.8 {
				val *= (1.0 - progress) / 0.2
			}

		case "laser":
			// Fast descending sweep (700Hz -> 100Hz)
			freq := 700.0 - 600.0*progress
			val = math.Sin(2 * math.Pi * freq * s.time)
			val *= math.Exp(-6.0 * progress)

		case "missile":
			// Ascending sweep (150Hz -> 900Hz) + some noise modulation
			freq := 150.0 + 750.0*progress
			noise := rand.Float64()*2.0 - 1.0
			val = math.Sin(2*math.Pi*freq*s.time) + 0.15*noise
			amp := 1.0
			if progress < 0.15 {
				amp = progress / 0.15
			} else if progress > 0.7 {
				amp = (1.0 - progress) / 0.3
			}
			val *= amp

		case "explosion":
			// Decaying low frequency rumble + white noise
			noise := rand.Float64()*2.0 - 1.0
			freq := 100.0 * (1.0 - progress)
			rumble := math.Sin(2 * math.Pi * freq * s.time)
			val = 0.6*noise + 0.4*rumble
			val *= math.Exp(-4.0 * progress)
		}

		samples[i][0] = val * s.volume
		samples[i][1] = val * s.volume
		s.time += 1.0 / float64(s.sampleRate)
		currentSample++
	}

	return len(samples), true
}

func (s *SynthSound) Err() error {
	return nil
}

// PlaySound plays a synthesized sound of a given type.
func PlaySound(soundType string) {
	soundMu.Lock()
	defer soundMu.Unlock()

	if !soundEnabled {
		return
	}

	// Rate limit: don't play the same sound type more than once every 80ms
	// to prevent volume clipping and muddy overlay in intense combat.
	now := time.Now()
	if prev, ok := lastPlayTime[soundType]; ok && now.Sub(prev) < 80*time.Millisecond {
		return
	}
	lastPlayTime[soundType] = now

	sr := beep.SampleRate(44100)
	var dur time.Duration
	volume := 0.25

	switch soundType {
	case "warning":
		dur = 150 * time.Millisecond
		volume = 0.18
	case "laser":
		dur = 100 * time.Millisecond
		volume = 0.12
	case "missile":
		dur = 300 * time.Millisecond
		volume = 0.18
	case "explosion":
		dur = 500 * time.Millisecond
		volume = 0.25
	default:
		return
	}

	speaker.Play(&SynthSound{
		sampleRate: sr,
		duration:   dur,
		soundType:  soundType,
		volume:     volume,
	})
}
