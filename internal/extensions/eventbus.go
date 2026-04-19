package extensions

import (
	"sync"
	"time"
)

type EventType string

const (
	EventActionStarted             EventType = "action:started"
	EventPermissionDecision        EventType = "permission:decision"
	EventActionCompleted           EventType = "action:completed"
	EventSessionCompleted          EventType = "session:completed"
	EventBudgetWarning             EventType = "budget:warning"
)

type Event struct {
	Type      EventType              `json:"type"`
	SessionID string                 `json:"session_id"`
	Timestamp string                 `json:"timestamp"`
	ToolName  string                 `json:"tool_name,omitempty"`
	Input     map[string]any        `json:"input,omitempty"`
	Output    map[string]any        `json:"output,omitempty"`
	Error     string                 `json:"error,omitempty"`
	Fields    map[string]any        `json:"fields,omitempty"`
}

type Callback func(Event)

type EventBus struct {
	mu        sync.RWMutex
	subs      map[string][]Callback
	enabled   map[string]bool
	eventLog  []Event
	maxLogLen int
}

func NewEventBus() *EventBus {
	return &EventBus{
		subs:      make(map[string][]Callback),
		enabled:   make(map[string]bool),
		maxLogLen: 1000,
	}
}

func (eb *EventBus) Subscribe(id string, fn Callback, events []EventType) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	if _, ok := eb.subs[id]; !ok {
		eb.subs[id] = nil
	}
	for _, ev := range events {
		eb.subs[id] = append(eb.subs[id], fn)
		eb.enabled[string(ev)] = true
	}
}

func (eb *EventBus) Unsubscribe(id string) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	delete(eb.subs, id)
}

func (eb *EventBus) Publish(ev Event) {
	eb.mu.Lock()
	defer eb.mu.Unlock()

	ev.Timestamp = time.Now().UTC().Format(time.RFC3339)
	eb.eventLog = append(eb.eventLog, ev)
	if len(eb.eventLog) > eb.maxLogLen {
		eb.eventLog = eb.eventLog[len(eb.eventLog)-eb.maxLogLen:]
	}

	for id, cbs := range eb.subs {
		_ = id
		for _, cb := range cbs {
			func() {
				defer func() {
					if r := recover(); r != nil {
					}
				}()
				cb(ev)
			}()
		}
	}
}

func (eb *EventBus) Events() []Event {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	out := make([]Event, len(eb.eventLog))
	copy(out, eb.eventLog)
	return out
}

func (eb *EventBus) IsEnabled(eventType EventType) bool {
	eb.mu.RLock()
	defer eb.mu.RUnlock()
	return eb.enabled[string(eventType)]
}
