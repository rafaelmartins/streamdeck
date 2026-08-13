package streamdeck

import (
	"testing"
	"time"
)

func TestWaitForReleaseUsesMatchingPress(t *testing.T) {
	in := newInputs(&Device{}, 1, 0)[0]
	keys := make(chan *Key, 2)
	in.key.addHandler(func(_ *Device, key *Key) error {
		keys <- key
		return nil
	})

	firstPress := time.Unix(100, 0)
	in.press(firstPress, nil)
	firstKey := <-keys
	in.release(firstPress.Add(time.Second))

	secondPress := firstPress.Add(2 * time.Second)
	in.press(secondPress, nil)
	secondKey := <-keys

	if got := firstKey.WaitForRelease(); got != time.Second {
		t.Fatalf("first press duration: got %s, want %s", got, time.Second)
	}

	select {
	case <-secondKey.input.channel:
		t.Fatal("second press was released before its release event")
	default:
	}

	in.release(secondPress.Add(3 * time.Second))
	if got := secondKey.WaitForRelease(); got != 3*time.Second {
		t.Fatalf("second press duration: got %s, want %s", got, 3*time.Second)
	}
}
