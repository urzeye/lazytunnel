package buildinfo

import (
	"fmt"
	"runtime"
	"strings"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
	Dirty   = "false"
)

func Details() string {
	return fmt.Sprintf(
		"version: %s\ncommit: %s\nbuilt: %s\ngo: %s\nos/arch: %s/%s\n",
		DisplayVersion(),
		CommitValue(),
		Date,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)
}

func ShortVersion() string {
	version := strings.TrimSpace(Version)
	if version == "" {
		return "dev"
	}
	if isCommitVersion(version) {
		return "dev"
	}
	return version
}

func DisplayVersion() string {
	version := strings.TrimSpace(Version)
	dirty := IsDirty()

	switch {
	case version == "" || version == "dev":
		return formatDevVersion(dirty)
	case isCommitVersion(version):
		return formatDevVersion(dirty || strings.HasSuffix(version, "-dirty"))
	case dirty:
		return fmt.Sprintf("%s (dirty)", version)
	default:
		return version
	}
}

func CommitValue() string {
	commit := strings.TrimSpace(Commit)
	if commit == "" {
		return "none"
	}
	return commit
}

func IsDirty() bool {
	value := strings.TrimSpace(strings.ToLower(Dirty))
	return value == "1" || value == "true" || value == "yes"
}

func formatDevVersion(dirty bool) string {
	commit := shortCommit()
	if commit == "" {
		if dirty {
			return "dev (dirty)"
		}
		return "dev"
	}

	if dirty {
		return fmt.Sprintf("dev (%s, dirty)", commit)
	}

	return fmt.Sprintf("dev (%s)", commit)
}

func shortCommit() string {
	commit := CommitValue()
	if commit == "none" {
		return ""
	}
	if len(commit) > 12 {
		return commit[:12]
	}
	return commit
}

func isCommitVersion(version string) bool {
	trimmed := strings.TrimSuffix(version, "-dirty")
	if len(trimmed) < 7 || len(trimmed) > 40 {
		return false
	}

	for _, r := range trimmed {
		switch {
		case r >= '0' && r <= '9':
		case r >= 'a' && r <= 'f':
		case r >= 'A' && r <= 'F':
		default:
			return false
		}
	}

	return true
}
