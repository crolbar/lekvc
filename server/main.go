package main

import (
	"fmt"
	"io"
	"net"
	"sync"

	p "github.com/crolbar/lekvc/lekvcs/protocol"
)

const Address = "0.0.0.0:9000"

type Client struct {
	id   ClientID
	name string
	conn net.Conn
	ch   chan p.Msg
}

type ClientID = uint8

var (
	clients = make(map[ClientID]*Client)
	mu      sync.Mutex

	nextClientID ClientID = 1
)

func (c *Client) sendToOthers(msg p.Msg) {
	mu.Lock()
	for _, other := range clients {
		if other.id == c.id {
			continue
		}

		select {
		case other.ch <- msg:
		default:
		}
	}
	mu.Unlock()
}

func (c *Client) handleRecivedAudio(samples []byte) {
	msg := p.NewMsg(
		p.Audio,
		c.id,
		samples,
		c.name,
	)

	c.sendToOthers(msg)
}

func (c *Client) handleRecivedText(text []byte) {
	msg := p.NewMsg(
		p.Audio,
		c.id,
		text,
		c.name,
	)

	c.sendToOthers(msg)
}

func (c *Client) readLoop() {
	defer func() {
		mu.Lock()
		delete(clients, c.id)
		mu.Unlock()
		c.conn.Close()
		close(c.ch)
	}()

	for {
		msg, err := p.ReadMsg(c.conn)
		if err == io.EOF {
			c.notifyClientLeave()
			break
		}
		if err != nil {
			panic(err)
		}

		switch msg.Type {
		case p.Audio:
			c.handleRecivedAudio(msg.Payload)
		case p.Text:
			c.handleRecivedText(msg.Payload)

			// case p.InitClient:
			// case p.ClientJoin:
			// case p.ClientLeave:
		}
	}
}

func (c *Client) writeLoop() {
	for msg := range c.ch {
		data, _ := p.EncodeMsg(msg)
		c.conn.Write(data)
	}
}

func (c *Client) notifyClientJoin() {
	joinMsg := fmt.Sprintf("CLIENT %s(%s) CONNECTED", c.name, c.conn.RemoteAddr().String())

	fmt.Printf("\x1b[34m%s\x1b[m", joinMsg)

	c.sendToOthers(p.NewMsgP(
		p.ClientJoin,
		c.id,
		[]byte(joinMsg),
	))
}

func (c *Client) notifyClientLeave() {
	discMsg := fmt.Sprintf("CLIENT %s(%s) DISCONNECTED", c.name, c.conn.RemoteAddr().String())

	fmt.Printf("\x1b[31m%s\x1b[m", discMsg)

	c.sendToOthers(p.NewMsgP(
		p.ClientLeave,
		c.id,
		[]byte(discMsg),
	))
}

func handleInitClient(conn net.Conn) {
	msg, err := p.ReadMsg(conn)
	if err != nil {
		conn.Close()
		panic(err)
	}

	// first msg should always be init client
	if msg.Type != p.InitClient {
		conn.Close()
		return
	}
	// magic value for id in initClient is 0
	if msg.ID != 0 {
		conn.Close()
		return
	}

	var name string
	mu.Lock()
	if msg.ClientNameSize > 0 {
		name = msg.ClientName
	} else {
		name = fmt.Sprintf("Client%d", nextClientID)
	}

	c := Client{
		id:   nextClientID,
		name: name,
		conn: conn,
		ch:   make(chan p.Msg, 50),
	}
	clients[nextClientID] = &c
	nextClientID += 1 // can get to 0
	mu.Unlock()

	// send back id and name to the client
	{
		data, err := p.EncodeMsg(
			p.NewMsgNP(
				p.InitClient,
				c.id,
				c.name,
			),
		)
		if err != nil {
			panic(err)
		}

		conn.Write(data)
	}

	c.notifyClientJoin()
	go c.writeLoop()
	go c.readLoop()
}

func main() {
	ln, err := net.Listen("tcp", Address)
	if err != nil {
		panic(err)
	}
	fmt.Println("Server listening on " + Address)

	for {
		conn, err := ln.Accept()
		if err != nil {
			fmt.Println("\x1b[31m" + "err: " + err.Error() + "\x1b[m")
			continue
		}

		go handleInitClient(conn)
	}
}
