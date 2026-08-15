package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "modernc.org/tk9.0"
)

type preference struct {
	Root string `json:"root"`
	// The multi-value fields are stored as the raw text of their entry, not as
	// the parsed list: the form is the source of truth, so what the user typed
	// comes back verbatim rather than re-spelled with one separator.
	Listen      string `json:"listen"`
	Archive     bool   `json:"archive"`
	Upload      bool   `json:"upload"`
	Mkdir       bool   `json:"mkdir"`
	Del         bool   `json:"del"`
	DirIndex    string `json:"dirIndex"`
	Hide        string `json:"hide"`
	ListenPlain string `json:"listenPlain"`
	ListenTLS   string `json:"listenTls"`
	Cert        string `json:"cert"`
	Key         string `json:"key"`
	// DirPerms maps an absolute directory path to the permissions granted on
	// it, e.g. {"/srv/share/pub": ["archive", "upload"]}.
	DirPerms map[string][]string `json:"dirPerms"`
	// Width/Height are the window size in pixels while it is not maximized;
	// zero means "never saved", in which case the built-in default size is
	// kept. Maximized is restored on top of that size, so un-maximizing after
	// a restart lands on the same size as before.
	Width     int  `json:"width"`
	Height    int  `json:"height"`
	Maximized bool `json:"maximized"`
}

func preferencePath() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "ghfs-gui", "preference.json"), nil
}

func loadPreference(widgets *uiWidgets) {
	pref := preference{Listen: "8080"}
	if path, err := preferencePath(); err == nil {
		if data, err := os.ReadFile(path); err == nil {
			_ = json.Unmarshal(data, &pref)
		}
	}

	// Paths are normalized on the way in as well, so a file saved before this
	// (or hand-edited) does not put a foreign separator back on screen.
	root := nativePath(pref.Root)
	widgets.root.Configure(Textvariable(root))
	widgets.listen.Configure(Textvariable(pref.Listen))
	setChecked(widgets.archive, pref.Archive)
	setChecked(widgets.upload, pref.Upload)
	setChecked(widgets.mkdir, pref.Mkdir)
	setChecked(widgets.del, pref.Del)
	widgets.dirIndex.Configure(Textvariable(pref.DirIndex))
	widgets.hide.Configure(Textvariable(pref.Hide))
	widgets.listenPlain.Configure(Textvariable(pref.ListenPlain))
	widgets.listenTLS.Configure(Textvariable(pref.ListenTLS))
	widgets.tlsCert.Configure(Textvariable(nativePath(pref.Cert)))
	widgets.tlsKey.Configure(Textvariable(nativePath(pref.Key)))

	// Grants name absolute paths, so drop any that the saved root no longer
	// covers or that have since disappeared. The tree itself is left unbuilt:
	// shownRoot stays empty, so it is populated the first time the Directory
	// tab is opened, and never for users who don't go there.
	widgets.dir.perms = dirPermsFromJSON(pref.DirPerms)
	widgets.dir.perms.prune(root)

	// The saved size is clamped rather than trusted: a hand-edited or truncated
	// file should not be able to open a window too small to operate.
	if pref.Width > 0 && pref.Height > 0 {
		widgets.winW = max(pref.Width, minWidth)
		widgets.winH = max(pref.Height, minHeight)
		resizeWindow(widgets.win, widgets.winW, widgets.winH)
	}
	if pref.Maximized {
		widgets.winMax = true
		maximizeWhenMapped(widgets.win)
	}
}

func savePreference(widgets *uiWidgets) {
	root := nativePath(widgets.root.Textvariable())
	widgets.dir.perms.prune(root)

	pref := preference{
		Root:        root,
		Listen:      widgets.listen.Textvariable(),
		Archive:     widgets.archive.Get() == "1",
		Upload:      widgets.upload.Get() == "1",
		Mkdir:       widgets.mkdir.Get() == "1",
		Del:         widgets.del.Get() == "1",
		DirIndex:    widgets.dirIndex.Textvariable(),
		Hide:        widgets.hide.Textvariable(),
		ListenPlain: widgets.listenPlain.Textvariable(),
		ListenTLS:   widgets.listenTLS.Textvariable(),
		Cert:        widgets.tlsCert.Textvariable(),
		Key:         widgets.tlsKey.Textvariable(),
		DirPerms:    widgets.dir.perms.toJSON(),
		Width:       widgets.winW,
		Height:      widgets.winH,
		Maximized:   widgets.winMax,
	}

	path, err := preferencePath()
	if err != nil {
		return
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return
	}
	data, err := json.MarshalIndent(pref, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}

func setChecked(v *VariableOpt, checked bool) {
	if checked {
		v.Set("1")
	} else {
		v.Set("0")
	}
}
