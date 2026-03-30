package cli

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"

	"github.com/urzeye/lazytunnel/internal/domain"
)

func runInteractiveProfileEdit(in io.Reader, out io.Writer, profile domain.Profile) (domain.Profile, error) {
	reader := bufio.NewReader(in)

	_, _ = fmt.Fprintln(out, "interactive profile editor")
	_, _ = fmt.Fprintln(out, "press Enter to keep the current value; type - to clear optional text fields")

	var err error
	if profile.Name, err = promptText(reader, out, "Name", profile.Name, false); err != nil {
		return domain.Profile{}, err
	}
	if profile.Description, err = promptText(reader, out, "Description", profile.Description, true); err != nil {
		return domain.Profile{}, err
	}

	typeValue, err := promptChoice(
		reader,
		out,
		"Tunnel Type",
		string(domain.PrepareProfileForType(profile, profile.Type).Type),
		[]string{
			string(domain.TunnelTypeSSHLocal),
			string(domain.TunnelTypeSSHRemote),
			string(domain.TunnelTypeSSHDynamic),
			string(domain.TunnelTypeKubernetesPortForward),
		},
	)
	if err != nil {
		return domain.Profile{}, err
	}
	profile = domain.PrepareProfileForType(profile, domain.TunnelType(typeValue))

	labels, err := promptText(reader, out, "Labels (comma-separated)", strings.Join(profile.Labels, ", "), true)
	if err != nil {
		return domain.Profile{}, err
	}
	profile.Labels = cleanList(splitInteractiveList(labels))

	switch profile.Type {
	case domain.TunnelTypeSSHRemote:
		if profile.SSHRemote.Host, err = promptText(reader, out, "SSH Host", profile.SSHRemote.Host, false); err != nil {
			return domain.Profile{}, err
		}
		if profile.SSHRemote.BindAddress, err = promptText(reader, out, "Bind Address", profile.SSHRemote.BindAddress, true); err != nil {
			return domain.Profile{}, err
		}
		if profile.SSHRemote.BindPort, err = promptInt(reader, out, "Bind Port", profile.SSHRemote.BindPort); err != nil {
			return domain.Profile{}, err
		}
		if profile.SSHRemote.TargetHost, err = promptText(reader, out, "Target Host", profile.SSHRemote.TargetHost, false); err != nil {
			return domain.Profile{}, err
		}
		if profile.SSHRemote.TargetPort, err = promptInt(reader, out, "Target Port", profile.SSHRemote.TargetPort); err != nil {
			return domain.Profile{}, err
		}
		profile.LocalPort = profile.SSHRemote.BindPort
	case domain.TunnelTypeSSHDynamic:
		if profile.SSHDynamic.Host, err = promptText(reader, out, "SSH Host", profile.SSHDynamic.Host, false); err != nil {
			return domain.Profile{}, err
		}
		if profile.SSHDynamic.BindAddress, err = promptText(reader, out, "Bind Address", profile.SSHDynamic.BindAddress, true); err != nil {
			return domain.Profile{}, err
		}
		if profile.LocalPort, err = promptInt(reader, out, "Local Port", profile.LocalPort); err != nil {
			return domain.Profile{}, err
		}
	case domain.TunnelTypeKubernetesPortForward:
		if profile.Kubernetes.Context, err = promptText(reader, out, "Context", profile.Kubernetes.Context, true); err != nil {
			return domain.Profile{}, err
		}
		if profile.Kubernetes.Namespace, err = promptText(reader, out, "Namespace", profile.Kubernetes.Namespace, true); err != nil {
			return domain.Profile{}, err
		}
		resourceType, err := promptChoice(reader, out, "Resource Type", profile.Kubernetes.ResourceType, []string{"pod", "service", "deployment"})
		if err != nil {
			return domain.Profile{}, err
		}
		profile.Kubernetes.ResourceType = resourceType
		if profile.Kubernetes.Resource, err = promptText(reader, out, "Resource", profile.Kubernetes.Resource, false); err != nil {
			return domain.Profile{}, err
		}
		if profile.Kubernetes.RemotePort, err = promptInt(reader, out, "Remote Port", profile.Kubernetes.RemotePort); err != nil {
			return domain.Profile{}, err
		}
		if profile.LocalPort, err = promptInt(reader, out, "Local Port", profile.LocalPort); err != nil {
			return domain.Profile{}, err
		}
	default:
		if profile.SSH.Host, err = promptText(reader, out, "SSH Host", profile.SSH.Host, false); err != nil {
			return domain.Profile{}, err
		}
		if profile.SSH.RemoteHost, err = promptText(reader, out, "Remote Host", profile.SSH.RemoteHost, false); err != nil {
			return domain.Profile{}, err
		}
		if profile.SSH.RemotePort, err = promptInt(reader, out, "Remote Port", profile.SSH.RemotePort); err != nil {
			return domain.Profile{}, err
		}
		if profile.LocalPort, err = promptInt(reader, out, "Local Port", profile.LocalPort); err != nil {
			return domain.Profile{}, err
		}
	}

	if profile.Restart.Enabled, err = promptBool(reader, out, "Restart Enabled", profile.Restart.Enabled); err != nil {
		return domain.Profile{}, err
	}
	if profile.Restart.MaxRetries, err = promptInt(reader, out, "Max Retries", profile.Restart.MaxRetries); err != nil {
		return domain.Profile{}, err
	}
	if profile.Restart.InitialBackoff, err = promptText(reader, out, "Initial Backoff", profile.Restart.InitialBackoff, true); err != nil {
		return domain.Profile{}, err
	}
	if profile.Restart.MaxBackoff, err = promptText(reader, out, "Max Backoff", profile.Restart.MaxBackoff, true); err != nil {
		return domain.Profile{}, err
	}

	return profile, nil
}

func runInteractiveStackEdit(in io.Reader, out io.Writer, stack domain.Stack) (domain.Stack, error) {
	reader := bufio.NewReader(in)

	_, _ = fmt.Fprintln(out, "interactive stack editor")
	_, _ = fmt.Fprintln(out, "press Enter to keep the current value; type - to clear optional text fields")

	var err error
	if stack.Name, err = promptText(reader, out, "Name", stack.Name, false); err != nil {
		return domain.Stack{}, err
	}
	if stack.Description, err = promptText(reader, out, "Description", stack.Description, true); err != nil {
		return domain.Stack{}, err
	}
	labels, err := promptText(reader, out, "Labels (comma-separated)", strings.Join(stack.Labels, ", "), true)
	if err != nil {
		return domain.Stack{}, err
	}
	stack.Labels = cleanList(splitInteractiveList(labels))

	profiles, err := promptText(reader, out, "Profiles (comma-separated)", strings.Join(stack.Profiles, ", "), false)
	if err != nil {
		return domain.Stack{}, err
	}
	stack.Profiles = cleanList(splitInteractiveList(profiles))

	return stack, nil
}

func promptText(reader *bufio.Reader, out io.Writer, label, current string, allowClear bool) (string, error) {
	for {
		if _, err := fmt.Fprintf(out, "%s [%s]: ", label, promptCurrentValue(current)); err != nil {
			return "", err
		}

		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}

		line = strings.TrimSpace(line)
		switch {
		case line == "":
			return current, nil
		case allowClear && line == "-":
			return "", nil
		default:
			return line, nil
		}
	}
}

func promptChoice(reader *bufio.Reader, out io.Writer, label, current string, options []string) (string, error) {
	joined := strings.Join(options, "/")
	for {
		if _, err := fmt.Fprintf(out, "%s [%s] (%s): ", label, promptCurrentValue(current), joined); err != nil {
			return "", err
		}

		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return "", err
		}

		line = strings.TrimSpace(strings.ToLower(line))
		if line == "" {
			return current, nil
		}

		for _, option := range options {
			if strings.ToLower(option) == line {
				return option, nil
			}
		}

		if _, err := fmt.Fprintf(out, "invalid choice %q; choose one of %s\n", line, joined); err != nil {
			return "", err
		}
	}
}

func promptBool(reader *bufio.Reader, out io.Writer, label string, current bool) (bool, error) {
	defaultValue := "false"
	if current {
		defaultValue = "true"
	}

	for {
		if _, err := fmt.Fprintf(out, "%s [%s] (true/false): ", label, defaultValue); err != nil {
			return false, err
		}

		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return false, err
		}

		line = strings.TrimSpace(strings.ToLower(line))
		switch line {
		case "":
			return current, nil
		case "true", "t", "yes", "y":
			return true, nil
		case "false", "f", "no", "n":
			return false, nil
		default:
			if _, err := fmt.Fprintf(out, "invalid boolean %q; use true/false\n", line); err != nil {
				return false, err
			}
		}
	}
}

func promptInt(reader *bufio.Reader, out io.Writer, label string, current int) (int, error) {
	for {
		if _, err := fmt.Fprintf(out, "%s [%d]: ", label, current); err != nil {
			return 0, err
		}

		line, err := reader.ReadString('\n')
		if err != nil && err != io.EOF {
			return 0, err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			return current, nil
		}

		value, err := strconv.Atoi(line)
		if err == nil {
			return value, nil
		}

		if _, err := fmt.Fprintf(out, "invalid number %q; use digits only\n", line); err != nil {
			return 0, err
		}
	}
}

func promptCurrentValue(value string) string {
	if strings.TrimSpace(value) == "" {
		return "empty"
	}
	return value
}

func splitInteractiveList(value string) []string {
	return strings.FieldsFunc(value, func(r rune) bool {
		return r == ',' || r == '\n'
	})
}
