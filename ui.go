package main

import (
	_ "embed"
	"fmt"
	"strconv"
	"strings"

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

	useLargerThemeTiles()
	useBuiltinTreeIndicator()
}

// Tcl names the tiling replacement installs itself under. Each element keeps
// the last dotted component of the one it stands in for, because widgets look
// their own parts up by that tail: Ttk_FindElement compares only the text after
// the final dot, so a notebook asking Ttk_ClientRegion for "client" would miss
// a plain "ghfsNotebookclient" and lay its tab out over the whole widget,
// losing the border inset. Registering the full dotted name is what keeps
// Ttk_GetElement from falling through to the element the tail names — the
// failure mode treeIndicator documents.
const (
	tileImageProc   = "ghfsTileImage"
	entryElement    = "ghfsEntry.field"
	buttonElement   = "ghfsButton.button"
	notebookElement = "ghfsNotebook.client"
	treeElement     = "ghfsTreeview.field"
	headingElement  = "ghfsTreeheading.cell"
)

// tileSize is the length a source image is grown to along an axis it is tiled
// on. Past a couple of hundred pixels the blits saved stop being measurable,
// and every image is copied at startup.
const tileSize = 200

// tiledElements are the azure image elements that get tiled over a large area,
// restated here so they can be given larger copies of the same images. Each
// entry mirrors an element in the theme's light.tcl — dark.tcl defines the same
// five identically — together with the layout that draws it.
//
// The states matter as much as the images: an element definition replaces
// azure's wholesale, so a state left out here would stop rendering.
var tiledElements = []struct {
	name, style, original string
	options               string // element options, as azure spells them
	border                int    // -border, which is also the 9-patch inset
	srcW, srcH            int    // size on disk, and the size pinned as natural
	growW, growH          int
	images                []struct{ state, image string }
}{{
	// Entries stretch the full width of the window, which makes this the widest
	// tiling in the app.
	name: entryElement, style: "TEntry", original: "Entry.field",
	options: "-border 5 -padding 8 -sticky news",
	border:  5, srcW: 20, srcH: 20, growW: tileSize, growH: 20,
	images: []struct{ state, image string }{
		{"", "box-basic"},
		{"focus hover", "box-accent"},
		{"invalid", "box-invalid"},
		{"disabled", "box-basic"},
		{"focus", "box-accent"},
		{"hover", "box-hover"},
	},
}, {
	// Start and Stop span half the window each.
	name: buttonElement, style: "TButton", original: "Button.button",
	options: "-border 4 -sticky ewns",
	border:  4, srcW: 20, srcH: 20, growW: tileSize, growH: 20,
	images: []struct{ state, image string }{
		{"", "rect-basic"},
		{"selected disabled", "rect-basic"},
		{"disabled", "rect-basic"},
		{"selected", "rect-basic"},
		{"pressed", "rect-basic"},
		{"active", "button-hover"},
		{"focus", "button-hover"},
	},
}, {
	// The body behind whichever tab is open, tiled in both directions.
	name: notebookElement, style: "TNotebook", original: "Notebook.client",
	options: "-border 5",
	border:  5, srcW: 50, srcH: 50, growW: tileSize, growH: tileSize,
	images: []struct{ state, image string }{{"", "notebook"}},
}, {
	// The Directory tab's tree, likewise.
	name: treeElement, style: "Treeview", original: "Treeview.field",
	options: "-border 5",
	border:  5, srcW: 50, srcH: 50, growW: tileSize, growH: tileSize,
	images: []struct{ state, image string }{{"", "card"}},
}, {
	name: headingElement, style: "Heading", original: "Treeheading.cell",
	options: "-border 5 -padding 4 -sticky ewns",
	border:  5, srcW: 20, srcH: 20, growW: tileSize, growH: 20,
	images: []struct{ state, image string }{
		{"", "tree-basic"},
		{"pressed", "tree-pressed"},
	},
}}

// useLargerThemeTiles redraws azure's stretched backgrounds from larger source
// images, which is most of what makes resizing the window smooth.
//
// An image element that has a -border is not scaled to its widget, it is tiled:
// ttkImage.c walks the nine regions of the image and issues one Tk_RedrawImage
// per source-sized tile (Ttk_Fill). Azure draws entries, buttons, the notebook
// body and the tree from 20x20 and 50x50 images, so one entry across a 1200px
// window costs some three hundred blits — and a window resize redraws every
// visible widget on every step. Measured under X11 at 1200x900: 13ms per resize
// step on the General tab and 25ms on the Directory tab, against 3.8ms and
// 9.9ms once the tiles are 200px. At the default window size, 7.0ms and 10.6ms
// against 2.7ms and 4.4ms.
//
// What is drawn does not change. ttk tiles these images from the same nine
// regions the copies are built from, and every image involved has a uniform
// middle band along the axis it is tiled on, so the pixels land where they did
// before — only the number of blits differs. Two things would change it, and
// both are avoided:
//
//   - An element's natural size is its image's, and Tk adds that to what a
//     treeview or an entry asks for, so a larger image would move the paned
//     window's sash and widen the "..." buttons. -width/-height pin the
//     reported size back to the original.
//   - azure shares images between elements — box-basic backs both Entry.field
//     and Checkbutton.indicator, which draws it whole rather than tiled, and
//     would show a slice of the grown copy. Hence private copies and private
//     elements rather than growing the theme's images in place.
//
// Nothing here is load-bearing. tclEval swallows errors, the Tcl side checks
// each image is still the size the table claims before copying it, and a theme
// that has moved on simply keeps drawing itself the slow way.
func useLargerThemeTiles() {
	// azure keeps its photos in the theme namespace's I array, one namespace
	// per variant.
	theme := tclEval("ttk::style theme use")
	if !strings.HasPrefix(theme, "azure-") {
		return
	}

	// The corners are copied at their original size; the edges and the centre
	// are copied into a larger destination rectangle, which a photo copy fills
	// by replicating the source — the same tiling ttk would do at draw time,
	// done once here instead.
	tclEval(fmt.Sprintf(`proc %s {theme name b sw sh w h} {
		set src [set ::ttk::theme::${theme}::I($name)]
		if {[image width $src] != $sw || [image height $src] != $sh} { return "" }
		set dst [image create photo -width $w -height $h]
		set sx [expr {$sw - $b}]
		set sy [expr {$sh - $b}]
		set dx [expr {$w - $b}]
		set dy [expr {$h - $b}]
		$dst copy $src -from 0 0 $b $b -to 0 0 -compositingrule set
		$dst copy $src -from $sx 0 $sw $b -to $dx 0 -compositingrule set
		$dst copy $src -from 0 $sy $b $sh -to 0 $dy -compositingrule set
		$dst copy $src -from $sx $sy $sw $sh -to $dx $dy -compositingrule set
		$dst copy $src -from $b 0 $sx $b -to $b 0 $dx $b -compositingrule set
		$dst copy $src -from $b $sy $sx $sh -to $b $dy $dx $h -compositingrule set
		$dst copy $src -from 0 $b $b $sy -to 0 $b $b $dy -compositingrule set
		$dst copy $src -from $sx $b $sw $sy -to $dx $b $w $dy -compositingrule set
		$dst copy $src -from $b $b $sx $sy -to $b $b $dx $dy -compositingrule set
		return $dst
	}`, tileImageProc))

	for _, e := range tiledElements {
		// The first image in the spec is the default one, the rest are keyed on
		// state.
		spec := make([]string, 0, 2*len(e.images))
		for _, i := range e.images {
			img := tclEval(fmt.Sprintf("%s %s %s %d %d %d %d %d",
				tileImageProc, theme, i.image, e.border, e.srcW, e.srcH, e.growW, e.growH))
			if img == "" {
				return
			}
			if i.state != "" {
				spec = append(spec, "{"+i.state+"}")
			}
			spec = append(spec, img)
		}

		tclEval(fmt.Sprintf("ttk::style element create %s image [list %s] %s -width %d -height %d",
			e.name, strings.Join(spec, " "), e.options, e.srcW, e.srcH))
		// Swapping the element name in the layout azure already built leaves
		// the rest of that layout — padding, sticky, children — alone.
		tclEval(fmt.Sprintf("ttk::style layout %s [string map {%s %s} [ttk::style layout %s]]",
			e.style, e.original, e.name, e.style))
	}
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
	dirIndex    *TEntryWidget
	hide        *TEntryWidget

	dir *dirTab

	listenPlain *TEntryWidget
	listenTLS   *TEntryWidget
	tlsCert     *TEntryWidget
	tlsCertPick *TButtonWidget
	tlsKey      *TEntryWidget
	tlsKeyPick  *TButtonWidget

	links *linksTab

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
	dirIndex := general.TEntry(Textvariable(""))
	hide := general.TEntry(Textvariable(""))

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
	formEntryRow(general.Window, 1, "Listen", listen)
	Grid(general.TLabel(Txt("Options")), Row(2), Column(0), Sticky("w"), Padx("1m"), Pady("1m"))
	Grid(options, Row(2), Column(1), Columnspan(2), Sticky("w"), Padx("1m"), Pady("1m"))
	formEntryRow(general.Window, 3, "Dir Index", dirIndex)
	formEntryRow(general.Window, 4, "Hide", hide)
	GridColumnConfigure(general, 1, Weight(1))

	// Directory tab
	dir := newDirTab(nb.Window)

	// Advanced tab
	advanced := nb.TFrame(Padding("2m"))
	listenPlain := advanced.TEntry(Textvariable(""))
	listenTLS := advanced.TEntry(Textvariable(""))
	tlsCert := advanced.TEntry(Textvariable(""))
	tlsCertPick := advanced.TButton(Txt("..."), Width(3))
	tlsKey := advanced.TEntry(Textvariable(""))
	tlsKeyPick := advanced.TButton(Txt("..."), Width(3))
	formEntryRow(advanced.Window, 0, "Listen Plain", listenPlain)
	formEntryRow(advanced.Window, 1, "Listen TLS", listenTLS)
	formRow(advanced.Window, 2, "TLS Certificate", tlsCert, tlsCertPick)
	formRow(advanced.Window, 3, "TLS Key", tlsKey, tlsKeyPick)
	GridColumnConfigure(advanced, 1, Weight(1))

	// Links tab: clickable URLs, populated after the server starts.
	links := newLinksTab(nb.Window)

	// About tab: static version information, no state and no handlers.
	about := newAboutTab(nb.Window)

	nb.Add(general, Txt("General"))
	nb.Add(dir.frame, Txt("Directory"))
	nb.Add(advanced, Txt("Advanced"))
	nb.Add(links.frame, Txt("Links"))
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
		dirIndex:    dirIndex,
		hide:        hide,

		dir: dir,

		listenPlain: listenPlain,
		listenTLS:   listenTLS,
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
			root.Window, listen.Window,
			dirIndex.Window, hide.Window,
			listenPlain.Window, listenTLS.Window,
			tlsCert.Window, tlsKey.Window,
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

// formRow lays out a "label : entry [...]" row in a form-style grid.
func formRow(parent *Window, row int, label string, entry *TEntryWidget, pick *TButtonWidget) {
	Grid(parent.TLabel(Txt(label)), Row(row), Column(0), Sticky("w"), Padx("1m"), Pady("1m"))
	Grid(entry, Row(row), Column(1), Sticky("we"), Padx("1m"), Pady("1m"))
	Grid(pick, Row(row), Column(2), Padx("1m"), Pady("1m"))
}

// formEntryRow lays out a "label : entry" row with no pick button, spanning the
// column formRow leaves for one so the entries of both kinds end flush.
func formEntryRow(parent *Window, row int, label string, entry *TEntryWidget) {
	Grid(parent.TLabel(Txt(label)), Row(row), Column(0), Sticky("w"), Padx("1m"), Pady("1m"))
	Grid(entry, Row(row), Column(1), Columnspan(2), Sticky("we"), Padx("1m"), Pady("1m"))
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
