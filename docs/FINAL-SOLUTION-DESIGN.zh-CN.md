# Agent Review Workflow 最终方案设计

> 状态：已确认的目标架构，供后续实现窗口直接执行。  
> 版本：v2 设计稿。  
> 重要：仓库当前的 PowerShell/Shell 脚本是 v1；本文描述的任务台账、MCP 服务和 VS Code 扩展尚未实现。不要把本文的目标能力误认为现有能力。

## 1. 目标与边界

Agent Review Workflow（ARW）让开发者以自然语言管理 AI 编写代码的任务，并在 IDE 中高效审查完整变更。

它要解决的是：AI 可能很快地改了许多代码，但人需要始终清楚知道“哪个任务改了什么、相对什么基准改、能否审、依赖谁、审查结论是否仍有效”。

### 1.1 必须达成的结果

- 以**任务**而不是日期或临时分支作为工作单元。
- 每个任务保存分支、基准提交、依赖任务、生命周期、测试证据和审查快照。
- 支持独立任务和堆叠任务（例如 `main <- A <- B`）。
- 在 Codex、Claude Code、OpenCode 中用自然语言触发受限的工作流操作。
- 在 Windows 和 Linux 上使用同一个短命令 `arw`，且不依赖保留本地源码仓库。
- 在 VS Code 中提供任务列表、依赖关系、正确基准下的文件列表和逐文件 diff。
- 默认不打断正在使用的 VS Code 窗口；需要时才在新窗口打开另一个任务。
- 审查、合并、推送和代码改写都有可审计、可确认的边界。

### 1.2 明确不做的事

- 不让 AI 仅凭自身判断自动合并、推送、rebase、reset、强推、删除分支或删除 worktree。
- 不提供 `run_arw("任意 shell 文本")` 这类泛化的 Agent 工具。
- 不用鼠标/屏幕自动化操纵 VS Code 内部 UI。
- v2 第一阶段不支持 vscode.dev、github.dev 或浏览器版 VS Code；这些环境不能可靠启动本地 `arw` 可执行文件。
- 不把 `.env`、令牌、数据库密码、完整源代码 diff 上传到中心服务；系统是本地优先的。

## 2. 总体架构

```text
用户的自然语言
        |
Codex / Claude Code / OpenCode
        |
统一规则与各自的 Skill/适配配置
        |
ARW MCP（仅暴露结构化、受限的任务工具）
        |
arw 核心程序（唯一的任务与 Git 业务实现）
        |--------------------|
        |                    |
Git / worktree         任务台账元数据分支
        ^                    ^
        |                    |
VS Code 扩展 ----------- JSON 输出 / CLI API
```

### 2.1 技术决策

| 层 | 技术 | 决策理由 |
| --- | --- | --- |
| 核心 CLI `arw` | Go | Windows/Linux 独立二进制、安装简单、适合可靠调度 Git 命令。 |
| MCP 服务 `arw-mcp` | Go | 与核心共享同一状态机和校验，避免再维护一套 Node 业务逻辑。 |
| VS Code 扩展 | TypeScript | VS Code 扩展 API 的原生生态。 |
| 安装器 | PowerShell + POSIX Shell | 仅下载、校验、配置和迁移；不承载业务逻辑。 |
| 层间契约 | JSON Schema | CLI、MCP、扩展和测试共同使用的稳定数据模型。 |

语言不是安全边界。避免错误 Git 操作依靠严格的命令白名单、状态校验、明确确认与审计记录，而不是依靠某种语言本身。

## 3. 关键概念

| 名称 | 含义 |
| --- | --- |
| 任务（Task） | 一个可独立理解、测试和审查的工作单元。 |
| 基准（Base） | 创建或审查任务时使用的具体 Git 提交，而不仅是 `main` 这个会移动的名字。 |
| 父任务（Parent） | 当前任务直接建立在其上的任务，例如 B 建立在 A 上。 |
| 审查快照（Review Snapshot） | 某次审查时的 `base SHA`、`head SHA`、文件统计、测试证据及结论。 |
| 条件审查 | B 相对未最终验收的 A 所做的审查；不能等同于 B 相对 `main` 的最终验收。 |
| 搁置（Parked） | 暂不推进但保留任务和历史；与放弃不同。 |
| Worktree | 同一 Git 仓库的独立工作目录，用来同时保留不同任务的代码现场。 |

## 4. 任务模型与状态机

不要把所有信息压进一个字符串状态，也不要仅根据分支是否存在来推断待审任务。任务有三个独立维度。

### 4.1 生命周期

```text
active -> ready_for_review -> in_review -> approved -> merged
   |              |                |             |
   +------------> parked <---------+-------------+
   |
   +------------> abandoned
```

| 值 | 含义 |
| --- | --- |
| `active` | 正在开发或等待继续开发。 |
| `ready_for_review` | 工作区干净、已生成审查材料，等待人工审查。 |
| `in_review` | 人正在审查。 |
| `approved` | 人明确确认通过，但尚未合并。 |
| `parked` | 暂不推进；可带有已审或未审结论。 |
| `merged` | 已确认进入目标分支。 |
| `abandoned` | 明确不再需要；保留历史和原因。 |

### 4.2 审查结论

| 值 | 含义 |
| --- | --- |
| `none` | 尚未有人记录审查结论。 |
| `changes_requested` | 审查要求修改。 |
| `conditional` | 相对父任务审过，但父任务尚未最终验收。 |
| `approved` | 对本次基准和 HEAD 的人工审查通过。 |
| `stale` | 代码、基准或父任务已变；旧结论不可再视为当前结论。 |

### 4.3 依赖状态

依赖状态是派生值，不要求人工维护。

| 值 | 含义 |
| --- | --- |
| `clear` | 没有阻塞当前最终审查的依赖。 |
| `awaiting_prerequisite_review` | 父任务未最终审查/未合入。 |
| `parent_changed` | 父任务在当前任务审查后发生变化，必须复核。 |
| `blocked` | 其他显式依赖未满足。 |

### 4.4 任务记录示例

任务记录必须是可读、可版本化且易于迁移的 YAML。每个任务一个文件，避免多人同时编辑一个总索引。

```yaml
schema_version: 1
id: fix-login-redirect
title: 修复登录跳转
kind: bugfix
branch: arw/fix-login-redirect
base:
  ref: main
  sha: 1a2b3c4d
parent_task: null
lifecycle: ready_for_review
review:
  status: none
  reviewed_base_sha: null
  reviewed_head_sha: null
dependencies: []
tests:
  - command: npm test
    result: passed
created_at: 2026-08-12T09:00:00+08:00
updated_at: 2026-08-12T11:00:00+08:00
```

## 5. 台账存储与同步

### 5.1 采用专用元数据分支

任务记录保存在同一仓库的专用分支 `arw/registry` 中，例如：

```text
.agent-review/
  tasks/
    fix-login-redirect.yaml
    add-search.yaml
  schema-version.json
```

理由：

- 任务元数据不污染 `main` 或功能分支的产品代码提交。
- 元数据可随 Git 备份、同步到另一台机器并恢复。
- 每个任务独立文件，冲突范围较小。
- CLI 可通过读取该分支列出所有任务，而不需要从模糊的分支名猜测状态。

`arw` 内部维护该分支；用户无需日常 checkout 它。元数据的发布和同步必须是明确动作，不能被 Agent 静默推送。

### 5.2 本地状态

本机特有信息放在 Git 忽略的位置，例如 `.git/arw/`：

- worktree 的绝对路径；
- 当前工作区占用锁；
- 安装与诊断缓存；
- 不含代码、密钥或环境文件的临时审查产物。

台账永远不保存 `.env` 内容、令牌或完整源代码正文。

## 6. 分支与依赖策略

### 6.1 独立任务

```text
main
 ├─ arw/fix-login-redirect
 └─ arw/fix-database
```

两个任务均从创建时记录的 `main@SHA` 开始，彼此独立，可按任意顺序审查。

### 6.2 堆叠任务

当 B 必须使用 A 中尚未合入的接口或代码时：

```text
main <- arw/feature-a <- arw/feature-b
```

规则：

1. 审查 B 时，比较 `A...B`，只看 B 相对 A 的改动。
2. 如果 A 尚未验收，B 的结论只能是 `conditional`。
3. 建议先审 A；用户仍可自由先审 B。
4. 默认不把 B 合入 A。先合 A 到 `main`，再将 B 更新到新的 `main`，最后对 B 做最终审查。
5. 只有用户明确说明 A、B 必须作为同一次交付时，才把它们合并为一个工作单元。

### 6.3 审查失效规则

以下任意情形发生，相关审查快照转为 `stale`：

- 任务 HEAD 改变；
- 审查基准 SHA 改变；
- 父任务在子任务的条件审查之后改变；
- 任务被 rebase 到不同基准；
- 合并冲突解决产生了新的代码。

系统可提示“快速复核”或“完整复核”，但不得把旧批准自动视为对新代码的批准。

## 7. Worktree 与并发规则

建议目录布局：

```text
C:\code\my-app
C:\code\my-app-worktrees\
  fix-login-redirect\
  fix-database\
```

规则：

- 一个任务对应一个 worktree；一个 worktree 同一时刻只允许一个写入 Agent。
- `arw task start` 在当前项目的配置位置创建任务 worktree。
- 当前 VS Code 窗口通常就是当前任务的 worktree；不会因审查其他任务被切走。
- `arw worktree open <task>` 只打开目标任务，不切换或覆盖原窗口。
- 不自动 stash 未提交改动。开始新任务或切换上下文时，如工作区不干净，必须报告并要求用户决定。
- `.env`、数据库、端口、生成依赖不由 Git worktree 自动复制。任务可声明运行前置条件，避免不同 worktree 争用同一数据库或端口。

## 8. 用户自然语言与动作契约

自然语言由 Agent 解析，但每种意图只能映射到一个受限的结构化工具，而非任意 shell 命令。

| 用户表达 | 默认动作 | 不会发生的动作 |
| --- | --- | --- |
| “开始修复登录跳转” | 创建/定位任务、分支、worktree、基准记录。 | 不会改 `main`、不会推送。 |
| “有哪些待审任务？” | 列出 `ready_for_review` 与 `in_review`，并标出依赖阻塞。 | 不把所有分支混入结果。 |
| “审查登录跳转” | 生成审查快照和对话摘要。 | 不切换 VS Code、不合并。 |
| “在 VS Code 中审查数据库修复” | 生成审查快照，并在新窗口打开目标 worktree/审查视图。 | 不抢占当前编码窗口。 |
| “先搁置功能 A” | 记录为 `parked`，保留审查结论。 | 不删除分支或代码。 |
| “A 审查通过” | 复述任务、基准、HEAD 和影响；确认后记录人工批准。 | 不自动 merge/push。 |

当用户要求审查 B，而 B 依赖尚未审查的 A 时，Agent 必须先提示：

```text
B 依赖 A；推荐先审 A。
如继续审 B，本次只比较 A...B，结论将是条件审查，不能替代最终验收。
```

这只是建议，不是强制。用户有权先审 B。

## 9. `arw` CLI 和机器接口

面向人类的命令保持简短；面向扩展和 MCP 的输出使用稳定 JSON。

```text
arw setup
arw doctor
arw update

arw task start "修复登录跳转"
arw task list [--view reviewable|active|parked|blocked]
arw task show <task-id>
arw task park <task-id>
arw task resume <task-id>

arw review prepare <task-id>
arw review status <task-id>
arw review approve <task-id>
arw review request-changes <task-id>

arw worktree open <task-id>
arw refresh
```

所有可供程序消费的子命令支持：

```text
--format json
```

例如 `arw review prepare fix-login-redirect --format json` 至少返回：

```json
{
  "taskId": "fix-login-redirect",
  "base": { "ref": "main", "sha": "1a2b3c4d" },
  "head": { "sha": "7d8e9f00" },
  "comparison": "main@1a2b3c4d...HEAD@7d8e9f00",
  "files": [],
  "commits": [],
  "workingTree": "clean",
  "tests": [],
  "dependencyStatus": "clear",
  "reviewStatus": "none",
  "risks": []
}
```

## 10. MCP 与多 Agent 接入

`arw-mcp` 是本机 stdio MCP 服务。它包装核心库，不复制 Git 逻辑。

允许暴露的工具：

```text
workflow_context()
workflow_list_tasks(filter)
workflow_start_task(title, parent_task?)
workflow_prepare_review(task_id)
workflow_open_review(task_id, new_window)
workflow_park_task(task_id)
workflow_resume_task(task_id)
workflow_refresh()
```

不暴露的工具：

```text
任意 shell 执行
git push
git merge
git rebase
git reset
git branch --delete
删除 worktree
```

人工审查结论的记录属于高影响动作。只有用户明确表达“通过”“要求修改”“搁置”等意图时，Agent 才能调用相应的受限操作；最佳 UI 是在 VS Code 扩展中由用户点击确认。

各 Agent 的适配层负责：

- 安装统一的安全规则和自然语言映射；
- 配置本地 `arw-mcp`；
- 告知 Agent：先读取任务上下文，再写代码或准备审查；
- 使 Codex、Claude Code、OpenCode 调用同一组工具和同一状态机。

## 11. VS Code 扩展设计

扩展名称：`Agent Review Workflow`。

### 11.1 MVP 功能

在活动栏提供以下视图：

```text
Agent Review
├─ 待审查
├─ 进行中
├─ 依赖阻塞
├─ 已搁置
└─ 已完成
```

选中任务显示：

- 任务名称、分支、基准 SHA、HEAD SHA；
- 生命周期、审查结论、依赖状态；
- 文件变更统计和提交列表；
- 测试证据、风险提示、父/子任务关系；
- “相对谁比较”的明确文字。

点击一个变更文件时，扩展使用 VS Code 的原生左右 diff 编辑器显示正确基准与任务 HEAD 的差异。颜色遵循用户当前主题；扩展不修改全局 diff 配色。

### 11.2 命令

```text
ARW: 审查当前任务
ARW: 打开任务审查
ARW: 在新窗口打开任务
ARW: 查看任务依赖图
ARW: 标记任务为搁置
ARW: 记录审查通过
ARW: 请求修改
```

“打开任务审查”必须：

1. 调用 `arw review prepare` 取得真实比较范围；
2. 如果该任务不是当前 worktree，则新开 VS Code 窗口；
3. 打开 ARW 的任务详情和变更文件树；
4. 不切换现有窗口的分支，不模拟鼠标点击。

第一阶段只支持桌面 VS Code。浏览器版扩展不能启动本地子进程，后续如需支持，需要采用远程服务/远程扩展宿主方案。

## 12. 安全、确认与审计

### 12.1 安全级别

| 级别 | 示例 | 策略 |
| --- | --- | --- |
| 只读 | 列表、状态、diff、诊断 | 可直接执行。 |
| 本地可逆 | 创建任务分支/worktree、搁置、恢复、准备审查 | 必须来自用户的明确任务意图。 |
| 人工结论 | 通过、要求修改、放弃 | 必须有用户明确表达或 VS Code 点击确认。 |
| 高风险/远程 | merge、push、rebase、reset、删除 | 不由 MCP 暴露；保留给用户直接执行。 |

### 12.2 审计内容

审查快照记录：

- 任务 ID、操作者标识、时间；
- base SHA、head SHA、父任务状态；
- 变更文件/提交统计；
- 已运行测试的命令、退出码和摘要；
- 审查结论与原因；
- 结论何时、为何失效。

不记录源代码全文、密钥和环境变量。

## 13. 分发、安装和升级

用户不应依赖任意本地克隆目录。发布物包括：

```text
GitHub Release
├─ arw_windows_amd64.exe
├─ arw_linux_amd64
├─ arw_linux_arm64
├─ arw-mcp_...
├─ agent-review-workflow-<version>.vsix
└─ checksums.txt / provenance
```

安装脚本仅：

1. 检测平台与架构；
2. 下载指定版本的发布物；
3. 校验 SHA-256；
4. 放入用户数据目录并把 `bin` 加入 PATH；
5. 写入 Codex、Claude Code、OpenCode 的受控配置块；
6. 安装或提示安装 VS Code 扩展。

运行时位置：

| 平台 | 位置 |
| --- | --- |
| Windows | `%LOCALAPPDATA%\\AgentReviewWorkflow` |
| Linux | `$XDG_DATA_HOME/agent-review-workflow`，默认 `~/.local/share/agent-review-workflow` |

升级必须显式执行 `arw update`，并显示版本、校验结果和变更摘要。不得从 GitHub `main` 分支直接下载并执行未经版本固定的业务脚本。

## 14. 仓库结构

```text
agent-review-workflow/
├─ cmd/
│  ├─ arw/                     Go CLI 入口
│  └─ arw-mcp/                 Go MCP server 入口
├─ internal/
│  ├─ git/                     Git 命令封装与安全校验
│  ├─ task/                    状态机与任务模型
│  ├─ ledger/                  arw/registry 读写与同步
│  ├─ review/                  审查快照、比较范围、失效规则
│  └─ worktree/                worktree 创建、发现、锁定
├─ schemas/                    JSON Schema 与契约测试样本
├─ vscode-extension/           TypeScript 扩展
├─ integrations/
│  ├─ codex/
│  ├─ claude-code/
│  └─ opencode/
├─ installers/                 PowerShell/Shell 安装、升级、迁移
├─ docs/
│  └─ FINAL-SOLUTION-DESIGN.zh-CN.md
└─ .github/workflows/
```

现有 `arw.ps1`、`arw.sh` 与 `scripts/` 作为 v1 兼容层保留到 v2 迁移完成，再在明确的主版本升级中弃用。

## 15. 实施分期与验收标准

### Phase 0：规范冻结与仓库准备

- 加入本文、JSON Schema 初稿、架构决策记录（ADR）。
- 标明 v1 已有能力和 v2 目标能力。
- 验收：新开发者只读文档即可理解范围、不可自动化的操作及任务关系。

### Phase 1：Go Core MVP

- 实现 `doctor`、任务创建、任务列表、任务详情、worktree 创建。
- 建立 `arw/registry` 并写入/读取 YAML 任务记录。
- 验收：Windows/Linux 可从 Release 安装；创建独立任务后可恢复其信息。

### Phase 2：审查快照与依赖图

- 实现正确的 `base...head` 比较、未提交改动检测、提交/文件统计、测试证据。
- 实现父子任务、条件审查和审查失效。
- 验收：`main <- A <- B` 下，B 不会被错误标记为最终通过。

### Phase 3：MCP 与 Agent 适配

- 实现受限 MCP 工具和统一 JSON Schema。
- 配置 Codex、Claude Code、OpenCode 的说明与 MCP 入口。
- 验收：三种 Agent 对同一任务得到一致的分支、状态和审查基准；无泛用 shell 工具。

### Phase 4：VS Code 扩展 MVP

- 任务树、任务详情、正确范围的文件列表、原生 diff 打开、新窗口 worktree 打开。
- 验收：用户正在 A 中编码时，能打开 B 的审查而不改变 A 的窗口或分支。

### Phase 5：发布、升级与稳固

- Release 自动构建、SHA-256、构建来源证明、升级与迁移。
- Windows/Linux 集成测试、MCP 契约测试、VS Code 扩展测试。
- 验收：在新机器中从公开地址安装后，不依赖原始克隆目录即可正常使用。

## 16. 测试策略

- **单元测试**：状态机转移、依赖派生、审查失效规则、任务 YAML 序列化。
- **Git 集成测试**：临时仓库中验证独立任务、堆叠任务、脏工作区、main 前进、冲突后的失效。
- **跨平台测试**：Windows 与 Linux 的安装、PATH、worktree、CLI JSON 输出。
- **MCP 契约测试**：每个工具的输入/输出均通过 JSON Schema，并验证禁止的动作不可调用。
- **扩展测试**：任务树、命令注册、diff URI、当前窗口不被切换的回归测试。
- **人工验收**：用 Codex、Claude Code、OpenCode 各跑一遍“开始任务 → 写代码 → 审查 → 搁置 → 恢复 → 审查依赖任务”。

## 17. 最终验收场景

用户在任一支持的 AI 中说：

> 审查数据库修复，并在 VS Code 中打开。

系统必须能够：

1. 找到“数据库修复”任务，或在存在歧义时要求澄清；
2. 检查其分支、worktree、基准 SHA 和依赖；
3. 若依赖未验收，建议先审父任务，并明确本次只能是条件审查；
4. 生成真实的审查快照和测试/风险摘要；
5. 在不影响当前编码窗口的前提下，打开目标任务的新 VS Code 审查窗口；
6. 展示正确基准下的文件变更和逐文件 diff；
7. 不自行 merge、push、rebase、reset 或删除任何工作内容。

满足这一场景，即说明 ARW 的核心价值已真正落地。
