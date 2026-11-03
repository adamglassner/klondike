// Klondike: Gold Rush Solitaire (c) 2026 Ironite.
// A native Windows game: its own window, drawing and sound, no browser.
package main

import (
	"fmt"
	"os"
)

const productName = "Klondike"

func main() {
	if len(os.Args) > 1 && (os.Args[1] == "--version" || os.Args[1] == "/version") {
		fmt.Println(version)
		return
	}
	if err := loadArt(); err != nil {
		fatal("Klondike couldn't load its card art:\n" + err.Error())
	}
	go mix.load()
	run()
}
