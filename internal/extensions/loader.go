package extensions

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/repo-necromancer/necro/internal/tools"
	"gopkg.in/yaml.v3"
)

// Extension is implemented by extensions that want to subscribe to events.
type Extension interface {
	Subscribe(bus *EventBus)
}

type Manifest struct {
	Name    string         `json:"name" yaml:"name"`
	Type    string         `json:"type" yaml:"type"`
	Enabled bool           `json:"enabled" yaml:"enabled"`
	Tool    ManifestTool   `json:"tool" yaml:"tool"`
	Config  map[string]any `json:"config" yaml:"config"`
}

type ManifestTool struct {
	Name    string `json:"name" yaml:"name"`
	Message string `json:"message" yaml:"message"`
}

type LoadResult struct {
	Tools       []tools.Tool
	Diagnostics []string
}

type Loader struct {
	bus *EventBus
}

func NewLoader() *Loader {
	return &Loader{bus: NewEventBus()}
}

func (l *Loader) Bus() *EventBus {
	return l.bus
}

// Load loads extensions from dir and optionally calls Subscribe if the tool implements Extension.
func (l *Loader) Load(ctx context.Context, dir string) LoadResult {
	return l.loadExt(ctx, dir, true)
}

// LoadTools loads extensions without calling Subscribe (tools only).
func (l *Loader) LoadTools(ctx context.Context, dir string) LoadResult {
	return l.loadExt(ctx, dir, false)
}

func (l *Loader) loadExt(ctx context.Context, dir string, callSubscribe bool) LoadResult {
	if strings.TrimSpace(dir) == "" {
		return LoadResult{}
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return LoadResult{}
		}
		return LoadResult{Diagnostics: []string{fmt.Sprintf("extension read dir failed: %v", err)}}
	}

	var (
		wg       sync.WaitGroup
		mu       sync.Mutex
		toolsOut []tools.Tool
		diag     []string
	)

	for _, entry := range entries {
		entry := entry
		if entry.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(entry.Name()))
		if ext != ".json" && ext != ".yaml" && ext != ".yml" {
			continue
		}
		wg.Add(1)
		go func() {
			defer wg.Done()
			select {
			case <-ctx.Done():
				mu.Lock()
				diag = append(diag, "extension loading interrupted")
				mu.Unlock()
				return
			default:
			}

			path := filepath.Join(dir, entry.Name())
			manifest, mErr := readManifest(path)
			if mErr != nil {
				mu.Lock()
				diag = append(diag, fmt.Sprintf("skip %s: %v", entry.Name(), mErr))
				mu.Unlock()
				return
			}
			if !manifest.Enabled {
				mu.Lock()
				diag = append(diag, fmt.Sprintf("skip %s: disabled", manifest.Name))
				mu.Unlock()
				return
			}
			tool, tErr := toolFromManifest(manifest)
			if tErr != nil {
				mu.Lock()
				diag = append(diag, fmt.Sprintf("skip %s: %v", manifest.Name, tErr))
				mu.Unlock()
				return
			}
			if callSubscribe {
				if ext, ok := tool.(Extension); ok {
					ext.Subscribe(l.bus)
				}
			}
			mu.Lock()
			toolsOut = append(toolsOut, tool)
			mu.Unlock()
		}()
	}
	wg.Wait()
	return LoadResult{
		Tools:       toolsOut,
		Diagnostics: diag,
	}
}

func readManifest(path string) (Manifest, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Manifest{}, err
	}
	var m Manifest
	switch strings.ToLower(filepath.Ext(path)) {
	case ".json":
		if err := json.Unmarshal(raw, &m); err != nil {
			return Manifest{}, err
		}
	default:
		if err := yaml.Unmarshal(raw, &m); err != nil {
			return Manifest{}, err
		}
	}
	if strings.TrimSpace(m.Name) == "" {
		return Manifest{}, errors.New("manifest.name is required")
	}
	if strings.TrimSpace(m.Type) == "" {
		return Manifest{}, errors.New("manifest.type is required")
	}
	return m, nil
}

func toolFromManifest(m Manifest) (tools.Tool, error) {
	switch m.Type {
	case "plugin", "skill", "template":
		if strings.TrimSpace(m.Tool.Name) == "" {
			return nil, errors.New("manifest.tool.name is required")
		}
		return &staticMessageTool{
			name:    m.Tool.Name,
			message: m.Tool.Message,
		}, nil
	case "subscriber":
		return newEventSubscriberTool(m)
	default:
		return nil, fmt.Errorf("unsupported extension type %q", m.Type)
	}
}

// eventSubscriberTool is a Tool + Extension that logs events.
type eventSubscriberTool struct {
	name    string
	message string
	events  []EventType
}

func newEventSubscriberTool(m Manifest) (*eventSubscriberTool, error) {
	name := m.Tool.Name
	if name == "" {
		name = m.Name
	}
	events := parseEventList(m.Config["events"])
	return &eventSubscriberTool{
		name:    name,
		message: m.Tool.Message,
		events:  events,
	}, nil
}

func parseEventList(v any) []EventType {
	var out []EventType
	if arr, ok := v.([]any); ok {
		for _, e := range arr {
			if s, ok := e.(string); ok {
				out = append(out, EventType(s))
			}
		}
	}
	return out
}

func (t *eventSubscriberTool) Name() string {
	return t.name
}

func (t *eventSubscriberTool) Run(_ context.Context, _ map[string]any) (map[string]any, error) {
	return map[string]any{
		"message": t.message,
		"events":  t.events,
	}, nil
}

// Subscribe implements Extension.
func (t *eventSubscriberTool) Subscribe(bus *EventBus) {
	for _, ev := range t.events {
		bus.Subscribe(t.name, func(e Event) {
			// default logging callback
		}, []EventType{ev})
	}
}

type staticMessageTool struct {
	name    string
	message string
}

func (t *staticMessageTool) Name() string {
	return t.name
}

func (t *staticMessageTool) Run(_ context.Context, _ map[string]any) (map[string]any, error) {
	return map[string]any{
		"message": t.message,
	}, nil
}

// Subscribe implements Extension.
func (t *staticMessageTool) Subscribe(_ *EventBus) {
	// staticMessageTool does not subscribe to events by default
}
