package cli

import (
	"fmt"
	"strings"

	"github.com/urzeye/lazytunnel/internal/domain"
)

func augmentProfileValidationError(profileName string, profile domain.Profile, err error) error {
	hints := collectProfileValidationHints(profileName, profile, err.Error())
	if len(hints) == 0 {
		return err
	}
	return fmt.Errorf("%w\nhint: %s", err, strings.Join(hints, "\nhint: "))
}

func augmentStackValidationError(stackName string, stack domain.Stack, err error) error {
	hints := collectStackValidationHints(stackName, stack, err.Error())
	if len(hints) == 0 {
		return err
	}
	return fmt.Errorf("%w\nhint: %s", err, strings.Join(hints, "\nhint: "))
}

func collectProfileValidationHints(profileName string, profile domain.Profile, problemText string) []string {
	interactiveCommand := fmt.Sprintf("lazytunnel profile edit %s --interactive", hintTargetName(profileName, profile.Name))
	seen := map[string]struct{}{}
	hints := make([]string, 0, 4)

	appendHint := func(hint string) {
		hint = strings.TrimSpace(hint)
		if hint == "" {
			return
		}
		if _, exists := seen[hint]; exists {
			return
		}
		seen[hint] = struct{}{}
		hints = append(hints, hint)
	}

	for _, problem := range splitProblemLines(problemText) {
		switch {
		case strings.Contains(problem, "name is required"):
			appendHint(fmt.Sprintf("set a non-empty name with --name <value>, or rerun %q", interactiveCommand))
		case strings.Contains(problem, "ssh settings are required"):
			appendHint(fmt.Sprintf("rerun %q and fill SSH Host, Remote Host, Remote Port, and Local Port", interactiveCommand))
			appendHint("or pass --host <host> --remote-host <host> --remote-port <port> --local-port <port>")
		case strings.Contains(problem, "ssh_remote settings are required"):
			appendHint(fmt.Sprintf("rerun %q and fill SSH Host, Bind Port, Target Host, and Target Port", interactiveCommand))
			appendHint("or pass --host <host> --bind-port <port> --target-host <host> --target-port <port>")
		case strings.Contains(problem, "ssh_dynamic settings are required"):
			appendHint(fmt.Sprintf("rerun %q and fill SSH Host and Local Port", interactiveCommand))
			appendHint("or pass --host <host> --local-port <port>")
		case strings.Contains(problem, "kubernetes settings are required"):
			appendHint(fmt.Sprintf("rerun %q and fill Resource Type, Resource, Remote Port, and Local Port", interactiveCommand))
			appendHint("or pass --resource-type <pod|service|deployment> --resource <name> --remote-port <port> --local-port <port>")
		case strings.Contains(problem, "remote_host is required"):
			appendHint("set --remote-host <host> or use --interactive to fill Remote Host")
		case strings.Contains(problem, "target_host is required"):
			appendHint("set --target-host <host> or use --interactive to fill Target Host")
		case strings.Contains(problem, "host is required"):
			appendHint("set --host <host> or use --interactive to fill SSH Host")
		case strings.Contains(problem, "remote_port must be between"):
			appendHint("set --remote-port to a value between 1 and 65535")
		case strings.Contains(problem, "target_port must be between"):
			appendHint("set --target-port to a value between 1 and 65535")
		case strings.Contains(problem, "bind_port must be between"):
			appendHint("set --bind-port to a value between 1 and 65535")
		case strings.Contains(problem, "local_port must be between"):
			appendHint("set --local-port to a value between 1 and 65535")
		case strings.Contains(problem, "resource_type must be one of"):
			appendHint("set --resource-type to pod, service, or deployment")
		case strings.Contains(problem, "resource is required"):
			appendHint("set --resource <name> or use --interactive to fill Resource")
		case strings.Contains(problem, "max_retries must be greater than or equal to 0"):
			appendHint("set --max-retries to 0 or a positive integer")
		case strings.Contains(problem, "invalid initial_backoff"):
			appendHint("set --initial-backoff to a valid duration like 2s or 500ms")
		case strings.Contains(problem, "invalid max_backoff"):
			appendHint("set --max-backoff to a valid duration like 30s or 5m")
		case strings.Contains(problem, "already exists"):
			appendHint("choose another --name value, or rerun with --interactive to rename it safely")
		case strings.Contains(problem, "unsupported tunnel type"):
			appendHint(fmt.Sprintf("rerun %q to switch tunnel types interactively", interactiveCommand))
		default:
			if hasLabel(profile.Labels, "draft") {
				appendHint(fmt.Sprintf("rerun %q to finish the imported draft fields", interactiveCommand))
			}
		}
	}

	return hints
}

func collectStackValidationHints(stackName string, stack domain.Stack, problemText string) []string {
	interactiveCommand := fmt.Sprintf("lazytunnel stack edit %s --interactive", hintTargetName(stackName, stack.Name))
	seen := map[string]struct{}{}
	hints := make([]string, 0, 4)

	appendHint := func(hint string) {
		hint = strings.TrimSpace(hint)
		if hint == "" {
			return
		}
		if _, exists := seen[hint]; exists {
			return
		}
		seen[hint] = struct{}{}
		hints = append(hints, hint)
	}

	for _, problem := range splitProblemLines(problemText) {
		switch {
		case strings.Contains(problem, "name is required"):
			appendHint(fmt.Sprintf("set a non-empty name with --name <value>, or rerun %q", interactiveCommand))
		case strings.Contains(problem, "profiles must include at least one profile name"):
			appendHint("set at least one --profile <name>, or rerun with --interactive to edit the member list")
		case strings.Contains(problem, "references unknown profile"):
			appendHint("replace the --profile list with existing profile names, or run lazytunnel profile list to inspect what is available")
		case strings.Contains(problem, "already exists"):
			appendHint("choose another --name value, or rerun with --interactive to rename it safely")
		default:
			if hasLabel(stack.Labels, "draft") {
				appendHint(fmt.Sprintf("rerun %q to finish the draft stack details", interactiveCommand))
			}
		}
	}

	return hints
}

func splitProblemLines(value string) []string {
	lines := strings.Split(value, "\n")
	trimmed := make([]string, 0, len(lines))
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		trimmed = append(trimmed, line)
	}
	return trimmed
}

func hintTargetName(fallback, current string) string {
	if strings.TrimSpace(current) != "" {
		return current
	}
	return fallback
}

func hasLabel(labels []string, label string) bool {
	for _, existing := range labels {
		if existing == label {
			return true
		}
	}
	return false
}
