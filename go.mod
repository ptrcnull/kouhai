module git.sr.ht/~taiite/senpai

go 1.16

require (
	git.sr.ht/~emersion/go-scfg v0.0.0-20201019143924-142a8aa629fc
	github.com/gdamore/tcell/v2 v2.3.11
	github.com/mattn/go-runewidth v0.0.13
	golang.org/x/net v0.0.0-20220127200216-cd36cc0744dd
	golang.org/x/term v0.0.0-20210927222741-03fcf44c2211
	golang.org/x/time v0.0.0-20210611083556-38a9dc6acbc6
	mvdan.cc/xurls/v2 v2.3.0
)

replace github.com/gdamore/tcell/v2 => github.com/delthas/tcell/v2 v2.4.1-0.20220223131437-2362f49a2b6c
