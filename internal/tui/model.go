package tui

import (
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/urzeye/lazytunnel/internal/app"
	"github.com/urzeye/lazytunnel/internal/domain"
	ltruntime "github.com/urzeye/lazytunnel/internal/runtime"
	"github.com/urzeye/lazytunnel/internal/storage"
)

var (
	appStyle = lipgloss.NewStyle().
			Padding(0, 1)
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
				Bold(true).
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("24")).
				Padding(0, 1)
	selectedOutlineRowStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("238")).
				Padding(0, 1)
	selectedMarkerStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("86"))
	selectedOutlineMarkerStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("110"))
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
	noticeBannerStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("29")).
				Padding(0, 1)
	deletePromptStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("94")).
				Padding(0, 1)
	filterIdleStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("250")).
			Background(lipgloss.Color("238")).
			Padding(0, 1)
	filterActiveStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("62")).
				Padding(0, 1)
	filterInputIdleStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252")).
				Background(lipgloss.Color("236")).
				Padding(0, 1)
	filterInputActiveStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("24")).
				Padding(0, 1)
	filterPromptStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("110"))
	filterPlaceholderStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))
	headerMetaLabelStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("246"))
	headerMetaSeparatorStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("240"))
	headerSelectedValueStyle = lipgloss.NewStyle().
					Bold(true).
					Foreground(lipgloss.Color("230")).
					Background(lipgloss.Color("24")).
					Padding(0, 1)
	headerSelectedEmptyValueStyle = lipgloss.NewStyle().
					Foreground(lipgloss.Color("250")).
					Background(lipgloss.Color("238")).
					Padding(0, 1)
	headerConfigValueStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("223")).
				Background(lipgloss.Color("236")).
				Padding(0, 1)
	hintStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("246"))
)

const (
	defaultContentWidth  = 80
	defaultContentHeight = 24
	minTwoColumnWidth    = 112
	minTwoColumnHeight   = 18
)

type runtimeEventMsg ltruntime.Event
type clockTickMsg time.Time
type editorFinishedMsg struct {
	err error
}

type listFocus int
type inspectorTab int

const (
	focusProfiles listFocus = iota
	focusStacks
)

const (
	inspectorTabDetails inspectorTab = iota
	inspectorTabLogs
)

type deleteKind string

const (
	deleteKindProfile deleteKind = "profile"
	deleteKindStack   deleteKind = "stack"
)

type deleteRequest struct {
	Kind    deleteKind
	Name    string
	Message string
}

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
	filterQuery     string
	filterMode      bool
	pendingDelete   *deleteRequest
	inspectorTab    inspectorTab
	inspectorScroll int
	lastNotice      string
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

	case editorFinishedMsg:
		if msg.err != nil {
			m.lastError = "Editor exited with an error: " + msg.err.Error()
			return m, nil
		}

		m = m.reloadConfigFromDisk("Reloaded config after editing.")
		return m, nil

	case tea.KeyMsg:
		profiles := filterProfileViews(m.service.ProfileViews(), m.filterQuery)
		stacks := filterStackViews(m.service.StackViews(), m.filterQuery)
		m = m.normalizeSelection(len(profiles), len(stacks))

		if m.pendingDelete != nil {
			var handled bool
			m, handled = m.handleDeleteKey(msg, profiles, stacks)
			if handled {
				return m, nil
			}
		}

		if m.filterMode {
			var handled bool
			m, handled = m.handleFilterKey(msg)
			if handled {
				return m, nil
			}
		}

		{
			var handled bool
			m, handled = m.handleInspectorKey(msg, profiles, stacks)
			if handled {
				return m, nil
			}
		}

		{
			var handled bool
			var cmd tea.Cmd
			m, cmd, handled = m.handleWorkspaceKey(msg, profiles, stacks)
			if handled {
				return m, cmd
			}
		}

		switch msg.String() {
		case "q", "ctrl+c":
			m.service.Unsubscribe(m.subscriptionID)
			return m, tea.Quit
		case "/":
			m.lastNotice = ""
			m.filterMode = true
			return m, nil
		case "esc":
			if m.filterQuery != "" {
				m.filterQuery = ""
				m.selectedProfile = 0
				m.selectedStack = 0
				m.inspectorScroll = 0
				return m, nil
			}
		case "d", "x":
			m.lastError = ""
			m.lastNotice = ""
			m.filterMode = false
			m.pendingDelete = m.buildDeleteRequest(profiles, stacks)
			return m, nil
		case "tab":
			if len(stacks) > 0 {
				if m.focus == focusProfiles {
					m.focus = focusStacks
				} else {
					m.focus = focusProfiles
				}
				m.inspectorScroll = 0
			}
		case "1":
			m.focus = focusProfiles
			m.inspectorScroll = 0
		case "2":
			if len(stacks) > 0 {
				m.focus = focusStacks
				m.inspectorScroll = 0
			}
		case "j", "down":
			if m.focus == focusStacks {
				if len(stacks) > 0 && m.selectedStack < len(stacks)-1 {
					m.selectedStack++
					m.inspectorScroll = 0
				}
			} else if len(profiles) > 0 && m.selectedProfile < len(profiles)-1 {
				m.selectedProfile++
				m.inspectorScroll = 0
			}
		case "k", "up":
			if m.focus == focusStacks {
				if len(stacks) > 0 && m.selectedStack > 0 {
					m.selectedStack--
					m.inspectorScroll = 0
				}
			} else if len(profiles) > 0 && m.selectedProfile > 0 {
				m.selectedProfile--
				m.inspectorScroll = 0
			}
		case "enter", "s":
			m.lastError = ""
			m.lastNotice = ""
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
	allProfiles := m.service.ProfileViews()
	allStacks := m.service.StackViews()
	profiles := filterProfileViews(allProfiles, m.filterQuery)
	stacks := filterStackViews(allStacks, m.filterQuery)
	m = m.normalizeSelection(len(profiles), len(stacks))
	width := m.contentWidth()
	height := m.contentHeight()

	headerLines := m.renderHeaderLines(profiles, stacks, len(allProfiles), len(allStacks), width)
	body := m.renderBody(profiles, stacks, width, m.bodyHeight())

	lines := append([]string(nil), headerLines...)
	if status := m.renderStatusLine(width); status != "" {
		lines = append(lines, status)
	}
	lines = append(lines, strings.Split(body, "\n")...)
	if hint := m.renderHintLine(width); hint != "" {
		lines = append(lines, hint)
	}

	lines = normalizeLineCount(lines, height)
	return appStyle.Render(strings.Join(lines, "\n"))
}

func (m Model) renderHeaderLines(profiles []app.ProfileView, stacks []app.StackView, totalProfiles, totalStacks, width int) []string {
	focusLabel := "profiles"
	if m.focus == focusStacks && len(stacks) > 0 {
		focusLabel = "stacks"
	}

	line1 := composeStyledLine(
		titleStyle.Render("LazyTunnel")+" ",
		fmt.Sprintf(
			"profiles %s | stacks %s | active %d | focus %s",
			formatVisibleCount(len(profiles), totalProfiles),
			formatVisibleCount(len(stacks), totalStacks),
			countActiveProfiles(m.service.ProfileViews()),
			focusLabel,
		),
		width,
	)

	line2 := m.renderHeaderMetaLine(profiles, stacks, width)

	return []string{line1, line2}
}

func (m Model) renderHeaderMetaLine(profiles []app.ProfileView, stacks []app.StackView, width int) string {
	separator := headerMetaSeparatorStyle.Render(" | ")
	filterSegment := m.renderHeaderFilterSegment(preferredFilterInputWidth(width))
	selectedSegment := m.renderHeaderMetaField(
		"selected",
		m.selectedLabel(profiles, stacks),
		m.selectedValueStyle(profiles, stacks),
		preferredSelectedValueWidth(width),
	)

	usedWidth := lipgloss.Width(filterSegment) + lipgloss.Width(separator) + lipgloss.Width(selectedSegment) + lipgloss.Width(separator)
	configValueWidth := max(8, width-usedWidth-headerMetaLabelWidth("config"))
	configSegment := m.renderHeaderMetaField("config", m.configPath, headerConfigValueStyle, configValueWidth)

	return filterSegment + separator + selectedSegment + separator + configSegment
}

func (m Model) renderStatusLine(width int) string {
	switch {
	case m.pendingDelete != nil:
		return renderInlineBanner(deletePromptStyle, m.pendingDelete.Message, width)
	case m.lastError != "":
		return renderInlineBanner(errorBannerStyle, "Last error: "+m.lastError, width)
	case m.lastNotice != "":
		return renderInlineBanner(noticeBannerStyle, m.lastNotice, width)
	default:
		return ""
	}
}

func (m Model) renderHintLine(width int) string {
	if !m.showHint() {
		return ""
	}

	return renderInlineText(hintStyle, m.hintMessage(), width)
}

func (m Model) hintMessage() string {
	switch {
	case m.pendingDelete != nil:
		return "Delete mode: y or Enter confirms. n or Esc cancels."
	case m.filterMode:
		return "Filter mode: type to search. Enter finishes. Esc cancels. Backspace/Ctrl+W deletes. Ctrl+U clears."
	case m.workspaceIsEmpty():
		return "i init sample  a draft profile  e edit config  r reload  / filter  q quit"
	}

	profileCount := len(m.service.ProfileViews())
	stackCount := len(m.service.StackViews())
	switch {
	case profileCount == 0:
		return "a draft profile  e edit config  r reload  / filter  q quit"
	case stackCount == 0:
		return "j/k move  h/l inspector  c clone profile  A draft stack  e edit config  / filter  q quit"
	case m.focus == focusStacks:
		return "j/k move  Tab lists  h/l inspector  Enter toggle stack  c clone stack  A draft stack  d delete  / filter  q quit"
	default:
		return "j/k move  Tab lists  h/l inspector  Enter toggle  c clone profile  A draft stack  d delete  / filter  q quit"
	}
}

func (m Model) renderBody(profiles []app.ProfileView, stacks []app.StackView, width, height int) string {
	if height <= 0 {
		return ""
	}

	if m.useTwoColumnLayout(width, height) {
		return m.renderTwoColumnBody(profiles, stacks, width, height)
	}

	return m.renderSingleColumnBody(profiles, stacks, width, height)
}

func (m Model) useTwoColumnLayout(width, height int) bool {
	return width >= minTwoColumnWidth && height >= minTwoColumnHeight
}

func (m Model) renderTwoColumnBody(profiles []app.ProfileView, stacks []app.StackView, width, height int) string {
	leftWidth := min(44, max(36, width/3))
	rightWidth := max(32, width-leftWidth-1)
	profilesHeight, stacksHeight := splitDualListHeights(height)

	left := lipgloss.JoinVertical(
		lipgloss.Left,
		m.renderProfilesPanel(profiles, leftWidth, profilesHeight),
		m.renderStacksPanel(stacks, leftWidth, stacksHeight),
	)

	right := m.renderInspectorPanel(profiles, stacks, rightWidth, height)
	return lipgloss.JoinHorizontal(lipgloss.Top, left, right)
}

func (m Model) renderSingleColumnBody(profiles []app.ProfileView, stacks []app.StackView, width, height int) string {
	listHeight, inspectorHeight := splitListInspectorHeights(height)
	listPanel := m.renderFocusedListPanel(profiles, stacks, width, listHeight)
	inspectorPanel := m.renderInspectorPanel(profiles, stacks, width, inspectorHeight)

	return lipgloss.JoinVertical(lipgloss.Left, listPanel, inspectorPanel)
}

func (m Model) renderFocusedListPanel(profiles []app.ProfileView, stacks []app.StackView, width, height int) string {
	if m.focus == focusStacks {
		return m.renderStacksPanel(stacks, width, height)
	}
	return m.renderProfilesPanel(profiles, width, height)
}

func (m Model) renderProfilesPanel(views []app.ProfileView, width, height int) string {
	innerWidth := panelInnerWidth(width)
	bodyHeight := panelBodyHeight(height)
	focused := m.focus == focusProfiles

	title := panelListTitle("Profiles", len(views), 0, len(views))
	if len(views) == 0 {
		if m.filterQuery != "" {
			message := fmt.Sprintf("No profiles match %q. Press Esc to clear the filter.", m.filterQuery)
			return renderFixedPanel(title, []string{mutedStyle.Render(truncateText(message, innerWidth))}, width, height, focused)
		}
		if m.workspaceIsEmpty() {
			return renderFixedPanel(title, m.renderEmptyProfilesLines(innerWidth), width, height, focused)
		}
		message := "No profiles yet. Press a to add a starter draft or e to edit the config file."
		return renderFixedPanel(title, []string{mutedStyle.Render(truncateText(message, innerWidth))}, width, height, focused)
	}

	start, end := windowAroundSelection(len(views), m.selectedProfile, bodyHeight)
	title = panelListTitle("Profiles", len(views), start, end)

	rows := make([]string, 0, end-start)
	for idx := start; idx < end; idx++ {
		rows = append(rows, m.renderProfileRow(views[idx], idx == m.selectedProfile, focused, innerWidth))
	}

	return renderFixedPanel(title, rows, width, height, focused)
}

func (m Model) renderStacksPanel(views []app.StackView, width, height int) string {
	innerWidth := panelInnerWidth(width)
	bodyHeight := panelBodyHeight(height)
	focused := m.focus == focusStacks

	title := panelListTitle("Stacks", len(views), 0, len(views))
	if len(views) == 0 {
		if m.filterQuery != "" {
			message := fmt.Sprintf("No stacks match %q. Press Esc to clear the filter.", m.filterQuery)
			return renderFixedPanel(title, []string{mutedStyle.Render(truncateText(message, innerWidth))}, width, height, focused)
		}
		if len(m.service.ProfileViews()) > 0 {
			return renderFixedPanel(title, m.renderEmptyStacksLines(innerWidth), width, height, focused)
		}
		message := "No stacks yet. Add stacks to your config to launch groups of tunnels together."
		return renderFixedPanel(title, []string{mutedStyle.Render(truncateText(message, innerWidth))}, width, height, focused)
	}

	start, end := windowAroundSelection(len(views), m.selectedStack, bodyHeight)
	title = panelListTitle("Stacks", len(views), start, end)

	rows := make([]string, 0, end-start)
	for idx := start; idx < end; idx++ {
		rows = append(rows, m.renderStackRow(views[idx], idx == m.selectedStack, focused, innerWidth))
	}

	return renderFixedPanel(title, rows, width, height, focused)
}

func (m Model) renderProfileRow(view app.ProfileView, selected bool, focused bool, width int) string {
	style := listRowStyleFor(selected, focused)
	marker := selectionMarker(selected, focused)
	contentWidth := max(1, width-style.GetHorizontalFrameSize()-lipgloss.Width(marker))
	line := composeStyledLine(
		renderStatusBadge(view.State.Status)+" ",
		fmt.Sprintf("%s  :%d  %s", view.Profile.Name, view.Profile.LocalPort, profileTarget(view.Profile)),
		contentWidth,
	)
	return renderSizedBlock(style, width, marker+line)
}

func (m Model) renderStackRow(view app.StackView, selected bool, focused bool, width int) string {
	style := listRowStyleFor(selected, focused)
	marker := selectionMarker(selected, focused)
	contentWidth := max(1, width-style.GetHorizontalFrameSize()-lipgloss.Width(marker))
	line := composeStyledLine(
		renderStackStatusBadge(view.Status)+" ",
		fmt.Sprintf("%s  %d/%d  %s", view.Stack.Name, view.ActiveCount, len(view.Members), stackMembersSummary(view)),
		contentWidth,
	)
	return renderSizedBlock(style, width, marker+line)
}

func (m Model) renderInspectorPanel(profiles []app.ProfileView, stacks []app.StackView, width, height int) string {
	innerWidth, pageSize := m.inspectorDimensions(width, height)
	lines := m.currentInspectorLines(profiles, stacks, innerWidth)
	scroll := m.normalizedInspectorScroll(len(lines), pageSize)

	body := make([]string, 0, pageSize+2)
	body = append(body, m.renderInspectorTabs(innerWidth))
	body = append(body, padLines(clipLines(lines, scroll, pageSize), pageSize)...)
	body = append(body, m.renderInspectorScrollLine(scroll, pageSize, len(lines), innerWidth))

	return renderFixedPanel(m.inspectorTitle(profiles, stacks), body, width, height, false)
}

func (m Model) inspectorTitle(profiles []app.ProfileView, stacks []app.StackView) string {
	label := m.selectedLabel(profiles, stacks)
	if label == "none" {
		return "Inspector"
	}
	return "Inspector " + label
}

func (m Model) renderInspectorTabs(width int) string {
	tabs := []string{
		renderInspectorTab("h", "Details", m.inspectorTab == inspectorTabDetails),
		renderInspectorTab("l", "Logs", m.inspectorTab == inspectorTabLogs),
	}
	line := strings.Join(tabs, " ")
	return lipgloss.NewStyle().Width(max(1, width)).Render(line)
}

func (m Model) renderInspectorScrollLine(scroll, pageSize, total, width int) string {
	if total == 0 {
		return renderInlineText(mutedStyle, "Lines 0/0", width)
	}

	start := min(total, scroll+1)
	end := min(total, scroll+pageSize)
	if pageSize >= total {
		start = 1
		end = total
	}

	return renderInlineText(
		mutedStyle,
		fmt.Sprintf("Lines %d-%d/%d", start, end, total),
		width,
	)
}

func (m Model) currentInspectorLines(profiles []app.ProfileView, stacks []app.StackView, width int) []string {
	if m.inspectorTab == inspectorTabLogs {
		if m.focus == focusStacks && len(stacks) > 0 {
			return m.renderStackLogLines(stacks[m.selectedStack], width)
		}
		if len(profiles) > 0 {
			return m.renderProfileLogLines(profiles[m.selectedProfile], width)
		}
		if m.filterQuery != "" {
			return []string{mutedStyle.Render(truncateText("No filtered profile is selected, so there are no logs to show.", width))}
		}
		return []string{mutedStyle.Render(truncateText("Start a tunnel to see runtime output here.", width))}
	}

	if m.focus == focusStacks && len(stacks) > 0 {
		return m.renderStackDetailLines(stacks[m.selectedStack], width)
	}
	if len(profiles) > 0 {
		return m.renderProfileDetailLines(profiles[m.selectedProfile], width)
	}
	if m.filterQuery != "" {
		return []string{mutedStyle.Render(truncateText("No profile matches the current filter.", width))}
	}
	if m.workspaceIsEmpty() {
		return m.renderEmptyInspectorLines(width)
	}
	return []string{mutedStyle.Render(truncateText("No profile selected.", width))}
}

func (m Model) renderProfileDetailLines(view app.ProfileView, width int) []string {
	now := m.currentTime()
	spec, specErr := app.BuildProcessSpec(view.Profile)
	lines := []string{
		composeStyledLine(
			renderStatusBadge(view.State.Status)+" ",
			fmt.Sprintf("%s  %s", view.Profile.Name, humanTunnelType(view.Profile.Type)),
			width,
		),
		groupTitleStyle.Render("Overview"),
		renderCompactKeyValue("Local", fmt.Sprintf(":%d", view.Profile.LocalPort), width),
		renderCompactKeyValue("Target", profileTarget(view.Profile), width),
		groupTitleStyle.Render("Runtime"),
		renderCompactKeyValue("Status", humanTunnelStatus(view.State.Status), width),
		renderCompactKeyValue("PID", formatPID(view.State.PID), width),
		renderCompactKeyValue("Uptime", formatUptime(view.State.StartedAt, now), width),
		renderCompactKeyValue("Restarts", fmt.Sprintf("%d", view.State.RestartCount), width),
		renderCompactKeyValue("Last Exit", formatLastExit(view.State, now), width),
		renderCompactKeyValue("Restart", restartPolicySummary(view.Profile.Restart), width),
	}

	lines = append(lines, m.renderProfileStartLines(view, specErr, width)...)

	configLines := make([]string, 0, 4)
	if view.Profile.Description != "" {
		configLines = append(configLines, renderCompactKeyValue("About", view.Profile.Description, width))
	}
	if len(view.Profile.Labels) > 0 {
		configLines = append(configLines, renderCompactKeyValue("Labels", strings.Join(view.Profile.Labels, ", "), width))
	}
	if len(configLines) > 0 {
		lines = append(lines, groupTitleStyle.Render("Config"))
		lines = append(lines, configLines...)
	}

	if specErr == nil {
		lines = append(lines, groupTitleStyle.Render("Command"))
		lines = append(lines, renderCompactKeyValue("Exec", spec.DisplayCommand(), width))
	}

	problemLines := make([]string, 0, 2)
	if specErr != nil {
		problemLines = append(problemLines, renderCompactKeyValue("Config", specErr.Error(), width))
	}
	if view.State.LastError != "" {
		problemLines = append(problemLines, renderCompactKeyValue("Error", view.State.LastError, width))
	}
	if len(problemLines) > 0 {
		lines = append(lines, groupTitleStyle.Render("Problem"))
		lines = append(lines, problemLines...)
	}

	lines = append(lines, groupTitleStyle.Render("Actions"))
	lines = append(lines, profileActionLines(view, width)...)

	return lines
}

func (m Model) renderStackDetailLines(view app.StackView, width int) []string {
	lines := []string{
		composeStyledLine(
			renderStackStatusBadge(view.Status)+" ",
			fmt.Sprintf("%s  %s", view.Stack.Name, humanStackStatus(view.Status)),
			width,
		),
		groupTitleStyle.Render("Overview"),
		renderCompactKeyValue("Members", fmt.Sprintf("%d", len(view.Members)), width),
		renderCompactKeyValue("Active", fmt.Sprintf("%d", view.ActiveCount), width),
		renderCompactKeyValue("Coverage", fmt.Sprintf("%d/%d running", view.ActiveCount, len(view.Members)), width),
		groupTitleStyle.Render("Members"),
	}

	if len(view.Members) == 0 {
		lines = append(lines, mutedStyle.Render(truncateText("No member profiles resolved from config.", width)))
	} else {
		for _, member := range view.Members {
			lines = append(lines, composeStyledLine(
				renderStatusBadge(member.State.Status)+" ",
				fmt.Sprintf("%s  :%d  %s", member.Profile.Name, member.Profile.LocalPort, profileTarget(member.Profile)),
				width,
			))
		}
	}

	lines = append(lines, m.renderStackStartLines(view, width)...)

	if missingProfiles := missingStackProfiles(view); len(missingProfiles) > 0 {
		lines = append(lines, groupTitleStyle.Render("Problem"))
		lines = append(lines, renderCompactKeyValue("Missing", strings.Join(missingProfiles, ", "), width))
	}

	configLines := make([]string, 0, 4)
	if view.Stack.Description != "" {
		configLines = append(configLines, renderCompactKeyValue("About", view.Stack.Description, width))
	}
	if len(view.Stack.Labels) > 0 {
		configLines = append(configLines, renderCompactKeyValue("Labels", strings.Join(view.Stack.Labels, ", "), width))
	}
	if len(configLines) > 0 {
		lines = append(lines, groupTitleStyle.Render("Config"))
		lines = append(lines, configLines...)
	}

	if view.Status == app.StackStatusPartial {
		lines = append(lines, groupTitleStyle.Render("Action"))
		lines = append(lines, mutedStyle.Render(truncateText("Press Enter to start the missing members and restore the stack.", width)))
	}

	lines = append(lines, groupTitleStyle.Render("Actions"))
	lines = append(lines, stackActionLines(view, width)...)

	return lines
}

func (m Model) renderProfileLogLines(view app.ProfileView, width int) []string {
	if len(view.State.RecentLogs) == 0 {
		return []string{mutedStyle.Render(truncateText("No logs yet. Start the tunnel to collect runtime output.", width))}
	}

	lines := make([]string, 0, len(view.State.RecentLogs))
	for idx := len(view.State.RecentLogs) - 1; idx >= 0; idx-- {
		entry := view.State.RecentLogs[idx]
		lines = append(lines, renderLogLine(entry.Timestamp, "", entry.Source, entry.Message, width))
	}

	return lines
}

func (m Model) renderStackLogLines(view app.StackView, width int) []string {
	totalEntries := 0
	for _, member := range view.Members {
		totalEntries += len(member.State.RecentLogs)
	}
	if totalEntries == 0 {
		return []string{mutedStyle.Render(truncateText("No recent stack activity yet. Start a member to begin collecting logs.", width))}
	}

	activity := recentStackActivity(view, totalEntries)
	lines := make([]string, 0, len(activity))
	for idx := len(activity) - 1; idx >= 0; idx-- {
		entry := activity[idx]
		lines = append(lines, renderLogLine(entry.Log.Timestamp, entry.ProfileName, entry.Log.Source, entry.Log.Message, width))
	}

	return lines
}

func (m Model) normalizeSelection(profileCount, stackCount int) Model {
	if profileCount == 0 {
		m.selectedProfile = 0
		if stackCount > 0 && m.focus == focusProfiles {
			m.focus = focusStacks
		}
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

func (m Model) renderFilterBar() string {
	label := filterIdleStyle.Render("Filter /")
	if m.filterMode {
		label = filterActiveStyle.Render("Filter typing")
	}

	query := m.filterQuery
	if query == "" {
		query = "name, label, target, port"
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		label,
		"  ",
		sectionTextStyle.Render(query),
	)
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (Model, bool) {
	switch msg.String() {
	case "esc":
		if m.filterQuery != "" {
			m.filterQuery = ""
			m.selectedProfile = 0
			m.selectedStack = 0
			m.inspectorScroll = 0
			return m, true
		}
		m.filterMode = false
		return m, true
	case "enter":
		m.filterMode = false
		return m, true
	case "backspace", "ctrl+h":
		m.filterQuery = trimLastRune(m.filterQuery)
		m.selectedProfile = 0
		m.selectedStack = 0
		m.inspectorScroll = 0
		return m, true
	case "ctrl+w":
		m.filterQuery = trimLastWord(m.filterQuery)
		m.selectedProfile = 0
		m.selectedStack = 0
		m.inspectorScroll = 0
		return m, true
	case "ctrl+u":
		m.filterQuery = ""
		m.selectedProfile = 0
		m.selectedStack = 0
		m.inspectorScroll = 0
		return m, true
	}

	switch msg.Type {
	case tea.KeySpace:
		m.filterQuery += " "
	case tea.KeyRunes:
		m.filterQuery += string(msg.Runes)
	default:
		return m, false
	}

	m.selectedProfile = 0
	m.selectedStack = 0
	m.inspectorScroll = 0
	return m, true
}

func (m Model) handleWorkspaceKey(msg tea.KeyMsg, profiles []app.ProfileView, stacks []app.StackView) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "r":
		m = m.reloadConfigFromDisk("Reloaded config from disk.")
		return m, nil, true
	case "e":
		if err := m.ensureConfigFileExists(); err != nil {
			m.lastError = err.Error()
			return m, nil, true
		}
		m.lastError = ""
		m.lastNotice = ""
		return m, openEditorCmd(m.configPath), true
	case "i":
		if !m.workspaceIsEmpty() {
			return m, nil, false
		}
		m = m.initializeSampleConfig()
		return m, nil, true
	case "a":
		m = m.createStarterProfileDraft()
		return m, nil, true
	case "c":
		m = m.cloneSelection(profiles, stacks)
		return m, nil, true
	case "A":
		m = m.createStarterStackDraft(profiles, stacks)
		return m, nil, true
	default:
		return m, nil, false
	}
}

func (m Model) workspaceIsEmpty() bool {
	config := m.service.Config()
	return len(config.Profiles) == 0 && len(config.Stacks) == 0
}

func (m Model) initializeSampleConfig() Model {
	m.lastError = ""
	m.lastNotice = ""
	m.filterQuery = ""
	m.filterMode = false

	cfg := storage.SampleConfig()
	if err := storage.SaveConfig(m.configPath, cfg); err != nil {
		m.lastError = "Initialize sample config: " + err.Error()
		return m
	}

	m.service.ReplaceConfig(cfg)
	m.focus = focusProfiles
	m.selectedProfile = 0
	m.selectedStack = 0
	m.inspectorTab = inspectorTabDetails
	m.inspectorScroll = 0
	m.lastNotice = "Initialized sample config with starter profiles and a stack."
	return m
}

func (m Model) createStarterProfileDraft() Model {
	m.lastError = ""
	m.lastNotice = ""
	m.filterQuery = ""
	m.filterMode = false

	cfg := m.service.Config()
	profile := starterSSHProfileDraft(cfg)
	cfg.SetProfile(profile)

	if err := storage.SaveConfig(m.configPath, cfg); err != nil {
		m.lastError = "Create starter profile: " + err.Error()
		return m
	}

	m.service.ReplaceConfig(cfg)
	m.focus = focusProfiles
	m.selectedStack = 0
	m.inspectorTab = inspectorTabDetails
	m.inspectorScroll = 0
	m.selectProfileByName(profile.Name)
	m.lastNotice = fmt.Sprintf("Created starter profile %s. Press e to refine the config.", profile.Name)
	return m
}

func (m Model) createStarterStackDraft(profiles []app.ProfileView, stacks []app.StackView) Model {
	m.lastError = ""
	m.lastNotice = ""

	cfg := m.service.Config()
	stack, err := starterStackDraft(cfg, profiles, stacks, m.focus, m.selectedProfile, m.selectedStack, m.filterQuery)
	if err != nil {
		m.lastError = err.Error()
		return m
	}

	m.filterQuery = ""
	m.filterMode = false
	cfg.SetStack(stack)

	if err := storage.SaveConfig(m.configPath, cfg); err != nil {
		m.lastError = "Create starter stack: " + err.Error()
		return m
	}

	m.service.ReplaceConfig(cfg)
	m.focus = focusStacks
	m.selectedProfile = 0
	m.inspectorTab = inspectorTabDetails
	m.inspectorScroll = 0
	m.selectStackByName(stack.Name)
	m.lastNotice = fmt.Sprintf("Created starter stack %s. Press e to refine the config.", stack.Name)
	return m
}

func (m Model) cloneSelection(profiles []app.ProfileView, stacks []app.StackView) Model {
	m.lastError = ""
	m.lastNotice = ""

	if m.focus == focusStacks && len(stacks) > 0 {
		return m.cloneSelectedStack(stacks)
	}
	if len(profiles) > 0 {
		return m.cloneSelectedProfile(profiles)
	}
	if m.filterQuery != "" {
		m.lastError = "No visible item to clone. Clear the filter or select a different item."
		return m
	}
	m.lastError = "Nothing is selected to clone yet."
	return m
}

func (m Model) cloneSelectedProfile(profiles []app.ProfileView) Model {
	selected := profiles[max(0, min(m.selectedProfile, len(profiles)-1))]
	cfg := m.service.Config()
	profile := cloneProfileDefinition(selected.Profile)
	profile.Name = nextCopyName(profileNames(cfg), selected.Profile.Name)
	profile.LocalPort = nextAvailableLocalPort(cfg, selected.Profile.LocalPort+1)
	profile.Labels = appendUniqueLabel(profile.Labels, "draft")

	cfg.SetProfile(profile)
	if err := storage.SaveConfig(m.configPath, cfg); err != nil {
		m.lastError = "Clone profile: " + err.Error()
		return m
	}

	m.service.ReplaceConfig(cfg)
	m.filterQuery = ""
	m.filterMode = false
	m.focus = focusProfiles
	m.selectedStack = 0
	m.inspectorTab = inspectorTabDetails
	m.inspectorScroll = 0
	m.selectProfileByName(profile.Name)
	m.lastNotice = fmt.Sprintf("Cloned profile %s to %s.", selected.Profile.Name, profile.Name)
	return m
}

func (m Model) cloneSelectedStack(stacks []app.StackView) Model {
	selected := stacks[max(0, min(m.selectedStack, len(stacks)-1))]
	cfg := m.service.Config()
	stack := cloneStackDefinition(selected.Stack)
	stack.Name = nextCopyName(stackNames(cfg), selected.Stack.Name)
	stack.Labels = appendUniqueLabel(stack.Labels, "draft")

	cfg.SetStack(stack)
	if err := storage.SaveConfig(m.configPath, cfg); err != nil {
		m.lastError = "Clone stack: " + err.Error()
		return m
	}

	m.service.ReplaceConfig(cfg)
	m.filterQuery = ""
	m.filterMode = false
	m.focus = focusStacks
	m.selectedProfile = 0
	m.inspectorTab = inspectorTabDetails
	m.inspectorScroll = 0
	m.selectStackByName(stack.Name)
	m.lastNotice = fmt.Sprintf("Cloned stack %s to %s.", selected.Stack.Name, stack.Name)
	return m
}

func (m Model) reloadConfigFromDisk(successNotice string) Model {
	m.lastError = ""
	m.lastNotice = ""

	cfg, err := storage.LoadConfig(m.configPath)
	if err != nil {
		m.lastError = "Reload config: " + err.Error()
		return m
	}

	m.service.ReplaceConfig(cfg)
	m.selectedProfile = 0
	m.selectedStack = 0
	m.inspectorScroll = 0
	m.pendingDelete = nil
	m.lastNotice = successNotice
	return m
}

func (m *Model) selectProfileByName(name string) {
	for idx, view := range m.service.ProfileViews() {
		if view.Profile.Name != name {
			continue
		}
		m.selectedProfile = idx
		return
	}
	m.selectedProfile = 0
}

func (m *Model) selectStackByName(name string) {
	for idx, view := range m.service.StackViews() {
		if view.Stack.Name != name {
			continue
		}
		m.selectedStack = idx
		return
	}
	m.selectedStack = 0
}

func (m Model) ensureConfigFileExists() error {
	if _, err := os.Stat(m.configPath); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return fmt.Errorf("stat config file: %w", err)
	}

	if err := storage.SaveConfig(m.configPath, m.service.Config()); err != nil {
		return fmt.Errorf("create config file: %w", err)
	}

	return nil
}

func openEditorCmd(path string) tea.Cmd {
	shell := os.Getenv("SHELL")
	if strings.TrimSpace(shell) == "" {
		shell = "/bin/sh"
	}

	editor := preferredEditor()
	command := fmt.Sprintf("%s %s", editor, shellQuote(path))
	process := exec.Command(shell, "-lc", command)

	return tea.ExecProcess(process, func(err error) tea.Msg {
		return editorFinishedMsg{err: err}
	})
}

func preferredEditor() string {
	if editor := strings.TrimSpace(os.Getenv("VISUAL")); editor != "" {
		return editor
	}
	if editor := strings.TrimSpace(os.Getenv("EDITOR")); editor != "" {
		return editor
	}
	return "vi"
}

func shellQuote(value string) string {
	return "'" + strings.ReplaceAll(value, "'", `'"'"'`) + "'"
}

func (m Model) handleInspectorKey(msg tea.KeyMsg, profiles []app.ProfileView, stacks []app.StackView) (Model, bool) {
	switch msg.String() {
	case "left", "h":
		if m.inspectorTab != inspectorTabDetails {
			m.inspectorTab = inspectorTabDetails
			m.inspectorScroll = 0
		}
		return m, true
	case "right", "l":
		if m.inspectorTab != inspectorTabLogs {
			m.inspectorTab = inspectorTabLogs
			m.inspectorScroll = 0
		}
		return m, true
	case "pgup", "ctrl+u":
		pageSize := m.inspectorPageSize()
		m.inspectorScroll = max(0, m.inspectorScroll-pageSize)
		return m, true
	case "pgdown", "ctrl+d":
		pageSize := m.inspectorPageSize()
		maxScroll := max(0, len(m.currentInspectorLines(profiles, stacks, panelInnerWidth(m.contentWidth())))-pageSize)
		m.inspectorScroll = min(maxScroll, m.inspectorScroll+pageSize)
		return m, true
	default:
		return m, false
	}
}

func (m Model) handleDeleteKey(msg tea.KeyMsg, profiles []app.ProfileView, stacks []app.StackView) (Model, bool) {
	switch msg.String() {
	case "n", "esc":
		m.pendingDelete = nil
		m.lastNotice = "Delete cancelled."
		return m, true
	case "y", "enter":
		m = m.confirmDelete()
		return m, true
	default:
		return m, true
	}
}

func (m Model) buildDeleteRequest(profiles []app.ProfileView, stacks []app.StackView) *deleteRequest {
	if m.focus == focusStacks && len(stacks) > 0 {
		view := stacks[m.selectedStack]
		message := fmt.Sprintf(
			"Delete stack %s? This removes the saved stack only; member tunnels keep running. Press y or Enter to confirm, n or Esc to cancel.",
			view.Stack.Name,
		)
		return &deleteRequest{
			Kind:    deleteKindStack,
			Name:    view.Stack.Name,
			Message: message,
		}
	}

	if len(profiles) == 0 {
		return nil
	}

	view := profiles[m.selectedProfile]
	config := m.service.Config()
	stackNames := config.StacksReferencingProfile(view.Profile.Name)
	impactParts := make([]string, 0, 4)
	if isActiveTunnelStatus(view.State.Status) {
		impactParts = append(impactParts, "the running tunnel will be stopped")
	}
	if len(stackNames) > 0 {
		emptyStacks := 0
		for _, stack := range m.service.Stacks() {
			if len(stack.Profiles) == 1 && stack.Profiles[0] == view.Profile.Name {
				emptyStacks++
			}
		}
		impact := fmt.Sprintf("%d stack references will be pruned", len(stackNames))
		if emptyStacks > 0 {
			impact += fmt.Sprintf(" and %d empty stacks will be removed", emptyStacks)
		}
		impactParts = append(impactParts, impact)
	}
	if len(impactParts) == 0 {
		impactParts = append(impactParts, "this profile will be removed from the saved config")
	}

	message := fmt.Sprintf(
		"Delete profile %s? %s. Press y or Enter to confirm, n or Esc to cancel.",
		view.Profile.Name,
		strings.Join(impactParts, "; "),
	)
	return &deleteRequest{
		Kind:    deleteKindProfile,
		Name:    view.Profile.Name,
		Message: message,
	}
}

func (m Model) confirmDelete() Model {
	request := m.pendingDelete
	m.pendingDelete = nil
	m.lastError = ""
	m.lastNotice = ""

	if request == nil {
		return m
	}

	switch request.Kind {
	case deleteKindProfile:
		result, err := m.service.RemoveProfile(request.Name, true, func(cfg domain.Config) error {
			return storage.SaveConfig(m.configPath, cfg)
		})
		if err != nil {
			m.lastError = err.Error()
			return m
		}

		m.lastNotice = profileDeleteNotice(result)
		return m

	case deleteKindStack:
		result, err := m.service.RemoveStack(request.Name, func(cfg domain.Config) error {
			return storage.SaveConfig(m.configPath, cfg)
		})
		if err != nil {
			m.lastError = err.Error()
			return m
		}

		m.lastNotice = fmt.Sprintf("Removed stack %s.", result.Name)
		return m
	default:
		return m
	}
}

func (m Model) contentWidth() int {
	if m.width <= 0 {
		return defaultContentWidth
	}
	frameSize := appStyle.GetHorizontalFrameSize()
	if m.width <= frameSize {
		return m.width
	}
	return m.width - frameSize
}

func (m Model) contentHeight() int {
	if m.height <= 0 {
		return defaultContentHeight
	}
	frameSize := appStyle.GetVerticalFrameSize()
	if m.height <= frameSize {
		return m.height
	}
	return m.height - frameSize
}

func (m Model) currentTime() time.Time {
	if m.now.IsZero() {
		return time.Now()
	}
	return m.now
}

func (m Model) showHint() bool {
	return m.contentHeight() >= 10
}

func (m Model) hasStatusLine() bool {
	return m.pendingDelete != nil || m.lastError != "" || m.lastNotice != ""
}

func (m Model) chromeLineCount() int {
	count := 2
	if m.hasStatusLine() {
		count++
	}
	if m.showHint() {
		count++
	}
	return count
}

func (m Model) bodyHeight() int {
	return max(3, m.contentHeight()-m.chromeLineCount())
}

func (m Model) inspectorDimensions(width, height int) (int, int) {
	innerWidth := panelInnerWidth(width)
	return innerWidth, max(1, panelBodyHeight(height)-2)
}

func (m Model) inspectorPageSize() int {
	bodyHeight := m.bodyHeight()
	if m.useTwoColumnLayout(m.contentWidth(), bodyHeight) {
		_, pageSize := m.inspectorDimensions(m.contentWidth()-min(44, max(36, m.contentWidth()/3))-1, bodyHeight)
		return pageSize
	}
	_, inspectorHeight := splitListInspectorHeights(bodyHeight)
	_, pageSize := m.inspectorDimensions(m.contentWidth(), inspectorHeight)
	return pageSize
}

func (m Model) normalizedInspectorScroll(totalLines, pageSize int) int {
	if pageSize <= 0 {
		return 0
	}
	return min(max(m.inspectorScroll, 0), max(0, totalLines-pageSize))
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

func panelInnerWidth(width int) int {
	return max(1, width-panelStyle.GetHorizontalFrameSize())
}

func panelBodyHeight(height int) int {
	return max(1, height-panelStyle.GetVerticalFrameSize()-1)
}

func splitDualListHeights(total int) (int, int) {
	if total <= 1 {
		return max(1, total), 0
	}

	available := max(1, total-1)
	first := available / 2
	second := available - first
	if first <= 0 {
		first = 1
		second = max(1, available-first)
	}
	if second <= 0 {
		second = 1
		first = max(1, available-second)
	}

	return first, second
}

func splitListInspectorHeights(total int) (int, int) {
	if total <= 1 {
		return max(1, total), 0
	}

	available := max(1, total-1)
	listHeight := available / 3
	if available >= 6 {
		listHeight = max(3, listHeight)
	}
	listHeight = min(listHeight, 8)
	if listHeight <= 0 {
		listHeight = 1
	}

	inspectorHeight := available - listHeight
	if inspectorHeight <= 0 {
		inspectorHeight = 1
		listHeight = max(1, available-inspectorHeight)
	}

	return listHeight, inspectorHeight
}

func panelListTitle(name string, total, start, end int) string {
	if total <= 0 {
		return name
	}
	if start == 0 && end >= total {
		return fmt.Sprintf("%s %d", name, total)
	}
	return fmt.Sprintf("%s %d-%d/%d", name, start+1, end, total)
}

func listRowStyleFor(selected, focused bool) lipgloss.Style {
	switch {
	case selected && focused:
		return selectedListRowStyle
	case selected:
		return selectedOutlineRowStyle
	default:
		return listRowStyle
	}
}

func renderFixedPanel(title string, bodyLines []string, width, height int, focused bool) string {
	titleStyle := panelTitleMutedStyle
	if focused {
		titleStyle = panelTitleStyle
	}

	innerWidth := panelInnerWidth(width)
	bodyHeight := panelBodyHeight(height)
	normalizedBody := padLines(bodyLines, bodyHeight)
	contentLines := make([]string, 0, len(normalizedBody)+1)
	contentLines = append(contentLines, renderInlineBanner(titleStyle, title, innerWidth))

	lineStyle := lipgloss.NewStyle().Width(innerWidth)
	for _, line := range normalizedBody {
		contentLines = append(contentLines, lineStyle.Render(line))
	}

	return panelStyle.Render(strings.Join(contentLines, "\n"))
}

func renderInlineBanner(style lipgloss.Style, text string, width int) string {
	return renderSizedBlock(style, width, truncateText(text, max(1, width-style.GetHorizontalFrameSize())))
}

func renderInlineText(style lipgloss.Style, text string, width int) string {
	return renderSizedBlock(style, width, truncateText(text, max(1, width-style.GetHorizontalFrameSize())))
}

func preferredFilterInputWidth(totalWidth int) int {
	return min(26, max(16, totalWidth/5))
}

func preferredSelectedValueWidth(totalWidth int) int {
	return min(18, max(10, totalWidth/8))
}

func headerMetaLabelWidth(label string) int {
	return lipgloss.Width(headerMetaLabelStyle.Render(label)) + 1
}

func renderHeaderValueChip(style lipgloss.Style, value string, width int) string {
	width = max(style.GetHorizontalFrameSize()+1, width)
	innerWidth := max(1, width-style.GetHorizontalFrameSize())
	return renderSizedBlock(style, width, truncateText(value, innerWidth))
}

func (m Model) renderHeaderMetaField(label, value string, valueStyle lipgloss.Style, valueWidth int) string {
	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		headerMetaLabelStyle.Render(label),
		" ",
		renderHeaderValueChip(valueStyle, value, valueWidth),
	)
}

func (m Model) renderHeaderFilterSegment(inputWidth int) string {
	labelStyle := filterIdleStyle
	inputStyle := filterInputIdleStyle
	if m.filterMode || m.filterQuery != "" {
		labelStyle = filterActiveStyle
	}
	if m.filterMode {
		inputStyle = filterInputActiveStyle
	}

	value := m.filterQuery
	valueStyle := sectionTextStyle
	if value == "" {
		if m.filterMode {
			value = "type to filter"
		} else {
			value = "name, label, target"
		}
		valueStyle = filterPlaceholderStyle
	}

	innerWidth := max(1, inputWidth-inputStyle.GetHorizontalFrameSize())
	prompt := filterPromptStyle.Render("/")
	valueWidth := max(1, innerWidth-lipgloss.Width(prompt)-1)
	content := lipgloss.JoinHorizontal(
		lipgloss.Center,
		prompt,
		" ",
		valueStyle.Render(truncateText(value, valueWidth)),
	)

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		labelStyle.Render("Filter"),
		" ",
		renderSizedBlock(inputStyle, inputWidth, content),
	)
}

func (m Model) selectedValueStyle(profiles []app.ProfileView, stacks []app.StackView) lipgloss.Style {
	if m.selectedLabel(profiles, stacks) == "none" {
		return headerSelectedEmptyValueStyle
	}
	return headerSelectedValueStyle
}

func (m Model) renderEmptyProfilesLines(width int) []string {
	rows := renderQuickActionRows(width, []quickAction{
		{key: "i", label: "sample config"},
		{key: "a", label: "draft profile"},
		{key: "e", label: "edit config"},
		{key: "r", label: "reload config"},
	})
	return append(rows, mutedStyle.Render(truncateText("No profiles yet. Start here.", width)))
}

func (m Model) renderEmptyStacksLines(width int) []string {
	rows := renderQuickActionRows(width, []quickAction{
		{key: "A", label: "draft stack"},
		{key: "e", label: "edit config"},
		{key: "r", label: "reload config"},
		{key: "Tab", label: "focus profiles"},
	})
	return append(rows, mutedStyle.Render(truncateText("No stacks yet. Create one from the selected profile.", width)))
}

func (m Model) renderEmptyInspectorLines(width int) []string {
	return []string{
		groupTitleStyle.Render("Quick Start"),
		sectionTextStyle.Render(truncateText("The workspace is empty. Create tunnels or load an example config.", width)),
		"",
		renderActionLine("i", "seed sample SSH and Kubernetes tunnels", width),
		renderActionLine("a", "create a starter SSH profile draft", width),
		renderActionLine("e", "open the YAML config in your editor", width),
		renderActionLine("r", "reload external config edits", width),
		"",
		renderCompactKeyValue("Config", m.configPath, width),
	}
}

func renderActionLine(key, description string, width int) string {
	prefix := codeStyle.Render(" " + key + " ")
	return composeStyledLine(prefix+" ", description, width)
}

func renderActionChip(key, label string) string {
	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		codeStyle.Render(" "+key+" "),
		" ",
		sectionTextStyle.Render(label),
	)
}

type quickAction struct {
	key   string
	label string
}

func renderQuickActionRows(width int, actions []quickAction) []string {
	if len(actions) == 0 || width <= 0 {
		return nil
	}

	if width < 40 {
		return renderQuickActionList(width, actions)
	}

	const gapWidth = 2
	leftWidth := max(1, (width-gapWidth)/2)
	rightWidth := max(1, width-gapWidth-leftWidth)
	keyWidth := quickActionKeyWidth(actions)

	minCellWidth := keyWidth + 8
	if leftWidth < minCellWidth || rightWidth < minCellWidth {
		return renderQuickActionList(width, actions)
	}

	rows := make([]string, 0, (len(actions)+1)/2)
	for idx := 0; idx < len(actions); idx += 2 {
		left := renderQuickActionCell(actions[idx], leftWidth, keyWidth)
		if idx+1 >= len(actions) {
			rows = append(rows, left)
			continue
		}

		right := renderQuickActionCell(actions[idx+1], rightWidth, keyWidth)
		rows = append(rows, lipgloss.JoinHorizontal(lipgloss.Top, left, strings.Repeat(" ", gapWidth), right))
	}

	return rows
}

func renderQuickActionList(width int, actions []quickAction) []string {
	keyWidth := quickActionKeyWidth(actions)
	rows := make([]string, 0, len(actions))
	for _, action := range actions {
		rows = append(rows, renderQuickActionCell(action, width, keyWidth))
	}
	return rows
}

func quickActionKeyWidth(actions []quickAction) int {
	width := 0
	for _, action := range actions {
		chipWidth := lipgloss.Width(codeStyle.Render(" " + action.key + " "))
		width = max(width, chipWidth)
	}
	return width
}

func renderQuickActionCell(action quickAction, width, keyWidth int) string {
	key := renderSizedBlock(lipgloss.NewStyle(), keyWidth, codeStyle.Render(" "+action.key+" "))
	remaining := max(1, width-keyWidth-1)
	content := lipgloss.JoinHorizontal(
		lipgloss.Top,
		key,
		" ",
		sectionTextStyle.Render(truncateText(action.label, remaining)),
	)
	return lipgloss.NewStyle().Width(width).Render(content)
}

func (m Model) renderProfileStartLines(view app.ProfileView, specErr error, width int) []string {
	if m.service == nil {
		return nil
	}

	analysis, err := m.service.AnalyzeProfileStart(view.Profile.Name)
	if err != nil {
		return nil
	}

	lines := []string{
		groupTitleStyle.Render("Start"),
		renderCompactKeyValue("Readiness", profileStartSummary(analysis.Status), width),
	}

	excludedProblem := ""
	if specErr != nil {
		excludedProblem = specErr.Error()
	}

	for _, problem := range analysis.Problems {
		if excludedProblem != "" && problem == excludedProblem {
			continue
		}
		lines = append(lines, renderCompactKeyValue("Blocker", problem, width))
	}

	return lines
}

func (m Model) renderStackStartLines(view app.StackView, width int) []string {
	if m.service == nil {
		return nil
	}

	analysis, err := m.service.AnalyzeStackStart(view.Stack.Name)
	if err != nil {
		return nil
	}

	lines := []string{
		groupTitleStyle.Render("Start Plan"),
		renderCompactKeyValue("Readiness", stackStartSummary(view, analysis), width),
		renderCompactKeyValue("Ready", formatCountNoun(analysis.ReadyCount, "member"), width),
		renderCompactKeyValue("Running", formatCountNoun(analysis.ActiveCount, "member"), width),
		renderCompactKeyValue("Blocked", formatCountNoun(analysis.BlockedCount, "member"), width),
	}

	if len(view.Stack.Profiles) == 0 {
		lines = append(lines, renderCompactKeyValue("Blocker", "Add a profile to this stack first.", width))
		return lines
	}

	for _, member := range analysis.Members {
		if member.Status != app.StartReadinessBlocked {
			continue
		}
		for _, problem := range member.Problems {
			lines = append(lines, renderCompactKeyValue(member.ProfileName, problem, width))
		}
	}

	return lines
}

func profileActionLines(view app.ProfileView, width int) []string {
	toggleLabel := "start tunnel"
	if isActiveTunnelStatus(view.State.Status) {
		toggleLabel = "stop tunnel"
	}

	return []string{
		renderActionLine("Enter", toggleLabel, width),
		renderActionLine("c", "clone profile draft", width),
		renderActionLine("A", "create stack draft from profile", width),
		renderActionLine("e", "edit config file", width),
		renderActionLine("d", "delete profile", width),
	}
}

func stackActionLines(view app.StackView, width int) []string {
	toggleLabel := "start stack"
	if view.Status == app.StackStatusRunning {
		toggleLabel = "stop stack"
	}

	return []string{
		renderActionLine("Enter", toggleLabel, width),
		renderActionLine("c", "clone stack draft", width),
		renderActionLine("A", "create another stack draft", width),
		renderActionLine("e", "edit config file", width),
		renderActionLine("d", "delete stack", width),
	}
}

func starterSSHProfileDraft(cfg domain.Config) domain.Profile {
	return domain.Profile{
		Name:        nextProfileDraftName(cfg, "draft-ssh"),
		Description: "Starter SSH tunnel draft. Update the target before using it.",
		Type:        domain.TunnelTypeSSHLocal,
		LocalPort:   nextAvailableLocalPort(cfg, 15432),
		Restart: domain.RestartPolicy{
			Enabled:        true,
			MaxRetries:     0,
			InitialBackoff: "2s",
			MaxBackoff:     "30s",
		},
		Labels: []string{"draft"},
		SSH: &domain.SSHLocal{
			Host:       "example-bastion",
			RemoteHost: "127.0.0.1",
			RemotePort: 5432,
		},
	}
}

func starterStackDraft(cfg domain.Config, profiles []app.ProfileView, stacks []app.StackView, focus listFocus, selectedProfile, selectedStack int, filterQuery string) (domain.Stack, error) {
	members, sourceLabel, err := starterStackMembers(profiles, stacks, focus, selectedProfile, selectedStack, filterQuery, len(cfg.Profiles))
	if err != nil {
		return domain.Stack{}, err
	}

	stack := domain.Stack{
		Name:        nextStackDraftName(cfg, "draft-stack"),
		Description: fmt.Sprintf("Starter stack draft seeded from %s.", sourceLabel),
		Labels:      []string{"draft"},
		Profiles:    members,
	}
	return stack, nil
}

func starterStackMembers(profiles []app.ProfileView, stacks []app.StackView, focus listFocus, selectedProfile, selectedStack int, filterQuery string, totalProfiles int) ([]string, string, error) {
	if focus == focusStacks && len(stacks) > 0 {
		selectedStack = max(0, min(selectedStack, len(stacks)-1))
		selected := stacks[selectedStack]
		members := make([]string, 0, len(selected.Members))
		for _, member := range selected.Members {
			members = append(members, member.Profile.Name)
		}
		if len(members) == 0 {
			return nil, "", fmt.Errorf("Selected stack has no resolved members to draft from.")
		}
		return members, "stack " + selected.Stack.Name, nil
	}

	if len(profiles) > 0 {
		selectedProfile = max(0, min(selectedProfile, len(profiles)-1))
		selected := profiles[selectedProfile]
		return []string{selected.Profile.Name}, "profile " + selected.Profile.Name, nil
	}

	switch {
	case totalProfiles == 0:
		return nil, "", fmt.Errorf("Add a profile first, then press A to create a stack draft.")
	case filterQuery != "":
		return nil, "", fmt.Errorf("No visible profile to seed the stack. Clear the filter or select a profile first.")
	default:
		return nil, "", fmt.Errorf("Select a profile first, then press A to create a stack draft.")
	}
}

func cloneProfileDefinition(profile domain.Profile) domain.Profile {
	cloned := profile
	cloned.Labels = append([]string(nil), profile.Labels...)
	if profile.SSH != nil {
		sshCopy := *profile.SSH
		cloned.SSH = &sshCopy
	}
	if profile.Kubernetes != nil {
		kubernetesCopy := *profile.Kubernetes
		cloned.Kubernetes = &kubernetesCopy
	}
	return cloned
}

func cloneStackDefinition(stack domain.Stack) domain.Stack {
	cloned := stack
	cloned.Labels = append([]string(nil), stack.Labels...)
	cloned.Profiles = append([]string(nil), stack.Profiles...)
	return cloned
}

func profileNames(cfg domain.Config) []string {
	names := make([]string, 0, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		names = append(names, profile.Name)
	}
	return names
}

func stackNames(cfg domain.Config) []string {
	names := make([]string, 0, len(cfg.Stacks))
	for _, stack := range cfg.Stacks {
		names = append(names, stack.Name)
	}
	return names
}

func nextCopyName(existing []string, base string) string {
	used := make(map[string]struct{}, len(existing))
	for _, name := range existing {
		used[name] = struct{}{}
	}

	candidate := base + "-copy"
	if _, exists := used[candidate]; !exists {
		return candidate
	}

	for idx := 2; ; idx++ {
		candidate = fmt.Sprintf("%s-copy-%d", base, idx)
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
}

func appendUniqueLabel(labels []string, label string) []string {
	for _, existing := range labels {
		if existing == label {
			return append([]string(nil), labels...)
		}
	}

	updated := append([]string(nil), labels...)
	return append(updated, label)
}

func nextProfileDraftName(cfg domain.Config, base string) string {
	names := make(map[string]struct{}, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		names[profile.Name] = struct{}{}
	}

	if _, exists := names[base]; !exists {
		return base
	}

	for idx := 2; ; idx++ {
		candidate := fmt.Sprintf("%s-%d", base, idx)
		if _, exists := names[candidate]; !exists {
			return candidate
		}
	}
}

func nextStackDraftName(cfg domain.Config, base string) string {
	names := make(map[string]struct{}, len(cfg.Stacks))
	for _, stack := range cfg.Stacks {
		names[stack.Name] = struct{}{}
	}

	if _, exists := names[base]; !exists {
		return base
	}

	for idx := 2; ; idx++ {
		candidate := fmt.Sprintf("%s-%d", base, idx)
		if _, exists := names[candidate]; !exists {
			return candidate
		}
	}
}

func nextAvailableLocalPort(cfg domain.Config, start int) int {
	used := make(map[int]struct{}, len(cfg.Profiles))
	for _, profile := range cfg.Profiles {
		used[profile.LocalPort] = struct{}{}
	}

	port := start
	for {
		if _, exists := used[port]; !exists {
			return port
		}
		port++
	}
}

func selectionMarker(selected, focused bool) string {
	switch {
	case selected && focused:
		return selectedMarkerStyle.Render("> ")
	case selected:
		return selectedOutlineMarkerStyle.Render("| ")
	default:
		return "  "
	}
}

func renderInspectorTab(key, label string, active bool) string {
	style := lipgloss.NewStyle().
		Foreground(lipgloss.Color("252")).
		Background(lipgloss.Color("238")).
		Padding(0, 1)
	if active {
		style = style.Bold(true).
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("24"))
	}
	return style.Render(key + " " + label)
}

func renderCompactKeyValue(label, value string, width int) string {
	if strings.TrimSpace(value) == "" {
		value = "-"
	}

	labelWidth := min(12, max(7, width/4))
	prefix := keyStyle.Copy().Width(labelWidth).Render(label)
	return prefix + sectionTextStyle.Render(truncateText(value, max(1, width-lipgloss.Width(prefix))))
}

func composeStyledLine(prefix, content string, width int) string {
	if width <= 0 {
		return prefix + content
	}
	if strings.TrimSpace(content) == "" {
		return prefix
	}

	remaining := width - lipgloss.Width(prefix)
	if remaining <= 0 {
		return prefix
	}

	return prefix + truncateText(content, remaining)
}

func clipLines(lines []string, offset, limit int) []string {
	if len(lines) == 0 || limit <= 0 {
		return nil
	}

	offset = max(0, min(offset, len(lines)))
	end := min(len(lines), offset+limit)
	return append([]string(nil), lines[offset:end]...)
}

func padLines(lines []string, height int) []string {
	if height <= 0 {
		return nil
	}

	normalized := append([]string(nil), lines...)
	if len(normalized) > height {
		return normalized[:height]
	}
	for len(normalized) < height {
		normalized = append(normalized, "")
	}
	return normalized
}

func normalizeLineCount(lines []string, height int) []string {
	if height <= 0 {
		return lines
	}
	if len(lines) > height {
		return lines[:height]
	}
	for len(lines) < height {
		lines = append(lines, "")
	}
	return lines
}

func windowAroundSelection(total, selected, visible int) (int, int) {
	if total <= 0 || visible <= 0 {
		return 0, 0
	}
	if visible >= total {
		return 0, total
	}

	selected = max(0, min(selected, total-1))
	start := selected - visible/2
	if start < 0 {
		start = 0
	}
	end := start + visible
	if end > total {
		end = total
		start = end - visible
	}

	return start, end
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

	prefix := keyStyle.Render(label)
	return prefix + sectionTextStyle.Render(truncateText(value, max(1, width-lipgloss.Width(prefix))))
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

	if profileName != "" {
		return composeStyledLine(content+" ", message, width)
	}
	return composeStyledLine(content+" ", message, width)
}

func renderSizedBlock(style lipgloss.Style, width int, body string) string {
	if width <= 0 {
		return style.Render(body)
	}

	innerWidth := max(1, width-style.GetHorizontalFrameSize())
	return style.Render(lipgloss.NewStyle().Width(innerWidth).Render(body))
}

func renderStatusBadge(status domain.TunnelStatus) string {
	label := "STOP"
	background := lipgloss.Color("240")

	switch status {
	case domain.TunnelStatusRunning:
		label = "RUN"
		background = lipgloss.Color("29")
	case domain.TunnelStatusStarting:
		label = "START"
		background = lipgloss.Color("31")
	case domain.TunnelStatusRestarting:
		label = "RETRY"
		background = lipgloss.Color("136")
	case domain.TunnelStatusFailed:
		label = "FAIL"
		background = lipgloss.Color("124")
	case domain.TunnelStatusExited:
		label = "EXIT"
		background = lipgloss.Color("239")
	}

	return renderStateBadge(label, background)
}

func renderStackStatusBadge(status app.StackStatus) string {
	label := "STOP"
	background := lipgloss.Color("240")

	switch status {
	case app.StackStatusRunning:
		label = "RUN"
		background = lipgloss.Color("29")
	case app.StackStatusPartial:
		label = "PART"
		background = lipgloss.Color("136")
	}

	return renderStateBadge(label, background)
}

func renderStateBadge(label string, background lipgloss.Color) string {
	return lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("230")).
		Background(background).
		Width(7).
		Align(lipgloss.Center).
		Render(label)
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

func missingStackProfiles(view app.StackView) []string {
	resolved := make(map[string]struct{}, len(view.Members))
	for _, member := range view.Members {
		resolved[member.Profile.Name] = struct{}{}
	}

	missing := make([]string, 0)
	for _, profileName := range view.Stack.Profiles {
		if _, exists := resolved[profileName]; exists {
			continue
		}
		missing = append(missing, profileName)
	}

	return missing
}

func profileStartSummary(status app.StartReadiness) string {
	switch status {
	case app.StartReadinessActive:
		return "Running now"
	case app.StartReadinessReady:
		return "Ready on Enter"
	case app.StartReadinessBlocked:
		return "Blocked"
	default:
		return "-"
	}
}

func stackStartSummary(view app.StackView, analysis app.StackStartAnalysis) string {
	switch {
	case len(view.Stack.Profiles) == 0:
		return "Blocked"
	case analysis.BlockedCount > 0:
		return "Blocked"
	case analysis.ReadyCount > 0:
		return "Ready for " + formatCountNoun(analysis.ReadyCount, "member")
	case analysis.ActiveCount > 0:
		return "Already running"
	default:
		return "Idle"
	}
}

func formatCountNoun(count int, noun string) string {
	if count == 1 {
		return fmt.Sprintf("%d %s", count, noun)
	}
	return fmt.Sprintf("%d %ss", count, noun)
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

func isActiveTunnelStatus(status domain.TunnelStatus) bool {
	switch status {
	case domain.TunnelStatusStarting, domain.TunnelStatusRunning, domain.TunnelStatusRestarting:
		return true
	default:
		return false
	}
}

func filterProfileViews(views []app.ProfileView, query string) []app.ProfileView {
	query = normalizeFilterQuery(query)
	if query == "" {
		return views
	}

	filtered := make([]app.ProfileView, 0, len(views))
	for _, view := range views {
		if !profileMatchesFilter(view, query) {
			continue
		}
		filtered = append(filtered, view)
	}

	return filtered
}

func filterStackViews(views []app.StackView, query string) []app.StackView {
	query = normalizeFilterQuery(query)
	if query == "" {
		return views
	}

	filtered := make([]app.StackView, 0, len(views))
	for _, view := range views {
		if !stackMatchesFilter(view, query) {
			continue
		}
		filtered = append(filtered, view)
	}

	return filtered
}

func profileMatchesFilter(view app.ProfileView, query string) bool {
	return strings.Contains(strings.ToLower(profileSearchText(view)), query)
}

func stackMatchesFilter(view app.StackView, query string) bool {
	return strings.Contains(strings.ToLower(stackSearchText(view)), query)
}

func profileSearchText(view app.ProfileView) string {
	parts := []string{
		view.Profile.Name,
		view.Profile.Description,
		string(view.Profile.Type),
		humanTunnelType(view.Profile.Type),
		profileTarget(view.Profile),
		fmt.Sprintf("%d", view.Profile.LocalPort),
	}
	parts = append(parts, view.Profile.Labels...)
	return strings.Join(parts, " ")
}

func stackSearchText(view app.StackView) string {
	parts := []string{
		view.Stack.Name,
		view.Stack.Description,
		string(view.Status),
		humanStackStatus(view.Status),
	}
	parts = append(parts, view.Stack.Labels...)

	for _, member := range view.Members {
		parts = append(parts,
			member.Profile.Name,
			member.Profile.Description,
			profileTarget(member.Profile),
			fmt.Sprintf("%d", member.Profile.LocalPort),
		)
		parts = append(parts, member.Profile.Labels...)
	}

	return strings.Join(parts, " ")
}

func normalizeFilterQuery(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}

func formatVisibleCount(visible, total int) string {
	if visible == total {
		return fmt.Sprintf("%d", total)
	}
	return fmt.Sprintf("%d/%d", visible, total)
}

func profileDeleteNotice(result app.RemoveProfileResult) string {
	parts := []string{fmt.Sprintf("Removed profile %s.", result.Name)}
	if result.WasActive {
		parts = append(parts, "Stopped the running tunnel first.")
	}
	if result.UpdatedStacks > 0 {
		impact := fmt.Sprintf("Pruned %d stack references", result.UpdatedStacks)
		if result.RemovedStacks > 0 {
			impact += fmt.Sprintf(" and removed %d empty stacks", result.RemovedStacks)
		}
		parts = append(parts, impact+".")
	}

	return strings.Join(parts, " ")
}

func trimLastRune(value string) string {
	runes := []rune(value)
	if len(runes) == 0 {
		return ""
	}
	return string(runes[:len(runes)-1])
}

func trimLastWord(value string) string {
	value = strings.TrimRight(value, " ")
	if value == "" {
		return ""
	}

	idx := strings.LastIndexByte(value, ' ')
	if idx == -1 {
		return ""
	}

	return strings.TrimRight(value[:idx], " ")
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
