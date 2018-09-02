package irc_go_test

import (
	"fmt"
	"sync"
	"testing"

	. "github.com/fatlotus/fast-irc-golang"
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

	streams := 10
	wg := sync.WaitGroup{}
	for s := 0; s < streams; s++ {
		wg.Add(1)
		go func(s int) {
			defer wg.Done()

			name_a := fmt.Sprintf("a%d", s)
			name_b := fmt.Sprintf("b%d", s)

			client_a, err := NewClient(name_a, addr)
			if err != nil {
				b.Fatal(err)
			}
			client_b, err := NewClient(name_b, addr)
			if err != nil {
				b.Fatal(err)
			}

			client_b.PrivMsg(name_a, "x")
			client_b.Writer.Flush()
			client_a.ReadMsg()

			batch := 200
			for i := 0; i < b.N/streams; i += batch {
				for i := 0; i < batch; i++ {
					client_a.PrivMsg(name_b, "hi")
				}
				client_a.Writer.Flush()
				for i := 0; i < batch; i++ {
					client_b.ReadMsg()
				}
			}

			client_a.Close()
			client_b.Close()
		}(s)
	}

	wg.Wait()
}

func BenchmarkHighFanout(b *testing.B) {
	s := NewServer()

	err := s.Listen("localhost:0")
	if err != nil {
		b.Fatal(err)
	}
	defer s.Listener.Close()

	addr := s.Listener.Addr().String()
	go s.Serve()

	sender, err := NewClient("sender", addr)
	if err != nil {
		b.Fatal(err)
	}

	sender.Join("#group")

	receivers := []*Client{}
	fanout := 100
	for i := 0; i < fanout; i++ {
		receiver, err := NewClient(fmt.Sprintf("receiver%d", i), addr)
		if err != nil {
			b.Fatal(err)
		}
		receiver.Join("#group")
		receivers = append(receivers, receiver)
	}

	for i := 0; i < b.N/fanout; i++ {
		sender.PrivMsg("#group", "hello world")
		sender.Writer.Flush()
		for _, reciever := range receivers {
			reciever.ReadMsg()
		}
	}
}
