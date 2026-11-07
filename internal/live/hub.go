// Package live carries in-process change signals from writers to SSE streams.
// A Hub knows only string topics and never learns what a topic means.
package live

import "sync"

// Hub fans change signals out to subscribers by topic. Every subscriber gets a
// channel with room for one signal, so a burst of publishes while a listener is
// busy coalesces into a single wake-up. The zero value is ready to use.
type Hub struct {
	mu     sync.Mutex
	topics map[string]map[chan struct{}]struct{}
}

// Subscribe returns a channel that receives a value after each published burst
// on topic and a func that stops delivery. The channel is buffered, so Publish
// never blocks; a full buffer drops the extra signals. Call the unsubscribe
// func when the listener exits; it is safe to call more than once.
func (h *Hub) Subscribe(topic string) (<-chan struct{}, func()) {
	ch := make(chan struct{}, 1)

	h.mu.Lock()
	if h.topics == nil {
		h.topics = make(map[string]map[chan struct{}]struct{})
	}
	subscribers := h.topics[topic]
	if subscribers == nil {
		subscribers = make(map[chan struct{}]struct{})
		h.topics[topic] = subscribers
	}
	subscribers[ch] = struct{}{}
	h.mu.Unlock()

	var once sync.Once
	unsubscribe := func() {
		once.Do(func() {
			h.mu.Lock()
			defer h.mu.Unlock()

			delete(subscribers, ch)
			if len(subscribers) == 0 {
				delete(h.topics, topic)
			}
		})
	}

	return ch, unsubscribe
}

// Publish signals every subscriber of topic without blocking. A subscriber
// whose buffer already holds a signal is skipped, so a burst of writes wakes
// each listener once.
func (h *Hub) Publish(topic string) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for ch := range h.topics[topic] {
		select {
		case ch <- struct{}{}:
		default:
		}
	}
}
