package tui

import (
	"fmt"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"

	"github.com/urzeye/lazytunnel/internal/domain"
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
)

type Model struct {
	configPath string
	config     domain.Config
	width      int
	height     int
}

func NewModel(config domain.Config, configPath string) Model {
	return Model{
		config:     config,
		configPath: configPath,
	}
}

func (m Model) Init() tea.Cmd {
	return nil
}

func (m Model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height

	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return m, tea.Quit
		}
	}

	return m, nil
}

func (m Model) View() string {
	sections := []string{
		titleStyle.Render("LazyTunnel"),
		mutedStyle.Render("Bootstrap mode: Go project initialized, config model ready, placeholder TUI online."),
		cardStyle.Render(strings.Join([]string{
			fmt.Sprintf("Config: %s", m.configPath),
			fmt.Sprintf("Profiles: %d", len(m.config.Profiles)),
			fmt.Sprintf("Stacks: %d", len(m.config.Stacks)),
		}, "\n")),
		sectionStyle.Render("Supported in v0.1"),
		"- SSH local forwards (`ssh -L`)\n- Kubernetes port-forwards (`kubectl port-forward`)",
		sectionStyle.Render("Next Focus"),
		"- Process supervisor\n- SSH adapter\n- Kubernetes adapter\n- Real runtime state in the UI",
		sectionStyle.Render("Hints"),
		"Run `just init-config` to copy the example config.\nPress `q` to quit.",
	}

	if len(m.config.Profiles) > 0 {
		var names []string
		for _, profile := range m.config.Profiles {
			names = append(names, fmt.Sprintf("- %s (%s)", profile.Name, profile.Type))
		}

		sections = append(sections,
			sectionStyle.Render("Profiles"),
			strings.Join(names, "\n"),
		)
	}

	view := strings.Join(sections, "\n")
	return appStyle.Width(max(m.width, 80)).Render(view)
}
