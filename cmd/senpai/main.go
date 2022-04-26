package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/signal"
	"path"
	"strings"
	"syscall"
	"time"

	"git.sr.ht/~taiite/senpai"
	"github.com/gdamore/tcell/v2"
)

func main() {
	tcell.SetEncodingFallback(tcell.EncodingFallbackASCII)

	var configPath string
	var debug bool
	flag.StringVar(&configPath, "config", "", "path to the configuration file")
	flag.BoolVar(&debug, "debug", false, "show raw protocol data in the home buffer")
	flag.Parse()

	if configPath == "" {
		configDir, err := os.UserConfigDir()
		if err != nil {
			panic(err)
		}
		configPath = path.Join(configDir, "senpai", "senpai.scfg")
	}

	cfg, err := senpai.LoadConfigFile(configPath)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to load the required configuration file at %q: %s\n", configPath, err)
		os.Exit(1)
	}

	cfg.Debug = cfg.Debug || debug

	app, err := senpai.NewApp(cfg)
	if err != nil {
		panic(err)
	}

	lastNetID, lastBuffer := getLastBuffer()
	app.SwitchToBuffer(lastNetID, lastBuffer)
	app.SetLastClose(getLastStamp())
	app.SetUnreads(getUnreads())

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP)

	go func() {
		<-sigCh
		app.Close()
	}()

	app.Run()
	app.Close()
	writeLastBuffer(app)
	writeLastStamp(app)
	writeUnreads(app)
}

func cachePath() string {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		panic(err)
	}
	cache := path.Join(cacheDir, "senpai")
	err = os.MkdirAll(cache, 0755)
	if err != nil {
		panic(err)
	}
	return cache
}

func lastBufferPath() string {
	return path.Join(cachePath(), "lastbuffer.txt")
}

func getLastBuffer() (netID, buffer string) {
	buf, err := ioutil.ReadFile(lastBufferPath())
	if err != nil {
		return "", ""
	}

	fields := strings.SplitN(string(buf), " ", 2)
	if len(fields) < 2 {
		return "", ""
	}

	return fields[0], fields[1]
}

func writeLastBuffer(app *senpai.App) {
	lastBufferPath := lastBufferPath()
	lastNetID, lastBuffer := app.CurrentBuffer()
	err := os.WriteFile(lastBufferPath, []byte(fmt.Sprintf("%s %s", lastNetID, lastBuffer)), 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to write last buffer at %q: %s\n", lastBufferPath, err)
	}
}

func lastStampPath() string {
	return path.Join(cachePath(), "laststamp.txt")
}

func getLastStamp() time.Time {
	buf, err := ioutil.ReadFile(lastStampPath())
	if err != nil {
		return time.Time{}
	}

	t, err := time.Parse(time.RFC3339Nano, string(buf))
	if err != nil {
		return time.Time{}
	}
	return t
}

func writeLastStamp(app *senpai.App) {
	lastStampPath := lastStampPath()
	last := app.LastMessageTime()
	if last.IsZero() {
		return
	}
	err := os.WriteFile(lastStampPath, []byte(last.Format(time.RFC3339Nano)), 0666)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to write last stamp at %q: %s\n", lastStampPath, err)
	}
}

func unreadsPath() string {
	return path.Join(cachePath(), "unreads.txt")
}

func getUnreads() map[string]time.Time {
	unreads := map[string]time.Time{}

	buf, err := ioutil.ReadFile(unreadsPath())
	if err != nil {
		return unreads
	}

	_ = json.Unmarshal(buf, &unreads)
	return unreads
}

func writeUnreads(app *senpai.App) {
	data, _ := json.Marshal(app.GetUnreads())
	err := ioutil.WriteFile(unreadsPath(), data, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "failed to write unreads: %s\n", err)
	}
}
