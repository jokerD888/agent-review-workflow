---
name: agent-review-workflow
description: Native Git-first task workflow for safe, reviewable coding agent delivery. Uses physical worktree isolation, immutable ledger branches, explicit human review boundaries, and validated fast-forward merges.
---

# Agent Review Workflow (ARW)

ARW provides structured, worktree-isolated task management with mandatory human approval gates and fast-forward merges.

## 1. 适用场景 (When to Use)

- **明确交付意图**：用户明确提出实现或重构独立功能（如“开始实现/修复...”、“新增功能...”、“这个需要审查”）。
- **需要隔离与审查**：代码修改涉及多个文件、持久变更，需要独立分支、干净的工作区和审查快照。
- **用户显式启用**：用户明确说明“使用 ARW”或“把这个纳入 ARW”。

> [!NOTE]
> 如果不确定是否需要创建 ARW 任务，仅在首次提问简短确认一次：“这是临时处理，还是要作为可审查任务纳入 ARW？”禁止反复询问。

## 2. 不适用场景 (When NOT to Use)

- **只读查询与解释**：解释代码、回答架构问题、搜索代码库、阅读文档。
- **临时排查与调查**：定位 bug 原因、验证假设、查看日志。
- **一次性丢弃脚本**：临时调试脚本、测试用例验证代码。
- **用户明确拒绝**：用户说明“这次不用 ARW”、“不建任务”、“临时处理”，当前会话直到用户再次明确要求前不得创建 ARW 任务。

## 3. Agent 调用规范 (Wrapper Invocation)

> [!IMPORTANT]
> **绝对禁止绕过 ARW CLI 直接执行高危 Git 操作**（如手动创建/切换 worktree、git merge、git reset --hard、git checkout --、git push 等）。
> **所有 ARW 操作必须通过 Skill 自带的 Wrapper 脚本执行，严禁调用系统 PATH 中的 `arw` 或直接猜测底层平台二进制路径**。

本文档中 `<ARW>` 指代当前平台对应的 Wrapper 命令：

- **Windows**:
  ```powershell
  powershell -ExecutionPolicy Bypass -File <skill-directory>/scripts/arw.ps1 <command> [args...] --json
  # 若宿主环境明确存在 PowerShell 7 (pwsh)，亦可调用：
  # pwsh -File <skill-directory>/scripts/arw.ps1 <command> [args...] --json
  ```
- **macOS / Linux**:
  ```sh
  <skill-directory>/scripts/arw.sh <command> [args...] --json
  ```

始终附加 `--json` 以获取稳定的结构化输出。

## 4. 主生命周期 (Lifecycle Workflow)

```text
[Start Task] ──► [Coding in Worktree] ──► [Mark Ready & Prepare Review]
                                                        │
[Clear (Optional)] ◄── [FF Merge] ◄── [Human Approval] ◄┘
```

### 步骤 1：创建任务 (Start)
与用户确认任务简要目标和 ID 后创建：
```sh
<ARW> task start --id <task-id> "<task-title>" --json
```
- 返回新创建的独立 worktree 路径与分支名（`arw/<task-id>`）。
- **后续代码编辑与测试必须在该 worktree 目录内进行**，不要修改主仓库工作区。

### 步骤 2：在专属 Worktree 中开发与自测
- 仅修改与当前任务直接相关的代码；
- 在 worktree 中运行测试与构建；
- 每次完成一个独立可构建/可测试单元后，在 worktree 中提交本地 Git commit。

### 步骤 3：标记就绪并准备审查 (Prepare Review)
完成编码与自测后：
```sh
<ARW> task ready <task-id> --json
<ARW> review prepare <task-id> --json
```
- `review prepare` 会计算 base/head SHA、文件 diff、提交记录、worktree 干净状态及依赖关系。
- 向用户展示：任务目的、变更文件、精确的 Base...HEAD SHA 范围、自测结果。
- **等待用户审查确认**。若用户需要修改，按反馈继续提交并重新准备审查。

### 步骤 4：记录人工批准 (Approve)
**必须在用户明确表示“审查通过”或“同意合并”后**，提取用户审查通过时的 exact base SHA 与 head SHA：
```sh
<ARW> review approve --confirm --base <reviewed-base-sha> --head <reviewed-head-sha> <task-id> --json
```
> [!WARNING]
> 严禁 Agent 自行假定审查通过！若用户审查后代码发生任何变动（HEAD 改变），Core 会自动拒绝过期的审批（stale review）。

### 步骤 5：执行本地快进合并 (Merge)
在用户单独指示合并后执行：
```sh
<ARW> task merge --confirm <task-id> --json
```
- Core 会执行 `--ff-only` 本地快进合并至基线分支。
- 绝不自动 push 到远程仓库。

### 步骤 6：清理资源 (Clear)
任务合并或废弃后，可清理本地分支和 worktree（台账审计记录永久保留）：
```sh
<ARW> task clear --confirm <task-id> --json
```

## 5. 关键安全红线 (Safety Invariants)

1. **唯一决策权在用户**：批准与合并决策权归人类用户所有，Agent 严禁代做批准决定。
2. **禁止静默创建任务**：创建前简要告知用户 intended task id 与 worktree。
3. **隔离开发**：任务代码必须在 `arw task start` 返回的专属 worktree 目录中修改。
4. **快进合并与拒绝污染**：ARW 仅支持 fast-forward 合并，若基线分支已移动或存在未提交修改，Core 保证直接拒绝合并。

## 6. 深入参考 (Progressive Disclosure)

当遇到以下复杂或非典型场景时，请阅读 [workflow-lifecycle.md](references/workflow-lifecycle.md)：
- **层叠任务与依赖阻塞 (Stacked Tasks & Blocked Dependency)**：父子任务链、父任务未审批时的审查规则；
- **Stale Approval 自动失效与恢复**：基线分支被更新或 HEAD 变动后的重审流程；
- **Dirty Worktree 处理**：审查或合并前工作区存在未提交变动时的恢复；
- **任务搁置与恢复 (Park & Resume)**：上下文切换时的处理；
- **任务废弃 (Abandon)**：放弃当前任务分支；
- **父任务已清理时的回溯推导 (Effective Target Resolution)**。
