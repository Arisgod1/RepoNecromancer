# Repo Necromancer

[![Go Version](https://img.shields.io/badge/Go-1.26.2-blue)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-green.svg)](https://github.com/repo-necromancer/necro)

**Repo Necromancer** is an AI-powered CLI tool that performs autopsies on abandoned GitHub repositories, determines the cause of death, and generates detailed reincarnation plans to bring them back to life.

---

## Table of Contents

- [Features](#features)
- [Architecture](#architecture)
- [Installation](#installation)
- [Configuration](#configuration)
- [Environment Variables](#environment-variables)
- [Usage](#usage)
  - [`necro scan`](#necro-scan)
  - [`necro autopsy`](#necro-autopsy)
  - [`necro report`](#necro-report)
  - [`necro reborn`](#necro-reborn)
- [Output Formats](#output-formats)
- [Examples](#examples)

---

## Features

- **Repository Scanning** — Discover abandoned repositories based on inactivity threshold, stars, language, and topics
- **Death-Cause Autopsy** — Deep analysis of issues, PRs, and commits to determine why a repo died
- **Cause Scoring Taxonomy** — Seven failure modes: maintainer burnout, architecture debt, ecosystem displacement, security trust collapse, governance failure, funding absence, and scope drift
- **AI-Enhanced Analysis** — Optional DashScope LLM integration for richer cause scoring and reincarnation planning
- **Reincarnation Plans** — Structured 90-day revival blueprints with architecture recommendations, migration steps, risks, and milestones
- **Extensible Tool Registry** — Built-in GitHub and web tools, plus custom extension loading
- **Permission Engine** — Configurable domain/IP allowlisting for safe tool execution
- **Multiple Output Formats** — JSON and Markdown report artifacts

---

## Architecture

```
┌─────────────────────────────────────────────────────────────────────┐
│                           necro CLI                                  │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  ┌──────────┐           │
│  │   scan   │  │ autopsy  │  │  report  │  │  reborn  │           │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  └────┬─────┘           │
└───────┼─────────────┼─────────────┼─────────────┼──────────────────┘
        │             │             │             │
        └─────────────┴──────┬──────┴─────────────┘
                             │
                    ┌────────▼────────┐
                    │  Query Engine  │  ← Budget-controlled execution
                    └────────┬────────┘
                             │
        ┌────────────────────┼────────────────────┐
        │                    │                    │
┌───────▼───────┐  ┌────────▼────────┐  ┌──────▼──────┐
│ Tool Registry  │  │Permissions Eng. │  │ Memory Store │
│ ─────────────  │  └─────────────────┘  └──────────────┘
│ github.* tools │            │
│ web.* tools    │     ┌──────▼──────┐
│ ext.* tools    │     │ Network Cl │
└────────────────┘     └─────────────┘
                              │
                    ┌─────────▼──────────┐
                    │   LLM Client       │  ← DashScope (optional)
                    │  (qwen3.6-plus)    │
                    └────────────────────┘
```

### Component Overview

| Component | Responsibility |
|-----------|----------------|
| **CLI Commands** (`scan`, `autopsy`, `report`, `reborn`) | User-facing entry points |
| **Query Engine** | Executes tool actions with budget limits (turns, tokens, cost) |
| **Tool Registry** | Manages built-in GitHub tools, web tools, and loaded extensions |
| **Permissions Engine** | Enforces domain/IP allowlists and private-network denials |
| **Network Client** | HTTP client with retry/backoff for tool execution |
| **LLM Client** | DashScope API client for AI-enhanced cause scoring and reincarnation plans |
| **Memory Store** | In-memory state management across command invocations |
| **Report Renderer** | Generates JSON and Markdown artifact files |

---

## Installation

### Prerequisites

- **Go 1.26.2+** (if building from source)
- **GitHub Personal Access Token** (`GITHUB_TOKEN`) — for repository API access
- **DashScope API Key** (`DASHSCOPE_API_KEY`) — for AI-enhanced features (optional)

### Build from Source

```bash
git clone https://github.com/repo-necromancer/necro.git
cd necro
go build -o necro ./cmd/necro
```

### Pre-built Binary

Download the appropriate binary for your platform from the releases page and make it executable:

```bash
chmod +x necro
./necro --help
```

---

## Configuration

Repo Necromancer uses a YAML configuration file. By default, it looks for `config.yaml` in the following order:

1. Path specified by `NECRO_CONFIG` environment variable
2. `./config.yaml` (current directory)
3. `./configs/config.yaml`

### Default `config.yaml`

```yaml
app:
  log_level: info
  output_dir: ./out
  cache_dir: ./.cache/necro

analysis:
  default_years: 3      # Default inactivity threshold for scans
  min_stars: 5000       # Minimum star count for candidate repos
  max_items: 500        # Maximum issues/PRs/commits to fetch

query:
  max_turns: 16         # Maximum tool-call turns in a session
  max_tokens: 0         # 0 = unlimited
  max_cost: 0           # 0 = unlimited (USD)

network:
  timeout_ms: 12000     # HTTP request timeout in milliseconds
  retry_max: 3          # Maximum retry attempts
  backoff_base_ms: 300   # Base backoff duration (doubles each retry)
  allow_domains:
    - github.com
    - api.github.com
  block_domains: []
  deny_private_networks: true

permissions:
  mode: default        # default | plan | dontAsk | bypass | acceptEdits | auto

tools:
  deny: []               # List of tool names to disable

llm:
  model: qwen3.6-plus
  api_base: https://dashscope.aliyuncs.com/compatible-mode/v1
  timeout_seconds: 300
```

### Configuration Precedence

Environment variables override config file values. The prefix `NECRO_` is used for env vars:

| Config Key | Environment Variable |
|------------|---------------------|
| `llm.model` | `NECRO_LLM_MODEL` |
| `llm.api_base` | `NECRO_LLM_API_BASE` |
| `app.output_dir` | `NECRO_APP_OUTPUT_DIR` |
| `app.log_level` | `NECRO_APP_LOG_LEVEL` |

### Permissions Mode Reference

| Mode | Description |
|------|-------------|
| `default` | Execute only tools targeting explicitly allowed domains/IPs (default, recommended) |
| `plan` | Ask for confirmation before executing any tool |
| `dontAsk` | Execute tools without confirmation (skip prompts) |
| `bypass` | Bypass all permission checks and execute directly |
| `acceptEdits` | Accept automatic edits to files without prompting |
| `auto` | Automatically determine the best behavior |

---

## Environment Variables

| Variable | Required | Description |
|----------|----------|-------------|
| `GITHUB_TOKEN` | Recommended | GitHub Personal Access Token for API access. Without it, you hit lower rate limits. |
| `DASHSCOPE_API_KEY` | Optional | DashScope API key for AI-enhanced analysis and reincarnation planning. |
| `DASHSCOPE_MODEL` | No | Override the default LLM model (`qwen3.6-plus`). |
| `DASHSCOPE_API_BASE` | No | Override the DashScope API base URL. |
| `NECRO_CONFIG` | No | Path to a custom YAML config file. |

### Obtaining a GitHub Token

1. Go to [GitHub Settings → Personal access tokens](https://github.com/settings/tokens)
2. Generate a new token (classic)
3. Select scopes: `repo` (for private repos) or `public_repo` (for public only)
4. Copy the token and set it as `GITHUB_TOKEN`

### Obtaining a DashScope API Key

Sign up at [Alibaba Cloud DashScope](https://dashscope.aliyuncs.com/) and generate an API key.

---

## Tools

Repo Necromancer provides built-in tools for GitHub API access and web fetching. Tools are executed by the Query Engine under budget controls (turns, tokens, cost).

### GitHub Tools (`github.*`)

| Tool | Description | Required Permission |
|------|-------------|---------------------|
| `github.search_repos` | Search repositories by query, language, stars, last activity | `github.com`, `api.github.com` |
| `github.get_repo` | Fetch repository metadata and stats | `github.com`, `api.github.com` |
| `github.list_issues` | List issues with state, labels, and timeline | `github.com`, `api.github.com` |
| `github.list_pulls` | List pull requests with review state | `github.com`, `api.github.com` |
| `github.get_commits` | Fetch commit history with author and timestamp | `github.com`, `api.github.com` |
| `github.list_collaborators` | List repository collaborators and permissions | `github.com`, `api.github.com` |

### Web Tools (`web.*`)

| Tool | Description | Required Permission |
|------|-------------|---------------------|
| `web.fetch` | HTTP GET a URL and return content | Matching `allow_domains` entry |
| `web.search` | Perform a web search via configured endpoint | Matching `allow_domains` entry |

### Permission Levels

Tools are gated by the Permissions Engine based on `permissions.mode`:

| Mode | Behavior |
|------|----------|
| `default` | Tool executes only if its target domain/IP is in `network.allow_domains` (default, recommended) |
| `plan` | Ask for confirmation before executing any tool |
| `dontAsk` | Execute tools without confirmation (skip prompts) |
| `bypass` | Bypass all permission checks and execute directly |
| `acceptEdits` | Accept automatic edits to files without prompting |
| `auto` | Automatically determine the best behavior |

### Disabling Tools

Use `tools.deny` to disable specific tools by name:

```yaml
tools:
  deny:
    - github.list_collaborators
    - web.search
```

---

## Observability

Repo Necromancer emits structured log fields and maintains an audit trail for all tool executions and LLM calls.

### Structured Log Fields

| Field | Type | Description |
|-------|------|-------------|
| `trace_id` | string | Unique execution ID shared across all events in a session |
| `session_id` | string | CLI invocation session identifier |
| `command` | string | Top-level command (`scan`, `autopsy`, `report`, `reborn`) |
| `target` | string | Target repository (`owner/repo`) |
| `tool_name` | string | Tool that was invoked |
| `tool_domain` | string | Target domain for the tool call |
| `allowed` | bool | Whether the Permissions Engine allowed the call |
| `turn` | int | Current turn number in the Query Engine budget |
| `tokens_used` | int | Cumulative token usage for the LLM session |
| `cost_usd` | float | Cumulative cost in USD |

### Audit Trail

Each command run produces an `audit.log` entry (written to `cache_dir/audit/`):

```
{"ts":"2026-04-19T13:45:00Z","trace_id":"abc123","session":"sess-001","command":"autopsy","target":"owner/repo","tool":"github.list_issues","allowed":true,"turn":3,"ms":142}
{"ts":"2026-04-19T13:45:01Z","trace_id":"abc123","session":"sess-001","command":"autopsy","target":"owner/repo","tool":"github.list_issues","allowed":true,"turn":4,"ms":89}
```

### Log Level Configuration

| Level | Use Case |
|-------|----------|
| `error` | Production; logs errors and denied tool calls only |
| `info` | Default; includes command progress and tool summaries |
| `debug` | Development; includes all tool inputs/outputs and LLM prompts/responses |
| `trace` | Verbose; full HTTP request/response bodies and permission checks |

---

## Usage

### `necro scan`

Discover candidate dead repositories matching inactivity and popularity criteria.

```bash
necro scan --years <N> --min-stars <N> [flags]
```

**Flags:**

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `--years` | int | Yes | Inactivity threshold in years |
| `--min-stars` | int | Yes | Minimum star count |
| `--language` | string | No | Filter by programming language (e.g., `Go`, `Python`) |
| `--topic` | string[] | No | Filter by GitHub topic (repeatable) |
| `--limit` | int | No | Maximum results to return (default: 20, max: 100) |

**Example:**

```bash
export GITHUB_TOKEN=ghp_your_token_here
necro scan --years 3 --min-stars 5000 --language Go --limit 10
```

**Sample Output:**

```
Ranked dead repository candidates (5):
 1. ownerA/abandoned-lib              stars=12400  inactivity_years=4.23 language=Go
 2. ownerB/old-framework              stars=8900   inactivity_years=3.87 language=Go
 3. ownerC/deprecated-tool            stars=6200   inactivity_years=5.12 language=Go
```

---

### `necro autopsy`

Perform a detailed death-cause analysis on a specific repository.

```bash
necro autopsy <owner/repo> --years <N> [flags]
```

**Flags:**

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `<owner/repo>` | string | Yes | Target repository in `owner/repo` format |
| `--years` | int | Yes | Inactivity threshold context in years |
| `--since` | string | No | Evidence lower bound (RFC3339 or `YYYY-MM-DD`) |
| `--until` | string | No | Evidence upper bound (RFC3339 or `YYYY-MM-DD`) |
| `--max-items` | int | No | Maximum issues/PRs/commits to fetch (default: 200) |

**Example:**

```bash
necro autopsy owner/repo-name --years 3 --max-items 300
```

**Sample Output:**

```
Autopsy for owner/repo-name
Stars: 12400 | Last commit: 2021-03-15T10:30:00Z
Cause scores:
- maintainer_burnout score=0.72 confidence=0.65
- architecture_debt score=0.55 confidence=0.48
- governance_failure score=0.30 confidence=0.28
- ecosystem_displacement score=0.22 confidence=0.35
- funding_absence score=0.15 confidence=0.25
- security_trust_collapse score=0.10 confidence=0.20
- scope_drift score=0.08 confidence=0.22
Evidence indexed: 142
```

**Cause Score Taxonomy:**

| Cause | Description |
|-------|-------------|
| `maintainer_burnout` | Maintainer overwhelmed, no time, explicitly abandoned |
| `architecture_debt` | Legacy code, refactor needs, technical debt mentions |
| `ecosystem_displacement` | Superseded by newer framework, migration away |
| `security_trust_collapse` | CVE, vulnerability, security exploit |
| `governance_failure` | Maintainer conflict, bus factor, decision deadlock |
| `funding_absence` | No funding, sponsorship, sustainability issues |
| `scope_drift` | Scope creep, feature chaos, roadmap drift |

---

### `necro report`

Run the full end-to-end pipeline and generate complete report artifacts (autopsy + reincarnation plan).

```bash
necro report <owner/repo> [flags]
```

**Flags:**

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `<owner/repo>` | string | Yes | Target repository |
| `--format` | string | No | Output format: `markdown`, `json`, or `both` (default: `both`) |
| `--out` | string | No | Output directory (default: `./out` from config) |
| `--years` | int | No | Inactivity threshold (default: from config `analysis.default_years`) |
| `--since` | string | No | Evidence lower bound |
| `--until` | string | No | Evidence upper bound |
| `--max-items` | int | No | Maximum artifacts to fetch (default: from config) |
| `--target-stack` | string | No | Override target implementation stack |
| `--constraints` | string | No | Constraint text or file path for migration design |

**Example:**

```bash
necro report owner/repo-name --format both --target-stack "Rust + Actix + PostgreSQL + Docker"
```

**Generated Artifacts:**

```
out/
├── report.json       # Full structured report with all fields
├── report.md        # Human-readable Markdown summary
└── evidence-index.json  # Indexed evidence items
```

---

### `necro reborn`

Generate a focused 2026 reincarnation plan with architecture, migration steps, risks, and milestones.

```bash
necro reborn <owner/repo> [flags]
```

**Flags:**

| Flag | Type | Required | Description |
|------|------|----------|-------------|
| `<owner/repo>` | string | Yes | Target repository |
| `--target-stack` | string | No | Target implementation stack |
| `--constraints` | string | No | Constraint text or file path |

**Example:**

```bash
necro reborn owner/repo-name --target-stack "Go 1.26 + gRPC + Postgres + Kubernetes"
```

**Sample Output:**

```
Reborn plan for owner/repo-name
Target stack: Go 1.26 + gRPC + Postgres + Kubernetes
Architecture:
- Domain core: typed business rules and explicit invariants.
- Interface adapters: CLI/API boundary with strict input validation.
- Data layer: migration-safe persistence + cache invalidation controls.
- Observability: structured logs, trace IDs, budget telemetry.
- Security: permission gate around all external tool/network operations.
Migration:
- Week 1-2: freeze feature surface and codify compatibility contract.
- Week 3-4: implement modular core and adapter shells behind feature gates.
- Week 5-8: backfill parity tests + staged data migration.
- Week 9-12: canary rollout with stop-loss metrics and rollback playbook.
Milestones:
- Day 1-30: Stabilize architecture foundation
  Deliverables: Compatibility spec, Core module skeleton, Permission matrix
- Day 31-60: Complete migration-critical flows
  Deliverables: Feature parity map, Data migration rehearsal, Canary environment
- Day 61-90: Ship guarded production rollout
  Deliverables: Operational runbook, Stop-loss alarms, Public release notes
Risks:
- [high] Scope expansion beyond parity rewrite | stop-loss: Reject net-new features until parity baseline reaches 90%.
- [medium] Migration churn destabilizes users | stop-loss: Run compatibility layer with telemetry.
- [high] Maintainer bandwidth remains constrained | stop-loss: Define ownership map + rotate on-call before launch.
```

---

## Output Formats

### JSON Report Structure

```json
{
  "repository": "owner/repo-name",
  "snapshot_date": "2026-04-19T12:00:00Z",
  "death_threshold_years": 3,
  "stars": 12400,
  "last_commit_at": "2021-03-15T10:30:00Z",
  "core_philosophy": [
    "Pragmatic maintainability and clear developer workflow.",
    "Preserve original project purpose while modernizing execution model."
  ],
  "timeline": [
    {
      "timestamp": "2018-06-01T00:00:00Z",
      "title": "Repository created",
      "description": "Project initialized.",
      "source_ref": "https://github.com/owner/repo"
    }
  ],
  "cause_scores": [
    {
      "label": "maintainer_burnout",
      "score": 0.72,
      "confidence": 0.65,
      "evidence_refs": ["E003", "E015", "E042"],
      "counter_evidence": []
    }
  ],
  "evidence": [
    {
      "id": "E001",
      "type": "issue",
      "url": "https://github.com/owner/repo/issues/42",
      "title": "Maintainer burnout — can someone take over?",
      "timestamp": "2020-11-15T08:20:00Z",
      "summary": "I can no longer maintain this...",
      "relevance": 0.86
    }
  ],
  "reincarnation_plan": {
    "target_stack": "Go 1.26 + gRPC + Postgres + Kubernetes",
    "architecture": ["..."],
    "migration_plan": ["..."],
    "successor_signals": ["adoption growth", "issue closure velocity"]
  },
  "risks": [
    {
      "title": "Scope expansion beyond parity rewrite",
      "severity": "high",
      "stop_loss_action": "Reject net-new features until parity baseline reaches 90%."
    }
  ],
  "next_90_days": [
    {
      "day_range": "Day 1-30",
      "objective": "Stabilize architecture foundation",
      "deliverables": ["Compatibility spec", "Core module skeleton"]
    }
  ]
}
```

### Markdown Report

Markdown reports include:
- Executive summary with key metrics
- Cause-of-death analysis with confidence scores
- Evidence timeline
- Reincarnation plan with 90-day milestones
- Risk register with stop-loss actions

---

## Examples

### Complete Workflow

```bash
# 1. Set up environment
export GITHUB_TOKEN=ghp_your_token
export DASHSCOPE_API_KEY=your_dashscope_key

# 2. Discover candidate dead repos
necro scan --years 3 --min-stars 5000 --language Python --limit 20

# 3. Autopsy a specific candidate
necro autopsy someuser/some-repo --years 3

# 4. Generate full report with reincarnation plan
necro report someuser/some-repo --format both --out ./reports

# 5. Generate focused reincarnation plan only
necro reborn someuser/some-repo --target-stack "Rust + Actix" --constraints ./constraints.txt
```

### Using a Constraints File

```bash
# constraints.txt
Must maintain backward API compatibility.
Target deployment: Kubernetes on AWS.
Team size: 2 engineers.
Budget: $0 (open-source only).
```

```bash
necro reborn owner/repo --constraints ./constraints.txt
```

### Pipeline with Custom Config

```bash
export NECRO_CONFIG=/path/to/custom-config.yaml
necro report owner/repo --format json --out /tmp/necro-reports
```

---

## License

MIT License — see [LICENSE](LICENSE) for details.

---

## Contributing

Contributions are welcome. Please open an issue or submit a pull request with improvements.

## Support

For questions and issues, please open a GitHub issue at [https://github.com/repo-necromancer/necro](https://github.com/repo-necromancer/necro).
