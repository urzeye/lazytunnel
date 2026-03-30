package tui

import (
	"strings"

	"github.com/urzeye/lazytunnel/internal/domain"
)

func (m Model) profileProblemFix(profile domain.Profile, problem string) string {
	switch {
	case strings.Contains(problem, "ssh settings are required"):
		return m.t("Press e and fill SSH Host, Remote Host, and Remote Port.", "按 e 补齐 SSH 主机、远端主机和远端端口。")
	case strings.Contains(problem, "ssh_remote settings are required"):
		return m.t("Press e and fill SSH Host, Bind Port, Target Host, and Target Port.", "按 e 补齐 SSH 主机、监听端口、目标主机和目标端口。")
	case strings.Contains(problem, "ssh_dynamic settings are required"):
		return m.t("Press e and fill SSH Host and Local Port.", "按 e 补齐 SSH 主机和本地端口。")
	case strings.Contains(problem, "kubernetes settings are required"):
		return m.t("Press e and fill Resource Type, Resource, Remote Port, and Local Port.", "按 e 补齐资源类型、资源名、远端端口和本地端口。")
	case strings.Contains(problem, "host is required"):
		switch profile.Type {
		case domain.TunnelTypeKubernetesPortForward:
			return ""
		default:
			return m.t("Press e and set the SSH Host field.", "按 e 填写 SSH 主机。")
		}
	case strings.Contains(problem, "remote_host is required"):
		return m.t("Press e and set Remote Host.", "按 e 填写远端主机。")
	case strings.Contains(problem, "target_host is required"):
		return m.t("Press e and set Target Host.", "按 e 填写目标主机。")
	case strings.Contains(problem, "resource is required"):
		return m.t("Press e and set Resource.", "按 e 填写资源名称。")
	case strings.Contains(problem, "resource_type must be one of"):
		return m.t("Press e and choose pod, service, or deployment.", "按 e 选择 pod、service 或 deployment。")
	case strings.Contains(problem, "remote_port must be between"):
		return m.t("Press e and enter a valid Remote Port between 1 and 65535.", "按 e 输入 1 到 65535 之间的有效远端端口。")
	case strings.Contains(problem, "target_port must be between"):
		return m.t("Press e and enter a valid Target Port between 1 and 65535.", "按 e 输入 1 到 65535 之间的有效目标端口。")
	case strings.Contains(problem, "bind_port must be between"):
		return m.t("Press e and enter a valid Bind Port between 1 and 65535.", "按 e 输入 1 到 65535 之间的有效监听端口。")
	case strings.Contains(problem, "local_port must be between"):
		return m.t("Press e and enter a valid Local Port between 1 and 65535.", "按 e 输入 1 到 65535 之间的有效本地端口。")
	case strings.Contains(problem, "max_retries must be greater than or equal to 0"):
		return m.t("Press e and use 0 or a positive Max Retries value.", "按 e 把最大重试次数改成 0 或正数。")
	case strings.Contains(problem, "invalid initial_backoff"):
		return m.t("Press e and use a valid duration like 2s or 500ms for Initial Backoff.", "按 e 把初始退避改成合法时长，例如 2s 或 500ms。")
	case strings.Contains(problem, "invalid max_backoff"):
		return m.t("Press e and use a valid duration like 30s or 5m for Max Backoff.", "按 e 把最大退避改成合法时长，例如 30s 或 5m。")
	case strings.Contains(problem, "already used by active profile"):
		owner := quotedName(problem)
		if owner == "" {
			return m.t("Stop the profile using this local port, or press e and choose another one.", "停止占用这个本地端口的配置，或按 e 改成其他端口。")
		}
		return m.tf(
			"Stop %s first, or press e and choose another local port.",
			"先停止 %s，或按 e 改成其他本地端口。",
			owner,
		)
	case strings.Contains(problem, "is unavailable"):
		return m.t("Stop the process using this local port, or press e and choose another port.", "停止占用这个本地端口的进程，或按 e 改成其他端口。")
	case strings.Contains(problem, "is still marked as draft"):
		return m.t("Press e to finish the draft fields, then remove the draft label when ready.", "按 e 完成草稿字段，准备好后再移除 draft 标签。")
	case strings.Contains(problem, "kubernetes context is empty"):
		return m.t("Press e to pin a specific Context, or keep it blank intentionally to follow kubectl.", "按 e 固定一个 Context，或者明确接受留空时跟随 kubectl 当前 context。")
	case strings.Contains(problem, "namespace is empty"):
		return m.t("Press e to pin a Namespace, or keep it blank intentionally to follow the context default.", "按 e 固定一个命名空间，或者明确接受留空时跟随当前 context 默认值。")
	case strings.Contains(problem, "remote bind address 0.0.0.0"):
		return m.t("Press e and switch Bind Address to 127.0.0.1 unless you need public exposure.", "除非你确实需要对外暴露，否则按 e 把监听地址改成 127.0.0.1。")
	case strings.Contains(problem, "is not loopback; other machines may reach this proxy"):
		return m.t("Press e and switch Bind Address to 127.0.0.1 unless you need LAN access.", "除非你确实需要局域网访问，否则按 e 把监听地址改成 127.0.0.1。")
	default:
		if hasLabel(profile.Labels, "draft") {
			return m.t("Press e to finish this draft, or press E for raw YAML editing.", "按 e 完成这个草稿，或按 E 直接编辑原始 YAML。")
		}
		return ""
	}
}

func (m Model) stackProblemFix(stack domain.Stack, profileName, problem string) string {
	switch {
	case strings.Contains(problem, "already reserved by profile"):
		owner := quotedName(problem)
		if owner == "" {
			return m.t("Press e and change the member port, or stop the conflicting running profile first.", "按 e 修改成员端口，或先停止冲突的运行中配置。")
		}
		return m.tf(
			"Press e and change the member port, or stop %s first.",
			"按 e 修改成员端口，或先停止 %s。",
			owner,
		)
	case strings.Contains(problem, "profile \"") && strings.Contains(problem, "\" not found"):
		missing := quotedName(problem)
		if missing == "" {
			missing = profileName
		}
		return m.tf(
			"Press e and remove or replace missing member %s in this stack.",
			"按 e 在这个组合里移除或替换缺失成员 %s。",
			missing,
		)
	case strings.Contains(problem, "profiles must include at least one profile name"):
		return m.t("Press e and add at least one member profile.", "按 e 至少添加一个成员配置。")
	default:
		return m.profileProblemFix(domain.Profile{
			Name:   profileName,
			Labels: stack.Labels,
		}, problem)
	}
}

func quotedName(value string) string {
	start := strings.IndexRune(value, '"')
	if start < 0 {
		return ""
	}
	end := strings.IndexRune(value[start+1:], '"')
	if end < 0 {
		return ""
	}
	return value[start+1 : start+1+end]
}

func hasLabel(labels []string, label string) bool {
	for _, existing := range labels {
		if existing == label {
			return true
		}
	}
	return false
}
