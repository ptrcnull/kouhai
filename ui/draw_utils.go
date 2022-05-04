package ui

import (
	"fmt"
	"time"

	"github.com/gdamore/tcell/v2"
)

func printString(screen tcell.Screen, x *int, y int, s StyledString) {
	style := tcell.StyleDefault
	nextStyles := s.styles
	for i, r := range s.string {
		if 0 < len(nextStyles) && nextStyles[0].Start == i {
			style = nextStyles[0].Style
			nextStyles = nextStyles[1:]
		}
		screen.SetContent(*x, y, r, nil, style)
		*x += runeWidth(r)
	}
}

func printIdent(screen tcell.Screen, x, y, width int, s StyledString) {
	s.string = truncate(s.string, width, "\u2026")
	x += width - stringWidth(s.string)
	st := tcell.StyleDefault
	if len(s.styles) != 0 && s.styles[0].Start == 0 {
		st = s.styles[0].Style
	}
	screen.SetContent(x-1, y, ' ', nil, st)
	printString(screen, &x, y, s)
	if len(s.styles) != 0 {
		// TODO check if it's not a style that is from the truncated
		// part of s.
		st = s.styles[len(s.styles)-1].Style
	}
	screen.SetContent(x, y, ' ', nil, st)
}

func printNumber(screen tcell.Screen, x *int, y int, st tcell.Style, n int) {
	s := Styled(fmt.Sprintf("%d", n), st)
	printString(screen, x, y, s)
}

func printTime(screen tcell.Screen, x int, y int, st tcell.Style, t time.Time) {
	printString(screen, &x, y, Styled(t.Format("15:04:05"), tcell.StyleDefault.Foreground(tcell.ColorGray)))
}

func clearArea(screen tcell.Screen, x0, y0, width, height int) {
	for x := x0; x < x0+width; x++ {
		for y := y0; y < y0+height; y++ {
			screen.SetContent(x, y, ' ', nil, tcell.StyleDefault)
		}
	}
}

func drawVerticalLine(screen tcell.Screen, x, y0, height int) {
	for y := y0; y < y0+height; y++ {
		screen.SetContent(x, y, 0x2502, nil, tcell.StyleDefault)
	}
}
