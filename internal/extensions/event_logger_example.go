//go:build ignore
// +build ignore

// event_logger_example.go demonstrates how to implement the Extension interface
// to subscribe to event bus lifecycle events. This file is NOT compiled —
// it serves as reference documentation and a copy-paste template.
//
// To use: copy this pattern into your extension's .go file (in a separate module
// that imports github.com/repo-necromancer/necro/internal/extensions).

package extensions

import (
	"context"
	"fmt"
)

// eventLoggerExample implements Extension to demonstrate Subscribe().
type eventLoggerExample struct {
	name   string
	events []EventType
}

func (e *eventLoggerExample) Name() string { return e.name }

func (e *eventLoggerExample) Run(_ context.Context, _ map[string]any) (map[string]any, error) {
	return map[string]any{"status": "logging", "message": "event-logger is active"}, nil
}

// Subscribe implements Extension — logs every subscribed event to stdout.
func (e *eventLoggerExample) Subscribe(bus *EventBus) {
	for _, ev := range e.events {
		bus.Subscribe(e.name, func(evt Event) {
			fmt.Printf("[%s] %s | session=%s tool=%s\n",
				evt.Timestamp, evt.Type, evt.SessionID, evt.ToolName)
		}, []EventType{ev})
	}
}
