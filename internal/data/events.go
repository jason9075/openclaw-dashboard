package data

import (
	"sync"
)

type Event struct {
	Type    string      `json:"type"`
	Payload interface{} `json:"payload"`
}

type Broadcaster struct {
	clients    map[chan Event]bool
	register   chan chan Event
	unregister chan chan Event
	broadcast  chan Event
	mu         sync.Mutex
}

func NewBroadcaster() *Broadcaster {
	return &Broadcaster{
		clients:    make(map[chan Event]bool),
		register:   make(chan chan Event),
		unregister: make(chan chan Event),
		broadcast:  make(chan Event),
	}
}

func (b *Broadcaster) Run() {
	for {
		select {
		case client := <-b.register:
			b.mu.Lock()
			b.clients[client] = true
			b.mu.Unlock()
		case client := <-b.unregister:
			b.mu.Lock()
			if _, ok := b.clients[client]; ok {
				delete(b.clients, client)
				close(client)
			}
			b.mu.Unlock()
		case event := <-b.broadcast:
			b.mu.Lock()
			for client := range b.clients {
				select {
				case client <- event:
				default:
					// Drop event if client is slow
				}
			}
			b.mu.Unlock()
		}
	}
}

func (b *Broadcaster) Register() chan Event {
	ch := make(chan Event, 10)
	b.register <- ch
	return ch
}

func (b *Broadcaster) Unregister(ch chan Event) {
	b.unregister <- ch
}

func (b *Broadcaster) Broadcast(e Event) {
	b.broadcast <- e
}
