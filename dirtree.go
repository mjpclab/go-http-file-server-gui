package main

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	. "modernc.org/tk9.0"
)

// dirTab is the Directory tab: Root above a lazily populated tree of the
// directories under it on the left, and the four permission toggles for the
// selected directory on the right.
type dirTab struct {
	frame   *TFrameWidget
	tree    *TTreeviewWidget
	refresh *TButtonWidget
	rootEnt *TEntryWidget
	pathEnt *TEntryWidget
	vars    [4]*VariableOpt
	checks  [4]*TCheckbuttonWidget
	hints   [4]*TLabelWidget

	perms *dirPerms

	// globals reports the General tab's four global switches, in permOrder.
	// Wired during attachDirHandlers; nil until then.
	globals func() [4]bool

	paths  map[string]string // item id  -> absolute path
	ids    map[string]string // absolute path -> item id
	loaded map[string]bool   // item ids whose children have been read
	opened map[string]bool   // absolute path -> expanded, preserved across refresh

	// hide is the compiled Hide pattern, nil when nothing is filtered;
	// shownHide is the entry text it was built from, so a tab change can tell a
	// changed pattern from an unchanged one without recompiling.
	hide      *regexp.Regexp
	shownHide string

	shownRoot string // root the tree was last built from
	sel       string // selected directory
	locked    bool   // server running: keep the right pane disabled

	searchBuf   []rune    // type-ahead: characters typed so far
	searchStart string    // item the current type-ahead sequence started from
	searchAt    time.Time // when the last character was typed

	shown paneState // what the right pane currently displays
}

// paneState is what updateSelection last wrote to the permission widgets. Arrow
// keys walk many rows in a row and most need an identical pane, so writing
// only what differs cuts the Tcl round-trips per keypress from fourteen to
// about two. It also avoids re-setting a hint label to the text it already has:
// that label has no fixed width, so a changed one resizes a grid column, which
// resizes the frame, which can make the paned window reallocate and relayout
// the whole tree.
type paneState struct {
	valid   bool // false until the first write, so zero values are not trusted
	checked [4]bool
	state   [4]string
	hint    [4]string
}

const noDirSelected = "(no directory selected)"

// A directory with nothing to list still gets one of these rows, which is what
// keeps its arrow: an item without children is a leaf to ttk::treeview, which
// draws no indicator for one, so the arrow would vanish and read as "not
// expandable" rather than "expanded, and there is nothing inside".
const (
	hintEmpty      = "(no subdirectories)"
	hintUnreadable = "(cannot be read)"
)

// Label rows have no dirTab.paths entry, which is what tells them apart from
// directories everywhere in this file. The procs are the Tcl-side helpers
// overrideTreeBindings installs.
const (
	hintTag       = "hint"
	hintTestProc  = "ghfsIsHintRow"
	topRowProc    = "ghfsTopRow"
	browseTopProc = "ghfsBrowseTop"
)

// hiddenTag marks a directory the Hide patterns keep out of its parent's
// listing. It is deliberately a font change and not a colour: #888888 already
// means "unreadable" and "label row" in this tree, and being left out of a
// listing is neither — the row stays selectable and its permission
// checkbuttons stay live, because ghfs still serves the directory, still lists
// its own contents, and still honours every grant made on it. Only the parent's
// listing omits it.
//
// The font is derived from the real TkDefaultFont rather than built from a
// family name, so it keeps whatever family and size the platform picked and
// changes nothing but the slant.
const (
	hiddenTag  = "hidden"
	hiddenFont = "ghfsHiddenFont"
)

// typeAheadTimeout matches Tk's own file dialog (library/iconlist.tcl), so
// typing behaves the same here as in the directory picker this app also uses.
const typeAheadTimeout = 500 * time.Millisecond

func newDirTab(parent *Window) *dirTab {
	d := &dirTab{
		perms:  newDirPerms(),
		paths:  map[string]string{},
		ids:    map[string]string{},
		loaded: map[string]bool{},
		opened: map[string]bool{},
	}

	d.frame = parent.TFrame(Padding("2m"))
	pane := d.frame.TPanedwindow(Orient("horizontal"))
	Pack(pane, Expand(true), Fill("both"))

	// Left: Root with a manual refresh beside it, over the directory tree.
	left := pane.TFrame()
	head := left.TFrame()
	d.rootEnt = head.TEntry(Textvariable(""), State("readonly"), Width(16))
	// A glyph rather than the word, so the button leaves the entry beside it as
	// much width as possible for a long root; the tooltip carries the word.
	d.refresh = head.TButton(Txt("↻"), Width(2))
	Tooltip(d.refresh, "Refresh")
	Grid(d.rootEnt, Row(0), Column(0), Sticky("we"))
	Grid(d.refresh, Row(0), Column(1), Padx("1m 0"))
	GridColumnConfigure(head, 0, Weight(1))

	sb := left.TScrollbar()
	d.tree = left.TTreeview(
		Selectmode("browse"),
		Columns("perm"),
		Show("tree headings"),
		Yscrollcommand(func(e *Event) { e.ScrollSet(sb) }),
	)
	sb.Configure(Command(func(e *Event) { e.Yview(d.tree) }))
	d.tree.Column("#0", Width(240), Anchor("w"))
	d.tree.Column("perm", Width(70), Anchor("w"))
	d.tree.Heading("#0", Txt("Directory"), Anchor("w"))
	d.tree.Heading("perm", Txt("Perm"), Anchor("w"))
	d.tree.TagConfigure("unreadable", Foreground("#888888"))
	d.tree.TagConfigure(hintTag, Foreground("#888888"))
	tclEval(fmt.Sprintf("font create %s {*}[font actual TkDefaultFont] -slant italic", hiddenFont))
	d.tree.TagConfigure(hiddenTag, Font(hiddenFont))

	Grid(head, Row(0), Column(0), Columnspan(2), Sticky("we"), Pady("1m"))
	Grid(d.tree, Row(1), Column(0), Sticky("news"))
	Grid(sb, Row(1), Column(1), Sticky("ns"))
	GridRowConfigure(left, 1, Weight(1))
	GridColumnConfigure(left, 0, Weight(1))

	// Right: permissions for the selected directory.
	right := pane.TFrame(Padding("2m"))
	// A readonly entry rather than a label: a deep path is wider than the pane,
	// and an entry can be scrolled sideways and copied out. Its requested width
	// is what the paned window sizes this side from, so it is stated in
	// characters rather than left to the (much longer) path text.
	d.pathEnt = right.TEntry(Textvariable(noDirSelected), State("readonly"), Width(24))
	Grid(d.pathEnt, Row(0), Column(0), Columnspan(2), Sticky("we"), Pady("1m"))
	for i, bit := range permOrder {
		v := Variable("0")
		d.vars[i] = v
		d.checks[i] = right.TCheckbutton(Txt(permLabels[bit]), v, State("disabled"))
		d.hints[i] = right.TLabel(Txt(""))
		Grid(d.checks[i], Row(i+1), Column(0), Sticky("w"), Padx("1m"), Pady("0.5m"))
		Grid(d.hints[i], Row(i+1), Column(1), Sticky("w"), Padx("1m"))
	}
	GridColumnConfigure(right, 1, Weight(1))

	pane.Add(left.Window, Weight(3))
	pane.Add(right.Window, Weight(2))

	return d
}

// attachDirHandlers wires the Directory tab's callbacks. Kept separate from
// newDirTab to match the build-then-wire order main.go relies on.
func attachDirHandlers(widgets *uiWidgets) {
	d := widgets.dir
	d.globals = func() [4]bool {
		return [4]bool{
			widgets.archive.Get() == "1",
			widgets.upload.Get() == "1",
			widgets.mkdir.Get() == "1",
			widgets.del.Get() == "1",
		}
	}

	Bind(d.tree, "<<TreeviewOpen>>", Command(func() {
		id := d.tree.Focus()
		if id == "" {
			return
		}
		if !d.loaded[id] {
			d.fill(id)
		}
		d.opened[d.paths[id]] = true
	}))

	Bind(d.tree, "<<TreeviewClose>>", Command(func() {
		if id := d.tree.Focus(); id != "" {
			d.opened[d.paths[id]] = false
		}
	}))

	swallowExtraClicks(d.tree)
	overrideTreeBindings(d.tree)

	Bind(d.tree, "<Key>", Command(func(e *Event) { d.typeAhead(e.Char) }))

	// A notebook unmaps the tab it leaves, so <Map> fires whenever the Directory
	// tab comes to the front: claim the keyboard so type-ahead works without
	// having to click the tree first.
	Bind(d.tree, "<Map>", Command(func() { Focus(d.tree) }))

	Bind(d.tree, "<<TreeviewSelect>>", Command(func() {
		sel := d.tree.Selection("")
		if len(sel) == 0 {
			d.sel = ""
			d.updateSelection()
			return
		}
		// overrideTreeBindings keeps plain clicks and the arrow keys off label rows,
		// but a modified click (Shift-Button-1, <<ToggleSelection>>) reaches the class
		// binding directly — hold the previous selection rather than blank the pane.
		if path, ok := d.paths[sel[0]]; ok {
			d.sel = path
		}
		d.updateSelection()
	}))

	d.refresh.Configure(Command(func() {
		d.rebuild(widgets.root.Textvariable(), true)
	}))

	for i, bit := range permOrder {
		i, bit := i, bit
		d.checks[i].Configure(Command(func() {
			if d.sel == "" {
				return
			}
			d.perms.set(d.sel, bit, d.vars[i].Get() == "1")
			d.refreshAbbrs()
			d.updateSelection()
		}))
	}

	// Rebuilding on tab change rather than on every keystroke in the Root entry
	// keeps the disk out of the typing path. Which tab was selected is not
	// checked: the rebuild only happens when Root actually changed, so at worst
	// it costs one os.ReadDir on the way to some other tab, and not comparing
	// widget path strings keeps this independent of tk9.0's naming.
	// The Hide pattern is picked up here too, and before the rebuild: insert
	// marks rows as it creates them, so a changed pattern has to be compiled
	// first, and a rebuild then leaves nothing for refreshHidden to do.
	Bind(widgets.nb, "<<NotebookTabChanged>>", Command(func() {
		hideChanged := d.setHide(widgets.hide.Textvariable())
		if root := nativePath(widgets.root.Textvariable()); root != d.shownRoot {
			d.rebuild(root, false)
		} else if hideChanged {
			d.refreshHidden()
		}
	}))
}

// swallowExtraClicks stops a fast click sequence past the second click from
// toggling the row again. ttk::treeview binds <Button-1> and <Double-Button-1>
// but nothing longer, and Tk falls back to the <Double-Button-1> binding for
// the third and every following click of a rapid sequence — so double-clicking
// a directory twice in a row expands and immediately collapses it. The break
// has to sit on the widget's own bindtag, which Tk runs before the class one;
// tk9.0's Bind cannot return break, hence the raw Tcl. Quadruple covers the
// fifth click onwards too, since Tk counts no higher.
func swallowExtraClicks(w Widget) {
	tclEval(fmt.Sprintf("bind %s <Triple-Button-1> {break}", w))
	tclEval(fmt.Sprintf("bind %s <Quadruple-Button-1> {break}", w))
}

// overrideTreeBindings replaces the treeview bindings that either have to know
// about label rows or that Tk leaves half-finished. Each one is installed on the
// widget's own bindtag, which Tk runs before the class one, and each breaks so
// the class binding does not also fire. That is also why this is Tcl and not Go
// handlers — tk9.0's Bind cannot return break.
//
//   - <Button-1>: a click must not select a label row, and ttk::treeview has no
//     per-item "not selectable" option.
//   - <Up>/<Down>: must step over label rows, and must start somewhere when no
//     row holds the focus yet — Keynav returns immediately in that case, which
//     left the arrow keys dead on a freshly opened tab while PageUp/PageDown,
//     not being focus-relative, worked.
//   - <Prior>/<Next>: Tk leaves these a bare `yview scroll ±1 pages`, so the
//     selection slides off screen and the arrows resume out of sight.
//
// <Return>/<space> need nothing — they reach Toggle, which returns early for a
// childless item — and there are no Home/End bindings.
func overrideTreeBindings(w Widget) {
	// `tag has` errors on an item that does not exist (ttkTreeview.c, FindItem),
	// and every caller below can hand it an empty one: identify past the last row,
	// focus while the tree is still empty.
	tclEval(fmt.Sprintf(`proc %s {w item} {
		expr {$item ne "" && [$w tag has %s $item]}
	}`, hintTestProc, hintTag))

	// The topmost row that is a directory, scanned down from the top edge because
	// the first pixels belong to the heading, where identify yields nothing.
	tclEval(fmt.Sprintf(`proc %s {w} {
		for {set y 0} {$y < 200} {incr y} {
			set item [$w identify item 4 $y]
			if {$item ne "" && ![%s $w $item]} { return $item }
		}
		return ""
	}`, topRowProc, hintTestProc))

	// BrowseTo is Tk's own see + focus + selection, so a row lands in exactly the
	// state an arrow key would have left it — including the focus that the next
	// Up/Down continues from.
	tclEval(fmt.Sprintf(`proc %s {w} {
		set item [%s $w]
		if {$item ne ""} { ttk::treeview::BrowseTo $w $item "" }
	}`, browseTopProc, topRowProc))

	// Tk falls back to this binding for the second click of a double-click too,
	// since the widget bindtag has no <Double-Button-1>, so double-clicking a label
	// is inert as well.
	tclEval(fmt.Sprintf(`bind %s <Button-1> {
		if {[%s %%W [%%W identify item %%x %%y]]} { break }
	}`, w, hintTestProc))

	// Repeating Keynav clears a label row: one is always its parent's only child,
	// so a second step always leaves it — down carries on past the parent, up
	// returns to it — which is why no arrival direction has to be tracked.
	for _, nav := range [2][2]string{{"Up", "up"}, {"Down", "down"}} {
		event, dir := nav[0], nav[1]
		tclEval(fmt.Sprintf(`bind %s <%s> {
			if {[%%W focus] eq ""} {
				%s %%W
			} else {
				ttk::treeview::Keynav %%W %s
				if {[%s %%W [%%W focus]]} { ttk::treeview::Keynav %%W %s }
			}
			break
		}`, w, event, browseTopProc, dir, hintTestProc, dir))
	}

	// Scroll a page, then take whatever row is now at the top — the same trick Tk's
	// listbox uses (listbox.tcl), and it means no page height in rows has to be
	// worked out, since the widget just scrolled by exactly that much.
	for _, nav := range [2][2]string{{"Prior", "-1"}, {"Next", "1"}} {
		event, dir := nav[0], nav[1]
		tclEval(fmt.Sprintf("bind %s <%s> {%%W yview scroll %s pages; %s %%W; break}",
			w, event, dir, browseTopProc))
	}
}

// rebuild reconstructs the tree from root. With keepState the expansion,
// selection and scroll position are carried over; without it (root changed)
// they are dropped and stale grants are pruned.
func (d *dirTab) rebuild(root string, keepState bool) {
	// Normalized once here so shownRoot, the row paths and the dirPerms keys are
	// all the same spelling — a hand-typed "D:/x" would otherwise never match
	// the cleaned form and rebuild on every tab change.
	root = nativePath(root)
	top := d.topVisiblePath()

	for _, id := range d.tree.Children("") {
		d.forget(id)
	}
	d.paths = map[string]string{}
	d.ids = map[string]string{}
	d.loaded = map[string]bool{}

	if !keepState {
		d.opened = map[string]bool{}
		d.sel = ""
		top = ""
		d.perms.prune(root)
	}

	d.shownRoot = root
	d.setEntry(d.rootEnt, root)
	if root == "" {
		d.updateSelection()
		return
	}

	// Root's subdirectories are the top-level rows; Root itself gets no row,
	// which is why fill has to accept "" as a node.
	d.fill("")
	d.restoreExpansion("")
	d.restoreSelection()
	d.restoreScroll(top)
	d.updateSelection()
}

// insert adds one directory row plus a placeholder child, which is what makes
// Tk draw an expand arrow; the real children are read on <<TreeviewOpen>>.
func (d *dirTab) insert(parentID, dir, label string) string {
	id := d.tree.Insert(parentID, "end", Txt(label), Values([]string{d.perms.abbr(dir)}))
	d.paths[id] = dir
	d.ids[dir] = id
	if d.isHidden(dir) {
		d.tree.TagAdd(hiddenTag, id)
	}
	d.tree.Insert(id, "end", Txt(""))
	return id
}

// isHidden reports whether Hide keeps this row out of its parent's listing.
//
// Two limits are deliberate. ghfs matches the base name only, so a pattern
// marks every directory of that name at any depth, exactly as the server
// filters them. And only the matching directory is marked, not its descendants:
// they are unreachable by *browsing* down from a hidden parent, but each is
// still served, and claiming otherwise would overstate what the pattern did.
func (d *dirTab) isHidden(dir string) bool {
	return d.hide != nil && d.hide.MatchString(filepath.Base(dir))
}

// setHide recompiles the filter from the Hide entry, reporting whether it
// changed. The text is compared rather than the pattern so that recompiling —
// and the redraw it implies — happens only on an actual edit.
func (d *dirTab) setHide(text string) bool {
	if text == d.shownHide {
		return false
	}
	d.shownHide = text
	d.hide = newHideFilter(parseMultiValues(text))
	return true
}

// refreshHidden re-marks every loaded row after the pattern changed. Rows are
// cleared in one call — `tag remove` with no items clears the whole tree — and
// no directory is read again, so this stays off the disk.
func (d *dirTab) refreshHidden() {
	tclEval(fmt.Sprintf("%s tag remove %s", d.tree, hiddenTag))
	if d.hide == nil {
		return
	}
	for id, dir := range d.paths {
		if d.isHidden(dir) {
			d.tree.TagAdd(hiddenTag, id)
		}
	}
}

// dirOf returns the directory a node stands for. The empty id is the tree's own
// root, which has no row and therefore no paths entry: it stands for Root.
func (d *dirTab) dirOf(id string) string {
	if id == "" {
		return d.shownRoot
	}
	return d.paths[id]
}

// fill replaces a node's placeholder with its subdirectories.
func (d *dirTab) fill(id string) {
	d.loaded[id] = true
	for _, kid := range d.tree.Children(id) {
		d.forget(kid)
	}

	dir := d.dirOf(id)
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Unreadable directories are ordinary (permissions), and browsing near
		// /root or /etc would otherwise mean a dialog per node. Grey the row
		// instead, so "empty" stays distinguishable from "can't look" — except
		// for Root, which has no row and is left to its hint alone.
		if id != "" {
			d.tree.TagAdd("unreadable", id)
		}
		d.insertHint(id, hintUnreadable)
		return
	}

	// Symlinked directories are not followed: DirEntry.IsDir is false for them.
	// That also keeps the tree free of cycles.
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if e.IsDir() {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	if len(names) == 0 {
		d.insertHint(id, hintEmpty)
		return
	}
	for _, name := range names {
		d.insert(id, filepath.Join(dir, name), name)
	}
}

// insertHint puts a label row under id, deliberately absent from d.paths/d.ids.
func (d *dirTab) insertHint(parentID, text string) {
	d.tree.Insert(parentID, "end", Txt(text), Tags(hintTag))
}

// forget deletes an item and drops it and its descendants from the maps.
func (d *dirTab) forget(id string) {
	for _, kid := range d.tree.Children(id) {
		d.forget(kid)
	}
	if dir, ok := d.paths[id]; ok {
		delete(d.ids, dir)
		delete(d.paths, id)
	}
	delete(d.loaded, id)
	d.tree.Delete(id)
}

func (d *dirTab) expand(id string) {
	if !d.loaded[id] {
		d.fill(id)
	}
	d.tree.Item(id, Open(true))
	d.opened[d.paths[id]] = true
}

func (d *dirTab) restoreExpansion(id string) {
	for _, kid := range d.tree.Children(id) {
		dir, ok := d.paths[kid]
		if !ok || !d.opened[dir] {
			continue
		}
		d.expand(kid)
		d.restoreExpansion(kid)
	}
}

func (d *dirTab) restoreSelection() {
	if d.sel == "" {
		return
	}
	id, ok := d.ids[d.sel]
	if !ok {
		d.sel = ""
		return
	}
	d.tree.Selection("set", id)
}

// topVisiblePath returns the path of the topmost visible row, used as the
// anchor for restoring the scroll position across a rebuild.
func (d *dirTab) topVisiblePath() string {
	if len(d.tree.Children("")) == 0 {
		return ""
	}
	// Scan down from the top edge; the first rows are the heading, where
	// IdentifyItem yields nothing. Label rows have no path to anchor on, so take
	// the first real directory below them.
	for y := 0; y < 200; y++ {
		id := d.tree.IdentifyItem(4, y)
		if id == "" {
			continue
		}
		if path, ok := d.paths[id]; ok {
			return path
		}
	}
	return ""
}

// restoreScroll puts the anchor row back at the top. tk9.0 exposes Yview only
// on TextWidget, so this scrolls past the anchor and back to it: See scrolls
// minimally, so approaching from below leaves the anchor at the top edge.
// Anchoring on a path rather than a fraction also survives directories being
// added or removed between refreshes.
func (d *dirTab) restoreScroll(top string) {
	if top == "" {
		return
	}
	id, ok := d.ids[top]
	if !ok {
		return
	}
	if last := d.lastVisibleItem(); last != "" {
		d.tree.See(last)
	}
	d.tree.See(id)
}

// lastVisibleItem walks the trailing edge of the expanded tree.
func (d *dirTab) lastVisibleItem() string {
	kids := d.tree.Children("")
	if len(kids) == 0 {
		return ""
	}
	id := kids[len(kids)-1]
	for {
		if !d.opened[d.paths[id]] {
			return id
		}
		kids = d.tree.Children(id)
		if len(kids) == 0 {
			return id
		}
		id = kids[len(kids)-1]
	}
}

// typeAhead jumps to the next visible directory whose name starts with what has
// been typed, the way an OS file browser does. Keystrokes accumulate until
// typeAheadTimeout passes without one; repeating a single character cycles
// through the entries beginning with it instead of searching for "aa".
// ttk::treeview binds only the arrow keys, Page Up/Down and Return/space, so
// there is no built-in behaviour to collide with.
func (d *dirTab) typeAhead(ch string) {
	r, size := utf8.DecodeRuneInString(ch)
	// Non-character keys (arrows, modifiers) arrive as an empty string, and
	// space belongs to the class binding's expand/collapse toggle.
	if size == 0 || r == ' ' || !unicode.IsPrint(r) {
		return
	}

	// Expiring lazily rather than on a Tk timer: the buffer is only ever read
	// from here, so the two are equivalent, and TclAfter would register a
	// callback per keystroke that tk9.0 never unregisters.
	now := time.Now()
	if now.Sub(d.searchAt) > typeAheadTimeout {
		d.searchBuf = nil
	}
	d.searchAt = now

	current := d.tree.Focus()
	if len(d.searchBuf) == 0 {
		d.searchStart = current
	}
	d.searchBuf = append(d.searchBuf, r)

	// Searching from where the sequence began, rather than from the row the
	// previous keystroke landed on, keeps "do" able to match the same row "d"
	// just matched.
	needle, from := string(d.searchBuf), d.searchStart
	if allSameRune(d.searchBuf) {
		needle, from = string(d.searchBuf[0]), current
	}

	if id := d.findRow(needle, from); id != "" {
		d.tree.Selection("set", id)
		d.tree.Focus(id)
		d.tree.See(id)
	}
}

// findRow scans the visible rows forward from `from`, wrapping around, for the
// first directory whose name starts with needle, case-insensitively.
func (d *dirTab) findRow(needle, from string) string {
	rows := d.visibleRows()
	if len(rows) == 0 {
		return ""
	}
	needle = strings.ToLower(needle)

	start := 0
	for i, id := range rows {
		if id == from {
			start = i + 1
			break
		}
	}

	for i := range rows {
		id := rows[(start+i)%len(rows)]
		if strings.HasPrefix(strings.ToLower(filepath.Base(d.paths[id])), needle) {
			return id
		}
	}
	return ""
}

// visibleRows lists item ids top to bottom as displayed, descending only into
// expanded nodes. Collapsed subtrees are skipped: their contents have not been
// read from disk, and searching them would mean walking the whole filesystem.
func (d *dirTab) visibleRows() []string {
	var rows []string
	var walk func(parent string)
	walk = func(parent string) {
		for _, id := range d.tree.Children(parent) {
			// Label rows carry no path, so there is no name to match them on.
			if _, ok := d.paths[id]; !ok {
				continue
			}
			rows = append(rows, id)
			if d.opened[d.paths[id]] {
				walk(id)
			}
		}
	}
	walk("")
	return rows
}

func allSameRune(rs []rune) bool {
	for _, r := range rs {
		if r != rs[0] {
			return false
		}
	}
	return len(rs) > 0
}

// refreshAbbrs re-renders the abbreviation column. Toggling one directory can
// change what its descendants inherit, so every loaded row is redrawn.
func (d *dirTab) refreshAbbrs() {
	for id, dir := range d.paths {
		d.tree.Item(id, Values([]string{d.perms.abbr(dir)}))
	}
}

// updateSelection redraws the right pane. A box shows what the directory grants
// on its own; a permission already arriving globally or from an ancestor only
// adds a hint, and stays clickable — ghfs merges the grants itself.
func (d *dirTab) updateSelection() {
	want := paneState{valid: true}

	var globals [4]bool
	if d.globals != nil {
		globals = d.globals()
	}

	// A global switch says the same thing about every directory, so it is
	// reported without waiting for one to be picked.
	if d.sel == "" {
		d.setEntry(d.pathEnt, noDirSelected)
		for i := range want.state {
			want.state[i] = "disabled"
			if globals[i] {
				want.hint[i] = "(granted globally)"
			}
		}
		d.applyPane(want)
		return
	}

	d.setEntry(d.pathEnt, d.relPath(d.sel))
	own := d.perms.get(d.sel)
	inherited := d.perms.inherited(d.sel)

	for i, bit := range permOrder {
		state, hint := "normal", ""
		switch {
		case globals[i]:
			hint = "(granted globally)"
		case inherited&bit != 0:
			hint = "(inherited from parent)"
		}
		if d.locked {
			state = "disabled"
		}
		want.state[i], want.hint[i], want.checked[i] = state, hint, own&bit != 0
	}
	d.applyPane(want)
}

// applyPane writes only the parts of want the widgets are not already showing.
func (d *dirTab) applyPane(want paneState) {
	stale := !d.shown.valid
	for i := range d.checks {
		if stale || d.shown.checked[i] != want.checked[i] {
			setChecked(d.vars[i], want.checked[i])
		}
		if stale || d.shown.state[i] != want.state[i] {
			d.checks[i].Configure(State(want.state[i]))
		}
		if stale || d.shown.hint[i] != want.hint[i] {
			d.hints[i].Configure(Txt(want.hint[i]))
		}
	}
	d.shown = want
}

// relPath spells dir relative to Root, which the header states once for every
// row. filepath.Rel fails only on a path outside Root, which no row is.
func (d *dirTab) relPath(dir string) string {
	if d.shownRoot == "" {
		return dir
	}
	rel, err := filepath.Rel(d.shownRoot, dir)
	if err != nil {
		return dir
	}
	return rel
}

// setEntry writes text to a readonly entry, scrolled back to its start: the
// entry keeps the horizontal offset it had when the text changes, which would
// leave a newly written path showing from somewhere in its middle. tk9.0 wraps
// xview only for TextWidget, hence the raw Tcl call.
func (d *dirTab) setEntry(e *TEntryWidget, text string) {
	e.Configure(Textvariable(text))
	tclEval(fmt.Sprintf("%s xview 0", e))
}

// setDirTabLocked freezes the right pane while the server runs. The tree and
// refresh button are handled by lockedControls; the checkbuttons need this hook
// because updateSelection recomputes their state from scratch.
func (d *dirTab) setLocked(locked bool) {
	d.locked = locked
	d.updateSelection()
}
