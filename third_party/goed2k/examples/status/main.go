package main

import (
	"fmt"
	"log"
	"time"

	"github.com/monkeyWie/goed2k"
)

func main() {
	client := goed2k.NewClient(goed2k.NewSettings())
	if err := client.Start(); err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	if err := client.ConnectServers("176.123.5.89:4725"); err != nil {
		log.Fatal(err)
	}

	if _, _, err := client.AddLink(
		"ed2k://|file|example-status.mp3|12345678|0123456789ABCDEF0123456789ABCDEF|/",
		"./downloads",
	); err != nil {
		log.Fatal(err)
	}

	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()

	for range ticker.C {
		status := client.Status()
		for _, transfer := range status.Transfers {
			donePct := 0.0
			recvPct := 0.0
			if transfer.Status.TotalWanted > 0 {
				donePct = float64(transfer.Status.TotalDone) * 100 / float64(transfer.Status.TotalWanted)
				recvPct = float64(transfer.Status.TotalReceived) * 100 / float64(transfer.Status.TotalWanted)
			}
			fmt.Printf("%s done=%.2f%% recv=%.2f%% peers=%d active=%d rate=%dB/s\n",
				transfer.FileName,
				donePct,
				recvPct,
				transfer.Status.NumPeers,
				transfer.ActivePeers,
				transfer.Status.DownloadRate)
		}
	}
}
