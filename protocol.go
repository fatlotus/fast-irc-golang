package irc_go

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"strings"
)

func (p *Peer) Write(msg string) {
	// allow tests to work
	if p.Conn == nil {
		return
	}
	if p.Server.Trace != nil {
		fmt.Fprintf(p.Server.Trace, "S -> %d  %s\n", p.Key, msg[:len(msg)-2])
	}

	p.Output.Write([]byte(msg))
}

func (p *Peer) SayFrom(source, format string, args ...interface{}) {
	p.Write(":" + source + fmt.Sprintf(" "+format+"\r\n", args...))
}

func (p *Peer) Say(format string, args ...interface{}) {
	p.Write(fmt.Sprintf(":s "+format+"\r\n", args...))
}

func (p *Peer) HandleLine(line []byte) (done bool) {
	if p.Server.Trace != nil {
		p.Server.TraceLock.Lock()

		// Report any updates to the motd file to the log.
		if p.Server.MessageOfTheDayPath != "" {
			buf, err := ioutil.ReadFile(p.Server.MessageOfTheDayPath)
			if err == nil {
				if p.Server.LastMotd != string(buf) {
					fmt.Fprintf(p.Server.Trace, "S motd  %s\n",
						strings.Replace(string(buf), "\n", "$", -1))
					p.Server.LastMotd = string(buf)
				}
			}
		}

		fmt.Fprintf(p.Server.Trace, "S <- %d  %s\n", p.Key, line)

		defer p.Server.TraceLock.Unlock()
	}

	if len(line) >= 495 {
		line = line[:495]
	}

	chunks := strings.SplitN(strings.TrimSpace(string(line)), ":", 2)
	message := ""
	if len(chunks) > 1 {
		message = chunks[1]
	}

	raw := strings.Split(chunks[0], " ")
	words := make([]string, 0, len(raw))
	for _, word := range raw {
		if word != "" {
			words = append(words, word)
		}
	}
	if len(words) == 0 {
		return false
	}

	if err := p.Route(words[0], words[1:], message); err != nil {
		p.Say("%s", err.Error())
		if _, ok := err.(*Quitting); ok {
			return true
		}
	}

	return false
}

func (p *Peer) HandleInput() {
	defer p.Conn.Close()
	defer p.Output.Close()
	defer p.Server.RemovePeer(p)

	sc := bufio.NewScanner(p.Conn)
	sc.Split(bufio.ScanLines)
	for sc.Scan() {
		if p.HandleLine(sc.Bytes()) {
			return
		}
	}
	if sc.Err() != nil {
		p.Server.Quit(p, sc.Err().Error())
	} else {
		p.Server.Quit(p, "dropped connection")
	}
}
