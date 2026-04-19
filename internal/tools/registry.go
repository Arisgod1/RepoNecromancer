package tools

import (
	"context"
	"fmt"
	"sort"
	"strings"
)

type Tool interface {
	Name() string
	Run(ctx context.Context, input map[string]any) (map[string]any, error)
}

type Registry struct {
	tools map[string]Tool
}

func NewRegistry(builtin, extensions []Tool, deny []string) *Registry {
	compiledDeny := compileDenyPatterns(deny)
	out := make(map[string]Tool)
	insert := func(t Tool) {
		if t == nil {
			return
		}
		name := strings.TrimSpace(t.Name())
		if name == "" || deniedByPattern(name, compiledDeny) {
			return
		}
		if _, exists := out[name]; exists {
			return
		}
		out[name] = t
	}
	for _, t := range builtin {
		insert(t)
	}
	for _, t := range extensions {
		insert(t)
	}
	return &Registry{tools: out}
}

func (r *Registry) Get(name string) (Tool, bool) {
	t, ok := r.tools[name]
	return t, ok
}

func (r *Registry) MustGet(name string) (Tool, error) {
	t, ok := r.Get(name)
	if !ok {
		return nil, fmt.Errorf("tool not found: %s", name)
	}
	return t, nil
}

func (r *Registry) Names() []string {
	names := make([]string, 0, len(r.tools))
	for n := range r.tools {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func compileDenyPatterns(items []string) []string {
	patterns := make([]string, 0, len(items))
	for _, v := range items {
		v = strings.TrimSpace(strings.ToLower(v))
		if v != "" {
			patterns = append(patterns, v)
		}
	}
	return patterns
}

func deniedByPattern(name string, patterns []string) bool {
	n := strings.ToLower(name)
	for _, p := range patterns {
		switch {
		case p == n:
			return true
		case strings.HasSuffix(p, "*"):
			prefix := strings.TrimSuffix(p, "*")
			if strings.HasPrefix(n, prefix) {
				return true
			}
		}
	}
	return false
}
