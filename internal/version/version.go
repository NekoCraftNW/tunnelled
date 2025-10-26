package version

import (
	"fmt"
	"runtime/debug"
	"time"
)

var (
	// Version will be set at build time using -ldflags
	Version = "dev"
	// BuildDate will be set at build time using -ldflags
	BuildDate = "unknown"
)

// GetVersion returns the current version in yyyy.DDmm format
func GetVersion() string {
	if Version != "dev" {
		return Version
	}

	// Generate version from current date if not set at build time
	now := time.Now()
	return fmt.Sprintf("%d.%02d%02d", now.Year(), now.Day(), int(now.Month()))
}

// GetBuildInfo returns formatted build information
func GetBuildInfo() string {
	return fmt.Sprintf("tunnelled v%s (built: %s)", GetVersion(), BuildDate)
}

// PrintVersion prints version information to stdout
func PrintVersion() {
	fmt.Println(GetBuildInfo())
}

func PrintGnet() {
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, dep := range info.Deps {
			if dep.Path == "github.com/panjf2000/gnet" || dep.Path == "github.com/panjf2000/gnet/v2" {
				fmt.Printf("Using gnet %s\n", dep.Version)
			}
		}
	} else {
		fmt.Printf("Could not read build info\n")
	}
}
