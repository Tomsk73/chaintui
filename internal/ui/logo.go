package ui

import (
	"strings"
	"sync"

	termimg "github.com/blacktop/go-termimg"
)

var (
	logoOnce   sync.Once
	logoString string
)

func logo() string {
	logoOnce.Do(func() {
		img, err := termimg.NewImageWidgetFromFile("../resources/linky_green.png")
		if err != nil {
			return
		}
		// Halfblocks produces plain Unicode block chars that lipgloss can measure.
		// SetSize(4,1) gives a small square; trim the trailing newline so
		// the result is a single line safe for JoinHorizontal.
		s, err := img.SetSize(4, 1).SetProtocol(termimg.Halfblocks).Render()
		if err != nil {
			return
		}
		logoString = strings.TrimRight(s, "\n") + " "
	})
	return logoString
}
