package tui

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/urzeye/lazytunnel/internal/app"
	"github.com/urzeye/lazytunnel/internal/domain"
)

type stackFailureSummary struct {
	reason  string
	members []string
}

type runtimeTimelineItem struct {
	timestamp   time.Time
	profileName string
	message     string
}

func (m Model) copyTextToClipboard(text string) error {
	writer := m.clipboardWriter
	if writer == nil {
		writer = writeClipboardText
	}
	return writer(text)
}

func (m Model) exportText(baseName, content string) (string, error) {
	exporter := m.textExporter
	if exporter == nil {
		return exportTextSnapshot(m.configPath, baseName, content, m.currentTime())
	}
	return exporter(baseName, content)
}

func (m Model) copySelectedExecCommand(profiles []app.ProfileView, stacks []app.StackView) Model {
	name, command, err := m.selectedExecCommand(profiles, stacks)
	if err != nil {
		m.lastError = err.Error()
		return m
	}
	if err := m.copyTextToClipboard(command); err != nil {
		m.lastError = m.t("Copy command: ", "复制命令失败: ") + err.Error()
		return m
	}
	m.setNotice(m.tf("Copied exec command for %s.", "已复制 %s 的执行命令。", name))
	return m
}

func (m Model) copyVisibleLogs(profiles []app.ProfileView, stacks []app.StackView) Model {
	baseName, content, lineCount, err := m.visibleLogsSnapshot(profiles, stacks)
	if err != nil {
		m.lastError = err.Error()
		return m
	}
	if err := m.copyTextToClipboard(content); err != nil {
		m.lastError = m.t("Copy logs: ", "复制日志失败: ") + err.Error()
		return m
	}
	if m.language() == domain.LanguageSimplifiedChinese {
		m.setNotice(fmt.Sprintf("已复制 %s 的 %d 行日志。", baseName, lineCount))
	} else {
		m.setNotice(fmt.Sprintf("Copied %d log lines for %s.", lineCount, baseName))
	}
	return m
}

func (m Model) exportVisibleLogs(profiles []app.ProfileView, stacks []app.StackView) Model {
	baseName, content, lineCount, err := m.visibleLogsSnapshot(profiles, stacks)
	if err != nil {
		m.lastError = err.Error()
		return m
	}
	path, err := m.exportText(baseName, content)
	if err != nil {
		m.lastError = m.t("Export logs: ", "导出日志失败: ") + err.Error()
		return m
	}
	if m.language() == domain.LanguageSimplifiedChinese {
		m.setNotice(fmt.Sprintf("已将 %s 的 %d 行日志导出到 %s。", baseName, lineCount, path))
	} else {
		m.setNotice(fmt.Sprintf("Exported %d log lines for %s to %s.", lineCount, baseName, path))
	}
	return m
}

func (m Model) clearVisibleLogs(profiles []app.ProfileView, stacks []app.StackView) Model {
	if m.focus == focusStacks && len(stacks) > 0 {
		view := stacks[max(0, min(m.selectedStack, len(stacks)-1))]
		if err := m.service.ClearStackLogs(view.Stack.Name); err != nil {
			m.lastError = m.t("Clear logs: ", "清空日志失败: ") + err.Error()
			return m
		}
		m.inspectorScroll = 0
		m.setNotice(m.tf("Cleared logs for stack %s.", "已清空组合 %s 的日志。", view.Stack.Name))
		return m
	}
	if len(profiles) > 0 {
		view := profiles[max(0, min(m.selectedProfile, len(profiles)-1))]
		if err := m.service.ClearProfileLogs(view.Profile.Name); err != nil {
			m.lastError = m.t("Clear logs: ", "清空日志失败: ") + err.Error()
			return m
		}
		m.inspectorScroll = 0
		m.setNotice(m.tf("Cleared logs for profile %s.", "已清空配置 %s 的日志。", view.Profile.Name))
		return m
	}
	m.lastError = m.t("No logs are selected yet.", "当前还没有可操作的日志。")
	return m
}

func (m Model) selectedExecCommand(profiles []app.ProfileView, stacks []app.StackView) (string, string, error) {
	if m.focus == focusStacks && len(stacks) > 0 {
		view := stacks[max(0, min(m.selectedStack, len(stacks)-1))]
		if len(view.Members) == 0 {
			return "", "", fmt.Errorf("%s", m.t("Selected stack has no member command to copy yet.", "当前组合还没有可复制命令的成员。"))
		}
		member := view.Members[max(0, min(m.selectedStackMember, len(view.Members)-1))]
		spec, err := app.BuildProcessSpec(member.Profile)
		if err != nil {
			return "", "", err
		}
		return member.Profile.Name, spec.DisplayCommand(), nil
	}
	if len(profiles) == 0 {
		return "", "", fmt.Errorf("%s", m.t("No profile command is selected yet.", "当前还没有选中可复制命令的配置。"))
	}

	view := profiles[max(0, min(m.selectedProfile, len(profiles)-1))]
	spec, err := app.BuildProcessSpec(view.Profile)
	if err != nil {
		return "", "", err
	}
	return view.Profile.Name, spec.DisplayCommand(), nil
}

func (m Model) visibleLogsSnapshot(profiles []app.ProfileView, stacks []app.StackView) (string, string, int, error) {
	if m.focus == focusStacks && len(stacks) > 0 {
		view := stacks[max(0, min(m.selectedStack, len(stacks)-1))]
		totalEntries := 0
		for _, member := range view.Members {
			totalEntries += len(member.State.RecentLogs)
		}
		activity := filterStackActivityBySource(recentStackActivity(view, totalEntries), m.logFilterQuery, m.logSourceFilter)
		if len(activity) == 0 {
			return "", "", 0, fmt.Errorf("%s", m.t("No visible stack logs to copy or export right now.", "当前没有可复制或导出的组合日志。"))
		}

		lines := make([]string, 0, len(activity))
		for idx := len(activity) - 1; idx >= 0; idx-- {
			entry := activity[idx]
			lines = append(lines, plainLogLine(entry.Log.Timestamp, entry.ProfileName, entry.Log.Source, entry.Log.Message))
		}
		return view.Stack.Name, strings.Join(lines, "\n"), len(lines), nil
	}
	if len(profiles) == 0 {
		return "", "", 0, fmt.Errorf("%s", m.t("No visible profile logs to copy or export right now.", "当前没有可复制或导出的配置日志。"))
	}

	view := profiles[max(0, min(m.selectedProfile, len(profiles)-1))]
	filtered := filterLogEntriesBySource(view.State.RecentLogs, m.logFilterQuery, m.logSourceFilter)
	if len(filtered) == 0 {
		return "", "", 0, fmt.Errorf("%s", m.t("No visible profile logs to copy or export right now.", "当前没有可复制或导出的配置日志。"))
	}

	lines := make([]string, 0, len(filtered))
	for idx := len(filtered) - 1; idx >= 0; idx-- {
		entry := filtered[idx]
		lines = append(lines, plainLogLine(entry.Timestamp, "", entry.Source, entry.Message))
	}
	return view.Profile.Name, strings.Join(lines, "\n"), len(lines), nil
}

func (m Model) renderProfileFailureLines(view app.ProfileView, width int) []string {
	reason := runtimeFailureReason(view.State)
	backoff := latestRetryBackoffMessage(view.State.RecentLogs)
	stderr := latestLogMessage(view.State.RecentLogs, domain.LogSourceStderr)
	if reason == "" && backoff == "" && stderr == "" {
		return nil
	}

	lines := []string{groupTitleStyle.Render(m.t("Recent Failure", "最近失败"))}
	if reason != "" {
		lines = append(lines, renderCompactKeyValue(m.t("Reason", "原因"), reason, width))
	}
	if backoff != "" {
		lines = append(lines, renderCompactKeyValue(m.t("Backoff", "退避"), backoff, width))
	}
	if stderr != "" && stderr != reason {
		lines = append(lines, renderCompactKeyValue("stderr", stderr, width))
	}
	return lines
}

func (m Model) renderProfileRuntimeHistoryLines(view app.ProfileView, width int) []string {
	items := limitedRuntimeTimeline(runtimeTimelineEntries(view.State.RecentLogs, ""), 4)
	return m.renderRuntimeTimelineLines(items, width)
}

func (m Model) renderProfileDraftGuideLines(view app.ProfileView, width int) []string {
	if !containsDraftLabel(view.Profile.Labels) {
		return nil
	}

	editor := newProfileEditorState(view.Profile, view.Profile.Name, m.language())
	nextKey := recommendedProfileEditorField(view.Profile)
	field, ok := editor.fieldByKey(nextKey)
	nextLabel := m.t("important fields", "关键字段")
	if ok {
		nextLabel = field.label
	}

	lines := []string{groupTitleStyle.Render(m.t("Draft Guide", "草稿向导"))}
	lines = append(lines, renderCompactKeyValue(m.t("Next", "下一步"), nextLabel, width))
	lines = append(lines, renderCompactKeyValue(m.t("Action", "动作"), m.profileDraftGuideAction(view.Profile, nextLabel), width))
	return lines
}

func (m Model) renderStackFailureLines(view app.StackView, width int) []string {
	summaries := aggregateStackFailureReasons(view.Members)
	retrying := retryingMemberSignals(view.Members)
	if len(summaries) == 0 && len(retrying) == 0 {
		return nil
	}

	lines := []string{groupTitleStyle.Render(m.t("Recent Failures", "最近失败"))}
	for _, summary := range summaries {
		lines = append(lines, renderCompactKeyValue(m.t("Reason", "原因"), formatStackFailureSummary(m.language(), summary), width))
	}
	if len(retrying) > 0 {
		lines = append(lines, renderCompactKeyValue(m.t("Backoff", "退避"), strings.Join(retrying, " • "), width))
	}
	return lines
}

func (m Model) renderStackRuntimeHistoryLines(view app.StackView, width int) []string {
	items := make([]runtimeTimelineItem, 0)
	for _, member := range view.Members {
		items = append(items, runtimeTimelineEntries(member.State.RecentLogs, member.Profile.Name)...)
	}
	items = limitedRuntimeTimeline(items, 4)
	return m.renderRuntimeTimelineLines(items, width)
}

func (m Model) renderStackDraftGuideLines(view app.StackView, width int) []string {
	if !containsDraftLabel(view.Stack.Labels) {
		return nil
	}

	editor := newStackEditorState(view.Stack, view.Stack.Name, m.language())
	nextKey := recommendedStackEditorField(view.Stack)
	field, ok := editor.fieldByKey(nextKey)
	nextLabel := m.t("stack details", "组合信息")
	if ok {
		nextLabel = field.label
	}

	lines := []string{groupTitleStyle.Render(m.t("Draft Guide", "草稿向导"))}
	lines = append(lines, renderCompactKeyValue(m.t("Next", "下一步"), nextLabel, width))
	lines = append(lines, renderCompactKeyValue(m.t("Action", "动作"), m.stackDraftGuideAction(view.Stack, nextLabel), width))
	return lines
}

func aggregateStackFailureReasons(members []app.ProfileView) []stackFailureSummary {
	reasons := make(map[string][]string)
	for _, member := range members {
		reason := runtimeFailureReason(member.State)
		if reason == "" {
			continue
		}
		reasons[reason] = append(reasons[reason], member.Profile.Name)
	}

	summaries := make([]stackFailureSummary, 0, len(reasons))
	for reason, names := range reasons {
		sort.Strings(names)
		summaries = append(summaries, stackFailureSummary{reason: reason, members: names})
	}
	sort.Slice(summaries, func(i, j int) bool {
		if len(summaries[i].members) == len(summaries[j].members) {
			return summaries[i].reason < summaries[j].reason
		}
		return len(summaries[i].members) > len(summaries[j].members)
	})
	if len(summaries) > 3 {
		return summaries[:3]
	}
	return summaries
}

func retryingMemberSignals(members []app.ProfileView) []string {
	signals := make([]string, 0, len(members))
	for _, member := range members {
		message := latestRetryBackoffMessage(member.State.RecentLogs)
		if message == "" {
			continue
		}
		signals = append(signals, fmt.Sprintf("%s (%s)", member.Profile.Name, message))
	}
	if len(signals) > 2 {
		return append(signals[:2], "...")
	}
	return signals
}

func (m Model) renderRuntimeTimelineLines(items []runtimeTimelineItem, width int) []string {
	if len(items) == 0 {
		return nil
	}

	lines := []string{groupTitleStyle.Render(m.t("Recent Runtime", "最近运行轨迹"))}
	for _, item := range items {
		value := item.message
		if item.profileName != "" {
			value = item.profileName + " • " + value
		}
		lines = append(lines, renderCompactKeyValue(item.timestamp.Format("15:04:05"), value, width))
	}
	return lines
}

func runtimeTimelineEntries(entries []domain.LogEntry, profileName string) []runtimeTimelineItem {
	items := make([]runtimeTimelineItem, 0, len(entries))
	for _, entry := range entries {
		if entry.Source != domain.LogSourceSystem {
			continue
		}
		message := normalizeLogMessage(entry.Message)
		if !isRuntimeTimelineMessage(message) {
			continue
		}
		items = append(items, runtimeTimelineItem{
			timestamp:   entry.Timestamp,
			profileName: profileName,
			message:     message,
		})
	}
	return items
}

func limitedRuntimeTimeline(items []runtimeTimelineItem, limit int) []runtimeTimelineItem {
	if len(items) == 0 {
		return nil
	}

	sort.Slice(items, func(i, j int) bool {
		switch {
		case items[i].timestamp.After(items[j].timestamp):
			return true
		case items[i].timestamp.Before(items[j].timestamp):
			return false
		default:
			return items[i].profileName < items[j].profileName
		}
	})

	if limit > 0 && len(items) > limit {
		return items[:limit]
	}
	return items
}

func isRuntimeTimelineMessage(message string) bool {
	switch {
	case strings.HasPrefix(message, "starting command:"):
		return true
	case strings.HasPrefix(message, "process started with pid "):
		return true
	case strings.HasPrefix(message, "process exited with code "):
		return true
	case strings.HasPrefix(message, "restarting in "):
		return true
	case strings.HasPrefix(message, "retrying after launch failure in "):
		return true
	case message == "process stopped":
		return true
	default:
		return false
	}
}

func formatStackFailureSummary(language domain.Language, summary stackFailureSummary) string {
	names := summary.members
	if len(names) > 3 {
		names = append(append([]string(nil), names[:3]...), "...")
	}
	return translatef(
		language,
		"%s • %s • %s",
		"%s • %s • %s",
		summary.reason,
		formatCountNoun(language, len(summary.members), "member", "members", "个成员"),
		strings.Join(names, ", "),
	)
}

func runtimeFailureReason(state domain.RuntimeState) string {
	lastError := strings.TrimSpace(state.LastError)
	if lastError != "" && !strings.EqualFold(lastError, "stopped by user") {
		return lastError
	}

	exitReason := strings.TrimSpace(state.ExitReason)
	if exitReason != "" && !strings.EqualFold(exitReason, "stopped by user") && !strings.EqualFold(exitReason, "process exited") {
		return exitReason
	}

	if state.LastExitCode != 0 {
		return fmt.Sprintf("exit code %d", state.LastExitCode)
	}

	return ""
}

func latestRetryBackoffMessage(entries []domain.LogEntry) string {
	for idx := len(entries) - 1; idx >= 0; idx-- {
		entry := entries[idx]
		if entry.Source != domain.LogSourceSystem {
			continue
		}
		message := normalizeLogMessage(entry.Message)
		if strings.Contains(message, "restarting in ") || strings.Contains(message, "retrying after launch failure in ") {
			return message
		}
	}
	return ""
}

func latestLogMessage(entries []domain.LogEntry, source domain.LogSource) string {
	for idx := len(entries) - 1; idx >= 0; idx-- {
		entry := entries[idx]
		if entry.Source != source {
			continue
		}
		return normalizeLogMessage(entry.Message)
	}
	return ""
}

func plainLogLine(timestamp time.Time, profileName string, source domain.LogSource, message string) string {
	parts := []string{timestamp.Format("2006-01-02 15:04:05"), plainLogSourceLabel(source)}
	if profileName != "" {
		parts = append(parts, "["+profileName+"]")
	}
	parts = append(parts, normalizeLogMessage(message))
	return strings.Join(parts, " ")
}

func plainLogSourceLabel(source domain.LogSource) string {
	switch source {
	case domain.LogSourceStdout:
		return "OUT"
	case domain.LogSourceStderr:
		return "ERR"
	default:
		return "SYS"
	}
}

func (m Model) profileDraftGuideAction(profile domain.Profile, nextLabel string) string {
	switch {
	case hasTUILabel(profile.Labels, "ssh-config"):
		return m.tf("Press e to confirm %s and replace the placeholder SSH target imported from ~/.ssh/config.", "按 e 确认 %s，并把从 ~/.ssh/config 导入的占位目标改成真实值。", nextLabel)
	case hasTUILabel(profile.Labels, "kube-context"):
		return m.tf("Press e to confirm %s and replace the placeholder Kubernetes resource before connecting.", "按 e 确认 %s，并先把占位的 Kubernetes 资源改成真实值。", nextLabel)
	default:
		return m.tf("Press e to fill %s, then save and remove the draft label when this tunnel is ready.", "按 e 先补 %s，准备好后保存并移除 draft 标签。", nextLabel)
	}
}

func (m Model) stackDraftGuideAction(stack domain.Stack, nextLabel string) string {
	if len(stack.Profiles) == 0 {
		return m.tf("Press e to add %s. You can paste comma/newline-separated profile names to build the stack quickly.", "按 e 添加 %s。也可以直接粘贴逗号或换行分隔的 profile 名称来快速生成组合。", nextLabel)
	}
	return m.tf("Press e to confirm %s, then save and remove the draft label when this stack is ready.", "按 e 确认 %s，准备好后保存并移除 draft 标签。", nextLabel)
}

func containsDraftLabel(labels []string) bool {
	return hasTUILabel(labels, "draft")
}

func hasTUILabel(labels []string, want string) bool {
	for _, label := range labels {
		if label == want {
			return true
		}
	}
	return false
}
