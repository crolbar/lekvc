package main

import (
	"fmt"
	"net"
	"sync"
)

type Client struct {
	conn net.Conn
	ch   chan []byte
}

var (
	clients = make(map[Client]bool)
	mu      sync.Mutex
)

func handleClient(c *Client) {
	defer func() {
		mu.Lock()
		delete(clients, *c)
		mu.Unlock()
		c.conn.Close()
	}()

	// scanner := bufio.NewScanner(conn)
	// for scanner.Scan() {
	// 	msg := scanner.Text()
	// 	mu.Lock()
	// 	for c := range clients {
	// 		if c != conn {
	// 			fmt.Fprintln(c, msg)
	// 		}
	// 	}
	// 	mu.Unlock()
	// }

	buf := make([]byte, 4096)
	for {
		n, err := c.conn.Read(buf)
		if err != nil {
			return
		}
		
		// Forward packet to all other clients
		mu.Lock()
		for other := range clients {
			// don't playback to sender
			if other.conn == c.conn {
				continue
			}

			// Non-blocking send to prevent blocking on slow clients
			chunk := make([]byte, n)
			copy(chunk, buf[:n])
			
			select {
			case other.ch <- chunk:
			default:
				// Drop packet if client buffer is full
			}
		}
		mu.Unlock()
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
		go handleClient(&c)
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
