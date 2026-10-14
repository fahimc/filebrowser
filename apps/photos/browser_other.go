//go:build !windows

package main

import (
	"os/exec"
	"runtime"
)

func openBrowser(address string) {
	command := "xdg-open"
	if runtime.GOOS == "darwin" {
		command = "open"
	}
	_ = exec.Command(command, address).Start()
}
