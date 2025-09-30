//go:build windows
// +build windows

package main

import (
	"golang.org/x/sys/windows"
	"os"
)

func enableANSI() {
	h := windows.Handle(os.Stdout.Fd())
	var mode uint32
	windows.GetConsoleMode(h, &mode)
	mode |= windows.ENABLE_VIRTUAL_TERMINAL_PROCESSING
	windows.SetConsoleMode(h, mode)
}
