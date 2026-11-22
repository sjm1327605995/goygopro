package tenon

import "testing"

func TestEventBusUnsubscribeOutOfOrder(t *testing.T) {
	bus := NewEventBus()
	calls := []string{}
	offA := bus.On("event", func(Event) { calls = append(calls, "a") })
	bus.On("event", func(Event) { calls = append(calls, "b") })
	offC := bus.On("event", func(Event) { calls = append(calls, "c") })
	offA()
	offC()
	offC()
	bus.mu.RLock()
	listeners := append([]*eventListener(nil), bus.listeners["event"]...)
	bus.mu.RUnlock()
	if len(listeners) != 1 {
		t.Fatalf("remaining listeners = %d, want 1", len(listeners))
	}
	for _, listener := range listeners {
		listener.fn(Event{})
	}
	if len(calls) != 1 || calls[0] != "b" {
		t.Fatalf("remaining listener calls = %v", calls)
	}
}

func TestEventBusNilListenerIsSafe(t *testing.T) {
	bus := NewEventBus()
	off := bus.On("event", nil)
	off()
	bus.Emit("event", nil)
	if len(bus.listeners["event"]) != 0 {
		t.Fatal("nil listener was registered")
	}
}
