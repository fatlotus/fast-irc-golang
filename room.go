package irc_go

import (
	"fmt"
)

type Room struct {
	Members  []*Peer
	Speakers []*Peer
	Topic    string

	IsModerated  bool
	IsFixedTopic bool
}

func (r *Room) SendMessage(cmd string, sender *Peer, nick, message string) error {
	msg := fmt.Sprintf(":%s!%s@c %s %s :%s\r\n", sender.Nick, sender.User, cmd, nick, message)
	for _, member := range r.Members {
		member.Write(msg)
	}
	return nil
}
