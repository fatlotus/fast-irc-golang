package main

import (
	"flag"
	"fmt"
	"log"
	"os"

	"net/http"
	_ "net/http/pprof"

	. "github.com/fatlotus/irc_go"
)

var port = flag.Int("p", 6667, "which port to bind on")
var password = flag.String("o", "", "operator password")
var prof = flag.Bool("prof", false, "whether to start a profiler port")
var trace = flag.String("t", "", "path to trace file")
var motd = flag.String("m", "motd.txt", "message of the day file")

func main() {
	flag.Parse()

	if *prof {
		go func() {
			log.Println(http.ListenAndServe("localhost:6060", nil))
		}()
	}

	server := NewServer()

	if *trace != "" {
		fp, err := os.Create(*trace)
		if err != nil {
			log.Fatal(err)
		}
		defer fp.Close()
		server.Trace = fp
	}

	server.Password = *password
	server.MessageOfTheDayPath = *motd

	err := server.ListenAndServe(fmt.Sprintf(":%d", *port))
	if err != nil {
		log.Fatal(err)
	}
}
