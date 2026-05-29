package main

import (
	_ "embed"

	. "modernc.org/tk9.0"
	"modernc.org/tk9.0/extensions/themedetector"
)

func applySystemTheme() {
	InitializeExtension("themedetector")
	dark := false
	if pref, err := themedetector.GetPreference(); err == nil && pref == themedetector.DarkTheme {
		dark = true
	}

	var linkColor string
	if dark {
		ActivateTheme("azure dark")
		linkColor = "#90caf9"
		// Azure dark's default disabled foreground (#aaaaaa) is too close to
		// the enabled #ffffff; the button face is the #333 window background
		// (rect-basic is transparent), so #909090 stays legible while widening
		// the gap from white.
		StyleMap("TButton", Foreground, "disabled", "#909090")
	} else {
		ActivateTheme("azure light")
		linkColor = "#1565c0"
	}

	linkFont := NewFont(Family("TkDefaultFont"), Underline(true))
	StyleConfigure("Link.TLabel", Foreground(linkColor), Font(linkFont))
}

//go:embed Icon.png
var iconPNG string

type uiWidgets struct {
	root     *TEntryWidget
	rootPick *TButtonWidget
	listen   *TEntryWidget
	archive  *VariableOpt
	upload   *VariableOpt
	mkdir    *VariableOpt
	del      *VariableOpt

	tlsCert     *TEntryWidget
	tlsCertPick *TButtonWidget
	tlsKey      *TEntryWidget
	tlsKeyPick  *TButtonWidget

	links *TFrameWidget

	start *TButtonWidget
	stop  *TButtonWidget

	// While the server runs, inputs become readonly (still selectable/
	// copyable, not greyed) and non-text controls become disabled.
	lockedInputs   []*Window
	lockedControls []*Window
}

func newUI() *uiWidgets {
	App.WmTitle("Go HTTP File Server GUI")
	WmGeometry(App, "650x400")
	App.IconPhoto(NewPhoto(Data([]byte(iconPNG))))

	nb := TNotebook()

	// General tab
	general := nb.TFrame(Padding("2m"))

	root := general.TEntry(Textvariable(""))
	rootPick := general.TButton(Txt("..."), Width(3))
	listen := general.TEntry(Textvariable(""))

	archiveVar := Variable("0")
	uploadVar := Variable("0")
	mkdirVar := Variable("0")
	delVar := Variable("0")
	options := general.TFrame()
	archive := options.TCheckbutton(Txt("Archive"), archiveVar)
	upload := options.TCheckbutton(Txt("Upload"), uploadVar)
	mkdir := options.TCheckbutton(Txt("Mkdir"), mkdirVar)
	del := options.TCheckbutton(Txt("Delete"), delVar)
	Grid(archive, Row(0), Column(0), Padx("1m"))
	Grid(upload, Row(0), Column(1), Padx("1m"))
	Grid(mkdir, Row(0), Column(2), Padx("1m"))
	Grid(del, Row(0), Column(3), Padx("1m"))

	formRow(general.Window, 0, "Root", root, rootPick)
	Grid(general.TLabel(Txt("Listen")), Row(1), Column(0), Sticky("w"), Padx("1m"), Pady("1m"))
	Grid(listen, Row(1), Column(1), Columnspan(2), Sticky("we"), Padx("1m"), Pady("1m"))
	Grid(general.TLabel(Txt("Options")), Row(2), Column(0), Sticky("w"), Padx("1m"), Pady("1m"))
	Grid(options, Row(2), Column(1), Columnspan(2), Sticky("w"), Padx("1m"), Pady("1m"))
	GridColumnConfigure(general, 1, Weight(1))

	// Advanced tab
	advanced := nb.TFrame(Padding("2m"))
	tlsCert := advanced.TEntry(Textvariable(""))
	tlsCertPick := advanced.TButton(Txt("..."), Width(3))
	tlsKey := advanced.TEntry(Textvariable(""))
	tlsKeyPick := advanced.TButton(Txt("..."), Width(3))
	formRow(advanced.Window, 0, "TLS Certificate", tlsCert, tlsCertPick)
	formRow(advanced.Window, 1, "TLS Key", tlsKey, tlsKeyPick)
	GridColumnConfigure(advanced, 1, Weight(1))

	nb.Add(general, Txt("General"))
	nb.Add(advanced, Txt("Advanced"))

	// links list (clickable, populated after server starts)
	links := TFrame(Padding("2m"))

	// buttons
	start := TButton(Txt("Start server"))
	stop := TButton(Txt("Stop server"), State("disabled"))

	// main layout
	Grid(nb, Row(0), Column(0), Columnspan(2), Sticky("news"), Padx("1m"), Pady("1m"))
	Grid(links, Row(1), Column(0), Columnspan(2), Sticky("news"), Padx("1m"), Pady("1m"))
	Grid(start, Row(2), Column(0), Sticky("we"), Padx("1m"), Pady("1m"))
	Grid(stop, Row(2), Column(1), Sticky("we"), Padx("1m"), Pady("1m"))
	GridRowConfigure(App, 1, Weight(1))
	GridColumnConfigure(App, 0, Weight(1))
	GridColumnConfigure(App, 1, Weight(1))

	return &uiWidgets{
		root:     root,
		rootPick: rootPick,
		listen:   listen,
		archive:  archiveVar,
		upload:   uploadVar,
		mkdir:    mkdirVar,
		del:      delVar,

		tlsCert:     tlsCert,
		tlsCertPick: tlsCertPick,
		tlsKey:      tlsKey,
		tlsKeyPick:  tlsKeyPick,

		links: links,

		start: start,
		stop:  stop,

		lockedInputs: []*Window{
			root.Window, listen.Window, tlsCert.Window, tlsKey.Window,
		},
		lockedControls: []*Window{
			rootPick.Window,
			archive.Window, upload.Window, mkdir.Window, del.Window,
			tlsCertPick.Window, tlsKeyPick.Window,
		},
	}
}

// formRow lays out a "label : entry [...]" row in a form-style grid.
func formRow(parent *Window, row int, label string, entry *TEntryWidget, pick *TButtonWidget) {
	Grid(parent.TLabel(Txt(label)), Row(row), Column(0), Sticky("w"), Padx("1m"), Pady("1m"))
	Grid(entry, Row(row), Column(1), Sticky("we"), Padx("1m"), Pady("1m"))
	Grid(pick, Row(row), Column(2), Padx("1m"), Pady("1m"))
}
