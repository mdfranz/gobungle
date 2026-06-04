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
			// Two-tone repeating alarm: 880/660Hz alternating at 8Hz, abrupt cutoff
			freq := 880.0
			if int(s.time*8)%2 == 0 {
				freq = 660.0
			}
			val = math.Sin(2 * math.Pi * freq * s.time)
			// Hard square-wave gating at 4Hz to match C64 pulsed beep rhythm
			if int(s.time*4)%2 == 0 {
				val = 0
			}
			if progress > 0.85 {
				val *= (1.0 - progress) / 0.15
			}

		case "laser":
			// C64 cannon: very short noise burst, sharp attack, fast exponential decay
			noise := rand.Float64()*2.0 - 1.0
			freq := 320.0 - 260.0*progress
			tone := math.Sin(2*math.Pi*freq*s.time) * 0.4
			val = 0.6*noise + tone
			val *= math.Exp(-18.0 * progress)

		case "missile":
			// Whoosh: shaped broadband noise with subtle tonal body
			noise := rand.Float64()*2.0 - 1.0
			freq := 200.0 + 120.0*math.Sin(math.Pi*progress)
			tone := math.Sin(2 * math.Pi * freq * s.time)
			val = 0.8*noise + 0.2*tone
			val *= math.Sin(math.Pi * progress)

		case "explosion":
			// C64-style boom: heavy noise + deep sub-bass ~30Hz, fast initial punch then slow rumble
			noise := rand.Float64()*2.0 - 1.0
			subFreq := 30.0 * (1.0 - progress*0.5)
			subBass := math.Sin(2 * math.Pi * subFreq * s.time)
			rumble := math.Sin(2 * math.Pi * subFreq * 2 * s.time)
			val = 0.55*noise + 0.30*subBass + 0.15*rumble
			val *= math.Exp(-1.8 * progress)

		case "speedboat":
			// High-speed turbine/engine whine + rhythmic water slap
			// Engine: slightly oscillating frequency for "moving" feel
			freq := 220.0 + 15.0*math.Sin(2*math.Pi*8.0*s.time)
			engine := math.Sin(2 * math.Pi * freq * s.time) * 0.4
			// Water slap noise: pulses at ~6Hz
			slap := 0.0
			if int(s.time*12)%2 == 0 {
				noise := rand.Float64()*2.0 - 1.0
				slap = noise * 0.4 * math.Exp(-30.0*math.Mod(s.time, 1.0/12.0))
			}
			val = engine + slap
			val *= 0.5
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
	if prev, ok := lastPlayTime[soundType]; ok && now.Sub(prev) < 60*time.Millisecond {
		return
	}
	lastPlayTime[soundType] = now

	sr := beep.SampleRate(44100)
	var dur time.Duration
	volume := 0.25

	switch soundType {
	case "warning":
		dur = 500 * time.Millisecond // long enough to hear the gated pulse rhythm
		volume = 0.20
	case "laser":
		dur = 55 * time.Millisecond // C64: ~30-40ms burst; 55ms gives audible tail
		volume = 0.28
	case "missile":
		dur = 400 * time.Millisecond
		volume = 0.20
	case "explosion":
		dur = 800 * time.Millisecond
		volume = 0.38
	case "speedboat":
		dur = 300 * time.Millisecond
		volume = 0.18
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
