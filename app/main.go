// Klondike: Gold Rush Solitaire (c) 2026 Ironite.
// Serves the game on localhost and opens it in its own app window
// (Microsoft Edge or Chrome in app mode), then exits when the window closes.
package main

import (
	"embed"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"
)

var version = "0.0.0-dev" // set with -ldflags "-X main.version=..."

const productName = "Klondike"

//go:embed web
var webFS embed.FS

var lastPing atomic.Int64

// profileDir holds the game window's browser profile, which is where the
// game keeps its settings and win record. Same folder as Klondike 1.0/1.1.
func profileDir() string {
	d := os.Getenv("LOCALAPPDATA")
	if d == "" {
		d, _ = os.UserCacheDir()
	}
	return filepath.Join(d, "KlondikeGoldRush", "window")
}

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "/version") {
		fmt.Println(version)
		return
	}
	sub, _ := fs.Sub(webFS, "web")
	mux := http.NewServeMux()
	mux.HandleFunc("/ping", func(w http.ResponseWriter, r *http.Request) {
		lastPing.Store(time.Now().UnixMilli())
		w.WriteHeader(http.StatusNoContent)
	})
	files := http.FileServer(http.FS(sub))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Cache-Control", "no-store")
		files.ServeHTTP(w, r)
	})

	// A fixed port keeps saved settings and the win record between sessions
	// (browser storage is tied to the address).
	var ln net.Listener
	var err error
	for _, p := range []int{48981, 48982, 48983, 48984, 48985, 0} {
		if ln, err = net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", p)); err == nil {
			break
		}
	}
	if err != nil {
		fatal("Klondike could not start: " + err.Error())
	}
	go func() { _ = http.Serve(ln, mux) }()
	url := fmt.Sprintf("http://%s/index.html", ln.Addr().String())
	if os.Getenv("IRONITE_DEV") != "" {
		fmt.Println(url)
		select {}
	}

	started := time.Now()
	if wait, ok := openWindow(url, profileDir()); ok {
		wait()
		if time.Since(started) > 5*time.Second {
			return
		}
	} else {
		openDefaultBrowser(url)
	}
	// The browser handed the page to an existing window: stay up while it pings.
	for {
		time.Sleep(3 * time.Second)
		lp := lastPing.Load()
		if (lp != 0 && time.Since(time.UnixMilli(lp)) > 30*time.Second) || (lp == 0 && time.Since(started) > 90*time.Second) {
			return
		}
	}
}
