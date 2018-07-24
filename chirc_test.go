package irc_go_test

import (
	"fmt"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	. "github.com/fatlotus/irc_go"

	"github.com/andreyvit/diff"
)

func RunTestCase(addr string, inputs []string) ([]string, error) {
	result := []string{}
	conns := []net.Conn{}

	for i, line := range inputs {
		frags := strings.SplitN(line, " ", 5)
		if len(frags) < 3 {
			continue
		}
		if frags[1] != "<-" {
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
		fmt.Printf(".")
		result = append(result, fmt.Sprintf("S <- %d  %s", cid, frags[4]))
		fmt.Fprintf(conns[cid], "%s\r\n", frags[4])

		// read the responses from the server (in parallel)
		wait := sync.WaitGroup{}
		wait.Add(len(conns))

		subs := make([][]string, len(conns))
		for cid, conn := range conns {
			go func(conn net.Conn, cid int) {
				conn.SetReadDeadline(time.Now().Add(5 * time.Millisecond))
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
	return result, nil
}

func RunTestFile(path string, t *testing.T) error {
	// set up a local instance of the server
	s := NewServer()
	s.Password = "foobar"
	err := s.Listen(":0")
	if err != nil {
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
	expected := strings.Split(string(text), "\n")

	actual, err := RunTestCase(addr, expected)
	if err != nil {
		return err
	}

	passed := true
	for i, actual_line := range actual {
		if actual_line != expected[i] {
			t.Fail()
			passed = false
		}
	}

	if passed {
		fmt.Printf(":")
		return nil
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
		RunTestFile("tests/"+test.Name(), t)
	}
}
