package websocket

import (
	"sync"
	"testing"
	"time"
)

// mockConn implements a minimal interface or we can test Hub registration directly.
func TestHub_RegisterAndUnregister(t *testing.T) {
	hub := NewHub()

	c1 := &Client{hub: hub, send: make(chan []byte, 10), windowNumber: 1}
	c2 := &Client{hub: hub, send: make(chan []byte, 10), windowNumber: 2}
	c3 := &Client{hub: hub, send: make(chan []byte, 10), windowNumber: 1}

	hub.Register(c1)
	hub.Register(c2)
	hub.Register(c3)

	if hub.ClientCount() != 3 {
		t.Fatalf("expected 3 clients, got %d", hub.ClientCount())
	}
	if hub.WindowClientCount(1) != 2 {
		t.Fatalf("expected 2 clients for window 1, got %d", hub.WindowClientCount(1))
	}
	if hub.WindowClientCount(2) != 1 {
		t.Fatalf("expected 1 client for window 2, got %d", hub.WindowClientCount(2))
	}
	if hub.WindowClientCount(3) != 0 {
		t.Fatalf("expected 0 clients for window 3, got %d", hub.WindowClientCount(3))
	}

	// Unregister c1
	hub.Unregister(c1)
	if hub.ClientCount() != 2 {
		t.Fatalf("expected 2 clients after unregister, got %d", hub.ClientCount())
	}
	if hub.WindowClientCount(1) != 1 {
		t.Fatalf("expected 1 client for window 1, got %d", hub.WindowClientCount(1))
	}

	// Change c2 window to window 1
	hub.RegisterWindow(c2, 1)
	if hub.WindowClientCount(1) != 2 {
		t.Fatalf("expected 2 clients for window 1 after re-assignment, got %d", hub.WindowClientCount(1))
	}
	if hub.WindowClientCount(2) != 0 {
		t.Fatalf("expected 0 clients for window 2, got %d", hub.WindowClientCount(2))
	}
}

func TestHub_BroadcastAndTargeted(t *testing.T) {
	hub := NewHub()

	c1 := &Client{hub: hub, send: make(chan []byte, 10), windowNumber: 1}
	c2 := &Client{hub: hub, send: make(chan []byte, 10), windowNumber: 2}

	hub.Register(c1)
	hub.Register(c2)

	// Global broadcast
	env := NewEnvelope(EventSyncStarted, map[string]string{"media": "M1"})
	hub.Broadcast(env)

	select {
	case msg := <-c1.send:
		if len(msg) == 0 {
			t.Fatal("expected non-empty message in c1")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for broadcast in c1")
	}

	select {
	case msg := <-c2.send:
		if len(msg) == 0 {
			t.Fatal("expected non-empty message in c2")
		}
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for broadcast in c2")
	}

	// Targeted window broadcast (Window 1 only)
	targetedEnv := NewEnvelope(EventPlaylistUpdated, map[string]int{"window": 1})
	hub.BroadcastToWindow(1, targetedEnv)

	select {
	case <-c1.send:
		// expected
	case <-time.After(100 * time.Millisecond):
		t.Fatal("timed out waiting for targeted message in c1")
	}

	// c2 should NOT have received the targeted message
	select {
	case <-c2.send:
		t.Fatal("c2 should NOT have received targeted message for window 1")
	case <-time.After(50 * time.Millisecond):
		// expected
	}
}

func TestHub_ConcurrentAccess(t *testing.T) {
	hub := NewHub()
	var wg sync.WaitGroup

	numGoroutines := 20
	clientsPerGoroutine := 10

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(gID int) {
			defer wg.Done()
			for j := 0; j < clientsPerGoroutine; j++ {
				wNum := (gID % 4) + 1
				client := &Client{hub: hub, send: make(chan []byte, 10), windowNumber: wNum}
				hub.Register(client)
				hub.Broadcast(NewEnvelope(EventPong, nil))
				hub.BroadcastToWindow(wNum, NewEnvelope(EventPlaylistUpdated, nil))
				hub.RegisterWindow(client, (wNum%4)+1)
				hub.Unregister(client)
			}
		}(i)
	}

	wg.Wait()
}
