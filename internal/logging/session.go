package logging

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"
)

type Session struct {
	ID        string
	Command   string
	StartedAt string
}

var (
	sm        sync.Map
	seq       uint64
	seqOnce   sync.Once
)

func NewSession(command string) Session {
	seqOnce.Do(func() { seq = uint64(time.Now().UnixNano()) })

	id := atomic.AddUint64(&seq, 1)
	return Session{
		ID:        fmt.Sprintf("%s-%s-%d", command, time.Now().UTC().Format("20060102150405"), id%10000),
		Command:   command,
		StartedAt: time.Now().UTC().Format(time.RFC3339),
	}
}

func (s Session) Logger() *Logger {
	return Default().WithSession(s.ID).WithCommand(s.Command)
}

func GetSession(id string) (Session, bool) {
	v, ok := sm.Load(id)
	if !ok {
		return Session{}, false
	}
	return v.(Session), true
}

func SaveSession(s Session) {
	sm.Store(s.ID, s)
}
