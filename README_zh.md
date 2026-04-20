# Repo Necromancer

[![Go Version](https://img.shields.io/badge/Go-1.26.2-blue)](https://golang.org/)
[![License: MIT](https://img.shields.io/badge/License-MIT-yellow.svg)](https://opensource.org/licenses/MIT)
[![Platform](https://img.shields.io/badge/Platform-macOS%20%7C%20Linux%20%7C%20Windows-green.svg)](https://github.com/Arisgod1/RepoNecromancer)

**Repo Necromancer** 是一款由 AI 驱动的 CLI 工具，用于对已废弃的 GitHub 仓库进行“尸检”，判定其“死亡原因”，并生成详细的“重生”计划，帮助项目重获新生。

---

## 目录

- [功能](#功能)
- [架构](#架构)
- [安装](#安装)
- [配置](#配置)
- [环境变量](#环境变量)
- [使用方法](#使用方法)
  - [`necro scan`](#necro-scan)
  - [`necro autopsy`](#necro-autopsy)
  - [`necro report`](#necro-report)
  - [`necro reborn`](#necro-reborn)
- [输出格式](#输出格式)
- [示例](#示例)

---

## 功能

- **仓库扫描** —— 根据不活跃阈值、Star 数、语言和主题发现被废弃的仓库
- **死亡原因尸检** —— 深度分析 issues、PRs 与 commits，判断仓库衰亡原因
- **原因评分分类体系** —— 七类失效模式：维护者倦怠、架构债务、生态位替代、安全信任崩塌、治理失效、资金缺失、范围漂移
- **AI 增强分析** —— 可选集成 DashScope LLM，提供更丰富的原因评分与重生规划
- **重生计划** —— 结构化 90 天复兴蓝图，包含架构建议、迁移步骤、风险与里程碑
- **可扩展工具注册表** —— 内置 GitHub 与 Web 工具，并支持加载自定义扩展
- **权限引擎** —— 可配置域名/IP 白名单，保障工具执行安全
- **多种输出格式** —— 支持 JSON 与 Markdown 报告产物

---

## 架构

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
                    │  Query Engine  │  ← 受预算控制的执行
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
                    │   LLM Client       │  ← DashScope（可选）
                    │  (qwen3.6-plus)    │
                    └────────────────────┘
```

### 组件概览

| 组件 | 职责 |
|-----------|----------------|
| **CLI Commands** (`scan`, `autopsy`, `report`, `reborn`) | 面向用户的入口命令 |
| **Query Engine** | 在预算限制（轮次、Token、成本）下执行工具操作 |
| **Tool Registry** | 管理内置 GitHub 工具、Web 工具与已加载扩展 |
| **Permissions Engine** | 执行域名/IP 白名单与私有网络拒绝策略 |
| **Network Client** | 为工具执行提供带重试/退避的 HTTP 客户端 |
| **LLM Client** | DashScope API 客户端，用于 AI 增强原因评分与重生规划 |
| **Memory Store** | 跨命令调用的内存状态管理 |
| **Report Renderer** | 生成 JSON 与 Markdown 产物文件 |

---

## 安装

### 前置要求

- **Go 1.26.2+**（若从源码构建）
- **GitHub Personal Access Token**（`GITHUB_TOKEN`）—— 用于仓库 API 访问
- **DashScope API Key**（`DASHSCOPE_API_KEY`）—— 用于 AI 增强功能（可选）

### 从源码构建

```bash
git clone https://github.com/Arisgod1/RepoNecromancer.git
cd necro
go build -o necro ./cmd/necro
```

### 使用预构建二进制

从 Releases 页面下载与你的平台匹配的二进制，并赋予可执行权限：

```bash
chmod +x necro
./necro --help
```

---

## 配置

Repo Necromancer 使用 YAML 配置文件。默认按以下顺序查找 `config.yaml`：

1. `NECRO_CONFIG` 环境变量指定的路径
2. `./config.yaml`（当前目录）
3. `./configs/config.yaml`

### 默认 `config.yaml`

```yaml
app:
  log_level: info
  output_dir: ./out
  cache_dir: ./.cache/necro

analysis:
  default_years: 3      # 扫描的默认不活跃阈值
  min_stars: 5000       # 候选仓库最小 Star 数
  max_items: 500        # 最多抓取的 issues/PRs/commits 数量

query:
  max_turns: 16         # 单次会话最大工具调用轮次
  max_tokens: 0         # 0 = 不限
  max_cost: 0           # 0 = 不限（USD）

network:
  timeout_ms: 12000     # HTTP 请求超时时间（毫秒）
  retry_max: 3          # 最大重试次数
  backoff_base_ms: 300   # 基础退避时长（每次重试翻倍）
  allow_domains:
    - github.com
    - api.github.com
  block_domains: []
  deny_private_networks: true

permissions:
  mode: default        # default | plan | dontAsk | bypass | acceptEdits | auto

tools:
  deny: []               # 需禁用的工具名列表

llm:
  model: qwen3.6-plus
  api_base: https://dashscope.aliyuncs.com/compatible-mode/v1
  timeout_seconds: 300
```

### 配置优先级

环境变量会覆盖配置文件中的值。环境变量统一使用 `NECRO_` 前缀：

| 配置键 | 环境变量 |
|------------|---------------------|
| `llm.model` | `NECRO_LLM_MODEL` |
| `llm.api_base` | `NECRO_LLM_API_BASE` |
| `app.output_dir` | `NECRO_APP_OUTPUT_DIR` |
| `app.log_level` | `NECRO_APP_LOG_LEVEL` |

### 权限模式说明

| 模式 | 描述 |
|------|-------------|
| `default` | 仅执行指向明确允许域名/IP 的工具（默认，推荐） |
| `plan` | 执行任何工具前都请求确认 |
| `dontAsk` | 执行工具时不请求确认（跳过提示） |
| `bypass` | 绕过所有权限检查并直接执行 |
| `acceptEdits` | 自动接受文件修改而不提示 |
| `auto` | 自动判定最佳行为 |

---

## 环境变量

| 变量 | 必需 | 描述 |
|----------|----------|-------------|
| `GITHUB_TOKEN` | 建议 | 用于 API 访问的 GitHub Personal Access Token。若缺失，将触发更低速率限制。 |
| `DASHSCOPE_API_KEY` | 可选 | 用于 AI 增强分析与重生规划的 DashScope API Key。 |
| `DASHSCOPE_MODEL` | 否 | 覆盖默认 LLM 模型（`qwen3.6-plus`）。 |
| `DASHSCOPE_API_BASE` | 否 | 覆盖 DashScope API 基础 URL。 |
| `NECRO_CONFIG` | 否 | 自定义 YAML 配置文件路径。 |

### 获取 GitHub Token

1. 打开 [GitHub Settings → Personal access tokens](https://github.com/settings/tokens)
2. 生成新 Token（classic）
3. 选择权限范围：`repo`（私有仓库）或 `public_repo`（仅公开仓库）
4. 复制 Token，并设置为 `GITHUB_TOKEN`

### 获取 DashScope API Key

在 [阿里云 DashScope](https://dashscope.aliyuncs.com/) 注册并生成 API Key。

---

## 工具

Repo Necromancer 提供用于 GitHub API 访问与网页抓取的内置工具。所有工具均由 Query Engine 在预算控制（轮次、Token、成本）下执行。

### GitHub 工具（`github.*`）

| 工具 | 描述 | 所需权限 |
|------|-------------|---------------------|
| `github.search_repos` | 按查询条件、语言、Star、最近活跃度搜索仓库 | `github.com`, `api.github.com` |
| `github.get_repo` | 获取仓库元数据与统计信息 | `github.com`, `api.github.com` |
| `github.list_issues` | 列出 issues（含状态、标签与时间线） | `github.com`, `api.github.com` |
| `github.list_pulls` | 列出 pull requests（含评审状态） | `github.com`, `api.github.com` |
| `github.get_commits` | 获取提交历史（含作者与时间戳） | `github.com`, `api.github.com` |
| `github.list_collaborators` | 列出仓库协作者与权限 | `github.com`, `api.github.com` |

### Web 工具（`web.*`）

| 工具 | 描述 | 所需权限 |
|------|-------------|---------------------|
| `web.fetch` | 对 URL 发起 HTTP GET 并返回内容 | 与 `allow_domains` 匹配的条目 |
| `web.search` | 通过已配置端点执行 Web 搜索 | 与 `allow_domains` 匹配的条目 |

### 权限级别

工具会根据 `permissions.mode` 受 Permissions Engine 管控：

| 模式 | 行为 |
|------|----------|
| `default` | 仅当目标域名/IP 在 `network.allow_domains` 中时执行工具（默认，推荐） |
| `plan` | 执行任何工具前都请求确认 |
| `dontAsk` | 执行工具时不请求确认（跳过提示） |
| `bypass` | 绕过所有权限检查并直接执行 |
| `acceptEdits` | 自动接受文件修改而不提示 |
| `auto` | 自动判定最佳行为 |

### 禁用工具

使用 `tools.deny` 按名称禁用特定工具：

```yaml
tools:
  deny:
    - github.list_collaborators
    - web.search
```

---

## 可观测性

Repo Necromancer 会输出结构化日志字段，并为所有工具执行与 LLM 调用维护审计轨迹。

### 结构化日志字段

| 字段 | 类型 | 描述 |
|-------|------|-------------|
| `trace_id` | string | 会话内所有事件共享的唯一执行 ID |
| `session_id` | string | CLI 调用会话标识 |
| `command` | string | 顶层命令（`scan`, `autopsy`, `report`, `reborn`） |
| `target` | string | 目标仓库（`owner/repo`） |
| `tool_name` | string | 被调用的工具 |
| `tool_domain` | string | 工具调用的目标域名 |
| `allowed` | bool | 权限引擎是否允许该调用 |
| `turn` | int | Query Engine 预算中的当前轮次 |
| `tokens_used` | int | LLM 会话累计 Token 使用量 |
| `cost_usd` | float | 累计美元成本 |

### 审计轨迹

每次命令运行都会产出一条 `audit.log` 记录（写入 `cache_dir/audit/`）：

```
{"ts":"2026-04-19T13:45:00Z","trace_id":"abc123","session":"sess-001","command":"autopsy","target":"owner/repo","tool":"github.list_issues","allowed":true,"turn":3,"ms":142}
{"ts":"2026-04-19T13:45:01Z","trace_id":"abc123","session":"sess-001","command":"autopsy","target":"owner/repo","tool":"github.list_issues","allowed":true,"turn":4,"ms":89}
```

### 日志级别配置

| 级别 | 使用场景 |
|-------|----------|
| `error` | 生产环境；仅记录错误与被拒绝的工具调用 |
| `info` | 默认；包含命令进度与工具摘要 |
| `debug` | 开发环境；包含所有工具输入/输出与 LLM 提示/响应 |
| `trace` | 详细模式；完整 HTTP 请求/响应体与权限检查 |

---

## 使用方法

### `necro scan`

按不活跃和热度标准发现候选“死亡”仓库。

```bash
necro scan --years <N> --min-stars <N> [flags]
```

**参数：**

| 参数 | 类型 | 必需 | 描述 |
|------|------|----------|-------------|
| `--years` | int | 是 | 不活跃阈值（年） |
| `--min-stars` | int | 是 | 最小 Star 数 |
| `--language` | string | 否 | 按编程语言筛选（如 `Go`、`Python`） |
| `--topic` | string[] | 否 | 按 GitHub Topic 筛选（可重复） |
| `--limit` | int | 否 | 返回结果上限（默认：20，最大：100） |

**示例：**

```bash
export GITHUB_TOKEN=ghp_your_token_here
necro scan --years 3 --min-stars 5000 --language Go --limit 10
```

**示例输出：**

```
排序后的死亡仓库候选（5）：
 1. ownerA/abandoned-lib              stars=12400  inactivity_years=4.23 language=Go
 2. ownerB/old-framework              stars=8900   inactivity_years=3.87 language=Go
 3. ownerC/deprecated-tool            stars=6200   inactivity_years=5.12 language=Go
```

---

### `necro autopsy`

对指定仓库执行详细的死亡原因分析。

```bash
necro autopsy <owner/repo> --years <N> [flags]
```

**参数：**

| 参数 | 类型 | 必需 | 描述 |
|------|------|----------|-------------|
| `<owner/repo>` | string | 是 | 目标仓库，格式为 `owner/repo` |
| `--years` | int | 是 | 不活跃阈值上下文（年） |
| `--since` | string | 否 | 证据时间下界（RFC3339 或 `YYYY-MM-DD`） |
| `--until` | string | 否 | 证据时间上界（RFC3339 或 `YYYY-MM-DD`） |
| `--max-items` | int | 否 | 最多抓取 issues/PRs/commits 数量（默认：200） |

**示例：**

```bash
necro autopsy owner/repo-name --years 3 --max-items 300
```

**示例输出：**

```
owner/repo-name 的尸检结果
Stars: 12400 | 最近提交: 2021-03-15T10:30:00Z
原因评分：
- maintainer_burnout score=0.72 confidence=0.65
- architecture_debt score=0.55 confidence=0.48
- governance_failure score=0.30 confidence=0.28
- ecosystem_displacement score=0.22 confidence=0.35
- funding_absence score=0.15 confidence=0.25
- security_trust_collapse score=0.10 confidence=0.20
- scope_drift score=0.08 confidence=0.22
已索引证据：142
```

**原因评分分类体系：**

| 原因 | 描述 |
|-------|-------------|
| `maintainer_burnout` | 维护者精力透支、无暇维护、明确声明弃坑 |
| `architecture_debt` | 旧代码负担、需要重构、技术债相关表述 |
| `ecosystem_displacement` | 被更新框架替代、生态迁移导致边缘化 |
| `security_trust_collapse` | CVE、漏洞或安全事件导致信任受损 |
| `governance_failure` | 维护者冲突、Bus Factor 低、决策僵局 |
| `funding_absence` | 缺乏资金、赞助或可持续性问题 |
| `scope_drift` | 范围蔓延、功能混乱、路线图偏离 |

---

### `necro report`

执行完整端到端流水线并生成完整报告产物（尸检 + 重生计划）。

```bash
necro report <owner/repo> [flags]
```

**参数：**

| 参数 | 类型 | 必需 | 描述 |
|------|------|----------|-------------|
| `<owner/repo>` | string | 是 | 目标仓库 |
| `--format` | string | 否 | 输出格式：`markdown`、`json` 或 `both`（默认：`both`） |
| `--out` | string | 否 | 输出目录（默认：配置中的 `./out`） |
| `--years` | int | 否 | 不活跃阈值（默认：配置 `analysis.default_years`） |
| `--since` | string | 否 | 证据时间下界 |
| `--until` | string | 否 | 证据时间上界 |
| `--max-items` | int | 否 | 抓取产物上限（默认：配置值） |
| `--target-stack` | string | 否 | 覆盖目标实现技术栈 |
| `--constraints` | string | 否 | 迁移设计约束文本或文件路径 |

**示例：**

```bash
necro report owner/repo-name --format both --target-stack "Rust + Actix + PostgreSQL + Docker"
```

**生成产物：**

```
out/
├── report.json       # 包含全部字段的完整结构化报告
├── report.md        # 面向阅读的 Markdown 摘要
└── evidence-index.json  # 已索引证据项
```

---

### `necro reborn`

生成聚焦 2026 的重生计划，涵盖架构、迁移步骤、风险与里程碑。

```bash
necro reborn <owner/repo> [flags]
```

**参数：**

| 参数 | 类型 | 必需 | 描述 |
|------|------|----------|-------------|
| `<owner/repo>` | string | 是 | 目标仓库 |
| `--target-stack` | string | 否 | 目标实现技术栈 |
| `--constraints` | string | 否 | 约束文本或文件路径 |

**示例：**

```bash
necro reborn owner/repo-name --target-stack "Go 1.26 + gRPC + Postgres + Kubernetes"
```

**示例输出：**

```
owner/repo-name 的重生计划
目标技术栈：Go 1.26 + gRPC + Postgres + Kubernetes
架构：
- 领域核心：类型化业务规则与显式不变量。
- 接口适配层：CLI/API 边界，执行严格输入校验。
- 数据层：可安全迁移的持久化 + 缓存失效控制。
- 可观测性：结构化日志、trace ID、预算遥测。
- 安全：对所有外部工具/网络操作设置权限闸门。
迁移：
- 第 1-2 周：冻结功能表面并固化兼容性契约。
- 第 3-4 周：在特性开关后实现模块化核心与适配层外壳。
- 第 5-8 周：补齐一致性测试并分阶段执行数据迁移。
- 第 9-12 周：以金丝雀发布推进，并配套止损指标与回滚手册。
里程碑：
- Day 1-30：稳定架构基础
  交付物：兼容性规范、核心模块骨架、权限矩阵
- Day 31-60：完成迁移关键流程
  交付物：功能对等映射、数据迁移演练、金丝雀环境
- Day 61-90：发布受控生产上线
  交付物：运维 Runbook、止损告警、公开发布说明
风险：
- [high] 对等重写之外的范围扩张 | 止损：在对等基线达到 90% 前拒绝新增功能。
- [medium] 迁移波动影响用户稳定性 | 止损：运行带遥测的兼容层。
- [high] 维护者带宽仍受限 | 止损：上线前明确所有权映射并轮值 on-call。
```

---

## 输出格式

### JSON 报告结构

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

### Markdown 报告

Markdown 报告包含：
- 关键指标执行摘要
- 带置信度分数的死亡原因分析
- 证据时间线
- 包含 90 天里程碑的重生计划
- 带止损动作的风险清单

---

## 示例

### 完整工作流

```bash
# 1. 设置环境
export GITHUB_TOKEN=ghp_your_token
export DASHSCOPE_API_KEY=your_dashscope_key

# 2. 发现候选死亡仓库
necro scan --years 3 --min-stars 5000 --language Python --limit 20

# 3. 对某个候选仓库进行尸检
necro autopsy someuser/some-repo --years 3

# 4. 生成包含重生计划的完整报告
necro report someuser/some-repo --format both --out ./reports

# 5. 仅生成聚焦重生计划
necro reborn someuser/some-repo --target-stack "Rust + Actix" --constraints ./constraints.txt
```

### 使用约束文件

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

### 使用自定义配置运行流水线

```bash
export NECRO_CONFIG=/path/to/custom-config.yaml
necro report owner/repo --format json --out /tmp/necro-reports
```

## 变更日志

本文档记录本项目的所有重要变更。

格式基于 [Keep a Changelog](https://keepachangelog.com/zh-CN/1.1.0/)，本项目遵循 [语义化版本](https://semver.org/lang/zh-CN/spec/v2.0.0.html)。

### [未发布]

#### 新增

- **i18n 国际化** (`f0d5a68`) — 中英文报告输出，默认中文
  - 新增 internal/i18n/ 目录，含 zh-CN.json 和 en-US.json 两套翻译
  - renderer 所有标题和标签支持中英文切换
  - 新增 --lang flag 和 config.yaml language 配置项
- **cmd/necro/main.go 入口点** (`5028355`) — 关键修复：磁盘上存在但从未被 git track
- **大仓库内存模式** (`0f81de8`) — --mode full/sample/lite 三种模式，--max-evidence flag，min-heap 流式处理
- **并行启动** (`23aaeaa`) — errgroup 并行 GitHub API 调用，parallel LLM 推理，worker pool 扫描
- **TTL + LRU 缓存** (`1f24ded`) — MemoryStore 新增 TTL 过期和 LRU 驱逐策略
- **Extension 接口** (`9048ded`) — Subscribe() 方法支持 5 种生命周期事件订阅
- **PDF 导出** (`d87b37c`) — gofpdf 支持纯 Go PDF 生成（format=pdf, pdf+markdown, both）
- **故障模拟测试** (`7a1ef7b`) — TestPermissionDenial、TestBudgetExhaustion、TestCacheDegradation、TestLLMGracefulDegradation
- **单元测试** (`6ea34b4`) — logging/state/tools/extensions 单元测试（覆盖率 27.7%-90.9%）

#### 修复

- **持久化缓存** (`b5c1ef1`) — GlobalCache() 改为文件持久化（~/.cache/necro/cache.data）
- **clone URL** (`7ad9705`) — 修复 README clone URL 和二进制路径说明

#### 文档

- **README 更新** (`2da5214`) — 缓存、large-repo mode、PDF 导出、EventBus 文档

---

## 许可证

MIT License —— 详见 [LICENSE](LICENSE)。

---

## 贡献

欢迎贡献。请通过 issue 或 pull request 提交改进建议。

## 支持

如有问题，请在 GitHub 提交 issue：[https://github.com/Arisgod1/RepoNecromancer](https://github.com/Arisgod1/RepoNecromancer)。
