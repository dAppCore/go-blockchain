package blockchain

import "testing"

func TestEventBus_SubscribeEmit_Good(t *testing.T) {
	bus := NewEventBus()
	received := false
	bus.Subscribe(EventBlockNew, func(e Event) {
		received = true
		if e.Height != 11500 {
			t.Errorf("height: got %d, want 11500", e.Height)
		}
	})
	bus.Emit(Event{Type: EventBlockNew, Height: 11500})
	if !received {
		t.Error("handler was not called")
	}
}

func TestEventBus_MultipleSubscribers_Good(t *testing.T) {
	bus := NewEventBus()
	count := 0
	bus.Subscribe(EventBlockNew, func(e Event) { count++ })
	bus.Subscribe(EventBlockNew, func(e Event) { count++ })
	bus.Subscribe(EventAlias, func(e Event) { count++ })

	bus.Emit(Event{Type: EventBlockNew})
	if count != 2 {
		t.Errorf("expected 2 handlers called, got %d", count)
	}
}

func TestEventBus_SubscriberCount_Good(t *testing.T) {
	bus := NewEventBus()
	bus.Subscribe(EventBlockNew, func(e Event) {})
	bus.Subscribe(EventBlockNew, func(e Event) {})
	if bus.SubscriberCount(EventBlockNew) != 2 {
		t.Error("expected 2 subscribers")
	}
	if bus.SubscriberCount(EventAlias) != 0 {
		t.Error("expected 0 subscribers for alias")
	}
}

func TestEventBus_NoSubscribers_Ugly(t *testing.T) {
	bus := NewEventBus()
	// Should not panic with no subscribers
	bus.Emit(Event{Type: EventBlockNew, Height: 100})
}

func TestEventBus_EventTypes_Good(t *testing.T) {
	if EventBlockNew != "blockchain.block.new" {
		t.Error("wrong event type string")
	}
	if EventHardfork != "blockchain.hardfork.activated" {
		t.Error("wrong hardfork event type")
	}
}
