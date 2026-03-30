package tui

import (
	"fmt"
	"strings"
	"time"

	"github.com/urzeye/lazytunnel/internal/app"
	"github.com/urzeye/lazytunnel/internal/domain"
)

func (m Model) language() domain.Language {
	if m.service == nil {
		return domain.LanguageEnglish
	}

	cfg := m.service.Config()
	if cfg.Language == "" {
		return domain.LanguageEnglish
	}

	return cfg.Language
}

func (m Model) t(english, chinese string) string {
	return translate(m.language(), english, chinese)
}

func (m Model) tf(english, chinese string, args ...any) string {
	return translatef(m.language(), english, chinese, args...)
}

func translate(language domain.Language, english, chinese string) string {
	if language == domain.LanguageSimplifiedChinese {
		return chinese
	}
	return english
}

func translatef(language domain.Language, english, chinese string, args ...any) string {
	return fmt.Sprintf(translate(language, english, chinese), args...)
}

func nextLanguage(language domain.Language) domain.Language {
	if language == domain.LanguageSimplifiedChinese {
		return domain.LanguageEnglish
	}
	return domain.LanguageSimplifiedChinese
}

func languageDisplayName(language domain.Language) string {
	switch language {
	case domain.LanguageSimplifiedChinese:
		return "简体中文"
	default:
		return "English"
	}
}

func humanTunnelStatus(language domain.Language, status domain.TunnelStatus) string {
	switch status {
	case domain.TunnelStatusRunning:
		return translate(language, "Running", "运行中")
	case domain.TunnelStatusStarting:
		return translate(language, "Starting", "启动中")
	case domain.TunnelStatusRestarting:
		return translate(language, "Restarting", "重试中")
	case domain.TunnelStatusFailed:
		return translate(language, "Failed", "失败")
	case domain.TunnelStatusExited:
		return translate(language, "Exited", "已退出")
	default:
		return translate(language, "Stopped", "已停止")
	}
}

func humanStackStatus(language domain.Language, status app.StackStatus) string {
	switch status {
	case app.StackStatusRunning:
		return translate(language, "Running", "运行中")
	case app.StackStatusPartial:
		return translate(language, "Partially Active", "部分运行")
	default:
		return translate(language, "Stopped", "已停止")
	}
}

func humanTunnelType(language domain.Language, kind domain.TunnelType) string {
	switch kind {
	case domain.TunnelTypeSSHLocal:
		return translate(language, "SSH local forward", "SSH 本地转发")
	case domain.TunnelTypeSSHRemote:
		return translate(language, "SSH remote forward", "SSH 远程转发")
	case domain.TunnelTypeSSHDynamic:
		return translate(language, "SSH SOCKS proxy", "SSH SOCKS 代理")
	case domain.TunnelTypeKubernetesPortForward:
		return translate(language, "Kubernetes port-forward", "Kubernetes 端口转发")
	default:
		return string(kind)
	}
}

func profileTarget(language domain.Language, profile domain.Profile) string {
	switch profile.Type {
	case domain.TunnelTypeSSHLocal:
		if profile.SSH == nil {
			return translate(language, "SSH target not configured", "SSH 目标尚未配置")
		}
		return fmt.Sprintf("%s -> %s:%d", profile.SSH.Host, profile.SSH.RemoteHost, profile.SSH.RemotePort)
	case domain.TunnelTypeSSHRemote:
		if profile.SSHRemote == nil {
			return translate(language, "SSH remote target not configured", "SSH 远程转发目标尚未配置")
		}
		return fmt.Sprintf(
			"%s • remote %s -> %s:%d",
			profile.SSHRemote.Host,
			profileRemoteBind(profile),
			profile.SSHRemote.TargetHost,
			profile.SSHRemote.TargetPort,
		)
	case domain.TunnelTypeSSHDynamic:
		if profile.SSHDynamic == nil {
			return translate(language, "SSH SOCKS target not configured", "SSH SOCKS 目标尚未配置")
		}
		return fmt.Sprintf("%s • SOCKS %s", profile.SSHDynamic.Host, profileLocalBind(profile))
	case domain.TunnelTypeKubernetesPortForward:
		if profile.Kubernetes == nil {
			return translate(language, "Kubernetes target not configured", "Kubernetes 目标尚未配置")
		}

		parts := make([]string, 0, 3)
		if profile.Kubernetes.Context != "" {
			parts = append(parts, profile.Kubernetes.Context)
		}
		if profile.Kubernetes.Namespace != "" {
			parts = append(parts, profile.Kubernetes.Namespace)
		}
		parts = append(parts, fmt.Sprintf(
			"%s/%s:%d",
			profile.Kubernetes.ResourceType,
			profile.Kubernetes.Resource,
			profile.Kubernetes.RemotePort,
		))
		return strings.Join(parts, " • ")
	default:
		return translate(language, "Unknown target", "未知目标")
	}
}

func profilePortSummaryLabel(language domain.Language, profile domain.Profile) string {
	switch profile.Type {
	case domain.TunnelTypeSSHRemote:
		return translate(language, "Remote Bind", "远端监听")
	default:
		return translate(language, "Local", "本地")
	}
}

func profilePortSummaryValue(profile domain.Profile) string {
	switch profile.Type {
	case domain.TunnelTypeSSHRemote:
		return profileRemoteBind(profile)
	default:
		return profileLocalBind(profile)
	}
}

func profileListPort(profile domain.Profile) string {
	switch profile.Type {
	case domain.TunnelTypeSSHRemote:
		return "R:" + fmt.Sprintf("%d", profileDisplayPort(profile))
	default:
		return ":" + fmt.Sprintf("%d", profileDisplayPort(profile))
	}
}

func profileDisplayPort(profile domain.Profile) int {
	switch profile.Type {
	case domain.TunnelTypeSSHRemote:
		if profile.SSHRemote != nil && profile.SSHRemote.BindPort > 0 {
			return profile.SSHRemote.BindPort
		}
	}
	return profile.LocalPort
}

func profileLocalBind(profile domain.Profile) string {
	return fmt.Sprintf(":%d", profileDisplayPort(profile))
}

func profileRemoteBind(profile domain.Profile) string {
	bindPort := profileDisplayPort(profile)
	if profile.SSHRemote == nil || profile.SSHRemote.BindAddress == "" {
		return fmt.Sprintf(":%d", bindPort)
	}
	return fmt.Sprintf("%s:%d", profile.SSHRemote.BindAddress, bindPort)
}

func stackMembersSummary(language domain.Language, view app.StackView) string {
	if len(view.Stack.Profiles) == 0 {
		return translate(language, "no members", "无成员")
	}

	return strings.Join(view.Stack.Profiles, ", ")
}

func profileStartSummary(language domain.Language, status app.StartReadiness) string {
	switch status {
	case app.StartReadinessActive:
		return translate(language, "Running now", "正在运行")
	case app.StartReadinessWarning:
		return translate(language, "Ready with warnings", "有提醒，可启动")
	case app.StartReadinessReady:
		return translate(language, "Ready on Enter", "可按 Enter 启动")
	case app.StartReadinessBlocked:
		return translate(language, "Blocked", "已阻塞")
	default:
		return "-"
	}
}

func stackStartSummary(language domain.Language, view app.StackView, analysis app.StackStartAnalysis) string {
	switch {
	case len(view.Stack.Profiles) == 0:
		return translate(language, "Blocked", "已阻塞")
	case analysis.BlockedCount > 0:
		return translate(language, "Blocked", "已阻塞")
	case analysis.WarningCount > 0:
		if language == domain.LanguageSimplifiedChinese {
			return fmt.Sprintf("%d 个成员有提醒", analysis.WarningCount)
		}
		return "Warnings for " + formatCountNoun(language, analysis.WarningCount, "member", "members", "个成员")
	case analysis.ReadyCount > 0:
		if language == domain.LanguageSimplifiedChinese {
			return fmt.Sprintf("可启动 %d 个成员", analysis.ReadyCount)
		}
		return "Ready for " + formatCountNoun(language, analysis.ReadyCount, "member", "members", "个成员")
	case analysis.ActiveCount > 0:
		return translate(language, "Already running", "已在运行")
	default:
		return translate(language, "Idle", "空闲")
	}
}

func formatCountNoun(language domain.Language, count int, singular, plural, chinese string) string {
	if language == domain.LanguageSimplifiedChinese {
		return fmt.Sprintf("%d%s", count, chinese)
	}
	if count == 1 {
		return fmt.Sprintf("%d %s", count, singular)
	}
	return fmt.Sprintf("%d %s", count, plural)
}

func formatLastExit(language domain.Language, state domain.RuntimeState, now time.Time) string {
	parts := make([]string, 0, 3)

	if state.ExitReason != "" {
		parts = append(parts, state.ExitReason)
	}
	if state.ExitedAt != nil {
		if language == domain.LanguageSimplifiedChinese {
			parts = append(parts, humanizeDuration(now.Sub(*state.ExitedAt))+"前")
		} else {
			parts = append(parts, humanizeDuration(now.Sub(*state.ExitedAt))+" ago")
		}
	}
	if state.ExitedAt != nil || state.LastExitCode != 0 {
		parts = append(parts, translatef(language, "code %d", "退出码 %d", state.LastExitCode))
	}

	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " • ")
}

func restartPolicySummary(language domain.Language, policy domain.RestartPolicy) string {
	if !policy.Enabled {
		return translate(language, "disabled", "已禁用")
	}

	initialBackoff := policy.InitialBackoff
	if initialBackoff == "" {
		initialBackoff = "2s"
	}

	maxBackoff := policy.MaxBackoff
	if maxBackoff == "" {
		maxBackoff = "30s"
	}

	if language == domain.LanguageSimplifiedChinese {
		maxRetries := "无限重试"
		if policy.MaxRetries > 0 {
			maxRetries = fmt.Sprintf("最多重试 %d 次", policy.MaxRetries)
		}
		return fmt.Sprintf("%s，退避 %s 到 %s", maxRetries, initialBackoff, maxBackoff)
	}

	maxRetries := "unlimited retries"
	if policy.MaxRetries > 0 {
		maxRetries = fmt.Sprintf("%d retries", policy.MaxRetries)
	}

	return fmt.Sprintf("%s, %s to %s", maxRetries, initialBackoff, maxBackoff)
}
