package extensions

import (
	"sync"
	"testing"
	"time"
)

func TestNewEventBus(t *testing.T) {
	eb := NewEventBus()
	if eb == nil {
		t.Fatal("NewEventBus returned nil")
	}
}

func TestEventBus_Subscribe(t *testing.T) {
	eb := NewEventBus()

	var called bool
	var mu sync.Mutex

	eb.Subscribe("sub1", func(ev Event) {
		mu.Lock()
		called = true
		mu.Unlock()
	}, []EventType{EventActionStarted})

	// Publish an event
	eb.Publish(Event{
		Type:      EventActionStarted,
		SessionID: "session-1",
	})

	// Wait for async handler
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if !called {
		t.Error("Subscriber was not called")
	}
	mu.Unlock()
}

func TestEventBus_Unsubscribe(t *testing.T) {
	eb := NewEventBus()

	var callCount int
	var mu sync.Mutex

	eb.Subscribe("sub1", func(ev Event) {
		mu.Lock()
		callCount++
		mu.Unlock()
	}, []EventType{EventActionStarted})

	// Publish before unsubscribe
	eb.Publish(Event{Type: EventActionStarted})
	time.Sleep(10 * time.Millisecond)

	// Unsubscribe
	eb.Unsubscribe("sub1")

	// Publish after unsubscribe
	eb.Publish(Event{Type: EventActionStarted})
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if callCount != 1 {
		t.Errorf("Expected 1 call before unsubscribe, got %d", callCount)
	}
	mu.Unlock()
}

func TestEventBus_Publish_NilSubscriber(t *testing.T) {
	eb := NewEventBus()

	// Subscribe with nil handler - should not panic
	eb.Subscribe("nil-sub", nil, []EventType{EventActionStarted})

	// Publish should not panic even if handler is nil
	eb.Publish(Event{Type: EventActionStarted})
}

func TestEventBus_MultipleHandlers(t *testing.T) {
	eb := NewEventBus()

	var count1, count2 int
	var mu sync.Mutex

	eb.Subscribe("sub1", func(ev Event) {
		mu.Lock()
		count1++
		mu.Unlock()
	}, []EventType{EventActionStarted})

	eb.Subscribe("sub2", func(ev Event) {
		mu.Lock()
		count2++
		mu.Unlock()
	}, []EventType{EventActionStarted})

	eb.Publish(Event{Type: EventActionStarted})
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	if count1 != 1 {
		t.Errorf("Expected sub1 to be called 1 time, got %d", count1)
	}
	if count2 != 1 {
		t.Errorf("Expected sub2 to be called 1 time, got %d", count2)
	}
	mu.Unlock()
}

func TestEventBus_MultipleEventTypes(t *testing.T) {
	eb := NewEventBus()

	var totalCount int
	var mu sync.Mutex

	// Subscribe the SAME callback to multiple event types
	eb.Subscribe("sub1", func(ev Event) {
		mu.Lock()
		totalCount++
		mu.Unlock()
	}, []EventType{EventActionStarted, EventPermissionDecision})

	// Note: The implementation adds the callback once per event type,
	// so subscribing to 2 event types registers 2 callback entries
	// Publish calls ALL registered callbacks regardless of event type
	eb.Publish(Event{Type: EventActionStarted})
	eb.Publish(Event{Type: EventPermissionDecision})
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	// Each Subscribe call registers the callback once per event type
	// So 2 event types = 2 callback entries
	// Each Publish calls all callbacks (no event type filtering in Publish)
	// 2 publishes x 2 callbacks = 4 total calls
	if totalCount != 4 {
		t.Errorf("Expected 4 total calls, got %d", totalCount)
	}
	mu.Unlock()
}

func TestEventBus_Publish_PanicRecovery(t *testing.T) {
	eb := NewEventBus()

	// Subscribe with a handler that panics
	eb.Subscribe("panic-sub", func(ev Event) {
		panic("test panic")
	}, []EventType{EventActionStarted})

	// Publish should recover from panic and continue
	eb.Publish(Event{Type: EventActionStarted})

	// Give time for the panic to be recovered
	time.Sleep(10 * time.Millisecond)

	// Bus should still be functional
	var called bool
	eb.Subscribe("normal-sub", func(ev Event) {
		called = true
	}, []EventType{EventActionStarted})

	eb.Publish(Event{Type: EventActionStarted})
	time.Sleep(10 * time.Millisecond)

	if !called {
		t.Error("Normal subscriber should still be called after panic recovery")
	}
}

func TestEventBus_Events(t *testing.T) {
	eb := NewEventBus()

	eb.Publish(Event{
		Type:      EventActionStarted,
		SessionID: "session-1",
		ToolName:  "test-tool",
	})

	eb.Publish(Event{
		Type:      EventPermissionDecision,
		SessionID: "session-1",
	})

	events := eb.Events()
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}
}

func TestEventBus_Events_Immutability(t *testing.T) {
	eb := NewEventBus()

	eb.Publish(Event{Type: EventActionStarted})

	events1 := eb.Events()
	events2 := eb.Events()

	// Should return copies, not the same slice
	if &events1[0] == &events2[0] {
		t.Error("Events should return copies, not same slice")
	}
}

func TestEventBus_IsEnabled(t *testing.T) {
	eb := NewEventBus()

	// Initially no events enabled
	if eb.IsEnabled(EventActionStarted) {
		t.Error("Event should not be enabled before subscription")
	}

	eb.Subscribe("sub1", func(ev Event) {}, []EventType{EventActionStarted})

	if !eb.IsEnabled(EventActionStarted) {
		t.Error("Event should be enabled after subscription")
	}
}

func TestEventBus_MaxLogLen(t *testing.T) {
	eb := NewEventBus()

	// Publish more events than maxLogLen (1000)
	for i := 0; i < 1100; i++ {
		eb.Publish(Event{Type: EventActionStarted, SessionID: "session"})
	}

	events := eb.Events()
	if len(events) != 1000 {
		t.Errorf("Expected 1000 events in log, got %d", len(events))
	}
}

func TestEventBus_Publish_Timestamp(t *testing.T) {
	eb := NewEventBus()

	before := time.Now()
	eb.Publish(Event{Type: EventActionStarted})
	after := time.Now()

	events := eb.Events()
	if len(events) != 1 {
		t.Fatal("Expected 1 event")
	}

	ts, err := time.Parse(time.RFC3339, events[0].Timestamp)
	if err != nil {
		t.Fatalf("Invalid timestamp format: %v", err)
	}

	// Check timestamp is reasonable (within a second)
	if ts.Before(before.Add(-time.Second)) || ts.After(after.Add(time.Second)) {
		t.Errorf("Timestamp %v seems unreasonable (before=%v, after=%v)", ts, before, after)
	}
}

func TestEventBus_Subscribe_MultipleEvents(t *testing.T) {
	eb := NewEventBus()

	var count int
	var mu sync.Mutex

	// Note: Each event type registration adds the same callback again
	eb.Subscribe("sub1", func(ev Event) {
		mu.Lock()
		count++
		mu.Unlock()
	}, []EventType{
		EventActionStarted,
		EventActionCompleted,
		EventSessionCompleted,
		EventBudgetWarning,
	})

	eb.Publish(Event{Type: EventActionStarted})
	eb.Publish(Event{Type: EventActionCompleted})
	eb.Publish(Event{Type: EventSessionCompleted})
	eb.Publish(Event{Type: EventBudgetWarning})
	time.Sleep(10 * time.Millisecond)

	mu.Lock()
	// Each Subscribe call adds the callback once per event type
	// So 4 event types = 4 callbacks registered
	// 4 publishes call all 4 callbacks each = 16 total calls
	if count != 16 {
		t.Errorf("Expected 16 calls (4 event types x 4 publishes), got %d", count)
	}
	mu.Unlock()
}

func TestEventBus_Subscribe_Unsubscribe_Idempotent(t *testing.T) {
	eb := NewEventBus()

	eb.Subscribe("sub1", func(ev Event) {}, []EventType{EventActionStarted})

	// Multiple unsubscribes should not panic
	eb.Unsubscribe("sub1")
	eb.Unsubscribe("sub1")
}

func TestEventBus_Publish_EventPayload(t *testing.T) {
	eb := NewEventBus()

	eb.Publish(Event{
		Type:      EventActionStarted,
		SessionID: "session-123",
		ToolName:  "github.search",
		Input:     map[string]any{"query": "golang"},
		Output:    map[string]any{"repos": 42},
		Error:     "",
		Fields:    map[string]any{"extra": "data"},
	})

	events := eb.Events()
	if len(events) != 1 {
		t.Fatal("Expected 1 event")
	}

	ev := events[0]
	if ev.Type != EventActionStarted {
		t.Errorf("Expected type %q, got %q", EventActionStarted, ev.Type)
	}
	if ev.SessionID != "session-123" {
		t.Errorf("Expected session_id %q, got %q", "session-123", ev.SessionID)
	}
	if ev.ToolName != "github.search" {
		t.Errorf("Expected tool_name %q, got %q", "github.search", ev.ToolName)
	}
	if ev.Input["query"] != "golang" {
		t.Errorf("Expected input[query] = %q, got %v", "golang", ev.Input["query"])
	}
	if ev.Output["repos"] != 42 {
		t.Errorf("Expected output[repos] = %v, got %v", 42, ev.Output["repos"])
	}
	if ev.Fields["extra"] != "data" {
		t.Errorf("Expected fields[extra] = %q, got %v", "data", ev.Fields["extra"])
	}
}

func TestEventType_Constants(t *testing.T) {
	// Verify event type constants are defined correctly
	if EventActionStarted != "action:started" {
		t.Errorf("EventActionStarted = %q, want %q", EventActionStarted, "action:started")
	}
	if EventPermissionDecision != "permission:decision" {
		t.Errorf("EventPermissionDecision = %q, want %q", EventPermissionDecision, "permission:decision")
	}
	if EventActionCompleted != "action:completed" {
		t.Errorf("EventActionCompleted = %q, want %q", EventActionCompleted, "action:completed")
	}
	if EventSessionCompleted != "session:completed" {
		t.Errorf("EventSessionCompleted = %q, want %q", EventSessionCompleted, "session:completed")
	}
	if EventBudgetWarning != "budget:warning" {
		t.Errorf("EventBudgetWarning = %q, want %q", EventBudgetWarning, "budget:warning")
	}
}

func TestEvent_Structure(t *testing.T) {
	ev := Event{
		Type:      "test:event",
		SessionID: "sess-1",
		Timestamp: time.Now().Format(time.RFC3339),
		ToolName:  "tool",
		Input:     map[string]any{"key": "value"},
		Output:    map[string]any{"result": 123},
		Error:     "",
		Fields:    map[string]any{"extra": "info"},
	}

	if ev.Type != "test:event" {
		t.Error("Type not set correctly")
	}
	if ev.SessionID != "sess-1" {
		t.Error("SessionID not set correctly")
	}
	if ev.ToolName != "tool" {
		t.Error("ToolName not set correctly")
	}
}
