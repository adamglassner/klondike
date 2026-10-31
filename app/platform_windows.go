//go:build windows

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"unsafe"
)

func fatal(text string) {
	t, _ := syscall.UTF16PtrFromString(text)
	c, _ := syscall.UTF16PtrFromString(productName)
	syscall.NewLazyDLL("user32.dll").NewProc("MessageBoxW").Call(0, uintptr(unsafe.Pointer(t)), uintptr(unsafe.Pointer(c)), 0x10)
	os.Exit(1)
}

func findBrowser() string {
	var roots []string
	for _, k := range []string{"ProgramFiles(x86)", "ProgramFiles", "LOCALAPPDATA"} {
		if v := os.Getenv(k); v != "" {
			roots = append(roots, v)
		}
	}
	for _, rel := range []string{`Microsoft\Edge\Application\msedge.exe`, `Google\Chrome\Application\chrome.exe`, `BraveSoftware\Brave-Browser\Application\brave.exe`} {
		for _, r := range roots {
			if p := filepath.Join(r, rel); fileExists(p) {
				return p
			}
		}
	}
	return ""
}

func fileExists(p string) bool { st, err := os.Stat(p); return err == nil && !st.IsDir() }

func openWindow(url, profile string) (func(), bool) {
	b := findBrowser()
	if b == "" {
		return nil, false
	}
	_ = os.MkdirAll(profile, 0o755)
	cmd := exec.Command(b, "--app="+url, "--user-data-dir="+profile, "--window-size=1440,920",
		"--autoplay-policy=no-user-gesture-required", "--no-first-run", "--no-default-browser-check",
		"--disable-features=Translate,msEdgeSidebarV2")
	if err := cmd.Start(); err != nil {
		return nil, false
	}
	return func() { _ = cmd.Wait() }, true
}

func openDefaultBrowser(url string) {
	_ = exec.Command("rundll32", "url.dll,FileProtocolHandler", url).Start()
}
