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
}

func savePreference(widgets *uiWidgets) {
	pref := preference{
		Root:    widgets.root.Textvariable(),
		Listen:  widgets.listen.Textvariable(),
		Archive: widgets.archive.Get() == "1",
		Upload:  widgets.upload.Get() == "1",
		Mkdir:   widgets.mkdir.Get() == "1",
		Del:     widgets.del.Get() == "1",
		Cert:    widgets.tlsCert.Textvariable(),
		Key:     widgets.tlsKey.Textvariable(),
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
