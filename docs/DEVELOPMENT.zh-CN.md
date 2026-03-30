# 开发计划

这个文档把产品路线图进一步拆成可执行的工程计划。

## 当前决策

- 语言：Go
- 产品形态：本地优先的终端应用
- v0.1 主要场景：
  - 用 `ssh -L` 做 SSH 本地转发
  - 用 `ssh -R` 做 SSH 远程转发
  - 用 `ssh -D` 做 SSH 动态 SOCKS
  - 用 `kubectl port-forward` 做 Kubernetes 端口转发
- v0.1 的发布目标：
  - 通过 GitHub Releases 提供二进制
  - 支持 mise 安装
  - 为 Go 用户提供可选的 `go install`
  - Homebrew 安装延后到 v0.1.x

## 建议技术栈

- CLI 入口：`cobra`
- TUI：`bubbletea`、`lipgloss`，按需引入 `bubbles`
- 日志：`slog` 或 `zap`
- 配置格式：应用配置使用 YAML，运行时动态生成实际命令
- 发布：`goreleaser`

## 工程原则

- 包装已经被用户信任的系统命令，而不是重写它们
- v0.1 保持本地优先、进程驱动的架构
- 不要求 daemon，也不要求 Web 服务端
- 在第一个工作流打磨顺之前，数据模型尽量保持小而稳
- 优先保证可观察性和可恢复性，而不是盲目加功能

## v0.1 范围

第一版只需要把一个黄金路径做得足够好：

- 保存 tunnel profile
- 在 TUI 中启动和停止 profile
- 支持 `ssh -L`、`ssh -R`、`ssh -D`
- 支持 `kubectl port-forward`
- 启动前检测本地端口冲突
- 进程异常退出后自动重连
- 展示实时状态、本地端口、目标地址、运行时长和最近日志
- 用 stack 管理一组 profile

## 建议目录结构

```text
cmd/lazytunnel/
internal/app/
internal/domain/
internal/runtime/
internal/adapters/ssh/
internal/adapters/kubernetes/
internal/storage/
internal/tui/
pkg/
```

推荐职责如下：

- `cmd/lazytunnel/`：进程入口和 CLI 参数
- `internal/app/`：应用服务和业务编排
- `internal/domain/`：profile、stack、运行状态、校验规则
- `internal/runtime/`：进程监管、重启策略、日志、事件
- `internal/adapters/ssh/`：SSH 命令构造和校验
- `internal/adapters/kubernetes/`：`kubectl port-forward` 命令构造和校验
- `internal/storage/`：配置读取和写入
- `internal/tui/`：Bubble Tea 的模型和视图

## 里程碑

### Milestone 1：项目初始化

- 初始化 Go module
- 确定核心依赖
- 增加 `justfile`
- 增加格式化和 lint 命令
- 增加示例配置文件

完成标准：

- `go test ./...` 能干净通过
- `just run` 能启动一个占位版 TUI

### Milestone 2：领域模型

- 定义 tunnel profile 模型
- 定义 stack 模型
- 定义运行时状态模型
- 定义重启策略和校验规则

完成标准：

- 可以从磁盘解析 profile
- 校验逻辑能拦住非法端口和不完整配置

### Milestone 3：运行时引擎

- 启动和停止子进程
- 捕获 stdout 和 stderr 日志
- 追踪 PID、状态、启动时间、退出原因
- 实现带退避的自动重启

完成标准：

- 一个模拟进程可以被监管并自动重启
- 运行时状态流转有测试覆盖

### Milestone 4：SSH 支持

- 从 profile 生成 `ssh -L`、`ssh -R`、`ssh -D` 命令
- 按不同 SSH 类型校验本地监听、远端监听、目标地址和主机字段
- 清晰展示常见启动失败原因

完成标准：

- 已保存的 SSH 本地、远程和动态 profile 都可以从应用层启动

### Milestone 5：Kubernetes 支持

- 从 profile 生成 `kubectl port-forward` 命令
- 支持 context、namespace 和目标资源
- 清晰展示 context 缺失、namespace 错误等问题

完成标准：

- 已保存的 Kubernetes profile 可以从应用层启动

### Milestone 6：TUI

- profile 列表
- 详情面板
- 状态标记
- 启动、停止操作
- 日志面板
- stack 启动动作

完成标准：

- v0.1 的完整黄金路径可以在终端界面中跑通

### Milestone 7：发布

- 维护并验证 `goreleaser` 发布自动化
- 发布 macOS、Linux、Windows 二进制
- 验证 mise 安装
- 记录 Homebrew 为后续版本支持

完成标准：

- 用户无需本地编译也能通过 release 安装

## 推荐开发顺序

1. 初始化仓库和依赖
2. 定住领域模型
3. 做运行时监管器
4. 接 SSH adapter
5. 接 Kubernetes adapter
6. 在真实运行时事件上搭 TUI
7. 打磨发布和安装链路

## v0.1 完成标准

当下面这些条件都成立时，v0.1 才算真正 ready：

- 用户可以直接从 release 安装
- 用户至少可以保存一个 SSH profile 和一个 Kubernetes profile
- 这两类 profile 都能从 TUI 启动
- 进程掉线后可以自动重连
- 启动前能检测本地端口冲突
- UI 中可以看到最近日志
- README 已包含安装方式和一个简短演示

## v0.1.0 发布决策

当前建议先发布 `v0.1.0`，范围控制在：

- GitHub Releases 二进制
- `go install`
- `mise` 可从 GitHub Releases 获取安装

暂不阻塞 `v0.1.0` 的事项：

- Homebrew
- 日志面板的进一步格式、样式、过滤优化
- README 截图和演示素材

## 接下来立刻要做的事

- 继续降低配置门槛，把 preset、导入草稿补全和 stack 编辑流做得更顺手
- 把启动前健康检查继续做深，让 SSH / Kubernetes 的真实问题尽量在启动前暴露
- 继续补强运行时可观察性，例如重试历史、退避信息和失败摘要
- 持续打磨 stack 编辑和成员控制的交互闭环

## v0.2 当前进展

启动前健康检查已经从“只看端口冲突”往前推进了一步，目前已覆盖：

- 单 profile 和 stack 启动前的本地端口占用检测
- `ssh` / `kubectl` 是否存在于 PATH 的检查
- 在 TUI 详情里区分可阻塞启动的问题和仍可启动的提醒
- 对 draft、空 Kubernetes context / namespace、SSH 暴露型 bind address 提供提醒和修复建议
- 通过 `ssh -G` 静态检查 SSH alias 是否可解析，并在必要时回退到 `~/.ssh/config` 导入视图
- 对 `~/.ssh/config` 里显式声明但不存在的 `IdentityFile` 路径给出提醒
- 在 Kubernetes profile 未显式填写 context 时，优先使用真实的 `kubectl current-context`
- 对 Kubernetes namespace / resource 做存在性校验，并把修复建议同时挂到 CLI 和 TUI

下一步可以继续补的方向：

- 更深入的 SSH 连通性和配置风险提示，但尽量避免把启动前检查做得太慢
- 更顺手的 preset、导入后补全向导和 stack 编辑体验，继续降低写 YAML 的频率
- 更完整的运行时可观察性，包括重试历史、退避过程和失败原因聚合
