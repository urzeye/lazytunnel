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

type Model struct {
	service        *app.Service
	configPath     string
	subscriptionID int
	events         <-chan ltruntime.Event
	selected       int
	width          int
	height         int
	lastError      string
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
		views := m.profileViews()
		switch msg.String() {
		case "q", "ctrl+c":
			m.service.Unsubscribe(m.subscriptionID)
			return m, tea.Quit
		case "j", "down":
			if len(views) > 0 && m.selected < len(views)-1 {
				m.selected++
			}
		case "k", "up":
			if len(views) > 0 && m.selected > 0 {
				m.selected--
			}
		case "enter", "s":
			if len(views) == 0 {
				break
			}
			m.lastError = ""
			if err := m.service.ToggleProfile(views[m.selected].Profile.Name); err != nil {
				m.lastError = err.Error()
			}
			return m, nil
		}
	}

	return m, nil
}

func (m Model) View() string {
	views := m.profileViews()

	sections := []string{
		titleStyle.Render("LazyTunnel"),
		mutedStyle.Render("Keyboard-first tunnel workspace. Press Enter to start or stop the selected profile."),
		cardStyle.Render(strings.Join([]string{
			fmt.Sprintf("Config: %s", m.configPath),
			fmt.Sprintf("Profiles: %d", len(views)),
			fmt.Sprintf("Active: %d", countActive(views)),
		}, "\n")),
		sectionStyle.Render("Profiles"),
		m.renderProfiles(views),
	}

	if len(views) > 0 {
		sections = append(sections,
			sectionStyle.Render("Details"),
			m.renderDetails(views[m.selected]),
		)
	}

	if m.lastError != "" {
		sections = append(sections, errorStyle.Render("Last error: "+m.lastError))
	}

	sections = append(sections,
		sectionStyle.Render("Hints"),
		"↑/↓ or j/k to move, Enter or s to start/stop, q to quit.",
	)

	view := strings.Join(sections, "\n")
	return appStyle.Width(max(m.width, 96)).Render(view)
}

func (m Model) profileViews() []app.ProfileView {
	views := m.service.ProfileViews()
	if m.selected >= len(views) && len(views) > 0 {
		m.selected = len(views) - 1
	}
	if m.selected < 0 {
		m.selected = 0
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
		if idx == m.selected {
			line = selectedStyle.Render("> " + line)
		} else {
			line = "  " + line
		}
		lines = append(lines, line)
	}

	return strings.Join(lines, "\n")
}

func (m Model) renderDetails(view app.ProfileView) string {
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

func waitForRuntimeEvent(events <-chan ltruntime.Event) tea.Cmd {
	return func() tea.Msg {
		event, ok := <-events
		if !ok {
			return nil
		}
		return runtimeEventMsg(event)
	}
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

func countActive(views []app.ProfileView) int {
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
