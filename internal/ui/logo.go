package ui

import (
	"bytes"
	"strings"
	"sync"

	termimg "github.com/blacktop/go-termimg"

	"github.com/Tomsk73/chaintui/internal/resources"
)

var (
	logoOnce   sync.Once
	logoString string
)

func logo() string {
	logoOnce.Do(func() {
		img, err := termimg.From(bytes.NewReader(resources.LinkyGreen))
		if err != nil {
			return
		}
		// Halfblocks produces plain Unicode block chars that lipgloss can measure.
		// Width(4)+Height(1) gives a small square; trim the trailing newline so
		// the result is a single line safe for JoinHorizontal.
		s, err := img.Width(4).Height(4).Protocol(termimg.Halfblocks).Render()
		if err != nil {
			return
		}
		logoString = strings.TrimRight(s, "\n") + " "
	})
	return logoString
}
