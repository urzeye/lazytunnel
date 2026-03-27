package tui

import (
	"fmt"
	"strings"

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
	sectionStyle = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("212")).
			MarginTop(1)
	focusedSectionStyle = lipgloss.NewStyle().
				Bold(true).
				Foreground(lipgloss.Color("230")).
				Background(lipgloss.Color("62")).
				Padding(0, 1).
				MarginTop(1)
	mutedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("244"))
	cardStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("240")).
			Padding(0, 1)
	selectedStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("230")).
			Background(lipgloss.Color("62")).
			Bold(true)
	errorStyle = lipgloss.NewStyle().
			Foreground(lipgloss.Color("203"))
)

type runtimeEventMsg ltruntime.Event

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
	lastError       string
}

func NewModel(service *app.Service, configPath string) Model {
	subscriptionID, events := service.Subscribe(64)

	return Model{
		service:        service,
		configPath:     configPath,
		subscriptionID: subscriptionID,
		events:         events,
	}
}

func (m Model) Init() tea.Cmd {
	return waitForRuntimeEvent(m.events)
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case runtimeEventMsg:
		return m, waitForRuntimeEvent(m.events)

	case tea.KeyMsg:
		profiles := m.profileViews()
		stacks := m.stackViews()

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
	profiles := m.profileViews()
	stacks := m.stackViews()

	sections := []string{
		titleStyle.Render("LazyTunnel"),
		mutedStyle.Render("Keyboard-first tunnel workspace. Use Tab to switch between profiles and stacks."),
		cardStyle.Render(strings.Join([]string{
			fmt.Sprintf("Config: %s", m.configPath),
			fmt.Sprintf("Profiles: %d", len(profiles)),
			fmt.Sprintf("Stacks: %d", len(stacks)),
			fmt.Sprintf("Active: %d", countActiveProfiles(profiles)),
		}, "\n")),
		renderSectionTitle("Profiles", m.focus == focusProfiles),
		m.renderProfiles(profiles),
		renderSectionTitle("Stacks", m.focus == focusStacks),
		m.renderStacks(stacks),
		renderSectionTitle("Details", false),
		m.renderDetails(profiles, stacks),
	}

	if m.lastError != "" {
		sections = append(sections, errorStyle.Render("Last error: "+m.lastError))
	}

	sections = append(sections,
		renderSectionTitle("Hints", false),
		"Tab or 1/2 to switch lists, ↑/↓ or j/k to move, Enter or s to start/stop the selected item, q to quit.",
	)

	view := strings.Join(sections, "\n")
	return appStyle.Width(max(m.width, 100)).Render(view)
}

func (m Model) profileViews() []app.ProfileView {
	views := m.service.ProfileViews()
	if len(views) == 0 {
		m.selectedProfile = 0
		return views
	}

	if m.selectedProfile >= len(views) {
		m.selectedProfile = len(views) - 1
	}
	if m.selectedProfile < 0 {
		m.selectedProfile = 0
	}

	return views
}

func (m Model) stackViews() []app.StackView {
	views := m.service.StackViews()
	if len(views) == 0 {
		m.selectedStack = 0
		if m.focus == focusStacks {
			m.focus = focusProfiles
		}
		return views
	}

	if m.selectedStack >= len(views) {
		m.selectedStack = len(views) - 1
	}
	if m.selectedStack < 0 {
		m.selectedStack = 0
	}

	return views
}

func (m Model) renderProfiles(views []app.ProfileView) string {
	if len(views) == 0 {
		return mutedStyle.Render("No profiles yet. Run `just init-config` to start from the example config.")
	}

	lines := make([]string, 0, len(views))
	for idx, view := range views {
		line := fmt.Sprintf("%s  %s  :%d", renderStatus(view.State.Status), view.Profile.Name, view.Profile.LocalPort)
		if idx == m.selectedProfile && m.focus == focusProfiles {
			line = selectedStyle.Render("> " + line)
		} else if idx == m.selectedProfile {
			line = "> " + line
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderStacks(views []app.StackView) string {
	if len(views) == 0 {
		return mutedStyle.Render("No stacks yet. Add stacks to your config to launch groups of tunnels together.")
	}

	lines := make([]string, 0, len(views))
	for idx, view := range views {
		line := fmt.Sprintf("%s  %s  %d/%d active", renderStackStatus(view.Status), view.Stack.Name, view.ActiveCount, len(view.Members))
		if idx == m.selectedStack && m.focus == focusStacks {
			line = selectedStyle.Render("> " + line)
		} else if idx == m.selectedStack {
			line = "> " + line
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderDetails(profiles []app.ProfileView, stacks []app.StackView) string {
	if m.focus == focusStacks && len(stacks) > 0 {
		return m.renderStackDetails(stacks[m.selectedStack])
	}

	if len(profiles) == 0 {
		return cardStyle.Render("No profile selected.")
	}

	return m.renderProfileDetails(profiles[m.selectedProfile])
}

func (m Model) renderProfileDetails(view app.ProfileView) string {
	spec, specErr := app.BuildProcessSpec(view.Profile)

	details := []string{
		fmt.Sprintf("Name: %s", view.Profile.Name),
		fmt.Sprintf("Type: %s", view.Profile.Type),
		fmt.Sprintf("Status: %s", view.State.Status),
		fmt.Sprintf("Local Port: %d", view.Profile.LocalPort),
	}

	if view.State.PID != 0 {
		details = append(details, fmt.Sprintf("PID: %d", view.State.PID))
	}
	if view.Profile.Description != "" {
		details = append(details, fmt.Sprintf("Description: %s", view.Profile.Description))
	}
	if len(view.Profile.Labels) > 0 {
		details = append(details, fmt.Sprintf("Labels: %s", strings.Join(view.Profile.Labels, ", ")))
	}
	if specErr == nil {
		details = append(details, fmt.Sprintf("Command: %s", spec.DisplayCommand()))
	}
	if view.State.ExitReason != "" {
		details = append(details, fmt.Sprintf("Exit: %s", view.State.ExitReason))
	}
	if view.State.LastError != "" {
		details = append(details, fmt.Sprintf("Error: %s", view.State.LastError))
	}

	logs := tailLogs(view.State.RecentLogs, 6)
	if len(logs) > 0 {
		details = append(details, "Recent Logs:")
		for _, log := range logs {
			details = append(details, fmt.Sprintf("  [%s] %s", log.Source, log.Message))
		}
	}

	return cardStyle.Render(strings.Join(details, "\n"))
}

func (m Model) renderStackDetails(view app.StackView) string {
	details := []string{
		fmt.Sprintf("Name: %s", view.Stack.Name),
		fmt.Sprintf("Status: %s", view.Status),
		fmt.Sprintf("Members: %d", len(view.Members)),
		fmt.Sprintf("Active: %d", view.ActiveCount),
	}

	if view.Stack.Description != "" {
		details = append(details, fmt.Sprintf("Description: %s", view.Stack.Description))
	}
	if len(view.Stack.Labels) > 0 {
		details = append(details, fmt.Sprintf("Labels: %s", strings.Join(view.Stack.Labels, ", ")))
	}

	if len(view.Members) > 0 {
		details = append(details, "Members:")
		for _, member := range view.Members {
			details = append(details, fmt.Sprintf("  %s  %s  :%d", renderStatus(member.State.Status), member.Profile.Name, member.Profile.LocalPort))
		}
	}

	if view.Status == app.StackStatusPartial {
		details = append(details, "Action: Enter will start the missing members.")
	}

	return cardStyle.Render(strings.Join(details, "\n"))
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

func renderSectionTitle(title string, focused bool) string {
	if focused {
		return focusedSectionStyle.Render(title)
	}

	return sectionStyle.Render(title)
}

func renderStatus(status domain.TunnelStatus) string {
	switch status {
	case domain.TunnelStatusRunning:
		return "RUN"
	case domain.TunnelStatusStarting:
		return "NEW"
	case domain.TunnelStatusRestarting:
		return "RET"
	case domain.TunnelStatusFailed:
		return "ERR"
	case domain.TunnelStatusExited:
		return "EXT"
	default:
		return "STP"
	}
}

func renderStackStatus(status app.StackStatus) string {
	switch status {
	case app.StackStatusRunning:
		return "RUN"
	case app.StackStatusPartial:
		return "PAR"
	default:
		return "STP"
	}
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
