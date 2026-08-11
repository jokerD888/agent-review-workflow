# 从 v1 迁移到 v2

v1 的 `arw.ps1` / `arw.sh` 和 `scripts/` 保持可用，直到 v2 发布为稳定主版本。
它们不理解任务台账、堆叠依赖或 VS Code 任务视图，不能与 v2 同时作为同一仓库
的写入入口。

迁移步骤：

1. 安装 v2 二进制并运行 `arw doctor`。
2. 在已有仓库运行 `arw setup`；该操作只建立 `arw/registry` 和本地 ARW 目录。
3. 对每个仍要追踪的 v1 分支运行 `arw task import <branch>`（稳定版提供）。
4. 确认 `arw task list` 的记录与分支、基准相符后，停止使用 v1 的 `start`/`review` 写入命令。

迁移不会删除旧分支、旧脚本或 worktree，也不会执行远程 Git 操作。
