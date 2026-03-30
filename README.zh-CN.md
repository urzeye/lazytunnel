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

## 安装

当前最直接的安装方式是从源码安装：

```bash
go install github.com/urzeye/lazytunnel/cmd/lazytunnel@latest
```

后续打 tag 之后，GitHub Releases 会同时提供预编译二进制、`mise` 安装入口，以及 Linux 的 `.deb` / `.rpm` 包。

### Go

```bash
go install github.com/urzeye/lazytunnel/cmd/lazytunnel@latest
```

### GitHub Releases

每个带 tag 的版本都会在
[GitHub Releases 页面](https://github.com/urzeye/lazytunnel/releases)
发布 macOS、Linux、Windows 的预编译压缩包。

### mise

如果你在用 `mise`，后续可以直接从 GitHub Releases 安装：

```bash
mise use -g github:urzeye/lazytunnel
```

### Linux 包

每个带 tag 的版本也会附带 `.deb` 和 `.rpm` 产物，方便偏好原生包管理方式的 Linux 发行版安装。

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

添加一个 SSH 远程转发 profile：

```bash
lazytunnel profile add ssh-remote \
  --name public-api \
  --host bastion-prod \
  --bind-address 0.0.0.0 \
  --bind-port 9000 \
  --target-host 127.0.0.1 \
  --target-port 8080
```

添加一个 SSH 动态 SOCKS profile：

```bash
lazytunnel profile add ssh-dynamic \
  --name dev-socks \
  --host bastion-prod \
  --local-port 1080
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

基于现有 profile 复制一个相近环境的配置：

```bash
lazytunnel profile clone prod-db \
  --name staging-db \
  --local-port 15432 \
  --description "Staging database tunnel"
```

直接就地修改一个已保存的 profile：

```bash
lazytunnel profile edit staging-db \
  --remote-host staging-db.internal \
  --label staging \
  --label db
```

也可以直接走交互式编辑：

```bash
lazytunnel profile edit staging-db --interactive
lazytunnel stack edit backend-dev --interactive
```

从现有 `~/.ssh/config` 导入 draft profile：

```bash
lazytunnel profile import ssh-config
```

从 kubeconfig context 导入 draft profile：

```bash
lazytunnel profile import kube-contexts
```

如果你要指定配置文件路径，或允许覆盖同名 profile：

```bash
lazytunnel --config ~/.config/lazytunnel/config.yaml profile import ssh-config --overwrite
lazytunnel profile import kube-contexts --kubeconfig ~/.kube/config --overwrite
```

导入后的 profile 会以可编辑草稿的形式写入配置。SSH 导入会先放一个占位转发目标，
Kubernetes 导入会先放一个占位资源目标，所以通常还需要再改一遍配置再连接。
在 TUI 里，按 `e` 可以直接用内置表单完善当前选中的草稿，按 `E` 则跳到原始 YAML。
如果你已经打开了 TUI，而且是从 CLI 发起导入，导入完成后按 `g` 重新加载配置即可看到新条目。

校验当前配置：

```bash
lazytunnel validate
```

启动终端 UI：

```bash
lazytunnel
```

在 TUI 里：

- 按 `i` 打开导入提示，可从 `~/.ssh/config`、Kubernetes context，或两者一起导入
- 按 `a` 选择一个 profile 预设，可直接从 SSH 本地、SSH 远程、SOCKS 或 Kubernetes 模板开始，再在表单里补全
- 按 `A` 选择一个 stack 预设，可从当前选择、当前可见配置或运行中的配置生成
- 当工作区为空时，按 `s` 写入示例配置
- 按 `e` 用引导式表单编辑当前选中的 profile / stack
- 按 `E` 在外部编辑器里打开原始 YAML 配置
- 在日志面板里，按 `f` 切换跟随 / 暂停，按 `t` / `T` 切换来源，按 `w` 切换换行 / 截断，按 `n` / `N` 在筛选命中之间跳转
- 在 stack 详情里，按 `[` / `]` 选择成员，按 `S` 启动或停止当前成员，按 `R` 重启当前成员，按 `p` 打开当前成员 profile
- 在 stack 表单编辑器里，输入 `,` 或直接粘贴逗号 / 换行分隔的 profile 名称可自动展开成员行，按 `+` 在当前成员下方新增一行，按 `Ctrl+X` 删除当前成员，按 `[` / `]` 调整成员顺序

## 核心能力

LazyTunnel 围绕几个高频场景来设计：

- 保存 tunnel profile，而不是反复手敲命令
- 在 TUI 里快速启动和停止 tunnel
- 在一个界面里查看状态、运行时长、端口、最近错误和最近日志
- 把多条 tunnel 组合成一个 stack，按项目批量启动
- 按名称、标签、目标和端口快速过滤 profile / stack
- 在日志面板里按文本和来源筛选日志，高亮命中内容，并在命中之间快速跳转
- 通过引导式 preset 创建新的 profile / stack，而不是从空白 YAML 开始写
- 通过 CLI 或 TUI 从 `~/.ssh/config` 和 kubeconfig context 导入 draft profile
- 通过 TUI 内置表单或 `profile edit --interactive` 继续完善导入草稿
- 在启动前识别本地端口冲突
- 校验失败时给出下一步可执行的修复提示
- 通过 CLI 完成 profile / stack 的新增、复制、修改和删除管理
- 直接在 stack 详情里控制单个成员，并在引导式表单里调整成员顺序
- 直接在 TUI 里带确认地删除 profile / stack
- 直接在 TUI 里切换英文和简体中文

## 当前支持的工作流

- SSH 本地转发：`ssh -L`
- SSH 远程转发：`ssh -R`
- SSH 动态代理：`ssh -D`
- Kubernetes `pod`、`service`、`deployment` 的端口转发

## 近期路线

- 在 TUI 里补齐更完整的运行状态、重连信息和日志视图
- 优化日志面板的格式、样式和过滤体验
- 继续增强按项目组织 stack、标签和预检查能力
- 打磨 tag 版本下的 release 和安装体验

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
