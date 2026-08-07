package main

import (
	"encoding/json"
	"os"
	"path/filepath"

	. "modernc.org/tk9.0"
)

type preference struct {
	Root    string `json:"root"`
	Listen  string `json:"listen"`
	Archive bool   `json:"archive"`
	Upload  bool   `json:"upload"`
	Mkdir   bool   `json:"mkdir"`
	Del     bool   `json:"del"`
	Cert    string `json:"cert"`
	Key     string `json:"key"`
	// DirPerms maps an absolute directory path to the permissions granted on
	// it, e.g. {"/srv/share/pub": ["archive", "upload"]}.
	DirPerms map[string][]string `json:"dirPerms"`
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

	widgets.root.Configure(Textvariable(pref.Root))
	widgets.listen.Configure(Textvariable(pref.Listen))
	setChecked(widgets.archive, pref.Archive)
	setChecked(widgets.upload, pref.Upload)
	setChecked(widgets.mkdir, pref.Mkdir)
	setChecked(widgets.del, pref.Del)
	widgets.tlsCert.Configure(Textvariable(pref.Cert))
	widgets.tlsKey.Configure(Textvariable(pref.Key))

	// Grants name absolute paths, so drop any that the saved root no longer
	// covers or that have since disappeared. The tree itself is left unbuilt:
	// shownRoot stays empty, so it is populated the first time the Directory
	// tab is opened, and never for users who don't go there.
	widgets.dir.perms = dirPermsFromJSON(pref.DirPerms)
	widgets.dir.perms.prune(pref.Root)
}

func savePreference(widgets *uiWidgets) {
	root := widgets.root.Textvariable()
	widgets.dir.perms.prune(root)

	pref := preference{
		Root:     root,
		Listen:   widgets.listen.Textvariable(),
		Archive:  widgets.archive.Get() == "1",
		Upload:   widgets.upload.Get() == "1",
		Mkdir:    widgets.mkdir.Get() == "1",
		Del:      widgets.del.Get() == "1",
		Cert:     widgets.tlsCert.Textvariable(),
		Key:      widgets.tlsKey.Textvariable(),
		DirPerms: widgets.dir.perms.toJSON(),
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
