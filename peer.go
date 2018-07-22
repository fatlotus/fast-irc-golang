package irc_go

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"os"
)

type Peer struct {
	Conn   net.Conn
	Output chan string

	Key    int
	Server *Server

	Nick     string
	User     string
	FullName string
	Away     string

	IsModOf          []string
	IsGlobalOperator bool

	SentWelcome bool
}

func (p *Peer) NickOrAsterix() string {
	if p.Nick == "" {
		return "*"
	} else {
		return p.Nick
	}
}

func (p *Peer) SendMotd() {
	motd, err := os.Open("motd.txt")
	if err != nil {
		p.Say("422 %s :MOTD File is missing", p.Nick)
		return
	}
	defer motd.Close()

	p.Say("375 %s :- Today's Message of the day - ", p.Nick)

	sc := bufio.NewScanner(motd)
	sc.Split(bufio.ScanLines)
	for sc.Scan() {
		fmt.Printf("line: %s\n", sc.Text())
		p.Say("372 %s :- %s", p.Nick, sc.Text())
	}
	p.Say("376 %s :End of MOTD command", p.Nick)
	if sc.Err() != nil {
		log.Print(sc.Err())
	}
	return
}

func (p *Peer) Route(cmd string, args []string, message string) error {
	switch cmd {
	case "NICK":
		if len(args) == 0 {
			return &NoNickSpecified{}
		}
		if err := p.Server.SetNick(p, args[0]); err != nil {
			return err
		}
		p.MaybeSendWelcome()
		return nil
	case "USER":
		if len(args) < 3 || message == "" {
			return fmt.Errorf("461 %s USER :Not enough parameters",
				p.NickOrAsterix())
		}
		p.User = args[0]
		p.FullName = message
		p.MaybeSendWelcome()
		return nil
	case "MOTD":
		p.SendMotd()
		return nil
	case "PRIVMSG", "NOTICE":
		if p.Nick != "" && p.User != "" {
			err := error(nil)
			if len(args) == 0 {
				err = &NoRecipient{p.NickOrAsterix()}
			} else if message == "" {
				err = &NoMessage{p.NickOrAsterix()}
			} else {
				err = p.Server.SendMessage(cmd, p, args[0], message)
			}
			if cmd == "PRIVMSG" && err != nil {
				return err
			}
			return nil
		} else {
			return &NotRegistered{p.NickOrAsterix()}
		}
	case "AWAY":
		if p.Nick != "" && p.User != "" {
			p.Server.SetAway(p, message)
			if message != "" {
				p.Say("306 %s :You have been marked as being away", p.Nick)
			} else {
				p.Say("305 %s :You are no longer marked as being away", p.Nick)
			}
		}
	case "JOIN":
		if p.Nick != "" && p.User != "" {
			if len(args) == 1 {
				return p.Server.Join(p, args[0])
			} else {
				return &NeedsMoreParams{p.Nick, "JOIN"}
			}
		}
	case "PART":
		if p.Nick != "" && p.User != "" {
			if len(args) == 1 {
				return p.Server.Part(p, args[0], message)
			} else {
				return &NeedsMoreParams{p.Nick, "PART"}
			}
		}
	case "NAMES":
		if p.Nick != "" && p.User != "" {
			if len(args) == 1 {
				return p.Server.SendNames(p, args[0])
			} else {
				return p.Server.SendAllNames(p)
			}
		}
	case "LIST":
		if p.Nick != "" && p.User != "" {
			if len(args) == 1 {
				return p.Server.ListChannels(p, args[0])
			} else {
				return p.Server.ListAllChannels(p)
			}
		}
	case "WHO":
		if p.Nick != "" && p.User != "" {
			if len(args) == 1 {
				if args[0] == "*" {
					return p.Server.WhoAll(p)
				} else {
					return p.Server.Who(p, args[0])
				}
			} else {
				return &NeedsMoreParams{p.Nick, "WHO"}
			}
		}
	case "MODE":
		if p.Nick != "" && p.User != "" {
			if len(args) == 3 {
				return p.Server.SetMembershipMode(p, args[0], args[1], args[2])
			} else if len(args) == 2 {
				return p.Server.SetMode(p, args[0], args[1])
			} else if len(args) == 1 {
				return p.Server.GetMode(p, args[0])
			} else {
				return &NeedsMoreParams{p.Nick, "MODE"}
			}
		}
		return nil
	case "TOPIC":
		if p.Nick != "" && p.User != "" {
			if len(args) == 1 {
				if message != "" {
					return p.Server.SetTopic(p, args[0], message)
				} else {
					topic, err := p.Server.GetTopic(p, args[0])
					if err != nil {
						return err
					}
					if topic != "" {
						p.Say("332 %s %s :%s", p.Nick, args[0], topic)
					} else {
						p.Say("331 %s %s :%s", p.Nick, args[0],
							"No topic is set")
					}
					return nil
				}
			} else {
				return &NeedsMoreParams{p.Nick, "TOPIC"}
			}
		}
	case "PING":
		p.Say("PONG %s", p.NickOrAsterix())
	case "PONG":
		return nil
	case "LUSERS":
		p.SendUserList()
	case "OPER":
		if p.Nick != "" && p.User != "" {
			if len(args) == 2 {
				if p.Server.Password == args[1] {
					p.Say("381 %s :You are now an IRC operator", p.Nick)
					p.IsGlobalOperator = true
				} else {
					return &IncorrectPassword{p.Nick}
				}
			} else {
				return &NeedsMoreParams{p.Nick, "OPER"}
			}
		}
	case "WHOIS":
		if len(args) == 0 {
			return nil
		}
		return p.Server.Whois(p, args[0])
	case "QUIT":
		return p.Server.Quit(p, message)
	default:
		if p.SentWelcome {
			return &UnknownCommand{p.NickOrAsterix(), cmd}
		}
	}

	return nil
}

func (p *Peer) SendUserList() {
	users := p.Server.NumUsers()
	clients := p.Server.NumClients()
	ops := p.Server.NumOps()

	p.Say("251 %s :There are %d users and 0 services on 1 servers",
		p.Nick, users)
	p.Say("252 %s %d :operator(s) online", p.Nick, ops)
	p.Say("253 %s %d :unknown connection(s)", p.Nick, clients-users)
	p.Say("254 %s %d :channels formed", p.Nick, len(p.Server.Rooms))
	p.Say("255 %s :I have %d clients and 0 servers", p.Nick, clients)
}

func (p *Peer) MaybeSendWelcome() {
	if p.Nick != "" && p.User != "" && !p.SentWelcome {
		p.SentWelcome = true
		p.Server.RegisteredUser(p)

		p.Say("001 %s :Welcome to the Internet Relay Network %s!%s@foo",
			p.Nick, p.Nick, p.User)
		p.Say("002 %s :TBD", p.Nick)
		p.Say("003 %s :TBD", p.Nick)
		p.Say("004 %s 1 2 3 4", p.Nick)

		p.SendUserList()
		p.SendMotd()
	}
}
