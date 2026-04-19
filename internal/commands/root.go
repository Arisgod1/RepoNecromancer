package commands

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"

	"github.com/repo-necromancer/necro/internal/extensions"
	"github.com/repo-necromancer/necro/internal/llm"
	"github.com/repo-necromancer/necro/internal/logging"
	"github.com/repo-necromancer/necro/internal/network"
	"github.com/repo-necromancer/necro/internal/permissions"
	"github.com/repo-necromancer/necro/internal/query"
	"github.com/repo-necromancer/necro/internal/report"
	"github.com/repo-necromancer/necro/internal/state"
	"github.com/repo-necromancer/necro/internal/tools"
)

type appContextKey struct{}

type App struct {
	Config      Config
	Store       *state.MemoryStore
	Permissions *permissions.Engine
	Registry    *tools.Registry
	Query       *query.Engine
	Renderer    *report.Renderer
	Network     *network.Client
	LLMClient   *llm.Client
	SessionID   string
	Logger      *logging.Logger
}

type Config struct {
	App struct {
		LogLevel  string `mapstructure:"log_level"`
		OutputDir string `mapstructure:"output_dir"`
		CacheDir  string `mapstructure:"cache_dir"`
		Language  string `mapstructure:"language"`
	} `mapstructure:"app"`
	Analysis struct {
		DefaultYears int `mapstructure:"default_years"`
		MinStars     int `mapstructure:"min_stars"`
		MaxItems     int `mapstructure:"max_items"`
		MaxEvidence  int `mapstructure:"max_evidence"`
	} `mapstructure:"analysis"`
	Query struct {
		MaxTurns  int     `mapstructure:"max_turns"`
		MaxTokens int     `mapstructure:"max_tokens"`
		MaxCost   float64 `mapstructure:"max_cost"`
	} `mapstructure:"query"`
	Network struct {
		TimeoutMS           int      `mapstructure:"timeout_ms"`
		RetryMax            int      `mapstructure:"retry_max"`
		BackoffBaseMS       int      `mapstructure:"backoff_base_ms"`
		AllowDomains        []string `mapstructure:"allow_domains"`
		BlockDomains        []string `mapstructure:"block_domains"`
		DenyPrivateNetworks bool     `mapstructure:"deny_private_networks"`
	} `mapstructure:"network"`
	Permissions struct {
		Mode string `mapstructure:"mode"`
	} `mapstructure:"permissions"`
	Tools struct {
		Deny []string `mapstructure:"deny"`
	} `mapstructure:"tools"`
	LLM struct {
		Model   string `mapstructure:"model"`
		APIBase string `mapstructure:"api_base"`
	} `mapstructure:"llm"`
}

func Execute() error {
	return newRootCommand().Execute()
}

func newRootCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "necro",
		Short: "Repo Necromancer CLI",
		Long:  "Analyze dead repositories and generate necropsy + reincarnation plans.",
		PersistentPreRunE: func(cmd *cobra.Command, _ []string) error {
			session := logging.NewSession(cmd.Name())
			app, err := bootstrapApp(cmd.Context(), session)
			if err != nil {
				return err
			}
			app.Logger = session.Logger()
			app.SessionID = session.ID
			ctx := context.WithValue(cmd.Context(), appContextKey{}, app)
			cmd.SetContext(ctx)
			return nil
		},
	}

	cmd.AddCommand(
		newScanCommand(),
		newAutopsyCommand(),
		newRebornCommand(),
		newReportCommand(),
		newCacheCommand(),
	)
	return cmd
}

func bootstrapApp(ctx context.Context, session logging.Session) (*App, error) {
	cfg, err := loadConfig()
	if err != nil {
		return nil, err
	}

	if err := os.MkdirAll(cfg.App.OutputDir, 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(cfg.App.CacheDir, 0o755); err != nil {
		return nil, err
	}

	mode, err := permissions.ParseMode(cfg.Permissions.Mode)
	if err != nil {
		return nil, err
	}

	store := state.NewMemoryStore()
	perm := permissions.NewEngine(permissions.Config{
		Mode:                mode,
		AllowedDomains:      cfg.Network.AllowDomains,
		BlockedDomains:      cfg.Network.BlockDomains,
		DenyPrivateNetworks: cfg.Network.DenyPrivateNetworks,
		ToolAllowOverrides:  map[string]permissions.Behavior{},
	})
	netClient := network.NewClient(network.Config{
		TimeoutMS:           cfg.Network.TimeoutMS,
		RetryMax:            cfg.Network.RetryMax,
		BackoffBaseMS:       cfg.Network.BackoffBaseMS,
		DenyPrivateNetworks: cfg.Network.DenyPrivateNetworks,
	})
	llmClient := llm.NewClient()
	builtinTools := append(tools.NewGitHubTools(os.Getenv("GITHUB_TOKEN")), tools.NewWebTools(netClient)...)

	loader := extensions.NewLoader()
	extRes := loader.Load(ctx, "extensions")
	registry := tools.NewRegistry(builtinTools, extRes.Tools, cfg.Tools.Deny)
	store.Set("extensions:diagnostics", extRes.Diagnostics)
	store.Set("tools:available", registry.Names())

	eventBus := extensions.NewEventBus()
	engine := query.NewEngine(registry, perm, store, eventBus)
	renderer := report.NewRenderer()

	app := &App{
		Config:      cfg,
		Store:       store,
		Permissions: perm,
		Registry:    registry,
		Query:       engine,
		Renderer:    renderer,
		Network:     netClient,
		LLMClient:   llmClient,
	}
	store.Set("app:initialized", true)
	return app, nil
}

func loadConfig() (Config, error) {
	v := viper.New()
	v.SetConfigType("yaml")
	v.SetDefault("app.log_level", "info")
	v.SetDefault("app.output_dir", "./out")
	v.SetDefault("app.cache_dir", "./.cache/necro")
	v.SetDefault("app.language", "zh")
	v.SetDefault("analysis.default_years", 3)
	v.SetDefault("analysis.min_stars", 5000)
	v.SetDefault("analysis.max_items", 500)
	v.SetDefault("analysis.max_evidence", 250)
	v.SetDefault("query.max_turns", 16)
	v.SetDefault("query.max_tokens", 0)
	v.SetDefault("query.max_cost", 0)
	v.SetDefault("network.timeout_ms", 12000)
	v.SetDefault("network.retry_max", 3)
	v.SetDefault("network.backoff_base_ms", 300)
	v.SetDefault("network.allow_domains", []string{"github.com", "api.github.com"})
	v.SetDefault("network.block_domains", []string{})
	v.SetDefault("network.deny_private_networks", true)
	v.SetDefault("permissions.mode", "default")
	v.SetDefault("tools.deny", []string{})
	v.SetDefault("llm.model", "qwen3.6-flash")
	v.SetDefault("llm.api_base", "https://dashscope.aliyuncs.com/compatible-mode/v1")

	if custom := strings.TrimSpace(os.Getenv("NECRO_CONFIG")); custom != "" {
		v.SetConfigFile(custom)
	} else {
		v.SetConfigName("config")
		v.AddConfigPath(".")
		v.AddConfigPath("./configs")
	}
	v.SetEnvPrefix("NECRO")
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		var notFound viper.ConfigFileNotFoundError
		if !errors.As(err, &notFound) {
			return Config{}, err
		}
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return Config{}, err
	}
	cfg.App.OutputDir = filepath.Clean(cfg.App.OutputDir)
	cfg.App.CacheDir = filepath.Clean(cfg.App.CacheDir)
	return cfg, nil
}

func appFromCmd(cmd *cobra.Command) (*App, error) {
	v := cmd.Context().Value(appContextKey{})
	app, ok := v.(*App)
	if !ok || app == nil {
		return nil, fmt.Errorf("app not initialized")
	}
	return app, nil
}
