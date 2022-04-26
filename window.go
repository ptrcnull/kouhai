package senpai

import (
	"hash/fnv"
	"strings"
	"time"

	"git.sr.ht/~taiite/senpai/ui"
	"github.com/gdamore/tcell/v2"
)

const welcomeMessage = "senpai production build. See senpai(1) for a list of keybindings and commands."

func (app *App) initWindow() {
	app.win.AddBuffer("", "(home)", "")
	app.win.AddLine("", "", ui.NotifyNone, ui.Line{
		Head: "--",
		Body: ui.PlainString(welcomeMessage),
		At:   time.Now(),
	})
}

type statusLine struct {
	netID string
	line  ui.Line
}

func (app *App) queueStatusLine(netID string, line ui.Line) {
	if line.At.IsZero() {
		line.At = time.Now()
	}
	app.events <- event{
		src: "*",
		content: statusLine{
			netID: netID,
			line:  line,
		},
	}
}

func (app *App) addStatusLine(netID string, line ui.Line) {
	currentNetID, buffer := app.win.CurrentBuffer()
	if currentNetID == netID && buffer != "" {
		app.win.AddLine(netID, buffer, ui.NotifyNone, line)
	}
	app.win.AddLine(netID, "", ui.NotifyNone, line)
}

func (app *App) setStatus() {
	netID, buffer := app.win.CurrentBuffer()
	s := app.sessions[netID]
	if s == nil {
		return
	}
	ts := s.Typings(buffer)
	status := ""
	if 3 < len(ts) {
		status = "several people are typing..."
	} else {
		verb := " is typing..."
		if 1 < len(ts) {
			verb = " are typing..."
			status = strings.Join(ts[:len(ts)-1], ", ") + " and "
		}
		if 0 < len(ts) {
			status += ts[len(ts)-1] + verb
		}
	}
	app.win.SetStatus(status)
}

func (app *App) setBufferNumbers() {
	input := app.win.InputContent()
	if !isCommand(input) {
		app.win.ShowBufferNumbers(false)
		return
	}
	commandEnd := len(input)
	for i := 1; i < len(input); i++ {
		if input[i] == ' ' {
			commandEnd = i
			break
		}
	}
	command := string(input[1:commandEnd])
	showBufferNumbers := len(command) != 0 && strings.HasPrefix("buffer", command)
	app.win.ShowBufferNumbers(showBufferNumbers)
}

func identColor(ident string) tcell.Color {
	h := fnv.New32()
	_, _ = h.Write([]byte(ident))
	return tcell.Color((h.Sum32()%15)+1) + tcell.ColorValid
}

func identString(ident string) ui.StyledString {
	color := identColor(ident)
	style := tcell.StyleDefault.Foreground(color)
	return ui.Styled(ident, style)
}
