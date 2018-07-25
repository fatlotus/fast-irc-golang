package irc_go

import (
	"bufio"
	"fmt"
	"log"
	"time"
)

func (p *Peer) HandleFlushes() {
	for {
		select {
		case p.Output <- "":
		default:
			// fmt.Printf("stopping flusher thread\n")
			return
		}
		time.Sleep(1 * time.Millisecond)
	}
}

func (p *Peer) Write(msg string) {
	// allow tests to work
	if p.Conn == nil {
		return
	}
	p.Conn.Write([]byte(msg))
	return

	if p.Server.Trace != nil {
		fmt.Fprintf(p.Server.Trace, "S -> %d  %s\n", p.Key, msg[:len(msg)-2])
		p.Conn.Write([]byte(msg))
		return
	}

	select {
	case p.Output <- msg:
	default:
		p.Conn.Close()
		fmt.Printf("closing connection due to backpressure\n")
	}
}

func (p *Peer) SayFrom(source, format string, args ...interface{}) {
	p.Write(":" + source + fmt.Sprintf(" "+format+"\r\n", args...))
}

func (p *Peer) Say(format string, args ...interface{}) {
	p.Write(fmt.Sprintf(":s "+format+"\r\n", args...))
}

func (p *Peer) HandleOutput() {
	w := bufio.NewWriter(p.Conn)
	for msg := range p.Output {
		if msg == "" {
			if w.Buffered() > 0 {
				if err := w.Flush(); err != nil {
					fmt.Printf("flush: %s\n", err)
					return
				}
			}
		} else {
			_, err := w.Write([]byte(msg))
			if err != nil {
				fmt.Printf("write: %s\n", err)
				return
			}
		}
	}
	w.Flush()
	p.Conn.Close()
}

func (p *Peer) HandleInput() {
	defer func() {
		// this is almost certainly wrong
		cha := p.Output
		p.Output = nil
		close(cha)
	}()
	defer p.Server.RemovePeer(p)

	sc := bufio.NewScanner(p.Conn)
	sc.Split(bufio.ScanLines)
	for sc.Scan() {
		line := sc.Bytes()

		if p.Server.Trace != nil {
			p.Server.TraceLock.Lock()
			fmt.Fprintf(p.Server.Trace, "S <- %d  %s\n", p.Key, line)
		}

		if len(line) >= 495 {
			line = line[:495]
		}

		cmd, next := line, []byte(nil)
		any := false
		st := 0
		for i, c := range line {
			if c == ' ' {
				if !any {
					st++
				} else {
					cmd, next = line[st:i], line[i+1:]
					break
				}
			} else {
				any = true
			}
		}
		if !any {
			continue
		}

		message := ""
		words := []string{}
		s := 0
		for i, c := range next {
			if c == ':' {
				if s < i {
					words = append(words, string(next[s:i]))
				}
				message = string(next[i+1:])
				s = len(next)
				break
			} else if c == ' ' {
				if s < i {
					words = append(words, string(next[s:i]))
				}
				s = i + 1
			}
		}
		if s < len(next) {
			words = append(words, string(next[s:]))
		}

		// old impl:
		// chunks := strings.SplitN(strings.TrimSpace(line), ":", 2)
		// message := ""
		// if len(chunks) > 1 {
		// 	message = chunks[1]
		// }
		//
		// raw := strings.Split(chunks[0], " ")
		// words := make([]string, 0, len(raw))
		// for _, word := range raw {
		// 	if word != "" {
		// 		words = append(words, word)
		// 	}
		// }
		// if len(words) == 0 {
		// 	continue
		// }

		if err := p.Route(string(cmd), words, message); err != nil {
			p.Say("%s", err.Error())
			if _, ok := err.(*Quitting); ok {
				break
			}
		}

		if p.Server.Trace != nil {
			p.Server.TraceLock.Unlock()
		}
	}
	if sc.Err() != nil {
		log.Print(sc.Err())
	}
}
