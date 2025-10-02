package main

import "fmt"

func handleStatus() {
	fmt.Printf("\x1b[34mConnected clients:\x1b[m\n[id]  [name]\n")
	for _, c := range clients {
		fmt.Printf("%d     \x1b[38;5;%dm%s\x1b[m\n", c.id, generateClientColorFromID(c.id), c.name)
	}
}

func handleHelp() {
	for cstr, c := range commands {
		fmt.Printf("%s => %s\n", cstr, c.desc)
	}
}

type cmd struct {
	f    func()
	desc string
}

// var c cmd = cmd{
// 	f: func() {},
// 	desc: "",
// }

var commands map[string]cmd

func InitCommands() {
	commands = map[string]cmd{
		"/s": cmd{f: handleStatus, desc: "connected clients"},

		"/h": cmd{
			f:    handleHelp,
			desc: "show help menu",
		},
	}
}
