package main

import (
	"fmt"
	"net/url"
	"strconv"

	. "modernc.org/tk9.0"
)

// linksTab is the Links tab. ttk has no scrollable container, so the URL labels
// are packed into a plain frame that a canvas carries as a single window item.
type linksTab struct {
	frame  *TFrameWidget // the notebook tab
	canvas *CanvasWidget
	sb     *TScrollbarWidget
	body   *TFrameWidget // the scrolled content: one label per URL
	item   string        // canvas id of body
	sbOn   bool          // whether sb is currently gridded
}

func newLinksTab(parent *Window) *linksTab {
	l := &linksTab{}
	l.frame = parent.TFrame(Padding("2m"))
	l.sb = l.frame.TScrollbar()

	opts := []Opt{
		Highlightthickness(0),
		Borderwidth(0),
		Yscrollcommand(func(e *Event) { e.ScrollSet(l.sb) }),
	}
	// A canvas paints its own background and would otherwise ignore the theme.
	// An empty lookup is left alone: an empty -background is a Tcl error, which
	// tk9.0 turns into a panic.
	if bg := tclEval("ttk::style lookup TFrame -background"); bg != "" {
		opts = append(opts, Background(bg))
	}
	l.canvas = l.frame.Canvas(opts...)
	l.sb.Configure(Command(func(e *Event) { e.Yview(l.canvas) }))

	l.body = l.canvas.TFrame()
	l.item = l.canvas.CreateWindow(0, 0, ItemWindow(l.body.Window), Anchor("nw"))

	Grid(l.canvas, Row(0), Column(0), Sticky("news"))
	GridRowConfigure(l.frame, 0, Weight(1))
	GridColumnConfigure(l.frame, 0, Weight(1))

	Bind(l.canvas, "<Configure>", Command(func(e *Event) { l.layout() }))
	Bind(l.body, "<Configure>", Command(func(e *Event) { l.layout() }))
	l.bindWheel(l.frame.Window)
	l.bindWheel(l.canvas.Window)
	l.bindWheel(l.body.Window)

	l.showPlaceholder()
	return l
}

// layout stretches the body to the canvas width, tracks the scrollregion and
// grids the scrollbar only while the content is taller than the viewport.
// Toggling the scrollbar changes the canvas width, not its height, and the
// labels do not wrap — so this cannot oscillate.
func (l *linksTab) layout() {
	cw := winfoInt(WinfoWidth(l.canvas.Window))
	ch := winfoInt(WinfoHeight(l.canvas.Window))
	// Tk reports 1x1 until the window is mapped; the <Configure> that follows
	// mapping runs this again with real numbers.
	if cw <= 1 || ch <= 1 {
		return
	}
	bh := winfoInt(WinfoHeight(l.body.Window))

	tclEval(fmt.Sprintf("%s itemconfigure %s -width %d", l.canvas, l.item, cw))
	l.canvas.Configure(Scrollregion(fmt.Sprintf("0 0 %d %d", cw, bh)))

	need := bh > ch
	if need == l.sbOn {
		return
	}
	l.sbOn = need
	if need {
		Grid(l.sb, Row(0), Column(1), Sticky("ns"))
		return
	}
	// Everything fits again; a leftover offset would leave a blank strip on top
	// with no way left to scroll it back.
	tclEval(fmt.Sprintf("%s yview moveto 0", l.canvas))
	GridRemove(l.sb.Window)
}

// bindWheel has to be called on every widget covering part of the tab: Tk
// routes an event to the widget, its class, its toplevel and "all" — never to
// the enclosing frames. A label is only as wide as its text, so the strip
// beside it belongs to the body frame, not to the label.
func (l *linksTab) bindWheel(w *Window) {
	Bind(w, "<MouseWheel>", Command(func(e *Event) {
		step := 1
		if e.Delta > 0 {
			step = -1
		}
		tclEval(fmt.Sprintf("%s yview scroll %d units", l.canvas, step))
	}))
}

func (l *linksTab) clear() {
	for _, child := range WinfoChildren(l.body.Window) {
		Destroy(child)
	}
}

// showPlaceholder puts a note where the URLs go. The Links tab is always
// visible, so leaving it blank while the server is stopped would read as a bug
// rather than as "nothing to show yet".
func (l *linksTab) showPlaceholder() {
	l.clear()
	l.pack(l.body.TLabel(Txt("Server is not running.")))
}

func (l *linksTab) show(origins []string) {
	l.clear()
	for _, origin := range origins {
		if _, err := url.Parse(origin); err != nil {
			continue
		}
		lbl := l.body.TLabel(Txt(origin), Cursor("hand2"), Style("Link.TLabel"))
		bindLink(lbl, origin)
		l.pack(lbl)
	}
}

func (l *linksTab) pack(lbl *TLabelWidget) {
	l.bindWheel(lbl.Window)
	Pack(lbl, Anchor("w"), Pady("1"))
}

func winfoInt(s string) int {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0
	}
	return n
}
