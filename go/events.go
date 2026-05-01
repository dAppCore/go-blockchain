// Copyright (c) 2017-2026 Lethean (https://lt.hn)
// SPDX-License-Identifier: EUPL-1.2

package blockchain

import (
	"sync"
	
	"context"

	"dappco.re/go/core"
)

// EventType identifies blockchain events.
type EventType string

const (
	EventBlockNew       EventType = "blockchain.block.new"
	EventAlias          EventType = "blockchain.alias.registered"
	EventHardfork       EventType = "blockchain.hardfork.activated"
	EventSyncProgress   EventType = "blockchain.sync.progress"
	EventSyncComplete   EventType = "blockchain.sync.complete"
	EventAssetDeployed  EventType = "blockchain.asset.deployed"
)

// Event is a blockchain event with typed data.
//
//	event := blockchain.Event{Type: EventBlockNew, Data: blockHeader}
type Event struct {
	Type   EventType
	Height uint64
	Data   interface{}
}

// EventBus distributes blockchain events to subscribers.
//
//	bus := blockchain.NewEventBus()
//	bus.Subscribe(blockchain.EventBlockNew, handler)
//	bus.Emit(blockchain.Event{Type: EventBlockNew, Height: 11500})
type EventBus struct {
	mu       sync.RWMutex
	handlers map[EventType][]EventHandler
}

// EventHandler processes a blockchain event.
type EventHandler func(Event)

// NewEventBus creates an event distribution bus.
//
//	bus := blockchain.NewEventBus()
func NewEventBus() *EventBus {
	return &EventBus{
		handlers: make(map[EventType][]EventHandler),
	}
}

// Subscribe registers a handler for an event type.
//
//	bus.Subscribe(blockchain.EventBlockNew, func(e blockchain.Event) {
//	    core.Print(nil, "new block at %d", e.Height)
//	})
func (b *EventBus) Subscribe(eventType EventType, handler EventHandler) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.handlers[eventType] = append(b.handlers[eventType], handler)
}

// Emit sends an event to all registered handlers.
//
//	bus.Emit(blockchain.Event{Type: EventBlockNew, Height: 11500})
func (b *EventBus) Emit(event Event) {
	b.mu.RLock()
	handlers := b.handlers[event.Type]
	b.mu.RUnlock()

	for _, h := range handlers {
		h(event)
	}
}

// SubscriberCount returns the number of handlers for an event type.
func (b *EventBus) SubscriberCount(eventType EventType) int {
	b.mu.RLock()
	defer b.mu.RUnlock()
	return len(b.handlers[eventType])
}

// RegisterEventActions registers event-related Core actions.
//
//	blockchain.RegisterEventActions(c, bus)
func RegisterEventActions(c *core.Core, bus *EventBus) {
	c.Action("blockchain.events.subscribe", func(_ context.Context, opts core.Options) core.Result {
		return core.Result{Value: map[string]interface{}{
			"available_events": []string{
				string(EventBlockNew), string(EventAlias),
				string(EventHardfork), string(EventSyncProgress),
				string(EventSyncComplete), string(EventAssetDeployed),
			},
			"note": "subscribe via SSE at /events/blocks or core/stream",
		}, OK: true}
	})
}
