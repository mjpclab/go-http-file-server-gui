package main

import (
	_ "embed"
	"fmt"
	"strconv"

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

	useBuiltinTreeIndicator()
}

// treeIndicator is the name Tk's own Treeitem.indicator is borrowed under. Two
// constraints, both easy to break by renaming:
//
//   - No dot. Ttk_GetElement resolves "a.b.c" by retrying after each dot, so
//     anything ending in ".indicator" finds azure's element again.
//   - Must end in a lowercase "indicator", which ttk::treeview::Press
//     glob-matches to decide whether a click toggles the row. Break this and the
//     arrow still flips, but only double-click expands — DoubleClick toggles
//     without consulting the element name, so the breakage hides.
const treeIndicator = "ghfsTreeindicator"

// useBuiltinTreeIndicator fixes the Directory tab's expand arrows, which
// otherwise always point right.
//
// azure keys Treeitem.indicator on the user1/user2 states — how Tk 8.6 spelled
// "open" and "leaf". Tk 9.0 gave both bits of their own (ttkThemeInt.h: 1<<16,
// 1<<17) and no state name for either, so the theme's map never matches. Tk's
// own indicator is a C element reading the new bits, so borrow it and point the
// item layout there. Runs after ActivateTheme: elements belong to the theme
// current when they are created.
//
// Colour needs no configuring — treeview feeds each row's -foreground into the
// element, outranking its black default.
//
// Not removable until tk9.0 bundles Tk 9.1+, where "open"/"leaf" became real
// state names, and azure switches to them; azure has had no commit since
// October 2023. It will not block a Tk upgrade, but will not stand aside once
// azure is fixed either — the layout override is unconditional.
func useBuiltinTreeIndicator() {
	tclEval(fmt.Sprintf("ttk::style element create %s from default Treeitem.indicator", treeIndicator))
	tclEval(fmt.Sprintf(`ttk::style layout Treeview.Item {
		Treeitem.padding -sticky nswe -children {
			%s -side left -sticky {}
			Treeitem.image -side left -sticky {}
			Treeitem.text -side left -sticky {}
		}
	}`, treeIndicator))
}

//go:embed Icon.png
var iconPNG []byte

const (
	// The Directory tab's tree needs the extra room; winHeight has to stay
	// above minHeight or the window opens already clamped to its minimum.
	winWidth, winHeight = 650, 520
	minWidth, minHeight = 500, 420

	// wmClass is the main window's WM_CLASS class name. Desktop shells match it
	// against StartupWMClass in build/ghfs-gui.desktop, so the two have to stay
	// equal. See newMainWindow.
	wmClass = "ghfs-gui"
)

type uiWidgets struct {
	// win is the toplevel the form lives in — not App. See newMainWindow.
	win *Window

	nb       *TNotebookWidget
	root     *TEntryWidget
	rootPick *TButtonWidget
	listen   *TEntryWidget
	archive  *VariableOpt
	upload   *VariableOpt
	mkdir    *VariableOpt
	del      *VariableOpt
	// globalPerms are the four General tab checkbuttons, in permOrder.
	globalPerms [4]*TCheckbuttonWidget

	dir *dirTab

	tlsCert     *TEntryWidget
	tlsCertPick *TButtonWidget
	tlsKey      *TEntryWidget
	tlsKeyPick  *TButtonWidget

	links *TFrameWidget

	start *TButtonWidget
	stop  *TButtonWidget

	// winW/winH track the toplevel's size while it is not maximized, winMax
	// whether it is; savePreference writes both. See attachWindowHandlers for
	// why they are tracked rather than read back on demand.
	winW, winH int
	winMax     bool

	// While the server runs, inputs become readonly (still selectable/
	// copyable, not greyed) and non-text controls become disabled.
	lockedInputs   []*Window
	lockedControls []*Window
}

// newMainWindow returns the toplevel the form is built in. It is a child of "."
// rather than "." itself so that it can carry a -class: Tk derives "."'s
// WM_CLASS when the interpreter starts and offers no way in, so it stays
// "tk"/"Tk" — and a second instance on the same display becomes "tk #2". A
// desktop shell that cannot map either name to ghfs-gui.desktop has to guess an
// icon and files every instance under a task entry of its own. The class below
// matches the desktop entry, identically for every instance. "." is kept only as
// the withdrawn root that App.Wait watches.
func newMainWindow() *Window {
	win := App.Toplevel(Class(wmClass)).Window
	WmWithdraw(App)
	// Closing the visible window has to take "." down with it, or App.Wait would
	// keep the process alive with nothing on screen.
	WmProtocol(win, "WM_DELETE_WINDOW", Command(func() { Destroy(App) }))

	win.WmTitle(appName)
	WmMinSize(win, minWidth, minHeight)
	resizeWindow(win, winWidth, winHeight)

	icon := NewPhoto(Data(iconPNG))
	win.IconPhoto(icon)
	// "." is never shown, but claiming its icon too keeps App.Wait from applying
	// tk9.0's own default one.
	App.IconPhoto(icon)

	return win
}

func newUI() *uiWidgets {
	win := newMainWindow()

	nb := win.TNotebook()

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

	// Directory tab
	dir := newDirTab(nb.Window)

	// Advanced tab
	advanced := nb.TFrame(Padding("2m"))
	tlsCert := advanced.TEntry(Textvariable(""))
	tlsCertPick := advanced.TButton(Txt("..."), Width(3))
	tlsKey := advanced.TEntry(Textvariable(""))
	tlsKeyPick := advanced.TButton(Txt("..."), Width(3))
	formRow(advanced.Window, 0, "TLS Certificate", tlsCert, tlsCertPick)
	formRow(advanced.Window, 1, "TLS Key", tlsKey, tlsKeyPick)
	GridColumnConfigure(advanced, 1, Weight(1))

	// Links tab: clickable URLs, populated after the server starts.
	links := nb.TFrame(Padding("2m"))
	showLinksPlaceholder(links)

	// About tab: static version information, no state and no handlers.
	about := newAboutTab(nb.Window)

	nb.Add(general, Txt("General"))
	nb.Add(dir.frame, Txt("Directory"))
	nb.Add(advanced, Txt("Advanced"))
	nb.Add(links, Txt("Links"))
	nb.Add(about, Txt("About"))

	// buttons
	start := win.TButton(Txt("Start server"))
	stop := win.TButton(Txt("Stop server"), State("disabled"))

	// main layout
	Grid(nb, Row(0), Column(0), Columnspan(2), Sticky("news"), Padx("1m"), Pady("1m"))
	Grid(start, Row(1), Column(0), Sticky("we"), Padx("1m"), Pady("1m"))
	Grid(stop, Row(1), Column(1), Sticky("we"), Padx("1m"), Pady("1m"))
	GridRowConfigure(win, 0, Weight(1))
	GridColumnConfigure(win, 0, Weight(1))
	GridColumnConfigure(win, 1, Weight(1))

	return &uiWidgets{
		win: win,

		nb:       nb,
		root:     root,
		rootPick: rootPick,
		listen:   listen,
		archive:  archiveVar,
		upload:   uploadVar,
		mkdir:    mkdirVar,
		del:      delVar,

		globalPerms: [4]*TCheckbuttonWidget{archive, upload, mkdir, del},

		dir: dir,

		tlsCert:     tlsCert,
		tlsCertPick: tlsCertPick,
		tlsKey:      tlsKey,
		tlsKeyPick:  tlsKeyPick,

		links: links,

		start: start,
		stop:  stop,

		winW: winWidth,
		winH: winHeight,

		lockedInputs: []*Window{
			root.Window, listen.Window, tlsCert.Window, tlsKey.Window,
		},
		// Two Directory tab widgets are deliberately absent here.
		// The tree: ttk::treeview has no -state option, so configuring one
		// raises a Tcl error, which tk9.0 turns into a panic. It needs no
		// locking anyway — browsing and expanding change no configuration, and
		// the checkbuttons that do are disabled while the server runs.
		// The checkbuttons: their enabled state depends on global and inherited
		// grants, so dirTab.setLocked drives them through updateSelection
		// rather than a blanket re-enable.
		lockedControls: []*Window{
			rootPick.Window,
			archive.Window, upload.Window, mkdir.Window, del.Window,
			dir.refresh.Window,
			tlsCertPick.Window, tlsKeyPick.Window,
		},
	}
}

func clearLinkChildren(links *TFrameWidget) {
	for _, child := range WinfoChildren(links.Window) {
		Destroy(child)
	}
}

// showLinksPlaceholder puts a note where the URLs go. The Links tab is always
// visible, so leaving it blank while the server is stopped would read as a bug
// rather than as "nothing to show yet".
func showLinksPlaceholder(links *TFrameWidget) {
	clearLinkChildren(links)
	Pack(links.TLabel(Txt("Server is not running.")), Anchor("w"), Pady("1"))
}

// formRow lays out a "label : entry [...]" row in a form-style grid.
func formRow(parent *Window, row int, label string, entry *TEntryWidget, pick *TButtonWidget) {
	Grid(parent.TLabel(Txt(label)), Row(row), Column(0), Sticky("w"), Padx("1m"), Pady("1m"))
	Grid(entry, Row(row), Column(1), Sticky("we"), Padx("1m"), Pady("1m"))
	Grid(pick, Row(row), Column(2), Padx("1m"), Pady("1m"))
}

// resizeWindow sizes the window and re-centers it on the screen. The size is
// passed in rather than read back with winfo, so no event loop round-trip is
// needed. Setting an explicit geometry also switches off tk9.0's own centering,
// which App.Wait would otherwise apply once the window is mapped.
func resizeWindow(w *Window, width, height int) {
	sw, _ := strconv.Atoi(WinfoScreenWidth(w))
	sh, _ := strconv.Atoi(WinfoScreenHeight(w))
	x := (sw - width) / 2
	y := (sh - height) / 2
	WmGeometry(w, fmt.Sprintf("%dx%d+%d+%d", width, height, x, y))
}
