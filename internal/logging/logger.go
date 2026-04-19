package logging

import (
	"encoding/json"
	"io"
	"os"
	"sync"
	"time"
)

type Level string

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"
	LevelAudit Level = "audit"
)

type Entry struct {
	Timestamp string         `json:"timestamp"`
	Level     Level          `json:"level"`
	SessionID string         `json:"session_id,omitempty"`
	Command   string         `json:"command,omitempty"`
	Message   string         `json:"msg,omitempty"`
	Fields    map[string]any `json:"fields,omitempty"`
	Error     string         `json:"error,omitempty"`
}

type Logger struct {
	mu        sync.Mutex
	w         io.Writer
	sessionID string
	command   string
}

func New(w io.Writer) *Logger {
	if w == nil {
		w = os.Stderr
	}
	return &Logger{w: w}
}

func (l *Logger) WithSession(sessionID string) *Logger {
	return &Logger{w: l.w, sessionID: sessionID, command: l.command}
}

func (l *Logger) WithCommand(command string) *Logger {
	return &Logger{w: l.w, sessionID: l.sessionID, command: command}
}

func (l *Logger) Debug(msg string, fields ...map[string]any) { l.log(LevelDebug, msg, fields...) }
func (l *Logger) Info(msg string, fields ...map[string]any) { l.log(LevelInfo, msg, fields...) }
func (l *Logger) Warn(msg string, fields ...map[string]any) { l.log(LevelWarn, msg, fields...) }
func (l *Logger) Error(msg string, fields ...map[string]any) { l.log(LevelError, msg, fields...) }

func (l *Logger) Audit(event string, fields ...map[string]any) {
	var f map[string]any
	if len(fields) > 0 && fields[0] != nil {
		f = fields[0]
	} else {
		f = make(map[string]any)
	}
	l.log(LevelAudit, event, f)
}

func (l *Logger) log(level Level, msg string, fields ...map[string]any) {
	entry := Entry{
		Timestamp: time.Now().UTC().Format(time.RFC3339Nano),
		Level:     level,
		SessionID: l.sessionID,
		Command:   l.command,
		Message:   msg,
	}
	if len(fields) > 0 && fields[0] != nil {
		entry.Fields = fields[0]
	}
	if level == LevelError && len(fields) > 0 {
		if err, ok := fields[0]["error"]; ok {
			entry.Error = toString(err)
		}
	}

	l.mu.Lock()
	defer l.mu.Unlock()
	json.NewEncoder(l.w).Encode(entry)
}

func toString(v any) string {
	switch x := v.(type) {
	case string:
		return x
	case error:
		return x.Error()
	default:
		if x == nil {
			return ""
		}
		return "unknown"
	}
}

var defaultLogger = New(os.Stderr)

func Default() *Logger { return defaultLogger }
func Info(msg string, fields ...map[string]any) { defaultLogger.Info(msg, fields...) }
func Warn(msg string, fields ...map[string]any) { defaultLogger.Warn(msg, fields...) }
func Error(msg string, fields ...map[string]any) { defaultLogger.Error(msg, fields...) }
func Audit(event string, fields ...map[string]any) { defaultLogger.Audit(event, fields...) }
