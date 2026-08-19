package main

import (
	"fmt"
	"os"

	"github.com/XiaoYouChR/Python-eD2k/internal/daemon"
)

var version = "dev"

func main() {
	if len(os.Args) > 1 && os.Args[1] == "--version" {
		fmt.Println(version)
		return
	}
	if err := daemon.New(os.Stdin, os.Stdout).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
