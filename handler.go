package main

import (
	"errors"
	"fmt"
	"net/url"
	"path/filepath"

	"mjpclab.dev/ghfs/src/app"
	"mjpclab.dev/ghfs/src/param"
	. "modernc.org/tk9.0"
)

func attachHandlers(widgets *uiWidgets) {
	attachBrowseHandlers(widgets)
	attachStartStopHandlers(widgets)
}

func attachBrowseHandlers(widgets *uiWidgets) {
	widgets.rootPick.Configure(Command(func() {
		dir := ChooseDirectory(Initialdir(widgets.root.Textvariable()))
		if dir != "" {
			widgets.root.Configure(Textvariable(dir))
		}
	}))
	attachFilePickHandler(widgets.tlsCertPick, widgets.tlsCert)
	attachFilePickHandler(widgets.tlsKeyPick, widgets.tlsKey)
}

func attachFilePickHandler(button *TButtonWidget, entry *TEntryWidget) {
	button.Configure(Command(func() {
		files := GetOpenFile(Initialdir(filepath.Dir(entry.Textvariable())))
		if len(files) > 0 && len(files[0]) > 0 {
			entry.Configure(Textvariable(files[0]))
		}
	}))
}

func attachStartStopHandlers(widgets *uiWidgets) {
	var appInst *app.App

	widgets.start.Configure(Command(func() {
		inst, errs := createApp(widgets)
		if len(errs) > 0 {
			showErrors(errs)
			return
		}
		appInst = inst
		widgets.start.Configure(State("disabled"))
		widgets.stop.Configure(State("normal"))
		setInputsEnabled(widgets, false)
		createLinks(appInst, widgets)
		go func() {
			openErrs := appInst.Open()
			// app.Open blocks while serving; UI updates must run on the GUI thread.
			PostEvent(func() {
				if len(openErrs) > 0 {
					showErrors(openErrs)
				}
				clearLinks(widgets)
				widgets.stop.Configure(State("disabled"))
				setInputsEnabled(widgets, true)
				widgets.start.Configure(State("normal"))
				appInst = nil
			}, false)
		}()
	}))

	widgets.stop.Configure(Command(func() {
		if appInst != nil {
			appInst.Close()
		}
	}))
}

func createApp(widgets *uiWidgets) (appInst *app.App, errs []error) {
	var certKeyPaths [][2]string
	cert := widgets.tlsCert.Textvariable()
	key := widgets.tlsKey.Textvariable()
	if len(cert) > 0 && len(key) > 0 {
		certKeyPaths = [][2]string{{cert, key}}
	}

	params, errs := param.NewParams([]param.Param{{
		Listens:       []string{widgets.listen.Textvariable()},
		IndexUrls:     []string{"/"},
		Root:          widgets.root.Textvariable(),
		DefaultSort:   "/n",
		GlobalArchive: widgets.archive.Get() == "1",
		GlobalUpload:  widgets.upload.Get() == "1",
		GlobalMkdir:   widgets.mkdir.Get() == "1",
		GlobalDelete:  widgets.del.Get() == "1",
		CertKeyPaths:  certKeyPaths,
	}})
	if len(errs) > 0 {
		return
	}

	appInst, errs = app.NewApp(params)
	return
}

func createLinks(appInst *app.App, widgets *uiWidgets) {
	clearLinks(widgets)
	accessOrigins := appInst.GetAccessibleUrls(false)
	if len(accessOrigins) == 0 {
		return
	}

	for _, origin := range accessOrigins[0] {
		if _, err := url.Parse(origin); err != nil {
			continue
		}
		origin := origin
		lbl := widgets.links.TLabel(Txt(origin), Cursor("hand2"), Style("Link.TLabel"))
		Bind(lbl, "<Button-1>", Command(func() {
			if err := openBrowser(origin); err != nil {
				fmt.Println(err)
			}
		}))
		Pack(lbl, Anchor("w"), Pady("1"))
	}
}

func clearLinks(widgets *uiWidgets) {
	for _, child := range WinfoChildren(widgets.links.Window) {
		Destroy(child)
	}
}

func setInputsEnabled(widgets *uiWidgets, enabled bool) {
	var inputState, ctrlState string
	if enabled {
		inputState, ctrlState = "normal", "normal"
	} else {
		inputState, ctrlState = "readonly", "disabled"
	}

	for _, w := range widgets.lockedInputs {
		w.Configure(State(inputState))
	}
	for _, w := range widgets.lockedControls {
		w.Configure(State(ctrlState))
	}
}

func showErrors(errs []error) {
	err := errors.Join(errs...)
	fmt.Println(err)
	MessageBox(Icon("error"), Title("Error"), Msg(err.Error()), Type("ok"))
}
