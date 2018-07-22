package irc_go

import (
	"fmt"
)

type NickAlreadyInUse struct {
	Nick string
}

func (n NickAlreadyInUse) Error() string {
	return fmt.Sprintf("433 * %s :Nickname is already in use", n.Nick)
}

type NoNickSpecified struct{}

func (n NoNickSpecified) Error() string {
	return fmt.Sprintf("431 * :No nickname given")
}

type NotRegistered struct {
	Nick string
}

func (n NotRegistered) Error() string {
	return fmt.Sprintf("451 %s :You have not registered", n.Nick)
}

type NoSuchUser struct {
	Sender string
	Nick   string
}

func (n NoSuchUser) Error() string {
	return fmt.Sprintf("401 %s %s :No such nick/channel", n.Sender, n.Nick)
}

type NoSuchChannel struct {
	Sender  string
	Channel string
}

func (n NoSuchChannel) Error() string {
	return fmt.Sprintf("403 %s %s :No such channel", n.Sender, n.Channel)
}

type NotOperator struct {
	Sender  string
	Channel string
}

func (n NotOperator) Error() string {
	return fmt.Sprintf("482 %s %s :You're not channel operator", n.Sender, n.Channel)
}

type NotOnChannel struct {
	Sender  string
	Channel string
}

func (n NotOnChannel) Error() string {
	return fmt.Sprintf("442 %s %s :You're not on that channel", n.Sender, n.Channel)
}

type CannotChangeForOtherUser struct {
	Sender string
}

func (c CannotChangeForOtherUser) Error() string {
	return fmt.Sprintf("502 %s :Cannot change mode for other users", c.Sender)
}

type UnknownChannelMode struct {
	Sender  string
	Channel string
	Mode    byte
}

func (n UnknownChannelMode) Error() string {
	return fmt.Sprintf("472 %s %c :is unknown mode char to me for %s", n.Sender, n.Mode, n.Channel)
}

type UnknownUserMode struct {
	Sender string
}

func (n UnknownUserMode) Error() string {
	return fmt.Sprintf("501 %s :Unknown MODE flag", n.Sender)
}

type PeerIsAway struct {
	Sender      string
	Peer        string
	AwayMessage string
}

func (p PeerIsAway) Error() string {
	return fmt.Sprintf("301 %s %s :%s", p.Sender, p.Peer, p.AwayMessage)
}

type CannotSendToChannel struct {
	Sender  string
	Channel string
}

func (c CannotSendToChannel) Error() string {
	return fmt.Sprintf("404 %s %s :Cannot send to channel", c.Sender, c.Channel)
}

type NoRecipient struct {
	Sender string
}

func (n NoRecipient) Error() string {
	return fmt.Sprintf("411 %s :No recipient given (PRIVMSG)", n.Sender)
}

type SubjectNotOnChannel struct {
	Sender  string
	Channel string
	Member  string
}

func (n SubjectNotOnChannel) Error() string {
	return fmt.Sprintf("441 %s %s %s :They aren't on that channel", n.Sender, n.Member, n.Channel)
}

type NoMessage struct {
	Sender string
}

func (n NoMessage) Error() string {
	return fmt.Sprintf("412 %s :No text to send", n.Sender)
}

type UnknownCommand struct {
	Sender  string
	Command string
}

func (u UnknownCommand) Error() string {
	return fmt.Sprintf("421 %s %s :Unknown command", u.Sender, u.Command)
}

type Quitting struct {
	Reason string
}

func (e Quitting) Error() string {
	return fmt.Sprintf("ERROR :Closing Link: user said (%s)", e.Reason)
}

type NeedsMoreParams struct {
	Sender  string
	Command string
}

func (n NeedsMoreParams) Error() string {
	return fmt.Sprintf("461 %s %s :Not enough parameters", n.Sender, n.Command)
}

type IncorrectPassword struct {
	Sender string
}

func (i IncorrectPassword) Error() string {
	return fmt.Sprintf("464 %s :Password incorrect", i.Sender)
}
