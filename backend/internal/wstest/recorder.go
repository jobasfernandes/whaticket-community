//go:build integration

package wstest

import (
	"sync"
)

type Event struct {
	Channel string
	Event   string
	Data    any
}

type Recorder struct {
	mu     sync.Mutex
	events []Event
}

func New() *Recorder {
	return &Recorder{}
}

func (r *Recorder) Publish(channel, event string, data any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = append(r.events, Event{Channel: channel, Event: event, Data: data})
}

func (r *Recorder) Snapshot() []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	out := make([]Event, len(r.events))
	copy(out, r.events)
	return out
}

func (r *Recorder) Reset() {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.events = nil
}

func (r *Recorder) Find(event string) []Event {
	r.mu.Lock()
	defer r.mu.Unlock()
	var out []Event
	for _, e := range r.events {
		if e.Event == event {
			out = append(out, e)
		}
	}
	return out
}
