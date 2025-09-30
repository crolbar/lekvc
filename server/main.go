package main

import (
	"fmt"
	"net"
	"sync"
)

var (
	clients = make(map[net.Conn]bool)
	mu      sync.Mutex
)

func handleClient(conn net.Conn) {
	defer func() {
		mu.Lock()
		delete(clients, conn)
		mu.Unlock()
		conn.Close()
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

	buf := make([]byte, 2048)
	for {
		n, err := conn.Read(buf)
		if err != nil {
			return
		}
		mu.Lock()
		for c := range clients {
			// don't playback
			if c == conn {
				continue
			}

			c.Write(buf[:n])

			fmt.Println("wrote n: ", n)
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
		clients[conn] = true
		mu.Unlock()
		go handleClient(conn)
	}
}
