package tui

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

func writeClipboardText(text string) error {
	if strings.TrimSpace(text) == "" {
		return errors.New("nothing to copy")
	}

	switch runtime.GOOS {
	case "darwin":
		return runClipboardCommand("pbcopy", nil, text)
	case "windows":
		return runClipboardCommand("cmd", []string{"/c", "clip"}, text)
	default:
		candidates := []struct {
			name string
			args []string
		}{
			{name: "wl-copy"},
			{name: "xclip", args: []string{"-selection", "clipboard"}},
			{name: "xsel", args: []string{"--clipboard", "--input"}},
		}
		for _, candidate := range candidates {
			if err := runClipboardCommand(candidate.name, candidate.args, text); err == nil {
				return nil
			}
		}
		return errors.New("no clipboard command found (tried wl-copy, xclip, xsel)")
	}
}

func runClipboardCommand(name string, args []string, text string) error {
	path, err := exec.LookPath(name)
	if err != nil {
		return err
	}

	cmd := exec.Command(path, args...)
	cmd.Stdin = strings.NewReader(text)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if trimmed := strings.TrimSpace(string(output)); trimmed != "" {
			return fmt.Errorf("%s: %s", err, trimmed)
		}
		return err
	}
	return nil
}

func exportTextSnapshot(configPath, baseName, content string, now time.Time) (string, error) {
	if strings.TrimSpace(content) == "" {
		return "", errors.New("nothing to export")
	}

	exportDir, err := exportRootDir(configPath)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(exportDir, 0o755); err != nil {
		return "", fmt.Errorf("create export directory: %w", err)
	}

	filename := fmt.Sprintf("%s-%s.log", sanitizeExportName(baseName), now.Format("20060102-150405"))
	path := filepath.Join(exportDir, filename)
	if !strings.HasSuffix(content, "\n") {
		content += "\n"
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		return "", fmt.Errorf("write export file: %w", err)
	}
	return path, nil
}

func exportRootDir(configPath string) (string, error) {
	if strings.TrimSpace(configPath) != "" {
		return filepath.Join(filepath.Dir(configPath), "exports"), nil
	}

	cwd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("determine export directory: %w", err)
	}
	return filepath.Join(cwd, "exports"), nil
}

func sanitizeExportName(value string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	if value == "" {
		return "logs"
	}

	replacer := strings.NewReplacer(
		"/", "-",
		"\\", "-",
		":", "-",
		" ", "-",
		"\t", "-",
		"\n", "-",
	)
	value = replacer.Replace(value)
	for strings.Contains(value, "--") {
		value = strings.ReplaceAll(value, "--", "-")
	}
	value = strings.Trim(value, "-.")
	if value == "" {
		return "logs"
	}
	return value
}
