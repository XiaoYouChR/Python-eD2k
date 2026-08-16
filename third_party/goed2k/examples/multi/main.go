package main

import (
	"log"

	"github.com/monkeyWie/goed2k"
)

func main() {
	client := goed2k.NewClient(goed2k.NewSettings())
	if err := client.Start(); err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	if err := client.ConnectServers("176.123.5.89:4725", "45.82.80.155:5687"); err != nil {
		log.Fatal(err)
	}

	if _, _, err := client.AddLink(
		"ed2k://|file|example-a.mp3|12345678|0123456789ABCDEF0123456789ABCDEF|/",
		"./music",
	); err != nil {
		log.Fatal(err)
	}

	if _, _, err := client.AddLink(
		"ed2k://|file|example-b.epub|456789|FEDCBA9876543210FEDCBA9876543210|/",
		"./books",
	); err != nil {
		log.Fatal(err)
	}

	if err := client.Wait(); err != nil && err != goed2k.ErrClientStopped {
		log.Fatal(err)
	}
}
