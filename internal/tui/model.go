package tui

import (
	"fmt"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/urzeye/lazytunnel/internal/app"
	"github.com/urzeye/lazytunnel/internal/domain"
	ltruntime "github.com/urzeye/lazytunnel/internal/runtime"
)

var (
	appStyle = lipgloss.NewStyle().
			Padding(1, 2)
	titleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("86"))
	subtitleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("246"))
	panelStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("238")).
			Padding(0, 1)
	panelTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("24")).
			Padding(0, 1)
	panelTitleMutedStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("252")).
				Background(lipgloss.Color("238")).
				Padding(0, 1)
	summaryCardStyle = lipgloss.NewStyle().
				Border(lipgloss.NormalBorder()).
				BorderForeground(lipgloss.Color("240")).
				Padding(0, 1)
	listRowStyle = lipgloss.NewStyle().
			Padding(0, 1)
	selectedListRowStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("24"))
	selectedOutlineRowStyle = lipgloss.NewStyle().
				Padding(0, 1).
				Border(lipgloss.NormalBorder(), false, false, false, true).
				BorderForeground(lipgloss.Color("24"))
	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))
	selectedMutedStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))
	sectionTextStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))
	groupTitleStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("223"))
	keyStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("110")).
			Width(12)
	codeStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("229")).
			Background(lipgloss.Color("236")).
			Padding(0, 1)
	errorBannerStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("124")).
				Padding(0, 1)
	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("246"))
)

const (
	defaultContentWidth = 80
	minTwoColumnWidth   = 112
	minLogLines         = 5
	maxLogLines         = 10
)

type runtimeEventMsg ltruntime.Event
type clockTickMsg time.Time

type listFocus int

const (
	focusProfiles listFocus = iota
	focusStacks
)

type Model struct {
	service         *app.Service
	configPath      string
	subscriptionID  int
	events          <-chan ltruntime.Event
	selectedProfile int
	selectedStack   int
	focus           listFocus
	width           int
	height          int
	now             time.Time
	lastError       string
}

func NewModel(service *app.Service, configPath string) Model {
	subscriptionID, events := service.Subscribe(64)

	return Model{
		service:        service,
		configPath:     configPath,
		subscriptionID: subscriptionID,
		events:         events,
		now:            time.Now(),
	}
}

func (m Model) Init() tea.Cmd {
	return tea.Batch(waitForRuntimeEvent(m.events), tickClock())
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case clockTickMsg:
		m.now = time.Time(msg)
		return m, tickClock()

	case runtimeEventMsg:
		return m, waitForRuntimeEvent(m.events)

	case tea.KeyMsg:
		profiles := m.service.ProfileViews()
		stacks := m.service.StackViews()
		m = m.normalizeSelection(len(profiles), len(stacks))

		switch msg.String() {
		case "q", "ctrl+c":
			m.service.Unsubscribe(m.subscriptionID)
			return m, tea.Quit
		case "tab":
			if len(stacks) > 0 {
				if m.focus == focusProfiles {
					m.focus = focusStacks
				} else {
					m.focus = focusProfiles
				}
			}
		case "1":
			m.focus = focusProfiles
		case "2":
			if len(stacks) > 0 {
				m.focus = focusStacks
			}
		case "j", "down":
			if m.focus == focusStacks {
				if len(stacks) > 0 && m.selectedStack < len(stacks)-1 {
					m.selectedStack++
				}
			} else if len(profiles) > 0 && m.selectedProfile < len(profiles)-1 {
				m.selectedProfile++
			}
		case "k", "up":
			if m.focus == focusStacks {
				if len(stacks) > 0 && m.selectedStack > 0 {
					m.selectedStack--
				}
			} else if len(profiles) > 0 && m.selectedProfile > 0 {
				m.selectedProfile--
			}
		case "enter", "s":
			m.lastError = ""
			if m.focus == focusStacks && len(stacks) > 0 {
				if err := m.service.ToggleStack(stacks[m.selectedStack].Stack.Name); err != nil {
					m.lastError = err.Error()
				}
				return m, nil
			}

			if len(profiles) > 0 {
				if err := m.service.ToggleProfile(profiles[m.selectedProfile].Profile.Name); err != nil {
					m.lastError = err.Error()
				}
			}
			return m, nil
		}
	}

	return m, nil
}

func (m Model) View() string {
	profiles := m.service.ProfileViews()
	stacks := m.service.StackViews()
	m = m.normalizeSelection(len(profiles), len(stacks))

	sections := []string{
		m.renderHeader(profiles, stacks),
		m.renderMain(profiles, stacks),
	}

	if m.lastError != "" {
		sections = append(sections, errorBannerStyle.Render("Last error: "+m.lastError))
	}

	sections = append(sections, hintStyle.Render(
		"Tab or 1/2 to switch lists, j/k or arrows to move, Enter or s to start or stop the selection, q to quit.",
	))

	return appStyle.Width(m.contentWidth()).Render(strings.Join(sections, "\n"))
}

func (m Model) renderHeader(profiles []app.ProfileView, stacks []app.StackView) string {
	titleBlock := lipgloss.JoinVertical(
		lipgloss.Left,
		titleStyle.Render("LazyTunnel"),
		subtitleStyle.Render("Keyboard-first tunnel workspace for SSH and Kubernetes forwards."),
	)

	stats := []string{
		renderKeyValueLine("Config", m.configPath, 42),
		renderKeyValueLine("Profiles", fmt.Sprintf("%d", len(profiles)), 42),
		renderKeyValueLine("Stacks", fmt.Sprintf("%d", len(stacks)), 42),
		renderKeyValueLine("Active", fmt.Sprintf("%d", countActiveProfiles(profiles)), 42),
		renderKeyValueLine("Selected", m.selectedLabel(profiles, stacks), 42),
	}
	statsBlock := renderSizedBlock(summaryCardStyle, min(48, m.contentWidth()), strings.Join(stats, "\n"))

	if m.contentWidth() >= minTwoColumnWidth {
		leftWidth := max(24, m.contentWidth()-lipgloss.Width(statsBlock)-1)
		return lipgloss.JoinHorizontal(
			lipgloss.Top,
			lipgloss.NewStyle().Width(leftWidth).Render(titleBlock),
			statsBlock,
		)
	}

	return lipgloss.JoinVertical(lipgloss.Left, titleBlock, statsBlock)
}

func (m Model) renderMain(profiles []app.ProfileView, stacks []app.StackView) string {
	width := m.contentWidth()
	if width >= minTwoColumnWidth {
		leftWidth := min(44, max(36, width/3))
		rightWidth := max(48, width-leftWidth-1)

		left := lipgloss.JoinVertical(
			lipgloss.Left,
			m.renderProfilesPanel(profiles, leftWidth),
			m.renderStacksPanel(stacks, leftWidth),
		)
		right := lipgloss.JoinVertical(
			lipgloss.Left,
			m.renderSelectionDetailsPanel(profiles, stacks, rightWidth),
			m.renderSelectionLogsPanel(profiles, stacks, rightWidth),
		)

		return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
	}

	return lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderProfilesPanel(profiles, width),
		m.renderStacksPanel(stacks, width),
		m.renderSelectionDetailsPanel(profiles, stacks, width),
		m.renderSelectionLogsPanel(profiles, stacks, width),
	)
}

func (m Model) renderProfilesPanel(views []app.ProfileView, width int) string {
	return renderPanel("Profiles", m.renderProfiles(views, width-4), width, m.focus == focusProfiles)
}

func (m Model) renderStacksPanel(views []app.StackView, width int) string {
	return renderPanel("Stacks", m.renderStacks(views, width-4), width, m.focus == focusStacks)
}

func (m Model) renderProfiles(views []app.ProfileView, width int) string {
	if len(views) == 0 {
		return mutedStyle.Render("No profiles yet. Run `lazytunnel init --sample` to start from the example config.")
	}

	rows := make([]string, 0, len(views))
	for idx, view := range views {
		selected := idx == m.selectedProfile
		rows = append(rows, m.renderProfileRow(view, selected, m.focus == focusProfiles, width))
	}

	return strings.Join(rows, "\n")
}

func (m Model) renderProfileRow(view app.ProfileView, selected bool, focused bool, width int) string {
	title := fmt.Sprintf("%s  %s", renderStatusBadge(view.State.Status), view.Profile.Name)
	meta := truncateText(
		fmt.Sprintf("localhost:%d -> %s", view.Profile.LocalPort, profileTarget(view.Profile)),
		max(16, width-2),
	)

	metaStyle := mutedStyle
	if selected {
		metaStyle = selectedMutedStyle
	}

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		metaStyle.Render(meta),
	)

	switch {
	case selected && focused:
		return renderSizedBlock(selectedListRowStyle, width, body)
	case selected:
		return renderSizedBlock(selectedOutlineRowStyle, width, body)
	default:
		return renderSizedBlock(listRowStyle, width, body)
	}
}

func (m Model) renderStacks(views []app.StackView, width int) string {
	if len(views) == 0 {
		return mutedStyle.Render("No stacks yet. Add stacks to your config to launch groups of tunnels together.")
	}

	rows := make([]string, 0, len(views))
	for idx, view := range views {
		selected := idx == m.selectedStack
		rows = append(rows, m.renderStackRow(view, selected, m.focus == focusStacks, width))
	}

	return strings.Join(rows, "\n")
}

func (m Model) renderStackRow(view app.StackView, selected bool, focused bool, width int) string {
	title := fmt.Sprintf("%s  %s", renderStackStatusBadge(view.Status), view.Stack.Name)
	meta := truncateText(
		fmt.Sprintf("%d/%d active • %s", view.ActiveCount, len(view.Members), stackMembersSummary(view)),
		max(16, width-2),
	)

	metaStyle := mutedStyle
	if selected {
		metaStyle = selectedMutedStyle
	}

	body := lipgloss.JoinVertical(
		lipgloss.Left,
		title,
		metaStyle.Render(meta),
	)

	switch {
	case selected && focused:
		return renderSizedBlock(selectedListRowStyle, width, body)
	case selected:
		return renderSizedBlock(selectedOutlineRowStyle, width, body)
	default:
		return renderSizedBlock(listRowStyle, width, body)
	}
}

func (m Model) renderSelectionDetailsPanel(profiles []app.ProfileView, stacks []app.StackView, width int) string {
	if m.focus == focusStacks && len(stacks) > 0 {
		return renderPanel("Stack Details", m.renderStackDetails(stacks[m.selectedStack], width-4), width, false)
	}

	if len(profiles) == 0 {
		return renderPanel("Profile Details", mutedStyle.Render("No profile selected."), width, false)
	}

	return renderPanel("Profile Details", m.renderProfileDetails(profiles[m.selectedProfile], width-4), width, false)
}

func (m Model) renderSelectionLogsPanel(profiles []app.ProfileView, stacks []app.StackView, width int) string {
	if m.focus == focusStacks && len(stacks) > 0 {
		return renderPanel("Stack Activity", m.renderStackLogs(stacks[m.selectedStack], width-4), width, false)
	}

	if len(profiles) == 0 {
		return renderPanel("Recent Logs", mutedStyle.Render("Start a tunnel to see runtime output here."), width, false)
	}

	return renderPanel("Recent Logs", m.renderProfileLogs(profiles[m.selectedProfile], width-4), width, false)
}

func (m Model) renderProfileDetails(view app.ProfileView, width int) string {
	spec, specErr := app.BuildProcessSpec(view.Profile)
	now := m.currentTime()

	overview := []string{
		renderKeyValueLine("Kind", humanTunnelType(view.Profile.Type), width),
		renderKeyValueLine("Local", fmt.Sprintf("localhost:%d", view.Profile.LocalPort), width),
		renderKeyValueLine("Target", profileTarget(view.Profile), width),
	}

	runtimeLines := []string{
		renderKeyValueLine("Status", humanTunnelStatus(view.State.Status), width),
		renderKeyValueLine("PID", formatPID(view.State.PID), width),
		renderKeyValueLine("Uptime", formatUptime(view.State.StartedAt, now), width),
		renderKeyValueLine("Restarts", fmt.Sprintf("%d", view.State.RestartCount), width),
		renderKeyValueLine("Last Exit", formatLastExit(view.State, now), width),
		renderKeyValueLine("Restart", restartPolicySummary(view.Profile.Restart), width),
	}

	configLines := []string{}
	if view.Profile.Description != "" {
		configLines = append(configLines, renderKeyValueLine("About", view.Profile.Description, width))
	}
	if len(view.Profile.Labels) > 0 {
		configLines = append(configLines, renderKeyValueLine("Labels", strings.Join(view.Profile.Labels, ", "), width))
	}

	sections := []string{
		renderDetailHeading(view.Profile.Name, renderStatusBadge(view.State.Status), humanTunnelType(view.Profile.Type), width),
		renderDetailGroup("Overview", overview),
		renderDetailGroup("Runtime", runtimeLines),
	}

	if len(configLines) > 0 {
		sections = append(sections, renderDetailGroup("Config", configLines))
	}

	if specErr == nil {
		sections = append(sections, renderDetailCodeGroup("Command", spec.DisplayCommand(), width))
	}

	if view.State.LastError != "" {
		sections = append(sections, renderDetailGroup("Problem", []string{
			renderKeyValueLine("Error", view.State.LastError, width),
		}))
	}

	return strings.Join(sections, "\n\n")
}

func (m Model) renderStackDetails(view app.StackView, width int) string {
	overview := []string{
		renderKeyValueLine("Status", humanStackStatus(view.Status), width),
		renderKeyValueLine("Members", fmt.Sprintf("%d", len(view.Members)), width),
		renderKeyValueLine("Active", fmt.Sprintf("%d", view.ActiveCount), width),
		renderKeyValueLine("Coverage", fmt.Sprintf("%d/%d running", view.ActiveCount, len(view.Members)), width),
	}

	configLines := []string{}
	if view.Stack.Description != "" {
		configLines = append(configLines, renderKeyValueLine("About", view.Stack.Description, width))
	}
	if len(view.Stack.Labels) > 0 {
		configLines = append(configLines, renderKeyValueLine("Labels", strings.Join(view.Stack.Labels, ", "), width))
	}

	memberLines := make([]string, 0, len(view.Members))
	for _, member := range view.Members {
		memberLines = append(memberLines, fmt.Sprintf(
			"%s  %s  localhost:%d -> %s",
			renderStatusBadge(member.State.Status),
			member.Profile.Name,
			member.Profile.LocalPort,
			profileTarget(member.Profile),
		))
	}
	if len(memberLines) == 0 {
		memberLines = append(memberLines, mutedStyle.Render("No member profiles resolved from config."))
	}

	sections := []string{
		renderDetailHeading(view.Stack.Name, renderStackStatusBadge(view.Status), humanStackStatus(view.Status), width),
		renderDetailGroup("Overview", overview),
		renderDetailGroup("Members", memberLines),
	}

	if len(configLines) > 0 {
		sections = append(sections, renderDetailGroup("Config", configLines))
	}

	if view.Status == app.StackStatusPartial {
		sections = append(sections, renderDetailGroup("Action", []string{
			sectionTextStyle.Render("Press Enter to start the missing members and restore the stack."),
		}))
	}

	return strings.Join(sections, "\n\n")
}

func (m Model) renderProfileLogs(view app.ProfileView, width int) string {
	logs := tailLogs(view.State.RecentLogs, m.logLimit())
	if len(logs) == 0 {
		return mutedStyle.Render("No logs yet. Start the tunnel to see stdout, stderr, and supervisor events.")
	}

	lines := make([]string, 0, len(logs))
	for _, entry := range logs {
		lines = append(lines, renderLogLine(entry.Timestamp, "", entry.Source, entry.Message, width))
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderStackLogs(view app.StackView, width int) string {
	activity := recentStackActivity(view, m.logLimit())
	if len(activity) == 0 {
		return mutedStyle.Render("No recent stack activity yet. Start a member to begin collecting logs.")
	}

	lines := make([]string, 0, len(activity))
	for _, entry := range activity {
		lines = append(lines, renderLogLine(entry.Log.Timestamp, entry.ProfileName, entry.Log.Source, entry.Log.Message, width))
	}

	return strings.Join(lines, "\n")
}

func (m Model) normalizeSelection(profileCount, stackCount int) Model {
	if profileCount == 0 {
		m.selectedProfile = 0
	} else {
		m.selectedProfile = max(0, min(m.selectedProfile, profileCount-1))
	}

	if stackCount == 0 {
		m.selectedStack = 0
		if m.focus == focusStacks {
			m.focus = focusProfiles
		}
		return m
	}

	m.selectedStack = max(0, min(m.selectedStack, stackCount-1))
	return m
}

func (m Model) selectedLabel(profiles []app.ProfileView, stacks []app.StackView) string {
	if m.focus == focusStacks && len(stacks) > 0 {
		return "stack/" + stacks[m.selectedStack].Stack.Name
	}
	if len(profiles) > 0 {
		return "profile/" + profiles[m.selectedProfile].Profile.Name
	}
	return "none"
}

func (m Model) contentWidth() int {
	if m.width <= 0 {
		return defaultContentWidth
	}
	if m.width <= 4 {
		return m.width
	}
	return m.width - 4
}

func (m Model) currentTime() time.Time {
	if m.now.IsZero() {
		return time.Now()
	}
	return m.now
}

func (m Model) logLimit() int {
	if m.height <= 0 {
		return 7
	}

	limit := m.height / 5
	return min(max(limit, minLogLines), maxLogLines)
}

func tickClock() tea.Cmd {
	return tea.Tick(time.Second, func(t time.Time) tea.Msg {
		return clockTickMsg(t)
	})
}

func waitForRuntimeEvent(events <-chan ltruntime.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return nil
		}
		return runtimeEventMsg(event)
	}
}

func renderPanel(title, body string, width int, focused bool) string {
	titleStyle := panelTitleMutedStyle
	if focused {
		titleStyle = panelTitleStyle
	}

	innerWidth := max(1, width-panelStyle.GetHorizontalFrameSize())
	return renderSizedBlock(
		panelStyle,
		width,
		lipgloss.JoinVertical(
			lipgloss.Left,
			titleStyle.Render(title),
			lipgloss.NewStyle().Width(innerWidth).Render(body),
		),
	)
}

func renderDetailHeading(name, badge, subtitle string, width int) string {
	lines := []string{
		lipgloss.JoinHorizontal(lipgloss.Center, badge, "  ", sectionTextStyle.Render(name)),
	}

	if subtitle != "" {
		lines = append(lines, mutedStyle.Copy().Width(max(1, width)).Render(subtitle))
	}

	return strings.Join(lines, "\n")
}

func renderDetailGroup(title string, lines []string) string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		groupTitleStyle.Render(title),
		strings.Join(lines, "\n"),
	)
}

func renderDetailCodeGroup(title, command string, width int) string {
	return lipgloss.JoinVertical(
		lipgloss.Left,
		groupTitleStyle.Render(title),
		renderSizedBlock(codeStyle, max(1, width), command),
	)
}

func renderKeyValueLine(label, value string, width int) string {
	if strings.TrimSpace(value) == "" {
		value = "-"
	}

	valueWidth := max(1, width-keyStyle.GetWidth())
	return lipgloss.JoinHorizontal(
		lipgloss.Top,
		keyStyle.Render(label),
		sectionTextStyle.Copy().Width(valueWidth).Render(value),
	)
}

func renderLogLine(timestamp time.Time, profileName string, source domain.LogSource, message string, width int) string {
	prefixParts := []string{
		mutedStyle.Render(timestamp.Format("15:04:05")),
		renderLogSourceBadge(source),
	}
	if profileName != "" {
		prefixParts = append(prefixParts, lipgloss.NewStyle().Foreground(lipgloss.Color("110")).Render(profileName))
	}

	content := lipgloss.JoinHorizontal(lipgloss.Center, prefixParts...)
	if strings.TrimSpace(message) == "" {
		message = "(empty)"
	}

	return lipgloss.NewStyle().Width(max(1, width)).Render(content + " " + message)
}

func renderSizedBlock(style lipgloss.Style, width int, body string) string {
	if width <= 0 {
		return style.Render(body)
	}

	innerWidth := max(1, width-style.GetHorizontalFrameSize())
	return style.Render(lipgloss.NewStyle().Width(innerWidth).Render(body))
}

func renderStatusBadge(status domain.TunnelStatus) string {
	label := " STP "
	style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("240"))

	switch status {
	case domain.TunnelStatusRunning:
		label = " RUN "
		style = style.Background(lipgloss.Color("29"))
	case domain.TunnelStatusStarting:
		label = " NEW "
		style = style.Background(lipgloss.Color("31"))
	case domain.TunnelStatusRestarting:
		label = " RET "
		style = style.Background(lipgloss.Color("136"))
	case domain.TunnelStatusFailed:
		label = " ERR "
		style = style.Background(lipgloss.Color("124"))
	case domain.TunnelStatusExited:
		label = " EXT "
		style = style.Background(lipgloss.Color("239"))
	}

	return style.Render(label)
}

func renderStackStatusBadge(status app.StackStatus) string {
	label := " STP "
	style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("240"))

	switch status {
	case app.StackStatusRunning:
		label = " RUN "
		style = style.Background(lipgloss.Color("29"))
	case app.StackStatusPartial:
		label = " PAR "
		style = style.Background(lipgloss.Color("136"))
	}

	return style.Render(label)
}

func renderLogSourceBadge(source domain.LogSource) string {
	label := "SYS"
	style := lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("230")).Background(lipgloss.Color("238")).Padding(0, 1)

	switch source {
	case domain.LogSourceStdout:
		label = "OUT"
		style = style.Background(lipgloss.Color("24"))
	case domain.LogSourceStderr:
		label = "ERR"
		style = style.Background(lipgloss.Color("124"))
	}

	return style.Render(label)
}

func humanTunnelStatus(status domain.TunnelStatus) string {
	switch status {
	case domain.TunnelStatusRunning:
		return "Running"
	case domain.TunnelStatusStarting:
		return "Starting"
	case domain.TunnelStatusRestarting:
		return "Restarting"
	case domain.TunnelStatusFailed:
		return "Failed"
	case domain.TunnelStatusExited:
		return "Exited"
	default:
		return "Stopped"
	}
}

func humanStackStatus(status app.StackStatus) string {
	switch status {
	case app.StackStatusRunning:
		return "Running"
	case app.StackStatusPartial:
		return "Partially Active"
	default:
		return "Stopped"
	}
}

func humanTunnelType(kind domain.TunnelType) string {
	switch kind {
	case domain.TunnelTypeSSHLocal:
		return "SSH local forward"
	case domain.TunnelTypeKubernetesPortForward:
		return "Kubernetes port-forward"
	default:
		return string(kind)
	}
}

func profileTarget(profile domain.Profile) string {
	switch profile.Type {
	case domain.TunnelTypeSSHLocal:
		if profile.SSH == nil {
			return "SSH target not configured"
		}
		return fmt.Sprintf("%s -> %s:%d", profile.SSH.Host, profile.SSH.RemoteHost, profile.SSH.RemotePort)
	case domain.TunnelTypeKubernetesPortForward:
		if profile.Kubernetes == nil {
			return "Kubernetes target not configured"
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
		return "Unknown target"
	}
}

func stackMembersSummary(view app.StackView) string {
	if len(view.Members) == 0 {
		return "no members"
	}

	names := make([]string, 0, len(view.Members))
	for _, member := range view.Members {
		names = append(names, member.Profile.Name)
	}

	return strings.Join(names, ", ")
}

func formatPID(pid int) string {
	if pid == 0 {
		return "-"
	}
	return fmt.Sprintf("%d", pid)
}

func formatUptime(startedAt *time.Time, now time.Time) string {
	if startedAt == nil {
		return "-"
	}
	return humanizeDuration(now.Sub(*startedAt))
}

func formatLastExit(state domain.RuntimeState, now time.Time) string {
	parts := make([]string, 0, 3)

	if state.ExitReason != "" {
		parts = append(parts, state.ExitReason)
	}
	if state.ExitedAt != nil {
		parts = append(parts, humanizeDuration(now.Sub(*state.ExitedAt))+" ago")
	}
	if state.ExitedAt != nil || state.LastExitCode != 0 {
		parts = append(parts, fmt.Sprintf("code %d", state.LastExitCode))
	}

	if len(parts) == 0 {
		return "-"
	}
	return strings.Join(parts, " • ")
}

func restartPolicySummary(policy domain.RestartPolicy) string {
	if !policy.Enabled {
		return "disabled"
	}

	maxRetries := "unlimited retries"
	if policy.MaxRetries > 0 {
		maxRetries = fmt.Sprintf("%d retries", policy.MaxRetries)
	}

	initialBackoff := policy.InitialBackoff
	if initialBackoff == "" {
		initialBackoff = "2s"
	}

	maxBackoff := policy.MaxBackoff
	if maxBackoff == "" {
		maxBackoff = "30s"
	}

	return fmt.Sprintf("%s, %s to %s", maxRetries, initialBackoff, maxBackoff)
}

func humanizeDuration(duration time.Duration) string {
	if duration < 0 {
		duration = 0
	}

	duration = duration.Round(time.Second)
	if duration < time.Second {
		return "0s"
	}
	if duration < time.Minute {
		return fmt.Sprintf("%ds", int(duration.Seconds()))
	}
	if duration < time.Hour {
		return fmt.Sprintf("%dm%02ds", int(duration.Minutes()), int(duration.Seconds())%60)
	}
	if duration < 24*time.Hour {
		return fmt.Sprintf("%dh%02dm", int(duration.Hours()), int(duration.Minutes())%60)
	}

	days := int(duration / (24 * time.Hour))
	hours := int(duration.Hours()) % 24
	return fmt.Sprintf("%dd%02dh", days, hours)
}

type stackActivityEntry struct {
	ProfileName string
	Log         domain.LogEntry
}

func recentStackActivity(view app.StackView, limit int) []stackActivityEntry {
	entries := make([]stackActivityEntry, 0)
	for _, member := range view.Members {
		for _, entry := range member.State.RecentLogs {
			entries = append(entries, stackActivityEntry{
				ProfileName: member.Profile.Name,
				Log:         entry,
			})
		}
	}

	slices.SortFunc(entries, func(a, b stackActivityEntry) int {
		switch {
		case a.Log.Timestamp.Before(b.Log.Timestamp):
			return -1
		case a.Log.Timestamp.After(b.Log.Timestamp):
			return 1
		default:
			return strings.Compare(a.ProfileName, b.ProfileName)
		}
	})

	if len(entries) <= limit {
		return entries
	}
	return entries[len(entries)-limit:]
}

func countActiveProfiles(views []app.ProfileView) int {
	total := 0
	for _, view := range views {
		switch view.State.Status {
		case domain.TunnelStatusStarting, domain.TunnelStatusRunning, domain.TunnelStatusRestarting:
			total++
		}
	}

	return total
}

func tailLogs(entries []domain.LogEntry, limit int) []domain.LogEntry {
	if len(entries) <= limit {
		return entries
	}

	return entries[len(entries)-limit:]
}

func truncateText(value string, limit int) string {
	if limit <= 0 {
		return value
	}

	runes := []rune(value)
	if len(runes) <= limit {
		return value
	}
	if limit <= 1 {
		return string(runes[:limit])
	}

	return string(runes[:limit-1]) + "…"
}
