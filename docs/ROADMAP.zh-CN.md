# 路线图

这个文档用于记录 LazyTunnel 当前阶段的产品方向。

## MVP 范围

第一版只做一个很小但很有价值的切口：

- 创建并保存 tunnel profile
- 支持 `ssh -L`
- 支持 `kubectl port-forward`
- 检测本地端口冲突
- 进程意外退出后自动重连
- 展示状态、本地端口、目标地址和最近日志
- 用 stack 批量启动相关 tunnel

## 版本阶段

### v0.1

- 本地优先 TUI
- SSH 本地转发
- Kubernetes 端口转发
- stack 启动
- 自动重连和日志

### v0.2

- `ssh -R`
- `ssh -D`
- 更完整的导入流程
- 更好的健康检查

### v0.3

- 面向常见开发场景的预设
- 启动 hooks
- 更深入地接入现有 SSH 和 Kubernetes 上下文

## 当前重点

- 把 tunnel profile 模型定清楚
- 设计主界面的 TUI 布局
- 优先把 `ssh -L` 和 `kubectl port-forward` 这两个工作流做好
