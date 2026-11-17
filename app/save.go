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
	// the claim register (local leaderboard)
	Nickname string       `json:"nickname,omitempty"`
	Scores   []scoreEntry `json:"scores,omitempty"`
	// options
	Vegas      bool `json:"vegas,omitempty"`
	Winnable   bool `json:"winnable,omitempty"`
	LeftHanded bool `json:"leftHanded,omitempty"`
	FourColor  bool `json:"fourColor,omitempty"`
	LargeCards bool `json:"largeCards,omitempty"`
	Controls   int  `json:"controls,omitempty"` // 0 automatic, 1 touch-sized, 2 standard
	Back       int  `json:"back,omitempty"`     // card back
	Table      int  `json:"table,omitempty"`
	// records
	Poke       int                    `json:"poke,omitempty"` // Vegas balance from finished deals
	Streak     int                    `json:"streak,omitempty"`
	BestStreak int                    `json:"bestStreak,omitempty"`
	Draw1      modeStats              `json:"draw1"`
	Draw3      modeStats              `json:"draw3"`
	Stamps     map[string]string      `json:"stamps,omitempty"` // assay stamp id -> date earned
	Daily      map[string]dailyResult `json:"daily,omitempty"`  // date -> best result
	// window placement
	WinW      int  `json:"winW,omitempty"`
	WinH      int  `json:"winH,omitempty"`
	Maximized bool `json:"maximized,omitempty"`
}

// modeStats are the statistics for one draw mode. Played counts deals where
// at least one move was made.
type modeStats struct {
	Played     int `json:"played"`
	Won        int `json:"won"`
	BestSecs   int `json:"bestSecs,omitempty"`
	WinSecs    int `json:"winSecs,omitempty"` // total time of won games, for the average
	BestScore  int `json:"bestScore,omitempty"`
	Streak     int `json:"streak,omitempty"`
	BestStreak int `json:"bestStreak,omitempty"`
}

type dailyResult struct {
	Won   bool `json:"won"`
	Score int  `json:"score"`
	Secs  int  `json:"secs"`
	Moves int  `json:"moves"`
}

func (s *saveData) mode(draw int) *modeStats {
	if draw == 3 {
		return &s.Draw3
	}
	return &s.Draw1
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
	keep := s.Scores[:0]
	for _, e := range s.Scores {
		if e.Name = cleanName(e.Name); e.Name != "" && e.Score >= 0 {
			keep = append(keep, e)
		}
	}
	sortScores(keep)
	if len(keep) > boardSize {
		keep = keep[:boardSize]
	}
	s.Scores = keep
	s.Nickname = cleanName(s.Nickname)
	if s.Stamps == nil {
		s.Stamps = map[string]string{}
	}
	if s.Daily == nil {
		s.Daily = map[string]dailyResult{}
	}
	s.Back = max(0, min(len(cardBacks)-1, s.Back))
	s.Table = max(0, min(len(tables)-1, s.Table))
	s.Controls = max(0, min(2, s.Controls))
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
