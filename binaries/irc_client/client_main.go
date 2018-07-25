package main

import (
	"fmt"
	. "github.com/fatlotus/irc_go"
	"time"
)

func main() {
	for attempt := 0; attempt < 5; attempt++ {
		a, err := NewClient("a", "localhost:6667")
		if err != nil {
			panic(err)
		}
		b, err := NewClient("b", "localhost:6667")
		if err != nil {
			panic(err)
		}

		sz := 512
		buf := "x"
		for len(buf) < sz {
			buf += buf
		}

		// drain the buffers from A?
		b.PrivMsg("a", buf)
		b.Writer.Flush()
		a.ReadMsg()

		c := 50000 // 00
		// qd := 4096 * 2
		st := time.Now()
		go func() {
			for i := 0; i < c; i++ {
				a.PrivMsg("b", buf)
			}
			a.Writer.Flush()
		}()
		for i := 0; i < c; i++ {
			b.ReadMsg()
		}

		ed := time.Now()
		fmt.Printf("Throughput: %.1f kQPS / %.1f MiB/s\n",
			float64(c)/ed.Sub(st).Seconds()/1000,
			float64(c*sz)/ed.Sub(st).Seconds()/1024/1024)

		a.Close()
		b.Close()
	}
}
