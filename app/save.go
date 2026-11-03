// Settings and the win record. Klondike: Gold Rush Solitaire (c) 2026 Ironite.
//
// Saved to %LOCALAPPDATA%\KlondikeGoldRush\save.json, outside the install
// folder - the same folder Klondike has always used, so updates, reinstalls
// and the setup's uninstaller (which asks first) all find it.
package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
)

type saveData struct {
	Version int  `json:"version"`
	Draw    int  `json:"draw"`
	Music   bool `json:"music"`
	Sfx     bool `json:"sfx"`
	Vol     int  `json:"vol"`
	Played  int  `json:"played"`
	Won     int  `json:"won"`
	// window placement
	WinW      int  `json:"winW,omitempty"`
	WinH      int  `json:"winH,omitempty"`
	Maximized bool `json:"maximized,omitempty"`
}

func defaultSave() saveData { return saveData{Version: 2, Draw: 1, Music: true, Sfx: true, Vol: 55} }

func dataDir() string {
	d := os.Getenv("LOCALAPPDATA")
	if d == "" {
		d, _ = os.UserCacheDir()
	}
	return filepath.Join(d, "KlondikeGoldRush")
}

func savePath() string { return filepath.Join(dataDir(), "save.json") }

func loadSave() saveData {
	s := defaultSave()
	b, err := os.ReadFile(savePath())
	if err == nil && json.Unmarshal(b, &s) == nil {
		s.normalize()
		return s
	}
	// First run of the Go version: bring over the record from Klondike 1.x,
	// which kept it in its browser window's storage.
	migrateWebSave(&s)
	s.normalize()
	return s
}

func (s *saveData) normalize() {
	if s.Draw != 3 {
		s.Draw = 1
	}
	s.Vol = max(0, min(100, s.Vol))
	s.Version = 2
}

func (s saveData) write() {
	_ = os.MkdirAll(dataDir(), 0o755)
	b, _ := json.MarshalIndent(s, "", "  ")
	tmp := savePath() + ".tmp"
	if os.WriteFile(tmp, b, 0o644) == nil {
		_ = os.Rename(tmp, savePath())
	}
}

var (
	reStats    = regexp.MustCompile(`"played":(\d+),"won":(\d+)`)
	reSettings = regexp.MustCompile(`"draw":([13]),"music":(true|false),"sfx":(true|false),"vol":(\d+)`)
)

// migrateWebSave looks for Klondike 1.x's settings and win record in the old
// browser profile (Chromium keeps localStorage in plain LevelDB logs). Best
// effort: if nothing readable is there, the defaults stand.
func migrateWebSave(s *saveData) {
	dir := filepath.Join(dataDir(), "window", "Default", "Local Storage", "leveldb")
	ents, err := os.ReadDir(dir)
	if err != nil {
		return
	}
	bestPlayed := -1
	for _, e := range ents {
		n := e.Name()
		if filepath.Ext(n) != ".log" && filepath.Ext(n) != ".ldb" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(dir, n))
		if err != nil {
			continue
		}
		for _, m := range reStats.FindAllSubmatch(b, -1) {
			p, _ := strconv.Atoi(string(m[1]))
			w, _ := strconv.Atoi(string(m[2]))
			if p >= bestPlayed && w <= p {
				bestPlayed, s.Played, s.Won = p, p, w
			}
		}
		if all := reSettings.FindAllSubmatch(b, -1); len(all) > 0 {
			m := all[len(all)-1]
			s.Draw, _ = strconv.Atoi(string(m[1]))
			s.Music = string(m[2]) == "true"
			s.Sfx = string(m[3]) == "true"
			s.Vol, _ = strconv.Atoi(string(m[4]))
		}
	}
}
