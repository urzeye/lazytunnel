package buildinfo

import "testing"

func TestDisplayVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		commit  string
		dirty   string
		want    string
	}{
		{name: "default", version: "dev", commit: "none", dirty: "false", want: "dev"},
		{name: "dev with commit", version: "dev", commit: "8b11454", dirty: "false", want: "dev (8b11454)"},
		{name: "dev dirty", version: "dev", commit: "8b11454", dirty: "true", want: "dev (8b11454, dirty)"},
		{name: "tagged release", version: "v0.1.0", commit: "8b11454", dirty: "false", want: "v0.1.0"},
		{name: "tagged dirty", version: "v0.1.0", commit: "8b11454", dirty: "true", want: "v0.1.0 (dirty)"},
		{name: "commit fallback", version: "99defd8-dirty", commit: "99defd8", dirty: "false", want: "dev (99defd8, dirty)"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previousVersion, previousCommit, previousDirty := Version, Commit, Dirty
			Version, Commit, Dirty = tt.version, tt.commit, tt.dirty
			t.Cleanup(func() {
				Version, Commit, Dirty = previousVersion, previousCommit, previousDirty
			})

			if got := DisplayVersion(); got != tt.want {
				t.Fatalf("DisplayVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestShortVersion(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		version string
		want    string
	}{
		{name: "default", version: "dev", want: "dev"},
		{name: "release", version: "v0.1.0", want: "v0.1.0"},
		{name: "commit fallback", version: "99defd8-dirty", want: "dev"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			previousVersion := Version
			Version = tt.version
			t.Cleanup(func() {
				Version = previousVersion
			})

			if got := ShortVersion(); got != tt.want {
				t.Fatalf("ShortVersion() = %q, want %q", got, tt.want)
			}
		})
	}
}
