package main

import (
	"bufio"
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

	scanner := bufio.NewScanner(conn)
	for scanner.Scan() {
		msg := scanner.Text()
		mu.Lock()
		for c := range clients {
			if c != conn {
				fmt.Fprintln(c, msg)
			}
		}
		mu.Unlock()
	}
}

func main() {
	ln, err := net.Listen("tcp", ":9000")
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
