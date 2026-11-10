package storage

import (
	"sync"
	"sync/atomic"
)

// broker signals subscribers that something changed. Signals coalesce, so a burst of writes
// causes a single re-render.
type broker struct {
	mu      sync.Mutex
	subs    map[chan struct{}]struct{}
	version atomic.Uint64
}

func newBroker() *broker {
	return &broker{subs: map[chan struct{}]struct{}{}}
}

func (b *broker) Subscribe() (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch, func() {
		b.mu.Lock()
		delete(b.subs, ch)
		b.mu.Unlock()
	}
}

func (b *broker) Version() uint64 {
	return b.version.Load()
}

func (b *broker) notify() {
	b.version.Add(1)
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
