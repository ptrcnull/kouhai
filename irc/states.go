package irc

import (
	"bufio"
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"strings"
	"sync/atomic"
	"time"
)

type SASLClient interface {
	Handshake() (mech string)
	Respond(challenge string) (res string, err error)
}

type SASLPlain struct {
	Username string
	Password string
}

func (auth *SASLPlain) Handshake() (mech string) {
	mech = "PLAIN"
	return
}

func (auth *SASLPlain) Respond(challenge string) (res string, err error) {
	if challenge != "+" {
		err = errors.New("unexpected challenge")
		return
	}

	user := []byte(auth.Username)
	pass := []byte(auth.Password)
	payload := bytes.Join([][]byte{user, user, pass}, []byte{0})
	res = base64.StdEncoding.EncodeToString(payload)

	return
}

var SupportedCapabilities = map[string]struct{}{
	"account-notify":    {},
	"account-tag":       {},
	"away-notify":       {},
	"batch":             {},
	"cap-notify":        {},
	"draft/chathistory": {},
	"echo-message":      {},
	"extended-join":     {},
	"invite-notify":     {},
	"labeled-response":  {},
	"message-tags":      {},
	"multi-prefix":      {},
	"server-time":       {},
	"sasl":              {},
	"setname":           {},
	"userhost-in-names": {},
}

const (
	TypingUnspec = iota
	TypingActive
	TypingPaused
	TypingDone
)

type action interface{}

type (
	actionSendRaw struct {
		raw string
	}

	actionJoin struct {
		Channel string
	}
	actionPart struct {
		Channel string
	}

	actionPrivMsg struct {
		Target  string
		Content string
	}

	actionTyping struct {
		Channel string
	}
	actionTypingStop struct {
		Channel string
	}

	actionRequestHistory struct {
		Target string
		Before time.Time
	}
)

type User struct {
	Nick    string
	AwayMsg string
}

type Channel struct {
	Name      string
	Members   map[string]string
	Topic     string
	TopicWho  string
	TopicTime time.Time
	Secret    bool
}

type SessionParams struct {
	Nickname string
	Username string
	RealName string

	Auth SASLClient

	Debug bool
}

type Session struct {
	conn io.ReadWriteCloser
	msgs chan Message
	acts chan action
	evts chan Event

	debug bool

	running      atomic.Value // bool
	registered   bool
	typingStamps map[string]time.Time

	nick   string
	nickCf string
	user   string
	real   string
	acct   string
	host   string
	auth   SASLClient

	availableCaps map[string]string
	enabledCaps   map[string]struct{}
	features      map[string]string

	users     map[string]User
	channels  map[string]Channel
	chBatches map[string]HistoryEvent
}

func NewSession(conn io.ReadWriteCloser, params SessionParams) (s Session, err error) {
	s = Session{
		conn:          conn,
		msgs:          make(chan Message, 16),
		acts:          make(chan action, 16),
		evts:          make(chan Event, 16),
		debug:         params.Debug,
		typingStamps:  map[string]time.Time{},
		nick:          params.Nickname,
		nickCf:        strings.ToLower(params.Nickname),
		user:          params.Username,
		real:          params.RealName,
		auth:          params.Auth,
		availableCaps: map[string]string{},
		enabledCaps:   map[string]struct{}{},
		features:      map[string]string{},
		users:         map[string]User{},
		channels:      map[string]Channel{},
		chBatches:     map[string]HistoryEvent{},
	}

	s.running.Store(true)

	err = s.send("CAP LS 302\r\nNICK %s\r\nUSER %s 0 * :%s\r\n", s.nick, s.user, s.real)
	if err != nil {
		return
	}

	go func() {
		r := bufio.NewScanner(conn)

		for r.Scan() {
			line := r.Text()

			msg, err := Tokenize(line)
			if err != nil {
				continue
			}

			err = msg.Validate()
			if err != nil {
				continue
			}

			if s.debug {
				s.evts <- RawMessageEvent{Message: line}
			}
			s.msgs <- msg
		}

		s.Stop()
	}()

	go s.run()

	return
}

func (s *Session) Running() bool {
	return s.running.Load().(bool)
}

func (s *Session) Stop() {
	s.running.Store(false)
	_ = s.conn.Close()
}

func (s *Session) Poll() (events <-chan Event) {
	return s.evts
}

func (s *Session) HasCapability(capability string) bool {
	_, ok := s.enabledCaps[capability]
	return ok
}

func (s *Session) Nick() string {
	return s.nick
}

func (s *Session) NickCf() string {
	return s.nickCf
}

func (s *Session) IsChannel(name string) bool {
	return strings.IndexAny(name, "#&") == 0 // TODO compute CHANTYPES
}

func (s *Session) SendRaw(raw string) {
	s.acts <- actionSendRaw{raw}
}

func (s *Session) sendRaw(act actionSendRaw) (err error) {
	err = s.send("%s\r\n", act.raw)
	return
}

func (s *Session) Join(channel string) {
	s.acts <- actionJoin{channel}
}

func (s *Session) join(act actionJoin) (err error) {
	err = s.send("JOIN %s\r\n", act.Channel)
	return
}

func (s *Session) Part(channel string) {
	s.acts <- actionPart{channel}
}

func (s *Session) part(act actionPart) (err error) {
	err = s.send("PART %s\r\n", act.Channel)
	return
}

func (s *Session) PrivMsg(target, content string) {
	s.acts <- actionPrivMsg{target, content}
}

func (s *Session) privMsg(act actionPrivMsg) (err error) {
	err = s.send("PRIVMSG %s :%s\r\n", act.Target, act.Content)
	return
}

func (s *Session) Typing(channel string) {
	s.acts <- actionTyping{channel}
}

func (s *Session) typing(act actionTyping) (err error) {
	if _, ok := s.enabledCaps["message-tags"]; !ok {
		return
	}

	to := strings.ToLower(act.Channel)
	now := time.Now()

	if t, ok := s.typingStamps[to]; ok && now.Sub(t).Seconds() < 3.0 {
		return
	}

	s.typingStamps[to] = now

	err = s.send("@+typing=active TAGMSG %s\r\n", act.Channel)
	return
}

func (s *Session) TypingStop(channel string) {
	s.acts <- actionTypingStop{channel}
}

func (s *Session) typingStop(act actionTypingStop) (err error) {
	if _, ok := s.enabledCaps["message-tags"]; !ok {
		return
	}

	err = s.send("@+typing=done TAGMSG %s\r\n", act.Channel)
	return
}

func (s *Session) RequestHistory(target string, before time.Time) {
	s.acts <- actionRequestHistory{target, before}
}

func (s *Session) requestHistory(act actionRequestHistory) (err error) {
	if _, ok := s.enabledCaps["draft/chathistory"]; !ok {
		return
	}

	t := act.Before
	err = s.send("CHATHISTORY BEFORE %s timestamp=%04d-%02d-%02dT%02d:%02d:%02d.%03dZ 100\r\n", act.Target, t.Year(), t.Month(), t.Day(), t.Hour(), t.Minute(), t.Second(), t.Nanosecond()/1e6)

	return
}

func (s *Session) run() {
	for s.Running() {
		var err error

		select {
		case act := <-s.acts:
			switch act := act.(type) {
			case actionSendRaw:
				err = s.sendRaw(act)
			case actionJoin:
				err = s.join(act)
			case actionPart:
				err = s.part(act)
			case actionPrivMsg:
				err = s.privMsg(act)
			case actionTyping:
				err = s.typing(act)
			case actionTypingStop:
				err = s.typingStop(act)
			case actionRequestHistory:
				err = s.requestHistory(act)
			}
		case msg := <-s.msgs:
			if s.registered {
				err = s.handle(msg)
			} else {
				err = s.handleStart(msg)
			}
		}

		if err != nil {
			s.evts <- err
		}
	}
}

func (s *Session) handleStart(msg Message) (err error) {
	switch msg.Command {
	case "AUTHENTICATE":
		if s.auth != nil {
			var res string

			res, err = s.auth.Respond(msg.Params[0])
			if err != nil {
				err = s.send("AUTHENTICATE *\r\n")
				return
			}

			err = s.send("AUTHENTICATE %s\r\n", res)
			if err != nil {
				return
			}
		}
	case rplLoggedin:
		err = s.send("CAP END\r\n")
		if err != nil {
			return
		}

		s.acct = msg.Params[2]
		_, _, s.host = FullMask(msg.Params[1])
	case errNicklocked, errSaslfail, errSasltoolong, errSaslaborted, errSaslalready, rplSaslmechs:
		err = s.send("CAP END\r\n")
		if err != nil {
			return
		}
	case "CAP":
		switch msg.Params[1] {
		case "LS":
			var willContinue bool
			var ls string

			if msg.Params[2] == "*" {
				willContinue = true
				ls = msg.Params[3]
			} else {
				willContinue = false
				ls = msg.Params[2]
			}

			for _, c := range TokenizeCaps(ls) {
				if c.Enable {
					s.availableCaps[c.Name] = c.Value
				} else {
					delete(s.availableCaps, c.Name)
				}
			}

			if !willContinue {
				var req strings.Builder

				for c := range s.availableCaps {
					if _, ok := SupportedCapabilities[c]; !ok {
						continue
					}

					_, _ = fmt.Fprintf(&req, "CAP REQ %s\r\n", c)
				}

				_, ok := s.availableCaps["sasl"]
				if s.auth == nil || !ok {
					_, _ = fmt.Fprintf(&req, "CAP END\r\n")
				}

				err = s.send(req.String())
				if err != nil {
					return
				}
			}
		case "ACK":
			for _, c := range strings.Split(msg.Params[2], " ") {
				s.enabledCaps[c] = struct{}{}

				if s.auth != nil && c == "sasl" {
					h := s.auth.Handshake()
					err = s.send("AUTHENTICATE %s\r\n", h)
					if err != nil {
						return
					}
				}
			}
		}
	case errNicknameinuse:
		s.nick = s.nick + "_"

		err = s.send("NICK %s\r\n", s.nick)
		if err != nil {
			return
		}
	default:
		err = s.handle(msg)
	}

	return
}

func (s *Session) handle(msg Message) (err error) {
	if id, ok := msg.Tags["batch"]; ok {
		if b, ok := s.chBatches[id]; ok {
			s.chBatches[id] = HistoryEvent{
				Target:   b.Target,
				Messages: append(b.Messages, s.privmsgToEvent(msg)),
			}
			return
		}
	}

	switch msg.Command {
	case rplWelcome:
		s.nick = msg.Params[0]
		s.nickCf = strings.ToLower(s.nick)
		s.registered = true
		s.evts <- RegisteredEvent{}

		if s.host == "" {
			err = s.send("WHO %s\r\n", s.nick)
			if err != nil {
				return
			}
		}
	case rplIsupport:
		s.updateFeatures(msg.Params[1 : len(msg.Params)-1])
	case rplWhoreply:
		if s.nickCf == strings.ToLower(msg.Params[5]) {
			s.host = msg.Params[3]
		}
	case "CAP":
		switch msg.Params[1] {
		case "ACK":
			for _, c := range strings.Split(msg.Params[2], " ") {
				s.enabledCaps[c] = struct{}{}
			}
		case "NAK":
			for _, c := range strings.Split(msg.Params[2], " ") {
				delete(s.enabledCaps, c)
			}
		case "NEW":
			diff := TokenizeCaps(msg.Params[2])

			for _, c := range diff {
				if c.Enable {
					s.availableCaps[c.Name] = c.Value
				} else {
					delete(s.availableCaps, c.Name)
				}
			}

			var req strings.Builder

			for _, c := range diff {
				_, ok := SupportedCapabilities[c.Name]
				if !c.Enable || !ok {
					continue
				}

				_, _ = fmt.Fprintf(&req, "CAP REQ %s\r\n", c.Name)
			}

			_, ok := s.availableCaps["sasl"]
			if s.acct == "" && ok {
				// TODO authenticate
			}

			err = s.send(req.String())
			if err != nil {
				return
			}
		case "DEL":
			diff := TokenizeCaps(msg.Params[2])

			for i := range diff {
				diff[i].Enable = !diff[i].Enable
			}

			for _, c := range diff {
				if c.Enable {
					s.availableCaps[c.Name] = c.Value
				} else {
					delete(s.availableCaps, c.Name)
				}
			}

			var req strings.Builder

			for _, c := range diff {
				_, ok := SupportedCapabilities[c.Name]
				if !c.Enable || !ok {
					continue
				}

				_, _ = fmt.Fprintf(&req, "CAP REQ %s\r\n", c.Name)
			}

			_, ok := s.availableCaps["sasl"]
			if s.acct == "" && ok {
				// TODO authenticate
			}

			err = s.send(req.String())
			if err != nil {
				return
			}
		}
	case "JOIN":
		nick, _, _ := FullMask(msg.Prefix)
		nickCf := strings.ToLower(nick)
		channelCf := strings.ToLower(msg.Params[0])

		if nickCf == s.nickCf {
			s.channels[channelCf] = Channel{
				Name:    msg.Params[0],
				Members: map[string]string{},
			}
		} else if c, ok := s.channels[channelCf]; ok {
			if _, ok := s.users[nickCf]; !ok {
				s.users[nickCf] = User{Nick: nick}
			}
			c.Members[nickCf] = ""

			t, ok := msg.Time()
			if !ok {
				t = time.Now()
			}

			s.evts <- UserJoinEvent{
				Channel: c.Name,
				Nick:    nick,
				Time:    t,
			}
		}
	case "PART":
		nick, _, _ := FullMask(msg.Prefix)
		nickCf := strings.ToLower(nick)
		channelCf := strings.ToLower(msg.Params[0])

		if nickCf == s.nickCf {
			delete(s.channels, channelCf)
			s.evts <- SelfPartEvent{Channel: msg.Params[0]}
		} else if c, ok := s.channels[channelCf]; ok {
			delete(c.Members, nickCf)

			t, ok := msg.Time()
			if !ok {
				t = time.Now()
			}

			s.evts <- UserPartEvent{
				Channels: []string{c.Name},
				Nick:     nick,
				Time:     t,
			}
		}
	case "QUIT":
		nick, _, _ := FullMask(msg.Prefix)
		nickCf := strings.ToLower(nick)

		t, ok := msg.Time()
		if !ok {
			t = time.Now()
		}

		var channels []string

		for _, c := range s.channels {
			if _, ok := c.Members[nickCf]; !ok {
				continue
			}
			channels = append(channels, c.Name)
		}

		s.evts <- UserPartEvent{
			Channels: channels,
			Nick:     nick,
			Time:     t,
		}
	case rplNamreply:
		channelCf := strings.ToLower(msg.Params[2])

		if c, ok := s.channels[channelCf]; ok {
			c.Secret = msg.Params[1] == "@"
			names := TokenizeNames(msg.Params[3], "~&@%+") // TODO compute prefixes

			for _, name := range names {
				nick := name.Nick
				nickCf := strings.ToLower(nick)

				if _, ok := s.users[nickCf]; !ok {
					s.users[nickCf] = User{Nick: nick}
				}
				c.Members[nickCf] = name.PowerLevel
			}
		}
	case rplEndofnames:
		channelCf := strings.ToLower(msg.Params[1])
		if c, ok := s.channels[channelCf]; ok {
			s.evts <- SelfJoinEvent{Channel: c.Name}
		}
	case rplTopic:
		channelCf := strings.ToLower(msg.Params[1])

		if c, ok := s.channels[channelCf]; ok {
			c.Topic = msg.Params[2]
		}
	case "PRIVMSG", "NOTICE":
		s.evts <- s.privmsgToEvent(msg)
	case "TAGMSG":
		nick, _, _ := FullMask(msg.Prefix)
		nickCf := strings.ToLower(nick)
		targetCf := strings.ToLower(msg.Params[0])

		if nickCf == s.nickCf {
			// TAGMSG from self
			break
		}

		typing := TypingUnspec
		if t, ok := msg.Tags["+typing"]; ok {
			if t == "active" {
				typing = TypingActive
			} else if t == "paused" {
				typing = TypingPaused
			} else if t == "done" {
				typing = TypingDone
			}
		} else {
			break
		}

		t, ok := msg.Time()
		if !ok {
			t = time.Now()
		}
		if targetCf == s.nickCf {
			// TAGMSG to self
			s.evts <- QueryTagEvent{
				Nick:   nick,
				Typing: typing,
				Time:   t,
			}
		} else if c, ok := s.channels[targetCf]; ok {
			// TAGMSG to channelCf
			s.evts <- ChannelTagEvent{
				Nick:    nick,
				Channel: c.Name,
				Typing:  typing,
				Time:    t,
			}
		}
	case "BATCH":
		batchStart := msg.Params[0][0] == '+'
		id := msg.Params[0][1:]

		if batchStart && msg.Params[1] == "chathistory" {
			s.chBatches[id] = HistoryEvent{Target: msg.Params[2]}
		} else if b, ok := s.chBatches[id]; ok {
			s.evts <- b
			delete(s.chBatches, id)
		}
	case "NICK":
		nick, _, _ := FullMask(msg.Prefix)
		nickCf := strings.ToLower(nick)
		newNick := msg.Params[0]
		newNickCf := strings.ToLower(newNick)

		t, ok := msg.Time()
		if !ok {
			t = time.Now()
		}

		if nickCf == s.nickCf {
			s.evts <- SelfNickEvent{
				FormerNick: s.nick,
				NewNick:    newNick,
				Time:       t,
			}
			s.nick = newNick
			s.nickCf = newNickCf
		} else {
			s.evts <- UserNickEvent{
				FormerNick: nick,
				NewNick:    newNick,
				Time:       t,
			}
			// TODO update state
		}
	case "FAIL":
		fmt.Println("FAIL", msg.Params)
	case "PING":
		err = s.send("PONG :%s\r\n", msg.Params[0])
		if err != nil {
			return
		}
	case "ERROR":
		err = errors.New("connection terminated")
		if len(msg.Params) > 0 {
			err = fmt.Errorf("connection terminated: %s", msg.Params[0])
		}
		_ = s.conn.Close()
	default:
	}

	return
}

func (s *Session) privmsgToEvent(msg Message) (ev Event) {
	nick, _, _ := FullMask(msg.Prefix)
	targetCf := strings.ToLower(msg.Params[0])

	t, ok := msg.Time()
	if !ok {
		t = time.Now()
	}

	if !s.IsChannel(targetCf) {
		// PRIVMSG to self
		ev = QueryMessageEvent{
			Nick:    nick,
			Command: msg.Command,
			Content: msg.Params[1],
			Time:    t,
		}
	} else if c, ok := s.channels[targetCf]; ok {
		// PRIVMSG to channel
		ev = ChannelMessageEvent{
			Nick:    nick,
			Channel: c.Name,
			Command: msg.Command,
			Content: msg.Params[1],
			Time:    t,
		}
	}

	return
}

func (s *Session) updateFeatures(features []string) {
	for _, f := range features {
		if f == "" || f == "-" || f == "=" || f == "-=" {
			continue
		}

		var (
			add   bool
			key   string
			value string
		)

		if strings.HasPrefix(f, "-") {
			add = false
			f = f[1:]
		} else {
			add = true
		}

		kv := strings.SplitN(f, "=", 2)
		key = strings.ToUpper(kv[0])
		if len(kv) > 1 {
			value = kv[1]
		}

		if add {
			s.features[key] = value
		} else {
			delete(s.features, key)
		}
	}
}

func (s *Session) send(format string, args ...interface{}) (err error) {
	msg := fmt.Sprintf(format, args...)
	_, err = s.conn.Write([]byte(msg))

	if s.debug {
		for _, line := range strings.Split(msg, "\r\n") {
			if line != "" {
				s.evts <- RawMessageEvent{
					Message:  line,
					Outgoing: true,
				}
			}
		}
	}

	return
}
