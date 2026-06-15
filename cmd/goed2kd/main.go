package main

import (
	"fmt"
	"os"

	"github.com/XiaoYouChR/Python-eD2k/internal/daemon"
)

func main() {
	if err := daemon.New(os.Stdin, os.Stdout).Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
