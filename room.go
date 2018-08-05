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

func (r *Room) ContainsMember(peer *Peer) bool {
	for _, member := range r.Members {
		if member == peer {
			return true
		}
	}
	return false
}

func (r *Room) RemoveMember(s *Server, name string, peer *Peer) {
	for i, member := range r.Members {
		if member == peer {
			r.Members = append(r.Members[:i], r.Members[i+1:]...)
			break
		}
	}

	if len(r.Members) == 0 {
		delete(s.Rooms, name)
	}
}
