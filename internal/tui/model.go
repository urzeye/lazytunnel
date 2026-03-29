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
	"github.com/charmbracelet/x/ansi"

	"github.com/urzeye/lazytunnel/internal/app"
	"github.com/urzeye/lazytunnel/internal/domain"
	profileimport "github.com/urzeye/lazytunnel/internal/profileimport"
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
	importPromptStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("62")).
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
	filterMatchStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("232")).
				Background(lipgloss.Color("220"))
	logTimestampStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("244"))
	logProfileBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("237")).
				Padding(0, 1)
	logMatchBadgeStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("232")).
				Background(lipgloss.Color("220")).
				Padding(0, 1)
	logSystemMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("248"))
	logStdoutMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("252"))
	logStderrMessageStyle = lipgloss.NewStyle().
				Foreground(lipgloss.Color("210"))
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
type filterScope int

const (
	focusProfiles listFocus = iota
	focusStacks
)

const (
	inspectorTabDetails inspectorTab = iota
	inspectorTabLogs
)

const (
	filterScopeList filterScope = iota
	filterScopeLogs
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

type mouseRect struct {
	x      int
	y      int
	width  int
	height int
}

func (r mouseRect) contains(x, y int) bool {
	return x >= r.x && x < r.x+r.width && y >= r.y && y < r.y+r.height
}

type mouseListRegion struct {
	panel   mouseRect
	visible bool
	start   int
	end     int
	focus   listFocus
}

type mouseInspectorTabRegion struct {
	rect  mouseRect
	tab   inspectorTab
	scope filterScope
}

type mouseImportActionRegion struct {
	rect   mouseRect
	action string
}

type mouseLayout struct {
	headerFilter  mouseRect
	profiles      mouseListRegion
	stacks        mouseListRegion
	inspector     mouseRect
	inspectorTabs []mouseInspectorTabRegion
	importActions []mouseImportActionRegion
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
	logFilterQuery  string
	filterMode      bool
	filterScope     filterScope
	pendingDelete   *deleteRequest
	importMode      bool
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
			m.lastError = m.t("Editor exited with an error: ", "编辑器异常退出: ") + msg.err.Error()
			return m, nil
		}

		m = m.reloadConfigFromDisk(m.t("Reloaded config after editing.", "编辑完成后已重新加载配置。"))
		return m, nil

	case tea.MouseMsg:
		profiles := filterProfileViews(m.service.ProfileViews(), m.filterQuery)
		stacks := filterStackViews(m.service.StackViews(), m.filterQuery)
		m = m.normalizeSelection(len(profiles), len(stacks))

		var handled bool
		m, handled = m.handleMouse(msg, profiles, stacks)
		if handled {
			return m, nil
		}

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

		if m.importMode {
			var handled bool
			m, handled = m.handleImportKey(msg)
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
			m.filterScope = m.defaultFilterScope()
			return m, nil
		case "esc":
			if m.defaultFilterScope() == filterScopeLogs {
				if m.logFilterQuery != "" {
					m.logFilterQuery = ""
					m.inspectorScroll = 0
					return m, nil
				}
			} else if m.filterQuery != "" {
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
		case "r", "R":
			m = m.restartSelection(profiles, stacks)
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
	focusLabel := m.t("profiles", "配置")
	if m.focus == focusStacks && len(stacks) > 0 {
		focusLabel = m.t("stacks", "组合")
	}

	line1 := composeStyledLine(
		titleStyle.Render("LazyTunnel")+" ",
		m.tf(
			"profiles %s | stacks %s | active %d | focus %s",
			"配置 %s | 组合 %s | 运行中 %d | 焦点 %s",
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
	selectedValue := m.selectedLabel(profiles, stacks)
	if selectedValue == "none" {
		selectedValue = m.t("none", "无")
	}
	selectedSegment := m.renderHeaderMetaField(
		m.t("selected", "选中"),
		selectedValue,
		m.selectedValueStyle(profiles, stacks),
		preferredSelectedValueWidth(width),
	)

	usedWidth := lipgloss.Width(filterSegment) + lipgloss.Width(separator) + lipgloss.Width(selectedSegment) + lipgloss.Width(separator)
	configLabel := m.t("config", "配置")
	configValueWidth := max(8, width-usedWidth-headerMetaLabelWidth(configLabel))
	configSegment := m.renderHeaderMetaField(configLabel, m.configPath, headerConfigValueStyle, configValueWidth)

	return filterSegment + separator + selectedSegment + separator + configSegment
}

func (m Model) renderStatusLine(width int) string {
	switch {
	case m.pendingDelete != nil:
		return renderInlineBanner(deletePromptStyle, m.pendingDelete.Message, width)
	case m.importMode:
		return renderInlineBanner(importPromptStyle, m.importPromptMessage(), width)
	case m.lastError != "":
		return renderInlineBanner(errorBannerStyle, m.t("Last error: ", "最近错误: ")+m.lastError, width)
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
		return m.t("Delete mode: y or Enter confirms. n or Esc cancels.", "删除模式: y 或 Enter 确认，n 或 Esc 取消。")
	case m.importMode:
		return m.t("Import mode: s imports ~/.ssh/config, k imports kube contexts, a imports both. Esc cancels.", "导入模式: s 导入 ~/.ssh/config，k 导入 Kubernetes context，a 两者都导入。Esc 取消。")
	case m.filterMode:
		if m.filterScope == filterScopeLogs {
			return m.t("Log filter mode: type to search messages, sources, and profile names. Enter finishes. Esc clears or exits. Backspace/Ctrl+W deletes. Ctrl+U clears.", "日志筛选模式: 可搜索消息、来源和 profile 名。Enter 完成，Esc 清空或退出，Backspace/Ctrl+W 删除，Ctrl+U 清空。")
		}
		return m.t("Filter mode: type to search names, labels, targets, and ports. Enter finishes. Esc clears or exits. Backspace/Ctrl+W deletes. Ctrl+U clears.", "筛选模式: 可搜索名称、标签、目标和端口。Enter 完成，Esc 清空或退出，Backspace/Ctrl+W 删除，Ctrl+U 清空。")
	case m.workspaceIsEmpty():
		return joinHintParts(
			m.t("i import drafts", "i 导入草稿"),
			m.t("s sample config", "s 示例配置"),
			m.t("a new tunnel draft", "a 新建隧道草稿"),
			m.t("e edit config", "e 编辑配置"),
			m.t("g reload config", "g 重新加载配置"),
			m.t("L switch language", "L 切换语言"),
			m.inlineFilterHint(),
			m.t("q quit", "q 退出"),
		)
	}

	profileCount := len(m.service.ProfileViews())
	stackCount := len(m.service.StackViews())
	switch {
	case profileCount == 0:
		return joinHintParts(
			m.t("i import drafts", "i 导入草稿"),
			m.t("a new tunnel draft", "a 新建隧道草稿"),
			m.t("e edit config", "e 编辑配置"),
			m.t("g reload config", "g 重新加载配置"),
			m.t("L switch language", "L 切换语言"),
			m.inlineFilterHint(),
			m.t("q quit", "q 退出"),
		)
	case stackCount == 0:
		return joinHintParts(
			m.t("j/k move", "j/k 移动"),
			m.t("h/l details/logs", "h/l 详情/日志"),
			m.inlineToggleHint(),
			m.inlineRestartHint(),
			m.t("c clone profile", "c 克隆配置"),
			m.t("i import drafts", "i 导入草稿"),
			m.t("A new stack draft", "A 新建组合草稿"),
			m.t("e edit config", "e 编辑配置"),
			m.t("g reload config", "g 重新加载配置"),
			m.inlineFilterHint(),
			m.t("q quit", "q 退出"),
		)
	case m.focus == focusStacks:
		return joinHintParts(
			m.t("j/k move", "j/k 移动"),
			m.t("Tab profiles/stacks", "Tab 配置/组合"),
			m.t("h/l details/logs", "h/l 详情/日志"),
			m.inlineToggleHint(),
			m.inlineRestartHint(),
			m.t("c clone stack", "c 克隆组合"),
			m.t("i import drafts", "i 导入草稿"),
			m.t("A new stack draft", "A 新建组合草稿"),
			m.t("d delete stack", "d 删除组合"),
			m.t("g reload config", "g 重新加载配置"),
			m.inlineFilterHint(),
			m.t("q quit", "q 退出"),
		)
	default:
		return joinHintParts(
			m.t("j/k move", "j/k 移动"),
			m.t("Tab profiles/stacks", "Tab 配置/组合"),
			m.t("h/l details/logs", "h/l 详情/日志"),
			m.inlineToggleHint(),
			m.inlineRestartHint(),
			m.t("c clone profile", "c 克隆配置"),
			m.t("i import drafts", "i 导入草稿"),
			m.t("A new stack draft", "A 新建组合草稿"),
			m.t("d delete profile", "d 删除配置"),
			m.t("g reload config", "g 重新加载配置"),
			m.inlineFilterHint(),
			m.t("q quit", "q 退出"),
		)
	}
}

func (m Model) importPromptMessage() string {
	return m.t("Import drafts: s SSH config, k kube contexts, a both, Esc cancel.", "导入草稿: s SSH 配置，k Kubernetes context，a 全部导入，Esc 取消。")
}

func joinHintParts(parts ...string) string {
	filtered := make([]string, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		filtered = append(filtered, part)
	}
	return strings.Join(filtered, "  ")
}

func (m Model) inlineToggleHint() string {
	normalized, profiles, stacks := m.currentHintViews()
	if normalized.focus == focusStacks && len(stacks) > 0 {
		if stacks[normalized.selectedStack].Status == app.StackStatusRunning {
			return m.t("Enter stop stack", "Enter 停止组合")
		}
		return m.t("Enter start stack", "Enter 启动组合")
	}
	if len(profiles) > 0 {
		if isActiveTunnelStatus(profiles[normalized.selectedProfile].State.Status) {
			return m.t("Enter stop tunnel", "Enter 停止隧道")
		}
		return m.t("Enter start tunnel", "Enter 启动隧道")
	}
	return ""
}

func (m Model) inlineRestartHint() string {
	normalized, profiles, stacks := m.currentHintViews()
	if normalized.focus == focusStacks && len(stacks) > 0 {
		return m.t("r restart stack", "r 重启组合")
	}
	if len(profiles) > 0 {
		return m.t("r restart tunnel", "r 重启隧道")
	}
	return ""
}

func (m Model) inlineFilterHint() string {
	if m.inspectorTab == inspectorTabLogs {
		return m.t("/ filter logs", "/ 筛选日志")
	}
	return m.t("/ filter", "/ 筛选")
}

func (m Model) currentHintViews() (Model, []app.ProfileView, []app.StackView) {
	profiles := filterProfileViews(m.service.ProfileViews(), m.filterQuery)
	stacks := filterStackViews(m.service.StackViews(), m.filterQuery)
	normalized := m.normalizeSelection(len(profiles), len(stacks))
	return normalized, profiles, stacks
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

	title := panelListTitle(m.t("Profiles", "配置"), len(views), 0, len(views))
	if len(views) == 0 {
		if m.filterQuery != "" {
			message := m.tf("No profiles match %q. Press Esc to clear the filter.", "没有匹配 %q 的配置。按 Esc 清除筛选。", m.filterQuery)
			return renderFixedPanel(title, []string{mutedStyle.Render(truncateText(message, innerWidth))}, width, height, focused)
		}
		if m.workspaceIsEmpty() {
			return renderFixedPanel(title, m.renderEmptyProfilesLines(innerWidth), width, height, focused)
		}
		message := m.t("No profiles yet. Press a to add a starter draft or e to edit the config file.", "还没有配置。按 a 创建草稿，或按 e 编辑配置文件。")
		return renderFixedPanel(title, []string{mutedStyle.Render(truncateText(message, innerWidth))}, width, height, focused)
	}

	start, end := windowAroundSelection(len(views), m.selectedProfile, bodyHeight)
	title = panelListTitle(m.t("Profiles", "配置"), len(views), start, end)

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

	title := panelListTitle(m.t("Stacks", "组合"), len(views), 0, len(views))
	if len(views) == 0 {
		if m.filterQuery != "" {
			message := m.tf("No stacks match %q. Press Esc to clear the filter.", "没有匹配 %q 的组合。按 Esc 清除筛选。", m.filterQuery)
			return renderFixedPanel(title, []string{mutedStyle.Render(truncateText(message, innerWidth))}, width, height, focused)
		}
		if len(m.service.ProfileViews()) > 0 {
			return renderFixedPanel(title, m.renderEmptyStacksLines(innerWidth), width, height, focused)
		}
		message := m.t("No stacks yet. Add stacks to your config to launch groups of tunnels together.", "还没有组合。把组合加进配置后，就能成组启动隧道。")
		return renderFixedPanel(title, []string{mutedStyle.Render(truncateText(message, innerWidth))}, width, height, focused)
	}

	start, end := windowAroundSelection(len(views), m.selectedStack, bodyHeight)
	title = panelListTitle(m.t("Stacks", "组合"), len(views), start, end)

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
	line := composeHighlightedLine(
		renderStatusBadge(m.language(), view.State.Status)+" ",
		fmt.Sprintf("%s  %s  %s", view.Profile.Name, profileListPort(view.Profile), profileTarget(m.language(), view.Profile)),
		m.filterQuery,
		contentWidth,
	)
	return renderSizedBlock(style, width, marker+line)
}

func (m Model) renderStackRow(view app.StackView, selected bool, focused bool, width int) string {
	style := listRowStyleFor(selected, focused)
	marker := selectionMarker(selected, focused)
	contentWidth := max(1, width-style.GetHorizontalFrameSize()-lipgloss.Width(marker))
	line := composeHighlightedLine(
		renderStackStatusBadge(m.language(), view.Status)+" ",
		fmt.Sprintf("%s  %d/%d  %s", view.Stack.Name, view.ActiveCount, len(view.Members), stackMembersSummary(m.language(), view)),
		m.filterQuery,
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
		return m.t("Inspector", "检查器")
	}
	return m.t("Inspector ", "检查器 ") + label
}

func (m Model) renderInspectorTabs(width int) string {
	tabs := []string{
		renderInspectorTab("h", m.t("Details", "详情"), m.inspectorTab == inspectorTabDetails),
		renderInspectorTab("l", m.t("Logs", "日志"), m.inspectorTab == inspectorTabLogs),
	}
	if m.inspectorTab == inspectorTabLogs || m.logFilterQuery != "" {
		tabs = append(tabs, renderInspectorTab("/", m.t("filter", "筛选"), m.filterMode && m.activeFilterScope() == filterScopeLogs))
	}
	line := strings.Join(tabs, " ")
	return lipgloss.NewStyle().Width(max(1, width)).Render(line)
}

func (m Model) renderInspectorScrollLine(scroll, pageSize, total, width int) string {
	if total == 0 {
		return renderInlineText(mutedStyle, m.t("Lines 0/0", "行 0/0"), width)
	}

	start := min(total, scroll+1)
	end := min(total, scroll+pageSize)
	if pageSize >= total {
		start = 1
		end = total
	}

	return renderInlineText(
		mutedStyle,
		m.tf("Lines %d-%d/%d", "行 %d-%d/%d", start, end, total),
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
			return []string{mutedStyle.Render(truncateText(m.t("No filtered profile is selected, so there are no logs to show.", "当前没有选中筛选后的配置，因此没有可显示的日志。"), width))}
		}
		return []string{mutedStyle.Render(truncateText(m.t("Start a tunnel to see runtime output here.", "启动隧道后，这里会显示运行日志。"), width))}
	}

	if m.focus == focusStacks && len(stacks) > 0 {
		return m.renderStackDetailLines(stacks[m.selectedStack], width)
	}
	if len(profiles) > 0 {
		return m.renderProfileDetailLines(profiles[m.selectedProfile], width)
	}
	if m.filterQuery != "" {
		return []string{mutedStyle.Render(truncateText(m.t("No profile matches the current filter.", "当前筛选下没有匹配的配置。"), width))}
	}
	if m.workspaceIsEmpty() {
		return m.renderEmptyInspectorLines(width)
	}
	return []string{mutedStyle.Render(truncateText(m.t("No profile selected.", "当前没有选中的配置。"), width))}
}

func (m Model) renderProfileDetailLines(view app.ProfileView, width int) []string {
	now := m.currentTime()
	language := m.language()
	spec, specErr := app.BuildProcessSpec(view.Profile)
	lines := []string{
		composeStyledLine(
			renderStatusBadge(language, view.State.Status)+" ",
			fmt.Sprintf("%s  %s", view.Profile.Name, humanTunnelType(language, view.Profile.Type)),
			width,
		),
		groupTitleStyle.Render(m.t("Overview", "概览")),
		renderCompactKeyValue(profilePortSummaryLabel(language, view.Profile), profilePortSummaryValue(view.Profile), width),
		renderCompactKeyValue(m.t("Target", "目标"), profileTarget(language, view.Profile), width),
		groupTitleStyle.Render(m.t("Runtime", "运行态")),
		renderCompactKeyValue(m.t("Status", "状态"), humanTunnelStatus(language, view.State.Status), width),
		renderCompactKeyValue("PID", formatPID(view.State.PID), width),
		renderCompactKeyValue(m.t("Uptime", "运行时长"), formatUptime(view.State.StartedAt, now), width),
		renderCompactKeyValue(m.t("Restarts", "重试次数"), fmt.Sprintf("%d", view.State.RestartCount), width),
		renderCompactKeyValue(m.t("Last Exit", "上次退出"), formatLastExit(language, view.State, now), width),
		renderCompactKeyValue(m.t("Restart", "重启策略"), restartPolicySummary(language, view.Profile.Restart), width),
	}

	lines = append(lines, m.renderProfileStartLines(view, specErr, width)...)

	configLines := make([]string, 0, 4)
	if view.Profile.Description != "" {
		configLines = append(configLines, renderCompactKeyValue(m.t("About", "说明"), view.Profile.Description, width))
	}
	if len(view.Profile.Labels) > 0 {
		configLines = append(configLines, renderCompactKeyValue(m.t("Labels", "标签"), strings.Join(view.Profile.Labels, ", "), width))
	}
	if len(configLines) > 0 {
		lines = append(lines, groupTitleStyle.Render(m.t("Config", "配置")))
		lines = append(lines, configLines...)
	}

	if specErr == nil {
		lines = append(lines, groupTitleStyle.Render(m.t("Command", "命令")))
		lines = append(lines, renderCompactKeyValue("Exec", spec.DisplayCommand(), width))
	}

	problemLines := make([]string, 0, 2)
	if specErr != nil {
		problemLines = append(problemLines, renderCompactKeyValue(m.t("Config", "配置"), specErr.Error(), width))
	}
	if view.State.LastError != "" {
		problemLines = append(problemLines, renderCompactKeyValue(m.t("Error", "错误"), view.State.LastError, width))
	}
	if len(problemLines) > 0 {
		lines = append(lines, groupTitleStyle.Render(m.t("Problem", "问题")))
		lines = append(lines, problemLines...)
	}

	lines = append(lines, groupTitleStyle.Render(m.t("Actions", "操作")))
	lines = append(lines, m.profileActionLines(view, width)...)

	return lines
}

func (m Model) renderStackDetailLines(view app.StackView, width int) []string {
	language := m.language()
	lines := []string{
		composeStyledLine(
			renderStackStatusBadge(language, view.Status)+" ",
			fmt.Sprintf("%s  %s", view.Stack.Name, humanStackStatus(language, view.Status)),
			width,
		),
		groupTitleStyle.Render(m.t("Overview", "概览")),
		renderCompactKeyValue(m.t("Members", "成员"), fmt.Sprintf("%d", len(view.Members)), width),
		renderCompactKeyValue(m.t("Active", "已运行"), fmt.Sprintf("%d", view.ActiveCount), width),
		renderCompactKeyValue(m.t("Coverage", "覆盖率"), m.tf("%d/%d running", "%d/%d 运行中", view.ActiveCount, len(view.Members)), width),
		groupTitleStyle.Render(m.t("Members", "成员")),
	}

	if len(view.Members) == 0 {
		lines = append(lines, mutedStyle.Render(truncateText(m.t("No member profiles resolved from config.", "配置里没有解析出任何成员配置。"), width)))
	} else {
		for _, member := range view.Members {
			lines = append(lines, composeStyledLine(
				renderStatusBadge(language, member.State.Status)+" ",
				fmt.Sprintf("%s  %s  %s", member.Profile.Name, profileListPort(member.Profile), profileTarget(language, member.Profile)),
				width,
			))
		}
	}

	lines = append(lines, m.renderStackStartLines(view, width)...)

	if missingProfiles := missingStackProfiles(view); len(missingProfiles) > 0 {
		lines = append(lines, groupTitleStyle.Render(m.t("Problem", "问题")))
		lines = append(lines, renderCompactKeyValue(m.t("Missing", "缺失"), strings.Join(missingProfiles, ", "), width))
	}

	configLines := make([]string, 0, 4)
	if view.Stack.Description != "" {
		configLines = append(configLines, renderCompactKeyValue(m.t("About", "说明"), view.Stack.Description, width))
	}
	if len(view.Stack.Labels) > 0 {
		configLines = append(configLines, renderCompactKeyValue(m.t("Labels", "标签"), strings.Join(view.Stack.Labels, ", "), width))
	}
	if len(configLines) > 0 {
		lines = append(lines, groupTitleStyle.Render(m.t("Config", "配置")))
		lines = append(lines, configLines...)
	}

	if view.Status == app.StackStatusPartial {
		lines = append(lines, groupTitleStyle.Render(m.t("Action", "动作")))
		lines = append(lines, mutedStyle.Render(truncateText(m.t("Press Enter to start the missing members and restore the stack.", "按 Enter 启动缺失成员并恢复这个组合。"), width)))
	}

	lines = append(lines, groupTitleStyle.Render(m.t("Actions", "操作")))
	lines = append(lines, m.stackActionLines(view, width)...)

	return lines
}

func (m Model) renderProfileLogLines(view app.ProfileView, width int) []string {
	filtered := filterLogEntries(view.State.RecentLogs, m.logFilterQuery)
	if len(view.State.RecentLogs) == 0 {
		return []string{mutedStyle.Render(truncateText(m.t("No logs yet. Start the tunnel to collect runtime output.", "还没有日志。启动隧道后，这里会显示运行输出。"), width))}
	}
	if len(filtered) == 0 {
		return []string{mutedStyle.Render(truncateText(m.tf("No logs match %q. Press Esc to clear the log filter.", "没有匹配 %q 的日志。按 Esc 清除日志筛选。", m.logFilterQuery), width))}
	}

	lines := make([]string, 0, len(filtered)+3)
	lines = append(lines, m.renderLogSummaryLines(
		m.tf("Showing %s logs • newest first", "显示 %s 条日志 • 最新在前", formatVisibleCount(len(filtered), len(view.State.RecentLogs))),
		logSourceCountsForEntries(filtered),
		m.logFilterQuery,
		width,
		false,
	)...)
	for idx := len(filtered) - 1; idx >= 0; idx-- {
		entry := filtered[idx]
		lines = append(lines, renderLogLine(entry.Timestamp, "", entry.Source, entry.Message, m.logFilterQuery, width))
	}

	return lines
}

func (m Model) renderStackLogLines(view app.StackView, width int) []string {
	totalEntries := 0
	for _, member := range view.Members {
		totalEntries += len(member.State.RecentLogs)
	}
	if totalEntries == 0 {
		return []string{mutedStyle.Render(truncateText(m.t("No recent stack activity yet. Start a member to begin collecting logs.", "还没有最近的组合活动。启动任一成员后，这里会开始显示日志。"), width))}
	}

	activity := filterStackActivity(recentStackActivity(view, totalEntries), m.logFilterQuery)
	if len(activity) == 0 {
		return []string{mutedStyle.Render(truncateText(m.tf("No stack logs match %q. Press Esc to clear the log filter.", "没有匹配 %q 的组合日志。按 Esc 清除日志筛选。", m.logFilterQuery), width))}
	}

	lines := make([]string, 0, len(activity)+3)
	lines = append(lines, m.renderLogSummaryLines(
		m.tf(
			"Showing %s logs from %s profiles • newest first",
			"显示 %s 条日志，来自 %s 个配置 • 最新在前",
			formatVisibleCount(len(activity), totalEntries),
			formatVisibleCount(uniqueStackActivityProfiles(activity), len(view.Members)),
		),
		logSourceCountsForStackActivity(activity),
		m.logFilterQuery,
		width,
		true,
	)...)
	for idx := len(activity) - 1; idx >= 0; idx-- {
		entry := activity[idx]
		lines = append(lines, renderLogLine(entry.Log.Timestamp, entry.ProfileName, entry.Log.Source, entry.Log.Message, m.logFilterQuery, width))
	}

	return lines
}

func (m Model) renderLogSummaryLines(summary string, counts logSourceCounts, query string, width int, includeProfiles bool) []string {
	lines := []string{mutedStyle.Render(truncateText(summary, width))}
	lines = append(lines, renderLogSourceCountLine(m.language(), counts, width))
	if query != "" {
		lines = append(lines, m.renderLogFilterSummaryLine(query, width, includeProfiles))
	}
	return lines
}

func renderLogSourceCountLine(language domain.Language, counts logSourceCounts, width int) string {
	line := strings.Join([]string{
		mutedStyle.Render(translate(language, "Sources", "来源")),
		renderLogSourceBadge(domain.LogSourceSystem, "") + " " + mutedStyle.Render(fmt.Sprintf("%d", counts.system)),
		renderLogSourceBadge(domain.LogSourceStdout, "") + " " + mutedStyle.Render(fmt.Sprintf("%d", counts.stdout)),
		renderLogSourceBadge(domain.LogSourceStderr, "") + " " + mutedStyle.Render(fmt.Sprintf("%d", counts.stderr)),
	}, "  ")
	return truncateText(line, width)
}

func (m Model) renderLogFilterSummaryLine(query string, width int, includeProfiles bool) string {
	fields := m.t("messages, sources", "消息、来源")
	if includeProfiles {
		fields = m.t("messages, sources, profiles", "消息、来源、profile")
	}

	text := m.tf("Filter: %s • %s", "筛选：%s • %s", query, fields)
	return renderHighlightedText(truncateText(text, width), query, mutedStyle, filterMatchStyle)
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
		return m.tf("stack/%s", "组合/%s", stacks[m.selectedStack].Stack.Name)
	}
	if len(profiles) > 0 {
		return m.tf("profile/%s", "配置/%s", profiles[m.selectedProfile].Profile.Name)
	}
	return "none"
}

func (m Model) renderFilterBar() string {
	label := filterIdleStyle.Render(m.currentFilterLabel() + " /")
	if m.filterMode {
		label = filterActiveStyle.Render(m.t("typing", "输入中"))
	}

	query := m.visibleFilterQuery()
	if query == "" {
		query = m.currentFilterPlaceholder(false)
	}

	return lipgloss.JoinHorizontal(
		lipgloss.Center,
		label,
		"  ",
		sectionTextStyle.Render(query),
	)
}

func (m Model) handleFilterKey(msg tea.KeyMsg) (Model, bool) {
	query := m.visibleFilterQuery()

	switch msg.String() {
	case "esc":
		if query != "" {
			if m.activeFilterScope() == filterScopeLogs {
				m.logFilterQuery = ""
				m.inspectorScroll = 0
			} else {
				m.filterQuery = ""
				m.selectedProfile = 0
				m.selectedStack = 0
				m.inspectorScroll = 0
			}
			return m, true
		}
		m.filterMode = false
		return m, true
	case "enter":
		m.filterMode = false
		return m, true
	case "backspace", "ctrl+h":
		m = m.updateActiveFilterQuery(trimLastRune(query))
		m.inspectorScroll = 0
		return m, true
	case "ctrl+w":
		m = m.updateActiveFilterQuery(trimLastWord(query))
		m.inspectorScroll = 0
		return m, true
	case "ctrl+u":
		m = m.updateActiveFilterQuery("")
		m.inspectorScroll = 0
		return m, true
	}

	nextQuery := query
	switch msg.Type {
	case tea.KeySpace:
		nextQuery += " "
	case tea.KeyRunes:
		nextQuery += string(msg.Runes)
	default:
		return m, false
	}

	m = m.updateActiveFilterQuery(nextQuery)
	m.inspectorScroll = 0
	return m, true
}

func (m Model) handleWorkspaceKey(msg tea.KeyMsg, profiles []app.ProfileView, stacks []app.StackView) (Model, tea.Cmd, bool) {
	switch msg.String() {
	case "g":
		m = m.reloadConfigFromDisk(m.t("Reloaded config from disk.", "已从磁盘重新加载配置。"))
		return m, nil, true
	case "i", "I":
		m.lastError = ""
		m.lastNotice = ""
		m.filterMode = false
		m.importMode = true
		return m, nil, true
	case "s", "S":
		if !m.workspaceIsEmpty() {
			return m, nil, false
		}
		m = m.initializeSampleConfig()
		return m, nil, true
	case "e":
		if err := m.ensureConfigFileExists(); err != nil {
			m.lastError = err.Error()
			return m, nil, true
		}
		m.lastError = ""
		m.lastNotice = ""
		return m, openEditorCmd(m.configPath), true
	case "a":
		m = m.createStarterProfileDraft()
		return m, nil, true
	case "L":
		m = m.toggleLanguage()
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
	cfg.Language = m.language()
	if err := storage.SaveConfig(m.configPath, cfg); err != nil {
		m.lastError = m.t("Initialize sample config: ", "初始化示例配置失败: ") + err.Error()
		return m
	}

	m.service.ReplaceConfig(cfg)
	m.focus = focusProfiles
	m.selectedProfile = 0
	m.selectedStack = 0
	m.inspectorTab = inspectorTabDetails
	m.inspectorScroll = 0
	m.lastNotice = m.t("Initialized sample config with starter profiles and a stack.", "已初始化示例配置，包含示例配置和一个组合。")
	return m
}

func (m Model) createStarterProfileDraft() Model {
	m.lastError = ""
	m.lastNotice = ""
	m.filterQuery = ""
	m.filterMode = false
	m.importMode = false

	cfg := m.service.Config()
	profile := starterSSHProfileDraft(cfg, m.language())
	cfg.SetProfile(profile)

	if err := storage.SaveConfig(m.configPath, cfg); err != nil {
		m.lastError = m.t("Create starter profile: ", "创建配置草稿失败: ") + err.Error()
		return m
	}

	m.service.ReplaceConfig(cfg)
	m.focus = focusProfiles
	m.selectedStack = 0
	m.inspectorTab = inspectorTabDetails
	m.inspectorScroll = 0
	m.selectProfileByName(profile.Name)
	m.lastNotice = m.tf("Created starter profile %s. Press e to refine the config.", "已创建配置草稿 %s。按 e 继续完善配置。", profile.Name)
	return m
}

func (m Model) createStarterStackDraft(profiles []app.ProfileView, stacks []app.StackView) Model {
	m.lastError = ""
	m.lastNotice = ""
	m.importMode = false

	cfg := m.service.Config()
	stack, err := starterStackDraft(cfg, profiles, stacks, m.focus, m.selectedProfile, m.selectedStack, m.filterQuery, m.language())
	if err != nil {
		m.lastError = err.Error()
		return m
	}

	m.filterQuery = ""
	m.filterMode = false
	cfg.SetStack(stack)

	if err := storage.SaveConfig(m.configPath, cfg); err != nil {
		m.lastError = m.t("Create starter stack: ", "创建组合草稿失败: ") + err.Error()
		return m
	}

	m.service.ReplaceConfig(cfg)
	m.focus = focusStacks
	m.selectedProfile = 0
	m.inspectorTab = inspectorTabDetails
	m.inspectorScroll = 0
	m.selectStackByName(stack.Name)
	m.lastNotice = m.tf("Created starter stack %s. Press e to refine the config.", "已创建组合草稿 %s。按 e 继续完善配置。", stack.Name)
	return m
}

func (m Model) toggleLanguage() Model {
	m.lastError = ""
	m.lastNotice = ""
	m.importMode = false

	cfg := m.service.Config()
	next := nextLanguage(cfg.Language)
	cfg.Language = next

	if err := storage.SaveConfig(m.configPath, cfg); err != nil {
		m.lastError = m.t("Switch language: ", "切换语言失败: ") + err.Error()
		return m
	}

	m.service.ReplaceConfig(cfg)
	m.lastNotice = translatef(next, "Switched language to %s.", "已切换语言为%s。", languageDisplayName(next))
	return m
}

func (m Model) cloneSelection(profiles []app.ProfileView, stacks []app.StackView) Model {
	m.lastError = ""
	m.lastNotice = ""
	m.importMode = false

	if m.focus == focusStacks && len(stacks) > 0 {
		return m.cloneSelectedStack(stacks)
	}
	if len(profiles) > 0 {
		return m.cloneSelectedProfile(profiles)
	}
	if m.filterQuery != "" {
		m.lastError = m.t("No visible item to clone. Clear the filter or select a different item.", "没有可见项可克隆。请清除筛选或换一个条目。")
		return m
	}
	m.lastError = m.t("Nothing is selected to clone yet.", "当前还没有选中任何可克隆的条目。")
	return m
}

func (m Model) restartSelection(profiles []app.ProfileView, stacks []app.StackView) Model {
	m.lastError = ""
	m.lastNotice = ""
	m.importMode = false

	if m.focus == focusStacks && len(stacks) > 0 {
		name := stacks[max(0, min(m.selectedStack, len(stacks)-1))].Stack.Name
		if err := m.service.RestartStack(name); err != nil {
			m.lastError = err.Error()
			return m
		}
		m.lastNotice = m.tf("Restarted stack %s.", "已重启组合 %s。", name)
		return m
	}

	if len(profiles) > 0 {
		name := profiles[max(0, min(m.selectedProfile, len(profiles)-1))].Profile.Name
		if err := m.service.RestartProfile(name); err != nil {
			m.lastError = err.Error()
			return m
		}
		m.lastNotice = m.tf("Restarted profile %s.", "已重启配置 %s。", name)
		return m
	}

	if m.filterQuery != "" {
		m.lastError = m.t("No visible item to restart. Clear the filter or select a different item.", "没有可见项可重启。请清除筛选或换一个条目。")
		return m
	}

	m.lastError = m.t("Nothing is selected to restart yet.", "当前还没有选中任何可重启的条目。")
	return m
}

func (m Model) cloneSelectedProfile(profiles []app.ProfileView) Model {
	selected := profiles[max(0, min(m.selectedProfile, len(profiles)-1))]
	cfg := m.service.Config()
	profile := cloneProfileDefinition(selected.Profile)
	profile.Name = nextCopyName(profileNames(cfg), selected.Profile.Name)
	assignProfileDisplayPort(&profile, nextAvailableLocalPort(cfg, profileDisplayPort(selected.Profile)+1))
	profile.Labels = appendUniqueLabel(profile.Labels, "draft")

	cfg.SetProfile(profile)
	if err := storage.SaveConfig(m.configPath, cfg); err != nil {
		m.lastError = m.t("Clone profile: ", "克隆配置失败: ") + err.Error()
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
	m.lastNotice = m.tf("Cloned profile %s to %s.", "已将配置 %s 克隆为 %s。", selected.Profile.Name, profile.Name)
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
		m.lastError = m.t("Clone stack: ", "克隆组合失败: ") + err.Error()
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
	m.lastNotice = m.tf("Cloned stack %s to %s.", "已将组合 %s 克隆为 %s。", selected.Stack.Name, stack.Name)
	return m
}

func (m Model) reloadConfigFromDisk(successNotice string) Model {
	m.lastError = ""
	m.lastNotice = ""
	m.importMode = false

	cfg, err := storage.LoadConfig(m.configPath)
	if err != nil {
		m.lastError = m.t("Reload config: ", "重新加载配置失败: ") + err.Error()
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
		m.lastNotice = m.t("Delete cancelled.", "已取消删除。")
		return m, true
	case "y", "enter":
		m = m.confirmDelete()
		return m, true
	default:
		return m, true
	}
}

func (m Model) handleMouse(msg tea.MouseMsg, profiles []app.ProfileView, stacks []app.StackView) (Model, bool) {
	layout := m.mouseLayout(profiles, stacks)

	if m.pendingDelete != nil {
		return m, false
	}

	if msg.Button == tea.MouseButtonWheelUp || msg.Button == tea.MouseButtonWheelDown {
		if layout.inspector.contains(msg.X, msg.Y) {
			m = m.scrollInspectorMouse(msg.Button, profiles, stacks)
			return m, true
		}
		return m, false
	}

	if msg.Action != tea.MouseActionPress || msg.Button != tea.MouseButtonLeft {
		return m, false
	}

	if m.importMode {
		for _, region := range layout.importActions {
			if !region.rect.contains(msg.X, msg.Y) {
				continue
			}

			switch region.action {
			case "ssh":
				m = m.importSSHDraftProfiles()
			case "kube":
				m = m.importKubernetesDraftProfiles()
			case "all":
				m = m.importAllDraftProfiles()
			}
			return m, true
		}
		return m, false
	}

	if layout.headerFilter.contains(msg.X, msg.Y) {
		m.lastNotice = ""
		m.filterMode = true
		m.filterScope = m.defaultFilterScope()
		return m, true
	}

	for _, tab := range layout.inspectorTabs {
		if !tab.rect.contains(msg.X, msg.Y) {
			continue
		}

		if tab.scope == filterScopeLogs {
			m.filterMode = true
			m.filterScope = filterScopeLogs
			return m, true
		}

		if m.inspectorTab != tab.tab {
			m.inspectorTab = tab.tab
			m.inspectorScroll = 0
		}
		return m, true
	}

	if idx, ok := layout.profiles.rowIndexAt(msg.X, msg.Y); ok {
		m.focus = focusProfiles
		m.selectedProfile = idx
		m.inspectorScroll = 0
		return m, true
	}

	if idx, ok := layout.stacks.rowIndexAt(msg.X, msg.Y); ok {
		m.focus = focusStacks
		m.selectedStack = idx
		m.inspectorScroll = 0
		return m, true
	}

	return m, false
}

func (m Model) handleImportKey(msg tea.KeyMsg) (Model, bool) {
	switch msg.String() {
	case "esc":
		m.importMode = false
		m.lastNotice = m.t("Import cancelled.", "已取消导入。")
		return m, true
	case "s", "S":
		m = m.importSSHDraftProfiles()
		return m, true
	case "k", "K":
		m = m.importKubernetesDraftProfiles()
		return m, true
	case "a", "A":
		m = m.importAllDraftProfiles()
		return m, true
	default:
		return m, true
	}
}

func (m Model) scrollInspectorMouse(button tea.MouseButton, profiles []app.ProfileView, stacks []app.StackView) Model {
	delta := max(1, m.inspectorPageSize()/4)
	total := len(m.currentInspectorLines(profiles, stacks, panelInnerWidth(m.inspectorPanelRect().width)))
	pageSize := m.inspectorPageSize()
	maxScroll := max(0, total-pageSize)

	switch button {
	case tea.MouseButtonWheelUp:
		m.inspectorScroll = max(0, m.inspectorScroll-delta)
	case tea.MouseButtonWheelDown:
		m.inspectorScroll = min(maxScroll, m.inspectorScroll+delta)
	}

	return m
}

func (m Model) buildDeleteRequest(profiles []app.ProfileView, stacks []app.StackView) *deleteRequest {
	if m.focus == focusStacks && len(stacks) > 0 {
		view := stacks[m.selectedStack]
		message := m.tf(
			"Delete stack %s? This removes the saved stack only; member tunnels keep running. Press y or Enter to confirm, n or Esc to cancel.",
			"删除组合 %s？这只会移除保存的组合，成员隧道会继续运行。按 y 或 Enter 确认，n 或 Esc 取消。",
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
		impactParts = append(impactParts, m.t("the running tunnel will be stopped", "正在运行的隧道会先被停止"))
	}
	if len(stackNames) > 0 {
		emptyStacks := 0
		for _, stack := range m.service.Stacks() {
			if len(stack.Profiles) == 1 && stack.Profiles[0] == view.Profile.Name {
				emptyStacks++
			}
		}
		impact := m.tf("%d stack references will be pruned", "会裁剪 %d 处组合引用", len(stackNames))
		if emptyStacks > 0 {
			impact += m.tf(" and %d empty stacks will be removed", "，并移除 %d 个空组合", emptyStacks)
		}
		impactParts = append(impactParts, impact)
	}
	if len(impactParts) == 0 {
		impactParts = append(impactParts, m.t("this profile will be removed from the saved config", "这个配置会从保存的配置中移除"))
	}

	message := m.tf(
		"Delete profile %s? %s. Press y or Enter to confirm, n or Esc to cancel.",
		"删除配置 %s？%s。按 y 或 Enter 确认，n 或 Esc 取消。",
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
	m.importMode = false

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

		m.lastNotice = profileDeleteNotice(m.language(), result)
		return m

	case deleteKindStack:
		result, err := m.service.RemoveStack(request.Name, func(cfg domain.Config) error {
			return storage.SaveConfig(m.configPath, cfg)
		})
		if err != nil {
			m.lastError = err.Error()
			return m
		}

		m.lastNotice = m.tf("Removed stack %s.", "已移除组合 %s。", result.Name)
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
	return m.pendingDelete != nil || m.importMode || m.lastError != "" || m.lastNotice != ""
}

func (m Model) contentOrigin() (int, int) {
	return appStyle.GetPaddingLeft(), appStyle.GetPaddingTop()
}

func panelContentX(rect mouseRect) int {
	return rect.x + panelStyle.GetBorderLeftSize() + panelStyle.GetPaddingLeft()
}

func panelBodyStartY(rect mouseRect) int {
	return rect.y + panelStyle.GetBorderTopSize() + panelStyle.GetPaddingTop() + 1
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
	query := m.visibleFilterQuery()
	if m.filterMode || query != "" {
		labelStyle = filterActiveStyle
	}
	if m.filterMode {
		inputStyle = filterInputActiveStyle
	}

	value := query
	valueStyle := sectionTextStyle
	if value == "" {
		if m.filterMode {
			value = m.currentFilterPlaceholder(true)
		} else {
			value = m.currentFilterPlaceholder(false)
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
		labelStyle.Render(m.currentFilterLabel()),
		" ",
		renderSizedBlock(inputStyle, inputWidth, content),
	)
}

func (m Model) mouseLayout(profiles []app.ProfileView, stacks []app.StackView) mouseLayout {
	contentX, contentY := m.contentOrigin()
	width := m.contentWidth()
	headerLines := m.renderHeaderLines(profiles, stacks, len(m.service.ProfileViews()), len(m.service.StackViews()), width)
	headerHeight := renderedLinesHeight(headerLines, width)
	filterY := contentY
	if len(headerLines) > 0 {
		filterY += renderedLineHeight(headerLines[0], width)
	}

	filterSegment := m.renderHeaderFilterSegment(preferredFilterInputWidth(width))
	layout := mouseLayout{
		headerFilter: mouseRect{
			x:      contentX,
			y:      filterY,
			width:  lipgloss.Width(filterSegment),
			height: 1,
		},
	}

	statusY := contentY + headerHeight
	if m.importMode {
		layout.importActions = m.importActionRegions(contentX, statusY, width)
	}

	bodyY := statusY
	if status := m.renderStatusLine(width); status != "" {
		bodyY += renderedLineHeight(status, width)
	}
	bodyHeight := m.bodyHeight()

	if m.useTwoColumnLayout(width, bodyHeight) {
		leftWidth := min(44, max(36, width/3))
		rightWidth := max(32, width-leftWidth-1)
		profilesHeight, stacksHeight := splitDualListHeights(bodyHeight)

		layout.profiles = buildMouseListRegion(contentX, bodyY, leftWidth, profilesHeight, len(profiles), m.selectedProfile, focusProfiles, true)
		layout.stacks = buildMouseListRegion(contentX, bodyY+profilesHeight+1, leftWidth, stacksHeight, len(stacks), m.selectedStack, focusStacks, true)
		layout.inspector = mouseRect{x: contentX + leftWidth + 1, y: bodyY, width: rightWidth, height: bodyHeight}
		layout.inspectorTabs = m.inspectorTabRegions(layout.inspector)
		return layout
	}

	listHeight, inspectorHeight := splitListInspectorHeights(bodyHeight)
	if m.focus == focusStacks {
		layout.stacks = buildMouseListRegion(contentX, bodyY, width, listHeight, len(stacks), m.selectedStack, focusStacks, true)
	} else {
		layout.profiles = buildMouseListRegion(contentX, bodyY, width, listHeight, len(profiles), m.selectedProfile, focusProfiles, true)
	}
	layout.inspector = mouseRect{x: contentX, y: bodyY + listHeight + 1, width: width, height: inspectorHeight}
	layout.inspectorTabs = m.inspectorTabRegions(layout.inspector)
	return layout
}

func buildMouseListRegion(x, y, width, height, total, selected int, focus listFocus, visible bool) mouseListRegion {
	bodyHeight := panelBodyHeight(height)
	start, end := windowAroundSelection(total, selected, bodyHeight)
	return mouseListRegion{
		panel:   mouseRect{x: x, y: y, width: width, height: height},
		visible: visible,
		start:   start,
		end:     end,
		focus:   focus,
	}
}

func (r mouseListRegion) rowIndexAt(x, y int) (int, bool) {
	if !r.visible || !r.panel.contains(x, y) {
		return 0, false
	}

	row := y - panelBodyStartY(r.panel)
	if row < 0 {
		return 0, false
	}

	idx := r.start + row
	if idx >= r.end {
		return 0, false
	}

	return idx, true
}

func (m Model) inspectorPanelRect() mouseRect {
	profiles := filterProfileViews(m.service.ProfileViews(), m.filterQuery)
	stacks := filterStackViews(m.service.StackViews(), m.filterQuery)
	layout := m.mouseLayout(profiles, stacks)
	return layout.inspector
}

func (m Model) inspectorTabRegions(panel mouseRect) []mouseInspectorTabRegion {
	innerX := panelContentX(panel)
	y := panelBodyStartY(panel)
	x := innerX

	type tabDef struct {
		key   string
		label string
		tab   inspectorTab
		scope filterScope
	}

	defs := []tabDef{
		{key: "h", label: m.t("Details", "详情"), tab: inspectorTabDetails},
		{key: "l", label: m.t("Logs", "日志"), tab: inspectorTabLogs},
	}
	if m.inspectorTab == inspectorTabLogs || m.logFilterQuery != "" {
		defs = append(defs, tabDef{key: "/", label: m.t("filter", "筛选"), scope: filterScopeLogs})
	}

	regions := make([]mouseInspectorTabRegion, 0, len(defs))
	for _, def := range defs {
		active := def.scope == filterScopeLogs && m.filterMode && m.activeFilterScope() == filterScopeLogs
		if def.scope != filterScopeLogs {
			active = def.tab == m.inspectorTab
		}

		width := lipgloss.Width(renderInspectorTab(def.key, def.label, active))
		regions = append(regions, mouseInspectorTabRegion{
			rect:  mouseRect{x: x, y: y, width: width, height: 1},
			tab:   def.tab,
			scope: def.scope,
		})
		x += width + 1
	}

	return regions
}

func (m Model) importActionRegions(contentX, y, width int) []mouseImportActionRegion {
	text := truncateText(m.importPromptMessage(), max(1, width-importPromptStyle.GetHorizontalFrameSize()))
	startX := contentX + importPromptStyle.GetPaddingLeft()

	actions := []struct {
		action string
		text   string
	}{
		{action: "ssh", text: m.t("s SSH config", "s SSH 配置")},
		{action: "kube", text: m.t("k kube contexts", "k Kubernetes context")},
		{action: "all", text: m.t("a both", "a 全部导入")},
	}

	regions := make([]mouseImportActionRegion, 0, len(actions))
	for _, action := range actions {
		idx := strings.Index(text, action.text)
		if idx < 0 {
			continue
		}

		regions = append(regions, mouseImportActionRegion{
			rect: mouseRect{
				x:      startX + lipgloss.Width(text[:idx]),
				y:      y,
				width:  lipgloss.Width(action.text),
				height: 1,
			},
			action: action.action,
		})
	}

	return regions
}

func (m Model) defaultFilterScope() filterScope {
	if m.inspectorTab == inspectorTabLogs {
		return filterScopeLogs
	}
	return filterScopeList
}

func (m Model) activeFilterScope() filterScope {
	if m.filterMode {
		return m.filterScope
	}
	return m.defaultFilterScope()
}

func (m Model) visibleFilterQuery() string {
	switch m.activeFilterScope() {
	case filterScopeLogs:
		return m.logFilterQuery
	default:
		return m.filterQuery
	}
}

func (m Model) currentFilterLabel() string {
	switch m.activeFilterScope() {
	case filterScopeLogs:
		return m.t("Logs", "日志")
	default:
		return m.t("Filter", "筛选")
	}
}

func (m Model) currentFilterPlaceholder(editing bool) string {
	switch m.activeFilterScope() {
	case filterScopeLogs:
		if editing {
			return m.t("type to search logs", "输入以筛选日志")
		}
		return m.t("message, source, profile", "消息、来源、profile")
	default:
		if editing {
			return m.t("type to filter", "输入以筛选")
		}
		return m.t("name, label, target", "名称、标签、目标")
	}
}

func (m Model) updateActiveFilterQuery(query string) Model {
	switch m.activeFilterScope() {
	case filterScopeLogs:
		m.logFilterQuery = query
	default:
		m.filterQuery = query
		m.selectedProfile = 0
		m.selectedStack = 0
	}
	return m
}

func (m Model) selectedValueStyle(profiles []app.ProfileView, stacks []app.StackView) lipgloss.Style {
	if m.selectedLabel(profiles, stacks) == "none" {
		return headerSelectedEmptyValueStyle
	}
	return headerSelectedValueStyle
}

func (m Model) renderEmptyProfilesLines(width int) []string {
	rows := renderQuickActionRows(width, []quickAction{
		{key: "i", label: m.t("import drafts", "导入草稿")},
		{key: "s", label: m.t("sample config", "示例配置")},
		{key: "a", label: m.t("draft profile", "配置草稿")},
		{key: "e", label: m.t("edit config", "编辑配置")},
		{key: "g", label: m.t("reload config", "重新加载配置")},
		{key: "L", label: m.t("switch language", "切换语言")},
	})
	return append(rows, mutedStyle.Render(truncateText(m.t("No profiles yet. Start here.", "还没有配置。从这里开始。"), width)))
}

func (m Model) renderEmptyStacksLines(width int) []string {
	rows := renderQuickActionRows(width, []quickAction{
		{key: "A", label: m.t("draft stack", "组合草稿")},
		{key: "i", label: m.t("import drafts", "导入草稿")},
		{key: "e", label: m.t("edit config", "编辑配置")},
		{key: "g", label: m.t("reload config", "重新加载配置")},
		{key: "L", label: m.t("switch language", "切换语言")},
		{key: "Tab", label: m.t("focus profiles", "切到配置")},
	})
	return append(rows, mutedStyle.Render(truncateText(m.t("No stacks yet. Create one from the selected profile.", "还没有组合。可以从当前选中的配置创建一个。"), width)))
}

func (m Model) renderEmptyInspectorLines(width int) []string {
	return []string{
		groupTitleStyle.Render(m.t("Quick Start", "快速开始")),
		sectionTextStyle.Render(truncateText(m.t("The workspace is empty. Create tunnels or load an example config.", "当前工作区是空的。创建隧道，或加载一份示例配置。"), width)),
		"",
		renderActionLine("i", m.t("import drafts from SSH and Kubernetes config", "从 SSH 和 Kubernetes 配置导入草稿"), width),
		renderActionLine("s", m.t("seed sample SSH and Kubernetes tunnels", "写入示例 SSH 和 Kubernetes 隧道"), width),
		renderActionLine("a", m.t("create a starter SSH profile draft", "创建一个 SSH 配置草稿"), width),
		renderActionLine("e", m.t("open the YAML config in your editor", "在编辑器里打开 YAML 配置"), width),
		renderActionLine("g", m.t("reload external config edits", "重新加载外部改动后的配置"), width),
		renderActionLine("L", m.t("switch between English and Chinese", "在英文和中文之间切换"), width),
		"",
		renderCompactKeyValue(m.t("Config", "配置"), m.configPath, width),
	}
}

func renderActionLine(key, description string, width int) string {
	prefix := codeStyle.Render(" " + key + " ")
	return composeStyledLine(prefix+" ", description, width)
}

func (m Model) importSSHDraftProfiles() Model {
	cfg, result, err := profileimport.ImportSSHConfig(m.service.Config(), "", false)
	if err != nil {
		m.importMode = false
		m.lastError = m.t("Import SSH config: ", "导入 SSH 配置失败: ") + err.Error()
		return m
	}

	return m.applyImportedConfig(
		cfg,
		result.ProfileNames,
		result.Created,
		result.Updated,
		result.Skipped,
		m.tf("Imported SSH drafts from %s: %d created, %d updated, %d skipped.", "已从 %s 导入 SSH 草稿: 新建 %d，更新 %d，跳过 %d。", result.SourcePath, result.Created, result.Updated, result.Skipped),
	)
}

func (m Model) importKubernetesDraftProfiles() Model {
	cfg, result, err := profileimport.ImportKubeContexts(m.service.Config(), "", false)
	if err != nil {
		m.importMode = false
		m.lastError = m.t("Import kube contexts: ", "导入 Kubernetes context 失败: ") + err.Error()
		return m
	}

	return m.applyImportedConfig(
		cfg,
		result.ProfileNames,
		result.Created,
		result.Updated,
		result.Skipped,
		m.tf("Imported kube drafts from %s: %d created, %d updated, %d skipped.", "已从 %s 导入 Kubernetes 草稿: 新建 %d，更新 %d，跳过 %d。", result.SourcePath, result.Created, result.Updated, result.Skipped),
	)
}

func (m Model) importAllDraftProfiles() Model {
	cfg := m.service.Config()
	importedNames := make([]string, 0)
	totalCreated := 0
	totalUpdated := 0
	totalSkipped := 0

	cfg, sshResult, err := profileimport.ImportSSHConfig(cfg, "", false)
	if err != nil {
		m.importMode = false
		m.lastError = m.t("Import drafts: ", "导入草稿失败: ") + err.Error()
		return m
	}
	importedNames = append(importedNames, sshResult.ProfileNames...)
	totalCreated += sshResult.Created
	totalUpdated += sshResult.Updated
	totalSkipped += sshResult.Skipped

	cfg, kubeResult, err := profileimport.ImportKubeContexts(cfg, "", false)
	if err != nil {
		m.importMode = false
		m.lastError = m.t("Import drafts: ", "导入草稿失败: ") + err.Error()
		return m
	}
	importedNames = append(importedNames, kubeResult.ProfileNames...)
	totalCreated += kubeResult.Created
	totalUpdated += kubeResult.Updated
	totalSkipped += kubeResult.Skipped

	return m.applyImportedConfig(
		cfg,
		importedNames,
		totalCreated,
		totalUpdated,
		totalSkipped,
		m.tf("Imported drafts from SSH and kube config: %d created, %d updated, %d skipped.", "已从 SSH 和 Kubernetes 配置导入草稿: 新建 %d，更新 %d，跳过 %d。", totalCreated, totalUpdated, totalSkipped),
	)
}

func (m Model) applyImportedConfig(cfg domain.Config, importedNames []string, created, updated, skipped int, successNotice string) Model {
	m.lastError = ""
	m.lastNotice = ""
	m.importMode = false

	if created == 0 && updated == 0 {
		m.lastNotice = m.tf("Import finished: 0 created, 0 updated, %d skipped.", "导入完成: 新建 0，更新 0，跳过 %d。", skipped)
		return m
	}

	if err := storage.SaveConfig(m.configPath, cfg); err != nil {
		m.lastError = m.t("Save imported drafts: ", "保存导入草稿失败: ") + err.Error()
		return m
	}

	m.service.ReplaceConfig(cfg)
	m.focus = focusProfiles
	m.selectedStack = 0
	m.inspectorTab = inspectorTabDetails
	m.inspectorScroll = 0
	m.pendingDelete = nil
	m.filterQuery = ""
	m.filterMode = false
	if len(importedNames) > 0 {
		m.selectProfileByName(importedNames[0])
	}
	m.lastNotice = successNotice
	return m
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
		groupTitleStyle.Render(m.t("Start", "启动")),
		renderCompactKeyValue(m.t("Readiness", "就绪度"), profileStartSummary(m.language(), analysis.Status), width),
	}

	excludedProblem := ""
	if specErr != nil {
		excludedProblem = specErr.Error()
	}

	for _, problem := range analysis.Problems {
		if excludedProblem != "" && problem == excludedProblem {
			continue
		}
		lines = append(lines, renderCompactKeyValue(m.t("Blocker", "阻塞项"), problem, width))
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
		groupTitleStyle.Render(m.t("Start Plan", "启动计划")),
		renderCompactKeyValue(m.t("Readiness", "就绪度"), stackStartSummary(m.language(), view, analysis), width),
		renderCompactKeyValue(m.t("Ready", "可启动"), formatCountNoun(m.language(), analysis.ReadyCount, "member", "members", "个成员"), width),
		renderCompactKeyValue(m.t("Running", "运行中"), formatCountNoun(m.language(), analysis.ActiveCount, "member", "members", "个成员"), width),
		renderCompactKeyValue(m.t("Blocked", "已阻塞"), formatCountNoun(m.language(), analysis.BlockedCount, "member", "members", "个成员"), width),
	}

	if len(view.Stack.Profiles) == 0 {
		lines = append(lines, renderCompactKeyValue(m.t("Blocker", "阻塞项"), m.t("Add a profile to this stack first.", "先给这个组合添加一个配置。"), width))
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

func (m Model) profileActionLines(view app.ProfileView, width int) []string {
	toggleLabel := m.t("start tunnel", "启动隧道")
	if isActiveTunnelStatus(view.State.Status) {
		toggleLabel = m.t("stop tunnel", "停止隧道")
	}

	return []string{
		renderActionLine("Enter", toggleLabel, width),
		renderActionLine("r", m.t("restart tunnel", "重启隧道"), width),
		renderActionLine("c", m.t("clone profile draft", "克隆配置草稿"), width),
		renderActionLine("A", m.t("create stack draft from profile", "从配置创建组合草稿"), width),
		renderActionLine("e", m.t("edit config file", "编辑配置文件"), width),
		renderActionLine("g", m.t("reload config from disk", "从磁盘重新加载配置"), width),
		renderActionLine("d", m.t("delete profile", "删除配置"), width),
	}
}

func (m Model) stackActionLines(view app.StackView, width int) []string {
	toggleLabel := m.t("start stack", "启动组合")
	if view.Status == app.StackStatusRunning {
		toggleLabel = m.t("stop stack", "停止组合")
	}

	return []string{
		renderActionLine("Enter", toggleLabel, width),
		renderActionLine("r", m.t("restart stack", "重启组合"), width),
		renderActionLine("c", m.t("clone stack draft", "克隆组合草稿"), width),
		renderActionLine("A", m.t("create another stack draft", "再创建一个组合草稿"), width),
		renderActionLine("e", m.t("edit config file", "编辑配置文件"), width),
		renderActionLine("g", m.t("reload config from disk", "从磁盘重新加载配置"), width),
		renderActionLine("d", m.t("delete stack", "删除组合"), width),
	}
}

func starterSSHProfileDraft(cfg domain.Config, language domain.Language) domain.Profile {
	return domain.Profile{
		Name:        nextProfileDraftName(cfg, "draft-ssh"),
		Description: translate(language, "Starter SSH tunnel draft. Update the target before using it.", "SSH 隧道草稿模板。使用前请先改成你的目标地址。"),
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

func starterStackDraft(cfg domain.Config, profiles []app.ProfileView, stacks []app.StackView, focus listFocus, selectedProfile, selectedStack int, filterQuery string, language domain.Language) (domain.Stack, error) {
	members, sourceLabel, err := starterStackMembers(profiles, stacks, focus, selectedProfile, selectedStack, filterQuery, len(cfg.Profiles), language)
	if err != nil {
		return domain.Stack{}, err
	}

	stack := domain.Stack{
		Name:        nextStackDraftName(cfg, "draft-stack"),
		Description: translatef(language, "Starter stack draft seeded from %s.", "组合草稿模板，来源于 %s。", sourceLabel),
		Labels:      []string{"draft"},
		Profiles:    members,
	}
	return stack, nil
}

func starterStackMembers(profiles []app.ProfileView, stacks []app.StackView, focus listFocus, selectedProfile, selectedStack int, filterQuery string, totalProfiles int, language domain.Language) ([]string, string, error) {
	if focus == focusStacks && len(stacks) > 0 {
		selectedStack = max(0, min(selectedStack, len(stacks)-1))
		selected := stacks[selectedStack]
		members := make([]string, 0, len(selected.Members))
		for _, member := range selected.Members {
			members = append(members, member.Profile.Name)
		}
		if len(members) == 0 {
			return nil, "", fmt.Errorf("%s", translate(language, "Selected stack has no resolved members to draft from.", "当前选中的组合没有可用于生成草稿的已解析成员。"))
		}
		return members, translatef(language, "stack %s", "组合 %s", selected.Stack.Name), nil
	}

	if len(profiles) > 0 {
		selectedProfile = max(0, min(selectedProfile, len(profiles)-1))
		selected := profiles[selectedProfile]
		return []string{selected.Profile.Name}, translatef(language, "profile %s", "配置 %s", selected.Profile.Name), nil
	}

	switch {
	case totalProfiles == 0:
		return nil, "", fmt.Errorf("%s", translate(language, "Add a profile first, then press A to create a stack draft.", "请先添加一个配置，然后按 A 创建组合草稿。"))
	case filterQuery != "":
		return nil, "", fmt.Errorf("%s", translate(language, "No visible profile to seed the stack. Clear the filter or select a profile first.", "当前没有可用于生成组合的可见配置。请清除筛选，或先选中一个配置。"))
	default:
		return nil, "", fmt.Errorf("%s", translate(language, "Select a profile first, then press A to create a stack draft.", "请先选中一个配置，然后按 A 创建组合草稿。"))
	}
}

func cloneProfileDefinition(profile domain.Profile) domain.Profile {
	cloned := profile
	cloned.Labels = append([]string(nil), profile.Labels...)
	if profile.SSH != nil {
		sshCopy := *profile.SSH
		cloned.SSH = &sshCopy
	}
	if profile.SSHRemote != nil {
		sshRemoteCopy := *profile.SSHRemote
		cloned.SSHRemote = &sshRemoteCopy
	}
	if profile.SSHDynamic != nil {
		sshDynamicCopy := *profile.SSHDynamic
		cloned.SSHDynamic = &sshDynamicCopy
	}
	if profile.Kubernetes != nil {
		kubernetesCopy := *profile.Kubernetes
		cloned.Kubernetes = &kubernetesCopy
	}
	return cloned
}

func assignProfileDisplayPort(profile *domain.Profile, port int) {
	profile.LocalPort = port
	if profile.Type == domain.TunnelTypeSSHRemote && profile.SSHRemote != nil {
		profile.SSHRemote.BindPort = port
	}
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

func composeHighlightedLine(prefix, content, query string, width int) string {
	if width <= 0 {
		return prefix + renderHighlightedText(content, query, lipgloss.NewStyle(), filterMatchStyle)
	}
	if strings.TrimSpace(content) == "" {
		return prefix
	}

	remaining := width - lipgloss.Width(prefix)
	if remaining <= 0 {
		return prefix
	}

	return prefix + renderHighlightedText(truncateText(content, remaining), query, lipgloss.NewStyle(), filterMatchStyle)
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

func renderLogLine(timestamp time.Time, profileName string, source domain.LogSource, message string, query string, width int) string {
	prefixParts := []string{
		renderHighlightedText(timestamp.Format("15:04:05"), query, logTimestampStyle, filterMatchStyle),
		renderLogSourceBadge(source, query),
	}
	if profileName != "" {
		prefixParts = append(prefixParts, renderLogProfileBadge(profileName, query))
	}

	prefix := strings.Join(prefixParts, " ")
	normalized := normalizeLogMessage(message)
	if width <= 0 {
		return prefix + " " + renderHighlightedText(normalized, query, logMessageStyle(source), filterMatchStyle)
	}

	remaining := width - lipgloss.Width(prefix) - 1
	if remaining <= 0 {
		return prefix
	}

	return prefix + " " + renderHighlightedText(truncateText(normalized, remaining), query, logMessageStyle(source), filterMatchStyle)
}

func renderSizedBlock(style lipgloss.Style, width int, body string) string {
	if width <= 0 {
		return style.Render(body)
	}

	innerWidth := max(1, width-style.GetHorizontalFrameSize())
	return style.Render(lipgloss.NewStyle().Width(innerWidth).Render(body))
}

func renderStatusBadge(language domain.Language, status domain.TunnelStatus) string {
	label := translate(language, "STOP", "停止")
	background := lipgloss.Color("240")

	switch status {
	case domain.TunnelStatusRunning:
		label = translate(language, "RUN", "运行")
		background = lipgloss.Color("29")
	case domain.TunnelStatusStarting:
		label = translate(language, "START", "启动")
		background = lipgloss.Color("31")
	case domain.TunnelStatusRestarting:
		label = translate(language, "RETRY", "重试")
		background = lipgloss.Color("136")
	case domain.TunnelStatusFailed:
		label = translate(language, "FAIL", "失败")
		background = lipgloss.Color("124")
	case domain.TunnelStatusExited:
		label = translate(language, "EXIT", "退出")
		background = lipgloss.Color("239")
	}

	return renderStateBadge(label, background)
}

func renderStackStatusBadge(language domain.Language, status app.StackStatus) string {
	label := translate(language, "STOP", "停止")
	background := lipgloss.Color("240")

	switch status {
	case app.StackStatusRunning:
		label = translate(language, "RUN", "运行")
		background = lipgloss.Color("29")
	case app.StackStatusPartial:
		label = translate(language, "PART", "部分")
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

func renderLogSourceBadge(source domain.LogSource, query string) string {
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

	if logSourceMatchesQuery(source, query) {
		style = logMatchBadgeStyle
	}

	return style.Render(label)
}

func renderLogProfileBadge(profileName, query string) string {
	style := logProfileBadgeStyle
	query = normalizeFilterQuery(query)
	if query != "" && strings.Contains(strings.ToLower(profileName), query) {
		style = logMatchBadgeStyle
	}
	return style.Render(profileName)
}

func logMessageStyle(source domain.LogSource) lipgloss.Style {
	switch source {
	case domain.LogSourceStderr:
		return logStderrMessageStyle
	case domain.LogSourceStdout:
		return logStdoutMessageStyle
	default:
		return logSystemMessageStyle
	}
}

func normalizeLogMessage(message string) string {
	message = strings.ReplaceAll(message, "\r\n", "\n")
	message = strings.ReplaceAll(message, "\r", "\n")

	lines := strings.Split(message, "\n")
	segments := make([]string, 0, len(lines))
	for _, line := range lines {
		segment := strings.Join(strings.Fields(line), " ")
		if segment == "" {
			continue
		}
		segments = append(segments, segment)
	}

	if len(segments) == 0 {
		return "(empty)"
	}

	return strings.Join(segments, " | ")
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

func filterLogEntries(entries []domain.LogEntry, query string) []domain.LogEntry {
	query = normalizeFilterQuery(query)
	if query == "" {
		return entries
	}

	filtered := make([]domain.LogEntry, 0, len(entries))
	for _, entry := range entries {
		if !logEntryMatchesFilter("", entry, query) {
			continue
		}
		filtered = append(filtered, entry)
	}

	return filtered
}

func filterStackActivity(entries []stackActivityEntry, query string) []stackActivityEntry {
	query = normalizeFilterQuery(query)
	if query == "" {
		return entries
	}

	filtered := make([]stackActivityEntry, 0, len(entries))
	for _, entry := range entries {
		if !logEntryMatchesFilter(entry.ProfileName, entry.Log, query) {
			continue
		}
		filtered = append(filtered, entry)
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
		humanTunnelType(domain.LanguageEnglish, view.Profile.Type),
		profileTarget(domain.LanguageEnglish, view.Profile),
		fmt.Sprintf("%d", profileDisplayPort(view.Profile)),
	}
	parts = append(parts, view.Profile.Labels...)
	return strings.Join(parts, " ")
}

func stackSearchText(view app.StackView) string {
	parts := []string{
		view.Stack.Name,
		view.Stack.Description,
		string(view.Status),
		humanStackStatus(domain.LanguageEnglish, view.Status),
	}
	parts = append(parts, view.Stack.Labels...)
	parts = append(parts, view.Stack.Profiles...)

	for _, member := range view.Members {
		parts = append(parts,
			member.Profile.Name,
			member.Profile.Description,
			profileTarget(domain.LanguageEnglish, member.Profile),
			fmt.Sprintf("%d", profileDisplayPort(member.Profile)),
		)
		parts = append(parts, member.Profile.Labels...)
	}

	return strings.Join(parts, " ")
}

func logEntryMatchesFilter(profileName string, entry domain.LogEntry, query string) bool {
	return strings.Contains(strings.ToLower(logSearchText(profileName, entry)), query)
}

func logSearchText(profileName string, entry domain.LogEntry) string {
	parts := []string{
		profileName,
		entry.Timestamp.Format("15:04:05"),
		string(entry.Source),
		normalizeLogMessage(entry.Message),
	}

	switch entry.Source {
	case domain.LogSourceStdout:
		parts = append(parts, "out", "stdout")
	case domain.LogSourceStderr:
		parts = append(parts, "err", "stderr")
	default:
		parts = append(parts, "sys", "system")
	}

	return strings.Join(parts, " ")
}

func normalizeFilterQuery(query string) string {
	return strings.ToLower(strings.TrimSpace(query))
}

type matchRange struct {
	start int
	end   int
}

func renderHighlightedText(value, query string, baseStyle, matchStyle lipgloss.Style) string {
	matches := filterMatchRanges(value, query)
	if len(matches) == 0 {
		return baseStyle.Render(value)
	}

	runes := []rune(value)
	var builder strings.Builder
	last := 0
	for _, match := range matches {
		if match.start > last {
			builder.WriteString(baseStyle.Render(string(runes[last:match.start])))
		}
		builder.WriteString(matchStyle.Render(string(runes[match.start:match.end])))
		last = match.end
	}
	if last < len(runes) {
		builder.WriteString(baseStyle.Render(string(runes[last:])))
	}

	return builder.String()
}

func filterMatchRanges(value, query string) []matchRange {
	query = normalizeFilterQuery(query)
	if query == "" {
		return nil
	}

	valueRunes := []rune(value)
	queryRunes := []rune(query)
	if len(queryRunes) == 0 || len(queryRunes) > len(valueRunes) {
		return nil
	}

	matches := make([]matchRange, 0, 2)
	for start := 0; start <= len(valueRunes)-len(queryRunes); {
		end := start + len(queryRunes)
		if strings.EqualFold(string(valueRunes[start:end]), query) {
			matches = append(matches, matchRange{start: start, end: end})
			start = end
			continue
		}
		start++
	}

	return matches
}

func logSourceMatchesQuery(source domain.LogSource, query string) bool {
	query = normalizeFilterQuery(query)
	if query == "" {
		return false
	}

	return strings.Contains(strings.ToLower(strings.Join(logSourceSearchTerms(source), " ")), query)
}

func logSourceSearchTerms(source domain.LogSource) []string {
	switch source {
	case domain.LogSourceStdout:
		return []string{"OUT", "out", "stdout"}
	case domain.LogSourceStderr:
		return []string{"ERR", "err", "stderr"}
	default:
		return []string{"SYS", "sys", "system"}
	}
}

func formatVisibleCount(visible, total int) string {
	if visible == total {
		return fmt.Sprintf("%d", total)
	}
	return fmt.Sprintf("%d/%d", visible, total)
}

func profileDeleteNotice(language domain.Language, result app.RemoveProfileResult) string {
	parts := []string{translatef(language, "Removed profile %s.", "已移除配置 %s。", result.Name)}
	if result.WasActive {
		parts = append(parts, translate(language, "Stopped the running tunnel first.", "已先停止正在运行的隧道。"))
	}
	if result.UpdatedStacks > 0 {
		impact := translatef(language, "Pruned %d stack references", "已裁剪 %d 处组合引用", result.UpdatedStacks)
		if result.RemovedStacks > 0 {
			impact += translatef(language, " and removed %d empty stacks", "，并移除 %d 个空组合", result.RemovedStacks)
		}
		if language == domain.LanguageSimplifiedChinese {
			parts = append(parts, impact+"。")
		} else {
			parts = append(parts, impact+".")
		}
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

type logSourceCounts struct {
	system int
	stdout int
	stderr int
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

func logSourceCountsForEntries(entries []domain.LogEntry) logSourceCounts {
	counts := logSourceCounts{}
	for _, entry := range entries {
		switch entry.Source {
		case domain.LogSourceStdout:
			counts.stdout++
		case domain.LogSourceStderr:
			counts.stderr++
		default:
			counts.system++
		}
	}
	return counts
}

func logSourceCountsForStackActivity(entries []stackActivityEntry) logSourceCounts {
	counts := logSourceCounts{}
	for _, entry := range entries {
		switch entry.Log.Source {
		case domain.LogSourceStdout:
			counts.stdout++
		case domain.LogSourceStderr:
			counts.stderr++
		default:
			counts.system++
		}
	}
	return counts
}

func uniqueStackActivityProfiles(entries []stackActivityEntry) int {
	names := make(map[string]struct{}, len(entries))
	for _, entry := range entries {
		if entry.ProfileName == "" {
			continue
		}
		names[entry.ProfileName] = struct{}{}
	}
	return len(names)
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
		return ""
	}

	return ansi.Truncate(value, limit, "…")
}

func renderedLineHeight(line string, width int) int {
	if width <= 0 {
		return 1
	}

	return max(1, (ansi.StringWidth(line)+width-1)/width)
}

func renderedLinesHeight(lines []string, width int) int {
	total := 0
	for _, line := range lines {
		total += renderedLineHeight(line, width)
	}
	return total
}
