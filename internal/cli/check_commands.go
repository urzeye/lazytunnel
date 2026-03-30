package cli

import (
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/urzeye/lazytunnel/internal/app"
	"github.com/urzeye/lazytunnel/internal/domain"
	ltruntime "github.com/urzeye/lazytunnel/internal/runtime"
	"github.com/urzeye/lazytunnel/internal/storage"
)

func newProfileCheckCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "check <name>",
		Short: "Run start preflight checks for a profile",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, service, err := loadCheckService(*configPath)
			if err != nil {
				return err
			}

			name := strings.TrimSpace(args[0])
			profile, exists := findProfile(cfg.Profiles, name)
			if !exists {
				return fmt.Errorf("profile %q not found", name)
			}

			analysis, err := service.AnalyzeProfileStart(name)
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "profile %s\n", profile.Name)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "readiness: %s\n", cliProfileReadinessLabel(analysis.Status))
			renderProfileIssues(cmd, profile, analysis)
			return nil
		},
	}
}

func newStackCheckCommand(configPath *string) *cobra.Command {
	return &cobra.Command{
		Use:   "check <name>",
		Short: "Run start preflight checks for a stack",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, service, err := loadCheckService(*configPath)
			if err != nil {
				return err
			}

			name := strings.TrimSpace(args[0])
			stack, exists := findStack(cfg.Stacks, name)
			if !exists {
				return fmt.Errorf("stack %q not found", name)
			}

			analysis, err := service.AnalyzeStackStart(name)
			if err != nil {
				return err
			}

			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "stack %s\n", stack.Name)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "readiness: %s\n", cliStackReadinessLabel(stack, analysis))
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "ready: %d\n", analysis.ReadyCount)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "warnings: %d\n", analysis.WarningCount)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "running: %d\n", analysis.ActiveCount)
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "blocked: %d\n", analysis.BlockedCount)

			for _, member := range analysis.Members {
				_, _ = fmt.Fprintf(cmd.OutOrStdout(), "member %s: %s\n", member.ProfileName, cliProfileReadinessLabel(member.Status))
				profile, exists := findProfile(cfg.Profiles, member.ProfileName)
				if !exists {
					for _, problem := range member.Problems {
						_, _ = fmt.Fprintf(cmd.OutOrStdout(), "blocker: %s\n", problem)
					}
					continue
				}

				for _, warning := range member.Warnings {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "warning: %s\n", warning)
				}
				for _, problem := range member.Problems {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "blocker: %s\n", problem)
				}
				for _, hint := range collectProfileValidationHints(profile.Name, profile, strings.Join(append(append([]string(nil), member.Problems...), member.Warnings...), "\n")) {
					_, _ = fmt.Fprintf(cmd.OutOrStdout(), "hint: %s\n", hint)
				}
			}

			return nil
		},
	}
}

func loadCheckService(configPath string) (domain.Config, *app.Service, error) {
	cfg, err := storage.LoadConfig(configPath)
	if err != nil {
		return domain.Config{}, nil, fmt.Errorf("load config: %w", err)
	}

	service, err := app.NewService(
		cfg,
		ltruntime.NewSupervisor(ltruntime.ExecProcessFactory{}),
		app.WithSystemCommandChecks(),
		app.WithSystemProfileProbeChecks(),
	)
	if err != nil {
		return domain.Config{}, nil, fmt.Errorf("build app service: %w", err)
	}

	return cfg, service, nil
}

func renderProfileIssues(cmd *cobra.Command, profile domain.Profile, analysis app.ProfileStartAnalysis) {
	for _, warning := range analysis.Warnings {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "warning: %s\n", warning)
	}
	for _, problem := range analysis.Problems {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "blocker: %s\n", problem)
	}
	for _, hint := range collectProfileValidationHints(profile.Name, profile, strings.Join(append(append([]string(nil), analysis.Problems...), analysis.Warnings...), "\n")) {
		_, _ = fmt.Fprintf(cmd.OutOrStdout(), "hint: %s\n", hint)
	}
}

func cliProfileReadinessLabel(status app.StartReadiness) string {
	switch status {
	case app.StartReadinessReady:
		return "Ready"
	case app.StartReadinessWarning:
		return "Warning"
	case app.StartReadinessActive:
		return "Running"
	case app.StartReadinessBlocked:
		return "Blocked"
	default:
		return "-"
	}
}

func cliStackReadinessLabel(stack domain.Stack, analysis app.StackStartAnalysis) string {
	switch {
	case len(stack.Profiles) == 0:
		return "Blocked"
	case analysis.BlockedCount > 0:
		return "Blocked"
	case analysis.WarningCount > 0:
		return "Warning"
	case analysis.ReadyCount > 0:
		return "Ready"
	case analysis.ActiveCount > 0:
		return "Running"
	default:
		return "-"
	}
}
