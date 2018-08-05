package irc_go

import (
	"fmt"
)

type Room struct {
	Members  map[int]*Peer
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

func (r *Room) AddMember(peer *Peer) {
	r.Members[peer.Key] = peer
}

func (r *Room) RemoveMember(s *Server, name string, peer *Peer) {
	delete(r.Members, peer.Key)

	if len(r.Members) == 0 {
		delete(s.Rooms, name)
	}
}
