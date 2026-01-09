package sound

import (
	"bytes"
	"embed"
	"io"
	"sync"
	"time"

	"github.com/gopxl/beep/v2"
	"github.com/gopxl/beep/v2/mp3"
	"github.com/gopxl/beep/v2/speaker"
	"github.com/gopxl/beep/v2/wav"
)

// Player handles sound playback
type Player struct {
	enabled     bool
	initialized bool
	mu          sync.Mutex
	soundsFS    embed.FS
}

// New creates a new sound player
func New(soundsFS embed.FS, enabled bool) *Player {
	return &Player{
		enabled:  enabled,
		soundsFS: soundsFS,
	}
}

// SetEnabled enables or disables sound playback
func (p *Player) SetEnabled(enabled bool) {
	p.mu.Lock()
	defer p.mu.Unlock()
	p.enabled = enabled
}

// IsEnabled returns whether sound is enabled
func (p *Player) IsEnabled() bool {
	p.mu.Lock()
	defer p.mu.Unlock()
	return p.enabled
}

// initSpeaker initializes the speaker if not already done
func (p *Player) initSpeaker(sampleRate beep.SampleRate) error {
	if p.initialized {
		return nil
	}
	err := speaker.Init(sampleRate, sampleRate.N(time.Second/10))
	if err != nil {
		return err
	}
	p.initialized = true
	return nil
}

// playFile plays an audio file from the embedded filesystem
func (p *Player) playFile(path string) {
	p.mu.Lock()
	if !p.enabled {
		p.mu.Unlock()
		return
	}
	p.mu.Unlock()

	data, err := p.soundsFS.ReadFile(path)
	if err != nil {
		return
	}

	reader := bytes.NewReader(data)
	var streamer beep.StreamSeekCloser
	var format beep.Format

	// Try to decode based on file extension
	if len(path) > 4 && path[len(path)-4:] == ".wav" {
		streamer, format, err = wav.Decode(io.NopCloser(reader))
	} else {
		streamer, format, err = mp3.Decode(io.NopCloser(reader))
	}

	if err != nil {
		return
	}
	defer func() { _ = streamer.Close() }()

	if err := p.initSpeaker(format.SampleRate); err != nil {
		return
	}

	done := make(chan bool)
	speaker.Play(beep.Seq(streamer, beep.Callback(func() {
		done <- true
	})))
	<-done
}

// PlayCTIncrement plays the CT increment sound
func (p *Player) PlayCTIncrement() {
	go p.playFile("sound/ct_increment.wav")
}

// PlayCTDecrement plays the CT decrement sound
func (p *Player) PlayCTDecrement() {
	go p.playFile("sound/ct_decrement.wav")
}

// PlayTIncrement plays the T increment sound
func (p *Player) PlayTIncrement() {
	go p.playFile("sound/t_increment.wav")
}

// PlayTDecrement plays the T decrement sound
func (p *Player) PlayTDecrement() {
	go p.playFile("sound/t_decrement.wav")
}

// PlayMatchEnd plays the match end sound
func (p *Player) PlayMatchEnd() {
	go p.playFile("sound/match_end.wav")
}

// PlayReset plays the reset sound
func (p *Player) PlayReset() {
	go p.playFile("sound/reset.wav")
}

// PlayCTSelect plays the CT team selection sound
func (p *Player) PlayCTSelect() {
	go p.playFile("sound/ct_select.wav")
}

// PlayTSelect plays the T team selection sound
func (p *Player) PlayTSelect() {
	go p.playFile("sound/t_select.wav")
}
