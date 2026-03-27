# 开发计划

这个文档把产品路线图进一步拆成可执行的工程计划。

## 当前决策

- 语言：Go
- 产品形态：本地优先的终端应用
- v0.1 主要场景：
  - 用 `ssh -L` 做 SSH 本地转发
  - 用 `kubectl port-forward` 做 Kubernetes 端口转发
- v0.1 的发布目标：
  - 通过 GitHub Releases 提供二进制
  - 支持 Homebrew 安装
  - 支持 mise 安装
  - 为 Go 用户提供可选的 `go install`

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
- 支持 `ssh -L`
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

- 从 profile 生成 `ssh -L` 命令
- 校验主机、目标地址和本地端口
- 清晰展示常见启动失败原因

完成标准：

- 已保存的 SSH 本地转发 profile 可以从应用层启动

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
- 启动、停止、重启操作
- 日志面板
- stack 启动动作

完成标准：

- v0.1 的完整黄金路径可以在终端界面中跑通

### Milestone 7：发布

- 增加 `goreleaser`
- 发布 macOS、Linux、Windows 二进制
- 增加 Homebrew formula 支持
- 验证 mise 安装

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

## 接下来立刻要做的事

- 增加 SSH 命令构造
- 增加 Kubernetes 命令构造
- 把运行时状态接进 TUI
- 从界面触发启动和停止动作
