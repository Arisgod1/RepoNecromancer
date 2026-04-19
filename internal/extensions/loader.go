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
}

func NewLoader() *Loader {
	return &Loader{}
}

func (l *Loader) Load(ctx context.Context, dir string) LoadResult {
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
	default:
		return nil, fmt.Errorf("unsupported extension type %q", m.Type)
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
