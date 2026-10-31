//go:build !windows

package main

import (
	"fmt"
	"os"
)

// Development stand-ins (the game ships for Windows).
func fatal(text string)                        { fmt.Fprintln(os.Stderr, text); os.Exit(1) }
func openWindow(string, string) (func(), bool) { return nil, false }
func openDefaultBrowser(url string)            { fmt.Println("Open:", url) }
