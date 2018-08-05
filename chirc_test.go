package irc_go_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"os"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/fatlotus/irc_go"

	"github.com/andreyvit/diff"
)

func NormalizeLine(message string) (string, string) {
	wrapper := strings.SplitN(message, " ", 5)

	// client to server messages
	if len(wrapper) < 2 || wrapper[1] != "->" {
		return "", message
	}
	unpacked := strings.SplitN(wrapper[4], " ", 2)

	// relayed message
	if strings.Contains(unpacked[0], "@") {
		return "", message
	}

	// parse out the opcode if it is a server to client message
	phrases := strings.SplitN(unpacked[1], ":", 2)
	words := strings.Split(phrases[0], " ")

	if words[0] == "319" || words[0] == "353" {
		members := strings.Split(" ", phrases[1])
		sort.Strings(members)
		return words[0], fmt.Sprintf(
			"S -> %s  %s %s:%s",
			wrapper[2], unpacked[0], phrases[0], strings.Join(members, " "))
	}

	return words[0], message
}

var canReorderResponses = map[string]bool{
	"322": true, // RPL_LIST
	"352": true, // RPL_WHOREPLY
	"353": true, // RPL_NAMEREPLY
}

func NormalizeTestCase(lines []string) []string {
	result := []string{}
	buffer := []string{}
	lastop := ""
	for _, line := range lines {
		op, line := NormalizeLine(line)
		if op != lastop || op == "" {
			if canReorderResponses[lastop] {
				sort.Strings(buffer)
			}
			result = append(result, buffer...)
			buffer = buffer[:0]
			lastop = op
		}
		buffer = append(buffer, line)
	}
	if canReorderResponses[lastop] {
		sort.Strings(buffer)
	}
	return append(result, buffer...)
}

func RunTestCase(motd string, addr string, inputs []string) ([]string, error) {
	result := []string{}
	conns := []net.Conn{}

	for i, line := range inputs {
		frags := strings.SplitN(line, " ", 5)
		if len(frags) < 3 {
			continue
		}
		if frags[1] == "motd" {
			buf := []byte(strings.Replace(frags[3], "$", "\n", -1))
			if err := ioutil.WriteFile(motd, buf, 0600); err != nil {
				return nil, err
			}
			result = append(result, line)
			continue
		} else if frags[1] != "<-" {
			continue
		}

		// look up the client to send the request from
		cid, _ := strconv.Atoi(frags[2])
		for cid >= len(conns) {
			conn, err := net.Dial("tcp", addr)
			if err != nil {
				return nil, err
			}
			conns = append(conns, conn)
		}

		// look up the order of subsequent messages
		subsequent := []int{}
		for j := i + 1; j < len(inputs); j++ {
			frags := strings.SplitN(inputs[j], " ", 5)
			if len(frags) < 3 {
				continue
			}
			if frags[1] != "->" {
				break
			}
			cid, _ := strconv.Atoi(frags[2])
			subsequent = append(subsequent, cid)
		}

		// write the message over the socket
		result = append(result, fmt.Sprintf("S <- %d  %s", cid, frags[4]))
		fmt.Fprintf(conns[cid], "%s\r\n", frags[4])

		// read the responses from the server (in parallel)
		wait := sync.WaitGroup{}
		wait.Add(len(conns))

		subs := make([][]string, len(conns))
		for cid, conn := range conns {
			go func(conn net.Conn, cid int) {
				conn.SetReadDeadline(time.Now().Add(10 * time.Millisecond))
				buf, err := ioutil.ReadAll(conn)

				nerr, ok := err.(net.Error)
				if err != nil && (!ok || !nerr.Timeout()) {
					fmt.Printf("Failed read: %v\n", err)
				} else {
					subs[cid] = strings.Split(string(buf), "\r\n")
				}
				wait.Done()
			}(conn, cid)
		}
		wait.Wait()

		// Try to read the inputs in test order first
		for _, cid := range subsequent {
			for len(subs[cid]) > 0 && subs[cid][0] == "" {
				subs[cid] = subs[cid][1:]
			}
			if len(subs[cid]) > 0 {
				result = append(result,
					fmt.Sprintf("S -> %d  %s", cid, subs[cid][0]))
				subs[cid] = subs[cid][1:]
			}
		}

		// Then read everything left over
		for cid := range conns {
			for _, line := range subs[cid] {
				if line != "" {
					result = append(result,
						fmt.Sprintf("S -> %d  %s", cid, line))
				}
			}
		}
	}
	for _, conn := range conns {
		conn.Close()
	}
	return result, nil
}

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

	actual, err := RunTestCase(tmpdir+"/motd.txt", addr, expected)
	if err != nil {
		return err
	}

	CheckForServerLeaks(s, t)

	actual = NormalizeTestCase(actual)
	expected = NormalizeTestCase(expected)

	if len(actual) != len(expected) {
		t.Fail()
	} else {
		passed := true
		for i, actual_line := range actual {
			if actual_line != expected[i] {
				t.Fail()
				passed = false
			}
		}
		if passed {
			return nil
		}
	}

	fmt.Printf("\nFAIL %s:\n", path)
	for _, line := range diff.LineDiffAsLines(
		strings.Join(actual, "\n"), strings.TrimSpace(string(text))) {
		if line[0] == '+' {
			fmt.Printf("\033[32m%s\033[0m\n", line)
		} else if line[0] == '-' {
			fmt.Printf("\033[31m%s\033[0m\n", line)
		} else {
			fmt.Printf("%s\n", line)
		}
	}
	fmt.Printf("\n")
	return nil
}

func TestServer(t *testing.T) {
	tests, err := ioutil.ReadDir("tests/")
	if err != nil {
		t.Fatal(err)
	}
	for _, test := range tests {
		name := test.Name()
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			RunTestFile("tests/"+name, t)
		})
	}
}
