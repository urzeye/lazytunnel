package buildinfo

import (
	"fmt"
	"runtime"
)

var (
	Version = "dev"
	Commit  = "none"
	Date    = "unknown"
)

func Details() string {
	return fmt.Sprintf(
		"version: %s\ncommit: %s\nbuilt: %s\ngo: %s\nos/arch: %s/%s\n",
		Version,
		Commit,
		Date,
		runtime.Version(),
		runtime.GOOS,
		runtime.GOARCH,
	)
}
