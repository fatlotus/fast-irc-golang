package irc_go_test

import (
	"io/ioutil"
	"os"
	"strings"
	"testing"
	"time"

	. "github.com/fatlotus/fast-irc-golang"
	"github.com/fatlotus/fast-irc-golang/testutil"
)

func CheckForServerLeaks(s *Server, t *testing.T) {
	// make sure we didn't leave and dangling references
	time.Sleep(10 * time.Millisecond)
	if len(s.Peers) != 0 {
		t.Errorf("leaked peers: %#v\n", s.Peers)
	}
	if len(s.Nicks) != 0 {
		t.Errorf("leaked nicks: %#v\n", s.Nicks)
	}
	if len(s.Rooms) != 0 {
		t.Errorf("leaked rooms: %#v\n", s.Rooms)
	}
}

func RunTestFile(path string, t *testing.T) error {
	// create a temporary motd file
	tmpdir, err := ioutil.TempDir("", "motd")
	if err != nil {
		t.Fatal(err)
	}

	defer os.RemoveAll(tmpdir)

	// set up a local instance of the server
	s := NewServer()
	s.Password = "foobar"
	s.MessageOfTheDayPath = tmpdir + "/motd.txt"
	if err := s.Listen(":0"); err != nil {
		t.Fatal(err)
	}
	defer s.Listener.Close()

	addr := s.Listener.Addr().String()
	go s.Serve()

	// read the fixture file and start running the trace
	text, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	expected := strings.Split(string(text[:len(text)-1]), "\n")

	err = testutil.DiffTestCase(tmpdir+"/motd.txt", addr, expected)
	s.Listener.Close()
	CheckForServerLeaks(s, t)

	return err
}

func TestServer(t *testing.T) {
	tests, err := ioutil.ReadDir("tests/")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		name := test.Name()
		if name[0] == '_' {
			continue
		}
		t.Run(name, func(t *testing.T) {
			err := RunTestFile("tests/"+name, t)
			if err != nil {
				t.Error(err)
			}
		})
	}
}
