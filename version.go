package main

import (
	"runtime"
	"runtime/debug"
)

// appVersion is injected at link time by the build scripts:
//
//	-ldflags "-X main.appVersion=$(git describe --tags | sed -e 's/-[0-9]*-g/-/')"
//
// It stays empty under `go run .` and under a plain `go build`, where the
// fallback to the VCS-derived Main.Version takes over. The injection exists
// because that fallback is a pseudo-version whenever HEAD is not an exact,
// clean tag (`v0.0.9-0.20260808044334-888df4e7986a+dirty`) and "(devel)" when
// there is no VCS stamp at all.
var appVersion string

const (
	// appName is the window title and the About tab's heading. Both read from
	// this constant so they cannot drift apart.
	appName = "Go HTTP File Server GUI"

	modGhfs = "mjpclab.dev/ghfs"
	modTk   = "modernc.org/tk9.0"

	unknownVersion = "unknown"
)

// aboutInfo is everything the About tab displays.
type aboutInfo struct {
	app      string
	ghfs     string
	tk       string
	goVer    string
	platform string
}

func collectAbout() aboutInfo {
	bi, ok := debug.ReadBuildInfo()
	return aboutFrom(bi, ok)
}

// aboutFrom is the pure half of collectAbout, kept separate so the resolution
// rules can be tested without a real build.
func aboutFrom(bi *debug.BuildInfo, ok bool) aboutInfo {
	info := aboutInfo{
		app:   appVersion,
		ghfs:  unknownVersion,
		tk:    unknownVersion,
		goVer: unknownVersion,
		// Read from runtime rather than from bi.Settings, so this line stays
		// correct even when there is no build info to read.
		platform: runtime.GOOS + "/" + runtime.GOARCH,
	}

	if ok && bi != nil {
		if info.app == "" {
			info.app = bi.Main.Version
		}
		if bi.GoVersion != "" {
			info.goVer = bi.GoVersion
		}
		for _, dep := range bi.Deps {
			switch dep.Path {
			case modGhfs:
				info.ghfs = dep.Version
			case modTk:
				info.tk = dep.Version
			}
		}
	}

	if info.app == "" {
		info.app = unknownVersion
	}
	return info
}
