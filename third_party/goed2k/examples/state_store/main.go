package main

import (
	"log"

	"github.com/monkeyWie/goed2k"
)

type memoryStore struct {
	state *goed2k.ClientState
}

func (m *memoryStore) Load() (*goed2k.ClientState, error) {
	return m.state, nil
}

func (m *memoryStore) Save(state *goed2k.ClientState) error {
	m.state = state
	return nil
}

func main() {
	client := goed2k.NewClient(goed2k.NewSettings())
	client.SetStateStore(&memoryStore{})

	if err := client.Start(); err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	if _, _, err := client.AddLink(
		"ed2k://|file|example-state.epub|456789|FEDCBA9876543210FEDCBA9876543210|/",
		"./downloads",
	); err != nil {
		log.Fatal(err)
	}

	if err := client.SaveState(""); err != nil {
		log.Fatal(err)
	}
}
