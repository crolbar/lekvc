package main

import (
	"fmt"
	"net"
	"sync"

	p "github.com/crolbar/lekvc/lekvcs/protocol"
)

type Client struct {
	conn net.Conn
	ch   chan []byte
}

var (
	clients = make(map[Client]bool)
	mu      sync.Mutex
)

func handleAudioSend(c *Client, payload []byte) {
	// Forward packet to all other clients
	mu.Lock()
	for other := range clients {
		// don't playback to sender
		if other.conn == c.conn {
			continue
		}

		select {
		case other.ch <- payload:
		default:
		}
	}
	mu.Unlock()
}

func readLoop(c *Client) {
	defer func() {
		mu.Lock()
		delete(clients, *c)
		mu.Unlock()
		c.conn.Close()
	}()

	buf := make([]byte, p.MsgHeaderSize)
	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			return
		}

		if n != p.MsgHeaderSize {
			panic("no header size package")
		}

		payloadSize := p.GetPayloadSize(buf)
		payload := p.GetPayload(buf, payloadSize)
		fmt.Println("payload: ", string(payload))

		clientNameSize := p.GetClientNameSize(buf, payloadSize)
		clientName := p.GetClientName(buf, payloadSize, clientNameSize)
		fmt.Println("client name: ", string(clientName))


	}
}

func writeLoop(c *Client) {
	for {
		select {
		case data, ok := <-c.ch:
			if !ok {
				c.ch = nil
				continue
			}
			c.conn.Write(data)
		}

		if c.ch == nil {
			break
		}
	}
}

func main() {
	ln, err := net.Listen("tcp", "0.0.0.0:9000")
	if err != nil {
		panic(err)
	}
	fmt.Println("Server listening on :9000")

	for {
		conn, err := ln.Accept()
		if err != nil {
			continue
		}
		mu.Lock()

		c := Client{
			conn: conn,
			ch:   make(chan []byte, 50), // Increased buffer size
		}

		clients[c] = true
		mu.Unlock()
		go writeLoop(&c)
		go readLoop(&c)
	}
}
