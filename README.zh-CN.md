[English](README.md) | 简体中文

# LazyTunnel

> 一个用来管理 SSH 隧道和 Kubernetes 端口转发的终端 UI。

LazyTunnel 是一个键盘优先的终端工作台，专门管理你每天会用到的各种隧道：

- 用 `ssh -L` 建立的本地端口转发
- 用 `ssh -R` 建立的远程端口转发
- 用 `ssh -D` 建立的 SOCKS 代理
- 用 `kubectl port-forward` 建立的 Kubernetes 端口转发

它的目标不是发明新的连接方式，而是把这些原本零散、重复、容易中断的命令，统一收进一个终端界面里管理。

## 它解决什么问题

隧道命令很强大，但真实使用时也很容易让人烦躁：

- 命令太长，参数不好记
- 一个项目往往要同时开多条 tunnel
- 本地端口冲突非常常见
- 网络一抖动，连接就断
- 经常记不住现在到底有哪些 tunnel 还活着
- SSH 和 Kubernetes 的转发工作流是割裂的

LazyTunnel 想做的，就是把这类日常操作做成像 `lazygit`、`lazydocker` 一样顺手的开发者工具。

## 快速开始

初始化一个空配置：

```bash
lazytunnel init
```

如果你想先从示例配置开始：

```bash
lazytunnel init --sample
```

添加一个 SSH 本地转发 profile：

```bash
lazytunnel profile add ssh-local \
  --name prod-db \
  --host bastion-prod \
  --remote-host db.internal \
  --remote-port 5432 \
  --local-port 5432
```

添加一个 Kubernetes 端口转发 profile：

```bash
lazytunnel profile add kubernetes \
  --name api-debug \
  --context dev-cluster \
  --namespace backend \
  --resource-type service \
  --resource api \
  --remote-port 80 \
  --local-port 8080
```

校验当前配置：

```bash
lazytunnel validate
```

## 核心能力

LazyTunnel 围绕几个高频场景来设计：

- 保存 tunnel profile，而不是反复手敲命令
- 一键启动、停止、重启 tunnel
- 在一个界面里查看状态、运行时长、端口和最近错误
- tunnel 意外断开后自动重连，并带退避策略
- 把多条 tunnel 组合成一个 stack，按项目批量启动
- 快速复制本地地址、`host:port` 或连接串

## 支持的工作流

- SSH 本地转发：`ssh -L`
- SSH 远程转发：`ssh -R`
- SSH 动态代理：`ssh -D`
- Kubernetes `pod`、`service`、`deployment` 的端口转发

## 计划能力

- 保存 tunnel profile
- 按项目组织 stack
- 用 `dev`、`staging`、`prod` 这类标签区分环境
- 检测端口冲突并给出建议
- 启动前做依赖检查和预检查
- 从 `~/.ssh/config` 导入现有 SSH 上下文
- 从 kube context 和 namespace 导入现有 Kubernetes 上下文

### 监控与状态

- 实时进程状态
- 运行时长和重连次数
- 最近日志和退出原因
- 活跃 tunnel 的健康指示

### 便捷操作

- 复制本地访问地址
- 复制 DSN 或连接片段
- 在浏览器中打开本地 URL
- 按名称、目标地址、标签进行模糊搜索

## 明确不做什么

LazyTunnel 不打算做成：

- 公网穿透 SaaS
- 需要部署服务端的 Web 面板
- OpenSSH 或 `kubectl` 的替代品
- 凭证或密钥管理器
- 大而全的云控制台

它是一个本地优先的终端工具，负责把你已经信任的命令包装成更顺手的工作流。

## 截图

等第一版可交互原型出来后，再补截图和演示 GIF。

## 当前状态

项目目前还处于早期阶段。

路线图： [English](docs/ROADMAP.md) | [简体中文](docs/ROADMAP.zh-CN.md)

## 欢迎反馈

非常欢迎尽早反馈，尤其是这些问题：

- 你最常用的是哪几类 tunnel
- 你最烦重复输入的是哪些命令
- 你最希望在一个界面里一眼看到哪些状态信息
