package irc_go

import (
	"fmt"
	"io"
	"net"
	"sync"
)

type Server struct {
	Peers map[int]*Peer
	Nicks map[string]*Peer

	Rooms map[string]*Room

	UserCount   int
	NextPeerKey int

	// NOTE: When tracing, the server effectively runs in a single-threaded
	// mode, so that we can induce a request ordering where all responses come
	// immediately after the request that generated it.
	TraceLock sync.Mutex
	Trace     io.Writer
	LastMotd  string

	Password            string
	MessageOfTheDayPath string

	Listener net.Listener

	sync.Mutex
}

func IsModerator(peer *Peer, room string) bool {
	if peer.IsGlobalOperator {
		return true
	}
	for _, r := range peer.IsModOf {
		if r == room {
			return true
		}
	}
	return false
}

func (s *Server) AddPeer(n net.Conn) *Peer {
	s.Lock()
	defer s.Unlock()
	p := &Peer{
		Conn:   n,
		Key:    s.NextPeerKey,
		Server: s,
		Output: make(chan string, 100000),
	}
	s.Peers[s.NextPeerKey] = p
	s.NextPeerKey += 1
	return p
}

func (s *Server) RegisteredUser(p *Peer) {
	s.Lock()
	defer s.Unlock()

	s.UserCount += 1
}

func (s *Server) Quit(p *Peer, message string) error {
	s.Lock()
	defer s.Unlock()

	if message == "" {
		message = "Client Quit"
	}
	// fixme: remove copy
	toremove := map[string]*Room{}
	for name, room := range s.Rooms {
		if room.ContainsMember(p) {
			toremove[name] = room
		}
	}
	for name, room := range toremove {
		for _, member := range room.Members {
			if member != p {
				member.SayFrom(p.Nick+"!u@h", "QUIT :%s", message)
			}
		}
		room.RemoveMember(s, name, p)
	}

	return &Quitting{message}
}

func (s *Server) RemovePeer(p *Peer) {
	s.Lock()
	defer s.Unlock()
	delete(s.Peers, p.Key)
	if p.Nick != "" {
		delete(s.Nicks, p.Nick)
	}
	if p.SentWelcome {
		s.UserCount -= 1
	}
}

func (s *Server) SetAway(p *Peer, away string) error {
	s.Lock()
	defer s.Unlock()

	p.Away = away
	return nil
}

func (s *Server) SetNick(p *Peer, nick string) error {
	s.Lock()
	defer s.Unlock()
	if s.Nicks[nick] != nil {
		return &NickAlreadyInUse{nick}
	}

	for _, room := range s.Rooms {
		if room.ContainsMember(p) {
			for _, member := range room.Members {
				member.SayFrom(p.Nick+"!u@h", "NICK :%s", nick)
			}
		}
	}

	if p.Nick != "" {
		delete(s.Nicks, p.Nick)
	}
	p.Nick = nick
	s.Nicks[p.Nick] = p

	return nil
}

func (s *Server) Whois(sender *Peer, nick string) error {
	s.Lock()
	defer s.Unlock()

	subject, ok := s.Nicks[nick]
	if !ok {
		return &NoSuchUser{sender.Nick, nick}
	}

	sender.Say("311 %s 1 2 3 4 :%s", sender.NickOrAsterix(), subject.FullName)

	channels := ""
	for channel, room := range s.Rooms {
		if room.ContainsMember(subject) {
			ismod := false
			isvoice := false
			for _, modroom := range subject.IsModOf {
				if modroom == channel {
					ismod = true
				}
			}
			for _, speaker := range room.Speakers {
				if speaker == subject {
					isvoice = true
				}
			}
			if ismod {
				channels += "@" + channel + " "
			} else if isvoice {
				channels += "+" + channel + " "
			} else {
				channels += channel + " "
			}
		}
	}

	if len(channels) > 0 {
		sender.Say("319 %s 1 :%s", sender.Nick, channels)
	}
	sender.Say("312 %s 1 2 3", sender.Nick)

	if subject.Away != "" {
		sender.Say("301 %s %s :%s", sender.Nick, nick, subject.Away)
	}
	if subject.IsGlobalOperator {
		sender.Say("313 %s %s :is an IRC operator", sender.Nick, nick)
	}

	sender.Say("318 %s 1 :End of WHOIS list", sender.Nick)

	return nil
}

func (s *Server) SetTopic(sender *Peer, channel, topic string) error {
	s.Lock()
	defer s.Unlock()

	room, exists := s.Rooms[channel]
	if !exists {
		return &NotOnChannel{sender.Nick, channel}
	}

	if !room.ContainsMember(sender) {
		return &NotOnChannel{sender.Nick, channel}
	}

	if !IsModerator(sender, channel) && room.IsFixedTopic {
		return &NotOperator{sender.Nick, channel}
	}

	room.Topic = topic
	for _, member := range room.Members {
		member.SayFrom(sender.Nick+"!u@h", "TOPIC %s :%s", channel, topic)
	}

	return nil
}

func (s *Server) GetTopic(sender *Peer, channel string) (string, error) {
	s.Lock()
	defer s.Unlock()

	room, exists := s.Rooms[channel]
	if !exists {
		return "", &NotOnChannel{sender.Nick, channel}
	}

	if !room.ContainsMember(sender) {
		return "", &NotOnChannel{sender.Nick, channel}
	}

	return room.Topic, nil
}

func (s *Server) Part(sender *Peer, name, message string) error {
	s.Lock()
	defer s.Unlock()

	room, exists := s.Rooms[name]
	if !exists {
		return &NoSuchChannel{sender.Nick, name}
	}

	if message == "" {
		for _, member := range room.Members {
			member.SayFrom(sender.Nick+"!u@h", "PART %s", name)
		}
	} else {
		for _, member := range room.Members {
			member.SayFrom(sender.Nick+"!u@h", "PART %s :%s", name, message)
		}
	}

	if !room.ContainsMember(sender) {
		return &NotOnChannel{sender.Nick, name}
	}
	room.RemoveMember(s, name, sender)
	return nil
}

func (s *Server) SendAllNames(sender *Peer) error {
	s.Lock()
	defer s.Unlock()

	leftover := map[int]bool{}
	for key := range s.Peers {
		leftover[key] = true
	}

	for name, room := range s.Rooms {
		s.sendNames(sender, name, room, &leftover)
	}
	if len(leftover) != 0 {
		users := ""
		for key := range leftover {
			users += " " + s.Peers[key].Nick
		}
		sender.Say("353 %s * * :%s", sender.Nick, users[1:])
	}
	sender.Say("366 %s * 3", sender.Nick)
	return nil
}

func (s *Server) sendNames(sender *Peer, name string, room *Room, leftover *map[int]bool) {
	members := ""
	for _, member := range room.Members {
		if leftover != nil {
			delete(*leftover, member.Key)
		}

		mod := false
		for _, room := range member.IsModOf {
			if room == name {
				mod = true
			}
		}
		voice := false
		for _, speaker := range room.Speakers {
			if speaker == member {
				voice = true
			}
		}
		if mod {
			members += " @" + member.Nick
		} else if voice {
			members += " +" + member.Nick
		} else {
			members += " " + member.Nick
		}
	}
	sender.Say("353 %s = %s :%s", sender.Nick, name, members[1:])
}

func (s *Server) ListChannels(sender *Peer, channel string) error {
	return nil
}

func (s *Server) ListAllChannels(sender *Peer) error {
	s.Lock()
	defer s.Unlock()

	for name, room := range s.Rooms {
		if room.Topic != "" {
			sender.Say("322 %s %s %d :%s", sender.Nick, name, len(room.Members), room.Topic)
		} else {
			sender.Say("322 %s %s %d :No topic set", sender.Nick, name, len(room.Members))
		}
	}
	sender.Say("323 %s :End of LIST", sender.Nick)

	return nil
}

func (s *Server) Who(sender *Peer, channel string) error {
	s.Lock()
	defer s.Unlock()

	room, exists := s.Rooms[channel]
	if !exists {
		return &NoSuchUser{sender.Nick, channel}
	}

	for _, member := range room.Members {
		mod := false
		for _, room := range member.IsModOf {
			if room == channel {
				mod = true
			}
		}
		voice := false
		for _, speaker := range room.Speakers {
			if speaker == member {
				voice = true
			}
		}

		flags := ""
		if member.Away == "" {
			flags += "H"
		} else {
			flags += "G"
		}
		if member.IsGlobalOperator {
			flags += "*"
		}
		if mod {
			flags += "@"
		} else if voice {
			flags += "+"
		}

		sender.Say(
			"352 %s %s 2 3 4 %s %s 7", sender.Nick, channel, member.Nick, flags)
	}
	sender.Say("315 %s %s :End of WHO list", sender.Nick, channel)
	return nil
}

func (s *Server) WhoAll(sender *Peer) error {
	s.Lock()
	defer s.Unlock()

	for _, member := range s.Peers {
		mutual := false
		for _, room := range s.Rooms {
			if room.ContainsMember(sender) && room.ContainsMember(member) {
				mutual = true
				break
			}
		}
		away := "H"
		if member.Away != "" {
			away = "G"
		}
		if !mutual {
			sender.Say("352 %s * 2 3 4 %s %s 7", sender.Nick, member.Nick, away)
		}
	}
	sender.Say("315 %s * :End of WHO list", sender.Nick)

	return nil
}

func (s *Server) SendNames(sender *Peer, name string) error {
	s.Lock()
	defer s.Unlock()

	room, exists := s.Rooms[name]
	if exists {
		s.sendNames(sender, name, room, nil)
	}

	sender.Say("366 %s %s 3", sender.Nick, name)

	return nil
}

func (s *Server) Join(sender *Peer, name string) error {
	s.Lock()
	defer s.Unlock()

	room, exists := s.Rooms[name]
	if !exists {
		room = &Room{Members: map[int]*Peer{}}
		s.Rooms[name] = room
		sender.IsModOf = append(sender.IsModOf, name)
	}
	if room.ContainsMember(sender) {
		return nil
	}
	room.AddMember(sender)

	for _, member := range room.Members {
		member.SayFrom(sender.Nick+"!u@h", "JOIN %s", name)
	}
	if room.Topic != "" {
		sender.Say("332 %s %s :%s", sender.Nick, name, room.Topic)
	}
	s.sendNames(sender, name, room, nil)
	sender.Say("366 %s %s 3", sender.Nick, name)

	return nil
}

func (s *Server) SendMessage(cmd string, sender *Peer, nick, message string) error {
	msg := fmt.Sprintf(":%s!%s@c %s %s :%s\r\n", sender.Nick, sender.User, cmd, nick, message)

	s.Lock()
	defer s.Unlock()

	if nick[0] == '#' {
		room, exists := s.Rooms[nick]
		if !exists {
			return &NoSuchUser{sender.Nick, nick}
		}

		// Make sure the user has joined the channel already.
		if !room.ContainsMember(sender) {
			return &CannotSendToChannel{sender.Nick, nick}
		}

		voice := false
		for _, member := range room.Speakers {
			if member == sender {
				voice = true
			}
		}

		if room.IsModerated && !IsModerator(sender, nick) && !voice {
			return &CannotSendToChannel{sender.Nick, nick}
		}

		for _, member := range room.Members {
			if member != sender {
				member.Write(msg)
			}
		}
	} else {
		peer, ok := s.Nicks[nick]
		if !ok {
			if cmd == "NOTICE" {
				return nil
			}
			return &NoSuchUser{sender.Nick, nick}
		}

		if peer.Away != "" {
			return &PeerIsAway{sender.Nick, nick, peer.Away}
		}
		peer.Write(msg)
	}
	return nil
}

func (s *Server) SetMode(sender *Peer, subject, mode string) error {
	s.Lock()
	defer s.Unlock()

	if subject[0] == '#' {
		room, ok := s.Rooms[subject]
		if !ok {
			return &NoSuchChannel{sender.Nick, subject}
		}

		if !IsModerator(sender, subject) {
			return &NotOperator{sender.Nick, subject}
		}

		if len(mode) != 2 {
			return &UnknownChannelMode{sender.Nick, subject, '?'}
		}

		enable := mode[0] == '+'
		switch mode[1] {
		case 'm':
			room.IsModerated = enable
		case 't':
			room.IsFixedTopic = enable
		default:
			return &UnknownChannelMode{sender.Nick, subject, mode[1]}
		}

		for _, member := range room.Members {
			member.SayFrom(sender.Nick+"!u@h", "MODE %s %s", subject, mode)
		}
		return nil
	} else {
		if subject != sender.Nick {
			return &CannotChangeForOtherUser{sender.Nick}
		}

		enable := mode[0] == '+'
		switch mode[1] {
		case 'o':
			if enable {
				return nil
			}
		case 'a':
			return nil
		default:
			return &UnknownUserMode{sender.Nick}
		}

		sender.SayFrom(sender.Nick, "MODE %s :%s", subject, mode)
	}
	return nil
}

func (s *Server) GetMode(sender *Peer, subject string) error {
	s.Lock()
	defer s.Unlock()

	room, ok := s.Rooms[subject]
	if !ok {
		return &NoSuchChannel{sender.Nick, subject}
	}

	mode := "+"
	if room.IsModerated {
		mode = mode + "m"
	}
	if room.IsFixedTopic {
		mode = mode + "t"
	}

	sender.Say("324 %s %s %s", sender.Nick, subject, mode)

	return nil
}

func (s *Server) SetMembershipMode(sender *Peer, channel, mode, subject string) error {
	s.Lock()
	defer s.Unlock()

	room, ok := s.Rooms[channel]
	if !ok {
		return &NoSuchChannel{sender.Nick, channel}
	}

	if !IsModerator(sender, channel) {
		return &NotOperator{sender.Nick, channel}
	}

	subjectuser := (*Peer)(nil)
	for _, member := range room.Members {
		if member.Nick == subject {
			subjectuser = member
			break
		}
	}

	if subjectuser == nil {
		return &SubjectNotOnChannel{sender.Nick, channel, subject}
	}

	enable := mode[0] == '+'
	switch mode[1] {
	case 'v':
		for i, speaker := range room.Speakers {
			if speaker == subjectuser {
				room.Speakers = append(room.Speakers[:i], room.Speakers[i+1:]...)
				break
			}
		}

		if enable {
			room.Speakers = append(room.Speakers, subjectuser)
		}
	case 'o':
		for i, mod_chan := range subjectuser.IsModOf {
			if mod_chan == channel {
				subjectuser.IsModOf = append(subjectuser.IsModOf[:i], subjectuser.IsModOf[i+1:]...)
				break
			}
		}

		if enable {
			subjectuser.IsModOf = append(subjectuser.IsModOf, channel)
		}
	default:
		return &UnknownChannelMode{sender.Nick, channel, mode[1]}
	}

	for _, member := range room.Members {
		member.SayFrom(sender.Nick+"!u@h", "MODE %s %s %s", channel, mode, subject)
	}

	return nil
}

func (s *Server) NumOps() int {
	s.Lock()
	defer s.Unlock()

	count := 0
	for _, user := range s.Peers {
		if user.IsGlobalOperator {
			count++
		}
	}
	return count
}

func (s *Server) NumUsers() int {
	s.Lock()
	defer s.Unlock()

	return s.UserCount
}

func (s *Server) NumClients() int {
	s.Lock()
	defer s.Unlock()

	return len(s.Peers)
}

func (s *Server) Listen(addr string) error {
	ln, err := net.Listen("tcp", addr)
	s.Listener = ln
	return err
}

func (s *Server) Serve() error {
	for {
		conn, err := s.Listener.Accept()
		if err != nil {
			return err
		}
		conn.(*net.TCPConn).SetNoDelay(false)
		peer := s.AddPeer(conn)
		go peer.HandleInput()
	}
}

func (s *Server) ListenAndServe(addr string) error {
	err := s.Listen(addr)
	if err != nil {
		return err
	}
	return s.Serve()
}

func NewServer() *Server {
	return &Server{
		Peers: map[int]*Peer{},
		Nicks: map[string]*Peer{},
		Rooms: map[string]*Room{},
	}
}
