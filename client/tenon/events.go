package tenon

import (
	"sync"

	"github.com/sjm1327605995/tenon/pkg/ui"
)

type Event struct {
	Name string
	Data any
}

type eventListener struct{ fn func(Event) }

type EventBus struct {
	mu        sync.RWMutex
	listeners map[string][]*eventListener
}

func NewEventBus() *EventBus {
	return &EventBus{listeners: make(map[string][]*eventListener)}
}

func (b *EventBus) On(name string, listener func(Event)) func() {
	if listener == nil {
		return func() {}
	}
	entry := &eventListener{fn: listener}
	b.mu.Lock()
	b.listeners[name] = append(b.listeners[name], entry)
	b.mu.Unlock()
	return func() {
		b.mu.Lock()
		items := b.listeners[name]
		for index, candidate := range items {
			if candidate == entry {
				b.listeners[name] = append(items[:index], items[index+1:]...)
				break
			}
		}
		b.mu.Unlock()
	}
}

func (b *EventBus) Emit(name string, data any) {
	b.mu.RLock()
	listeners := append([]*eventListener{}, b.listeners[name]...)
	b.mu.RUnlock()
	event := Event{Name: name, Data: data}
	for _, listener := range listeners {
		listener := listener
		ui.Post(func() { listener.fn(event) })
	}
}
