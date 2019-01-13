package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"regexp"
	"runtime"
	"strings"

	"github.com/fatlotus/fast-irc-golang/testutil"
)

var match = regexp.MustCompile("Listening on (.*)")

func RunTestCaseOnce(path, cmd string, args []string) error {
	child := exec.Command(cmd, args...)
	stdout, err := child.StdoutPipe()
	if err != nil {
		return err
	}

	if err := child.Start(); err != nil {
		return err
	}

	text, err := ioutil.ReadFile(path)
	if err != nil {
		return err
	}
	expected := strings.Split(string(text[:len(text)-1]), "\n")

	scanner := bufio.NewScanner(stdout)
	for scanner.Scan() {
		match := match.FindStringSubmatch(scanner.Text())
		if len(match) > 0 {
			err := testutil.DiffTestCase("/tmp/motd.txt", match[1], expected)
			if err != nil {
				return err
			}
			break
		}
		fmt.Printf("%s\n", scanner.Text())
	}

	if err := child.Process.Kill(); err != nil {
		return err
	}

	child.Wait()
	for scanner.Scan() {
		fmt.Printf("%s\n", scanner.Text())
	}
	return nil
}

func main() {
	dir := "tests"
	if _, file, _, ok := runtime.Caller(0); ok {
		dir = path.Clean(path.Join(file, "..", "..", "..", "tests"))
	}
	tests, err := ioutil.ReadDir(dir)
	if err != nil {
		fmt.Printf("%s\n", err)
		return
	}
	failures := 0
	for _, test := range tests {
		if test.Name()[0] == '_' {
			continue
		}
		err := RunTestCaseOnce(path.Join(dir, test.Name()), os.Args[1], os.Args[2:])
		if err != nil {
			fmt.Printf("\n%s:\n%s\n", test.Name(), err)
			if _, ok := err.(*testutil.DiffError); !ok {
				// if there's a a test harness issue, we might as well end early
				break
			}
			failures += 1
			if failures > 5 {
				break
			}
		} else {
			fmt.Printf(".")
		}
	}
}
