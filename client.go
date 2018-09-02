package irc_go

import (
	"bufio"
	"fmt"
	"net"
	"strings"
)

type Client struct {
	Conn   net.Conn
	Reader *bufio.Reader
	Writer *bufio.Writer
}

func (c *Client) PrivMsg(nick, message string) {
	fmt.Fprintf(c.Writer, "PRIVMSG %s :%s\r\n", nick, message)
}

func (c *Client) Join(channel string) {
	fmt.Fprintf(c.Writer, "JOIN %s\r\n", channel)
	c.Writer.Flush()
	for {
		line, err := c.Reader.ReadString('\n')
		if err != nil {
			panic(err)
		}
		if strings.Contains(line, "JOIN") {
			return
		}
	}
}

func (c *Client) ReadMsg() {
	for {
		line, err := c.Reader.ReadString('\n')
		if err != nil {
			panic(err)
		}
		if strings.Contains(line, "PRIVMSG") {
			return
		}
	}
}

func (c *Client) Close() {
	c.Conn.Close()
}

func NewClient(nick, addr string) (*Client, error) {
	conn, err := net.Dial("tcp", addr)
	if err != nil {
		return nil, err
	}
	conn.(*net.TCPConn).SetNoDelay(true)
	fmt.Fprintf(conn, "NICK %s\r\nUSER %s * * :%s\r\n", nick, nick, nick)
	return &Client{
		Conn:   conn,
		Reader: bufio.NewReader(conn),
		Writer: bufio.NewWriter(conn),
	}, nil
}
