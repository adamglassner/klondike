//go:build !windows

package main

import (
	"fmt"
	"os"
)

// The game ships for Windows; elsewhere it only builds for tests.
func fatal(text string) { fmt.Fprintln(os.Stderr, text); os.Exit(1) }

func run() { fatal("Klondike runs on Windows.") }
