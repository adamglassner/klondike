// Audio: the original game's music and sound effects, rendered from its own
// synthesizer by tools/export and stored as IMA ADPCM. This mixes them for
// the platform's audio output (waveOut on Windows).
// Klondike: Gold Rush Solitaire (c) 2026 Ironite.
package main

import (
	"encoding/binary"
	"encoding/json"
	"errors"
	"math"
	"math/rand"
	"sort"
	"strings"
	"sync"
)

const sampleRate = 32000

type pcm struct {
	frames int
	data   []int16 // interleaved stereo
}

var imaStep = [89]int32{7, 8, 9, 10, 11, 12, 13, 14, 16, 17, 19, 21, 23, 25, 28, 31, 34, 37, 41, 45, 50, 55, 60, 66, 73, 80, 88, 97, 107, 118, 130, 143, 157, 173, 190, 209, 230, 253, 279, 307, 337, 371, 408, 449, 494, 544, 598, 658, 724, 796, 876, 963, 1060, 1166, 1282, 1411, 1552, 1707, 1878, 2066, 2272, 2499, 2749, 3024, 3327, 3660, 4026, 4428, 4871, 5358, 5894, 6484, 7132, 7845, 8630, 9493, 10442, 11487, 12635, 13899, 15289, 16818, 18500, 20350, 22385, 24623, 27086, 29794, 32767}
var imaIdx = [16]int32{-1, -1, -1, -1, 2, 4, 6, 8, -1, -1, -1, -1, 2, 4, 6, 8}

const imaBlock = 1024

// decodeIMA decodes the game's stereo ADPCM files (see tools/export/pack.py).
func decodeIMA(b []byte) (*pcm, error) {
	if len(b) < 14 || string(b[:4]) != "KIMA" {
		return nil, errors.New("not an audio file")
	}
	ch := binary.LittleEndian.Uint16(b[8:])
	n := int(binary.LittleEndian.Uint32(b[10:]))
	if ch != 2 {
		return nil, errors.New("expected stereo audio")
	}
	out := &pcm{frames: n, data: make([]int16, n*2)}
	p := 14
	var pred [2]int32
	var idx [2]int32
	step := func(c int, code byte) int16 {
		s := imaStep[idx[c]]
		d := s >> 3
		if code&4 != 0 {
			d += s
		}
		if code&2 != 0 {
			d += s >> 1
		}
		if code&1 != 0 {
			d += s >> 2
		}
		if code&8 != 0 {
			pred[c] -= d
		} else {
			pred[c] += d
		}
		pred[c] = max(-32768, min(32767, pred[c]))
		idx[c] = max(0, min(88, idx[c]+imaIdx[code&15]))
		return int16(pred[c])
	}
	for f := 0; f < n; f += imaBlock {
		if p+6 > len(b) {
			return nil, errors.New("audio file is truncated")
		}
		pred[0] = int32(int16(binary.LittleEndian.Uint16(b[p:])))
		idx[0] = int32(b[p+2])
		pred[1] = int32(int16(binary.LittleEndian.Uint16(b[p+3:])))
		idx[1] = int32(b[p+5])
		p += 6
		cnt := min(imaBlock, n-f)
		if p+cnt > len(b) {
			return nil, errors.New("audio file is truncated")
		}
		for i := 0; i < cnt; i++ {
			v := b[p+i]
			out.data[(f+i)*2] = step(0, v&15)
			out.data[(f+i)*2+1] = step(1, v>>4)
		}
		p += cnt
	}
	return out, nil
}

type voice struct {
	s     *pcm
	pos   int
	delay int
	gain  float32
}

type mixer struct {
	mu     sync.Mutex
	ready  bool
	tunes  []*pcm
	names  []string
	sfx    map[string][]*pcm
	voices []voice

	musicOn bool    // the player wants music
	vol     float64 // 0..1 slider
	sfxOn   bool
	playing bool    // a tune is playing (not stopped)
	tune    int     // current tune
	pos     int     // frame in the current tune
	wait    int     // frames of silence before the tune starts
	gain    float32 // current music gain (ramps toward target)
	onTune  func(name string)
}

var mix = &mixer{sfx: map[string][]*pcm{}, tune: rand.Intn(4)}

// loadAudio decodes everything in the background; music starts when ready.
func (m *mixer) load() {
	b, _ := assetFS.ReadFile("assets/audio/audio.json")
	var meta struct {
		Names map[string]string `json:"names"`
	}
	_ = json.Unmarshal(b, &meta)
	ents, err := assetFS.ReadDir("assets/audio")
	if err != nil {
		return
	}
	var files []string
	for _, e := range ents {
		if strings.HasSuffix(e.Name(), ".ima") {
			files = append(files, e.Name())
		}
	}
	sort.Strings(files)
	tunes := map[int]*pcm{}
	sfx := map[string][]*pcm{}
	for _, f := range files {
		data, err := assetFS.ReadFile("assets/audio/" + f)
		if err != nil {
			continue
		}
		p, err := decodeIMA(data)
		if err != nil {
			continue
		}
		base := strings.TrimSuffix(f, ".ima")
		name := strings.TrimRight(base, "0123456789")
		if name == "tune" {
			var i int
			for _, c := range base[4:] {
				i = i*10 + int(c-'0')
			}
			tunes[i] = p
		} else {
			sfx[name] = append(sfx[name], p)
		}
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	for i := 0; i < len(tunes); i++ {
		if tunes[i] != nil {
			m.tunes = append(m.tunes, tunes[i])
			m.names = append(m.names, meta.Names["tune"+itoa(i)])
		}
	}
	m.sfx = sfx
	m.ready = true
	if len(m.tunes) > 0 {
		m.tune %= len(m.tunes)
	}
	if m.musicOn {
		m.startLocked()
	}
}

func (m *mixer) volFactor() float32 {
	if !m.musicOn {
		return 0
	}
	return float32(math.Pow(m.vol, 1.5))
}

func (m *mixer) startLocked() {
	if !m.ready || len(m.tunes) == 0 || m.playing {
		return
	}
	m.playing, m.pos, m.wait = true, 0, sampleRate/4
	if m.onTune != nil {
		go m.onTune(m.names[m.tune])
	}
}

// setMusic turns music on or off. Turning it off fades out; the next start
// plays the next tune, like the original.
func (m *mixer) setMusic(on bool) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.musicOn = on
	if on {
		m.startLocked()
	}
}

func (m *mixer) setVol(v float64) { m.mu.Lock(); m.vol = clamp(v, 0, 1); m.mu.Unlock() }
func (m *mixer) setSfx(on bool)   { m.mu.Lock(); m.sfxOn = on; m.mu.Unlock() }

// play starts a sound effect, optionally after a delay in seconds.
func (m *mixer) play(name string, delay float64) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if !m.sfxOn || !m.ready {
		return
	}
	v := m.sfx[name]
	if len(v) == 0 {
		return
	}
	g := float32(1)
	if name == "fanfare" {
		// The fanfare plays through the music channel: at the music volume,
		// or at a fixed level when music is off (as in the original).
		g = m.volFactor()
		if !m.playing || !m.musicOn {
			g = float32(math.Max(float64(g), .35/.9))
		}
	}
	if len(m.voices) >= 24 {
		m.voices = m.voices[1:]
	}
	m.voices = append(m.voices, voice{s: v[rand.Intn(len(v))], delay: int(delay * sampleRate), gain: g})
}

// fill mixes the next len(out)/2 stereo frames.
func (m *mixer) fill(out []int16) {
	m.mu.Lock()
	defer m.mu.Unlock()
	frames := len(out) / 2
	target := m.volFactor()
	if !m.playing {
		target = 0
	}
	// gain ramps: .3 s when starting, .12 s when stopping, .1 s otherwise
	tc := float32(.1)
	if target > m.gain && m.gain < .02 {
		tc = .3
	} else if target == 0 {
		tc = .12
	}
	k := 1 - float32(math.Exp(-1/float64(tc*sampleRate)))
	var t *pcm
	if m.ready && len(m.tunes) > 0 {
		t = m.tunes[m.tune]
	}
	for i := 0; i < frames; i++ {
		m.gain += (target - m.gain) * k
		var l, r float32
		if t != nil && m.playing {
			if m.wait > 0 {
				m.wait--
			} else if m.pos < t.frames {
				l, r = float32(t.data[m.pos*2])*m.gain, float32(t.data[m.pos*2+1])*m.gain
				m.pos++
			} else { // tune over: 1.6 s gap, then the next one
				m.tune = (m.tune + 1) % len(m.tunes)
				t = m.tunes[m.tune]
				m.pos, m.wait = 0, sampleRate*16/10
				if m.onTune != nil {
					go m.onTune(m.names[m.tune])
				}
			}
		}
		out[i*2], out[i*2+1] = 0, 0
		for j := range m.voices {
			v := &m.voices[j]
			if v.delay > 0 {
				v.delay--
				continue
			}
			if v.pos < v.s.frames {
				l += float32(v.s.data[v.pos*2]) * v.gain
				r += float32(v.s.data[v.pos*2+1]) * v.gain
				v.pos++
			}
		}
		out[i*2] = int16(max(-32768, min(32767, l)))
		out[i*2+1] = int16(max(-32768, min(32767, r)))
	}
	// music switched off and faded out: stop, and move on to the next tune
	if m.playing && !m.musicOn && m.gain < .001 {
		m.playing = false
		if len(m.tunes) > 0 {
			m.tune = (m.tune + 1) % len(m.tunes)
		}
	}
	live := m.voices[:0]
	for _, v := range m.voices {
		if v.delay > 0 || v.pos < v.s.frames {
			live = append(live, v)
		}
	}
	m.voices = live
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var b []byte
	for i > 0 {
		b = append([]byte{byte('0' + i%10)}, b...)
		i /= 10
	}
	if neg {
		b = append([]byte{'-'}, b...)
	}
	return string(b)
}
