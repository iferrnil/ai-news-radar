package radar

import "sync"

// Msg is one event on the in-process bus (bus.ts BusMsg); SSE clients receive it as `event: <Event>` + JSON data.
type Msg struct {
	Event string       `json:"event"` // item | update | removed | sources | notify
	Item  *Row         `json:"item,omitempty"`
	ID    any          `json:"id,omitempty"`
	Batch *NotifyBatch `json:"batch,omitempty"`
}

// Bus fans messages out to subscribers (SSE streams, the desktop shell).
type Bus struct {
	mu   sync.Mutex
	subs map[chan Msg]struct{}
}

func NewBus() *Bus { return &Bus{subs: map[chan Msg]struct{}{}} }

func (b *Bus) Subscribe() chan Msg {
	ch := make(chan Msg, 256)
	b.mu.Lock()
	b.subs[ch] = struct{}{}
	b.mu.Unlock()
	return ch
}

func (b *Bus) Unsubscribe(ch chan Msg) {
	b.mu.Lock()
	delete(b.subs, ch)
	b.mu.Unlock()
}

func (b *Bus) Publish(m Msg) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for ch := range b.subs {
		select {
		case ch <- m:
		default: // slow client: drop rather than block the poller
		}
	}
}
