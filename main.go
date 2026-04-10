package main

import (
	"fyne.io/fyne/v2/app"
	"go-crawler-demo/internal/ui"
)

func main() {
	a := app.New()
	window := ui.NewWindow(a)
	window.ShowAndRun()
}
