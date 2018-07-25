package irc_go_test

import (
	// "io/ioutil"
	// "fmt"
	"testing"

	. "github.com/fatlotus/irc_go"
)

func must(b *testing.B, err error) {
	if err != nil {
		b.Fatal(err)
	}
}

func BenchmarkPrivmsgJustHandlingTheRequest(b *testing.B) {
	s := NewServer()
	client_a := s.AddPeer(nil)
	client_b := s.AddPeer(nil)
	must(b, s.SetNick(client_a, "a"))
	must(b, s.SetNick(client_b, "b"))

	for i := 0; i < b.N; i++ {
		must(b, s.SendMessage("PRIVMSG", client_a, "b", "hi"))
	}
}

func BenchmarkPrivmsgMessageDispatch(b *testing.B) {
	s := NewServer()
	client_a := s.AddPeer(nil)
	client_b := s.AddPeer(nil)

	must(b, client_a.Route("USER", []string{"*", "*", "a"}, "a"))
	must(b, client_a.Route("NICK", []string{"a"}, ""))

	must(b, client_b.Route("USER", []string{"*", "*", "b"}, "b"))
	must(b, client_b.Route("NICK", []string{"b"}, ""))

	args := []string{"b"}
	message := "hi"
	for i := 0; i < b.N; i++ {
		must(b, client_a.Route("PRIVMSG", args, message))
	}
}

func BenchmarkPrivmsgLocalSocket(b *testing.B) {
	s := NewServer()
	err := s.Listen("localhost:0")
	if err != nil {
		b.Fatal(err)
	}
	defer s.Listener.Close()

	addr := s.Listener.Addr().String()
	go s.Serve()

	client_a, err := NewClient("a", addr)
	if err != nil {
		b.Fatal(err)
	}
	client_b, err := NewClient("b", addr)
	if err != nil {
		b.Fatal(err)
	}

	client_b.PrivMsg("a", "x")
	client_b.Writer.Flush()
	client_a.ReadMsg()

	qd := 200
	for i := 0; i < b.N; i += qd {
		for i := 0; i < qd; i++ {
			client_a.PrivMsg("b", "hi")
		}
		client_a.Writer.Flush()
		for i := 0; i < qd; i++ {
			client_b.ReadMsg()
		}
	}

	client_a.Close()
	client_b.Close()
}
