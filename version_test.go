package main

import (
	"runtime"
	"runtime/debug"
	"testing"
)

// setAppVersion overrides the ldflags-injected variable for one test.
func setAppVersion(t *testing.T, v string) {
	t.Helper()
	prev := appVersion
	appVersion = v
	t.Cleanup(func() { appVersion = prev })
}

func fullBuildInfo() *debug.BuildInfo {
	return &debug.BuildInfo{
		GoVersion: "go1.25.0",
		Main:      debug.Module{Path: "mjpclab.dev/ghfs-gui", Version: "(devel)"},
		Deps: []*debug.Module{
			{Path: "github.com/adrg/xdg", Version: "v0.5.3"},
			{Path: modGhfs, Version: "v1.21.7"},
			{Path: modTk, Version: "v1.76.1"},
		},
	}
}

func TestAboutFrom(t *testing.T) {
	platform := runtime.GOOS + "/" + runtime.GOARCH

	cases := []struct {
		name     string
		injected string
		bi       *debug.BuildInfo
		ok       bool
		want     aboutInfo
	}{
		{
			name:     "injected version wins over build info",
			injected: "v0.0.9-888df4e",
			bi:       fullBuildInfo(),
			ok:       true,
			want: aboutInfo{
				app: "v0.0.9-888df4e", ghfs: "v1.21.7", tk: "v1.76.1",
				goVer: "go1.25.0", platform: platform,
			},
		},
		{
			name:     "falls back to the VCS-derived main version",
			injected: "",
			bi:       fullBuildInfo(),
			ok:       true,
			want: aboutInfo{
				app: "(devel)", ghfs: "v1.21.7", tk: "v1.76.1",
				goVer: "go1.25.0", platform: platform,
			},
		},
		{
			name:     "dependencies absent from the module graph",
			injected: "v0.0.9",
			bi: &debug.BuildInfo{
				GoVersion: "go1.25.0",
				Main:      debug.Module{Version: "v0.0.9"},
			},
			ok: true,
			want: aboutInfo{
				app: "v0.0.9", ghfs: unknownVersion, tk: unknownVersion,
				goVer: "go1.25.0", platform: platform,
			},
		},
		{
			name:     "no build info at all, nothing injected",
			injected: "",
			bi:       nil,
			ok:       false,
			want: aboutInfo{
				app: unknownVersion, ghfs: unknownVersion, tk: unknownVersion,
				goVer: unknownVersion, platform: platform,
			},
		},
		{
			name:     "no build info but a version was injected",
			injected: "v0.0.9",
			bi:       nil,
			ok:       false,
			want: aboutInfo{
				app: "v0.0.9", ghfs: unknownVersion, tk: unknownVersion,
				goVer: unknownVersion, platform: platform,
			},
		},
		{
			name:     "empty main version and nothing injected",
			injected: "",
			bi:       &debug.BuildInfo{GoVersion: "go1.25.0"},
			ok:       true,
			want: aboutInfo{
				app: unknownVersion, ghfs: unknownVersion, tk: unknownVersion,
				goVer: "go1.25.0", platform: platform,
			},
		},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			setAppVersion(t, c.injected)
			if got := aboutFrom(c.bi, c.ok); got != c.want {
				t.Errorf("aboutFrom() = %+v, want %+v", got, c.want)
			}
		})
	}
}

// The About tab reports the versions of exactly these two modules; a rename in
// go.mod would otherwise turn both rows into "unknown" with nothing failing.
func TestModulePathsArePresentInThisBuild(t *testing.T) {
	bi, ok := debug.ReadBuildInfo()
	if !ok {
		t.Skip("no build info available")
	}
	for _, want := range []string{modGhfs, modTk} {
		found := false
		for _, dep := range bi.Deps {
			if dep.Path == want {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("module %q is not in this binary's dependency list", want)
		}
	}
}
