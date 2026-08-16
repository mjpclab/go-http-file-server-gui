package main

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"

	"mjpclab.dev/ghfs/src/app"
	"mjpclab.dev/ghfs/src/param"
	. "modernc.org/tk9.0"
)

func attachHandlers(widgets *uiWidgets) {
	attachWindowHandlers(widgets)
	attachBrowseHandlers(widgets)
	attachGlobalPermHandlers(widgets)
	attachDirHandlers(widgets)
	attachStartStopHandlers(widgets)
}

// attachWindowHandlers records the toplevel's size as the user resizes it.
// savePreference runs after App.Wait returns — the window is gone by then, and
// winfo on a destroyed window raises a Tcl error, which tk9.0 turns into a
// panic — so the size has to be captured while the window still exists.
func attachWindowHandlers(widgets *uiWidgets) {
	Bind(widgets.win, "<Configure>", Command(func(e *Event) {
		// A child widget's bindtags include its toplevel, so this binding also
		// fires for every child resize; only the toplevel's own size is wanted.
		if e.EventWindow != widgets.win {
			return
		}
		// While maximized the reported size is the screen's. Keeping the last
		// normal size instead means un-maximizing — now or on the next launch —
		// goes back to the size the user actually chose.
		widgets.winMax = isMaximized(widgets.win)
		if widgets.winMax {
			return
		}
		w, errW := strconv.Atoi(e.Width)
		h, errH := strconv.Atoi(e.Height)
		// Tk reports 1x1 for a window that has not been mapped yet.
		if errW != nil || errH != nil || w <= 1 || h <= 1 {
			return
		}
		widgets.winW, widgets.winH = w, h
	}))
}

// attachGlobalPermHandlers keeps the Directory tab in step with the General
// tab: a globally granted permission cannot be revoked per-directory, so the
// matching per-directory checkbutton has to show that.
func attachGlobalPermHandlers(widgets *uiWidgets) {
	for _, cb := range widgets.globalPerms {
		cb.Configure(Command(func() { widgets.dir.updateSelection() }))
	}
}

func attachBrowseHandlers(widgets *uiWidgets) {
	// The dialogs are parented explicitly: their default parent is ".", which is
	// withdrawn, and they would center on an off-screen window.
	widgets.rootPick.Configure(Command(func() {
		dir := ChooseDirectory(Initialdir(widgets.root.Textvariable()), Parent(widgets.win))
		if dir != "" {
			widgets.root.Configure(Textvariable(nativePath(dir)))
		}
	}))
	attachFilePickHandler(widgets.win, widgets.tlsCertPick, widgets.tlsCert)
	attachFilePickHandler(widgets.win, widgets.tlsKeyPick, widgets.tlsKey)
}

func attachFilePickHandler(parent *Window, button *TButtonWidget, entry *TEntryWidget) {
	button.Configure(Command(func() {
		files := GetOpenFile(Initialdir(filepath.Dir(entry.Textvariable())), Parent(parent))
		if len(files) > 0 && len(files[0]) > 0 {
			entry.Configure(Textvariable(nativePath(files[0])))
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
		savePreference(widgets)
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
				widgets.links.showPlaceholder()
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

	// Directory grants go through the *Dirs fields rather than the *Urls ones.
	// With a single root ghfs makes the two equivalent (param.NewParams turns
	// Root into the alias {"/", Root}, and fsPath is just dir+urlPath), but a
	// filesystem path names the directory itself: it stays correct if aliases
	// or vhosts are ever added, and it cannot silently follow Root elsewhere.
	perms := widgets.dir.perms
	// IndexUrls is the "may list a directory" permission, unrelated to
	// DirIndexes below, which names the file served in place of that listing.
	params, errs := param.NewParams([]param.Param{{
		Listens:       parseMultiValues(widgets.listen.Textvariable()),
		ListensPlain:  parseMultiValues(widgets.listenPlain.Textvariable()),
		ListensTLS:    parseMultiValues(widgets.listenTLS.Textvariable()),
		IndexUrls:     []string{"/"},
		Root:          widgets.root.Textvariable(),
		DefaultSort:   "/n",
		DirIndexes:    parseMultiValues(widgets.dirIndex.Textvariable()),
		Hides:         parseMultiValues(widgets.hide.Textvariable()),
		GlobalArchive: widgets.archive.Get() == "1",
		GlobalUpload:  widgets.upload.Get() == "1",
		GlobalMkdir:   widgets.mkdir.Get() == "1",
		GlobalDelete:  widgets.del.Get() == "1",
		GlobalCors:    widgets.cors.Get() == "1",
		ArchiveDirs:   perms.dirsWith(permArchive),
		UploadDirs:    perms.dirsWith(permUpload),
		MkdirDirs:     perms.dirsWith(permMkdir),
		DeleteDirs:    perms.dirsWith(permDelete),
		CorsDirs:      perms.dirsWith(permCors),
		CertKeyPaths:  certKeyPaths,
	}})
	if len(errs) > 0 {
		return
	}

	appInst, errs = app.NewApp(params)
	return
}

func createLinks(appInst *app.App, widgets *uiWidgets) {
	accessOrigins := appInst.GetAccessibleUrls(false)
	if len(accessOrigins) == 0 {
		widgets.links.showPlaceholder()
		return
	}

	widgets.links.show(accessOrigins[0])

	// The URLs are what the user came for once the server is up, and the Links
	// tab may not be the one on screen — bring it forward.
	widgets.nb.Select(widgets.links.frame)
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
	widgets.dir.setLocked(!enabled)
}

func showErrors(errs []error) {
	err := errors.Join(errs...)
	fmt.Println(err)
	MessageBox(Icon("error"), Title("Error"), Msg(err.Error()), Type("ok"))
}
