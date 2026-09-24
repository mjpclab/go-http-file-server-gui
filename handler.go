package main

import (
	"errors"
	"fmt"
	"io"
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

	// closeApp closes the instance at most once; a second app.Close would close
	// ghfs's log channels again and panic.
	closeApp := func() {
		if appInst == nil {
			return
		}
		inst := appInst
		appInst = nil
		inst.Close()
	}

	widgets.start.Configure(Command(func() {
		inst, errs := createApp(widgets)
		if len(errs) > 0 {
			showErrors(widgets.win, errs)
			return
		}
		appInst = inst
		savePreference(widgets)
		widgets.start.Configure(State("disabled"))
		widgets.stop.Configure(State("normal"))
		setInputsEnabled(widgets, false)
		createLinks(inst, widgets)
		go func() {
			openErrs := inst.Open()
			// app.Open blocks while serving; UI updates must run on the GUI thread.
			PostEvent(func() {
				if len(openErrs) > 0 {
					showErrors(widgets.win, openErrs)
				}
				widgets.links.showPlaceholder()
				widgets.stop.Configure(State("disabled"))
				setInputsEnabled(widgets, true)
				widgets.start.Configure(State("normal"))
				// Still set only if Open failed on its own: ghfs then leaves the
				// log manager, and its goroutine, open.
				closeApp()
			}, false)
		}()
	}))

	widgets.stop.Configure(Command(closeApp))
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
	// An empty Root would otherwise be resolved to the working directory by
	// param.NewParams (filepath.Abs("")), serving whatever the app was started
	// from. EmptyRoot makes ghfs serve an empty listing instead.
	root := widgets.root.Textvariable()
	params, errs := param.NewParams([]param.Param{{
		Listens:      parseMultiValues(widgets.listen.Textvariable()),
		ListensPlain: parseMultiValues(widgets.listenPlain.Textvariable()),
		ListensTLS:   parseMultiValues(widgets.listenTLS.Textvariable()),
		Root:         root,
		EmptyRoot:    len(root) == 0,
		DefaultSort:  "/n",
		DirIndexes:   parseMultiValues(widgets.dirIndex.Textvariable()),
		Hides:        parseMultiValues(widgets.hide.Textvariable()),
		// GlobalList is the "may list a directory" permission, unrelated to
		// DirIndexes above, which names the file served in place of that listing.
		GlobalList:    widgets.list.Get() == "1",
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
		ListDirs:      perms.dirsWith(permList),
		CertKeyPaths:  certKeyPaths,
		// EntriesToKVs is ghfs's own "<name>:<value>" split, the same one --global-header
		// goes through; it drops an entry without a colon on either side rather than
		// erroring, so a half-typed header is ignored instead of blocking Start.
		GlobalHeaders: param.EntriesToKVs(parseMultiValues(widgets.headers.Textvariable())),
	}})
	if len(errs) > 0 {
		return
	}

	// Left as untyped nil when unchecked: a nil *logWriter stored in the
	// interface would pass ghfs's nil test and be written to.
	var accessLog, errorLog io.Writer
	if widgets.logAccess.Get() == "1" {
		accessLog = widgets.logs.accessLog
	}
	if widgets.logError.Get() == "1" {
		errorLog = widgets.logs.errorLog
	}
	appInst, errs = app.NewWriterLogApp(params, [][2]io.Writer{{accessLog, errorLog}})
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

// showErrors needs the visible window as parent: the default, ".", is withdrawn,
// so the box would be neither centered over nor modal to the form.
func showErrors(parent *Window, errs []error) {
	err := errors.Join(errs...)
	fmt.Println(err)
	MessageBox(Icon("error"), Title("Error"), Msg(err.Error()), Type("ok"), Parent(parent))
}
