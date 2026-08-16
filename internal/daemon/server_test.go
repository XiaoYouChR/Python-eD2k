package daemon

import (
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestConnectServersBestEffortContinuesAfterFailure(t *testing.T) {
	var attempted []string
	err := connectServersBestEffort([]string{"dead:1", "live:2", "later:3"}, func(address string) error {
		attempted = append(attempted, address)
		if address == "dead:1" {
			return errors.New("timed out")
		}
		return nil
	})

	if err != nil {
		t.Fatalf("connectServersBestEffort() error = %v", err)
	}
	if want := []string{"dead:1", "live:2", "later:3"}; !reflect.DeepEqual(attempted, want) {
		t.Fatalf("attempted = %v, want %v", attempted, want)
	}
}

func TestConnectServersBestEffortFailsWhenEveryServerFails(t *testing.T) {
	err := connectServersBestEffort([]string{"first:1", "second:2"}, func(address string) error {
		return errors.New("timed out")
	})

	if err == nil {
		t.Fatal("connectServersBestEffort() error = nil")
	}
	for _, address := range []string{"first:1", "second:2"} {
		if !strings.Contains(err.Error(), address) {
			t.Fatalf("error %q does not contain %q", err, address)
		}
	}
}

func TestConnectServersBestEffortRejectsAnEmptyList(t *testing.T) {
	err := connectServersBestEffort(nil, func(string) error { return nil })
	if err == nil || !strings.Contains(err.Error(), "no server address") {
		t.Fatalf("connectServersBestEffort() error = %v", err)
	}
}
