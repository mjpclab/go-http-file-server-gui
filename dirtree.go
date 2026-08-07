package main

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	. "modernc.org/tk9.0"
)

// dirTab is the Directory tab: a lazily populated tree of the directories under
// Root on the left, and the four permission toggles for the selected directory
// on the right.
type dirTab struct {
	frame   *TFrameWidget
	tree    *TTreeviewWidget
	refresh *TButtonWidget
	pathLbl *TLabelWidget
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

	shownRoot string // root the tree was last built from
	sel       string // selected directory
	locked    bool   // server running: keep the right pane disabled

	searchBuf   []rune    // type-ahead: characters typed so far
	searchStart string    // item the current type-ahead sequence started from
	searchAt    time.Time // when the last character was typed
}

const noDirSelected = "(no directory selected)"

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

	// Left: directory tree with a scrollbar and a manual refresh.
	left := pane.TFrame()
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

	d.refresh = left.TButton(Txt("Refresh"))
	Grid(d.tree, Row(0), Column(0), Sticky("news"))
	Grid(sb, Row(0), Column(1), Sticky("ns"))
	Grid(d.refresh, Row(1), Column(0), Columnspan(2), Sticky("w"), Pady("1m"))
	GridRowConfigure(left, 0, Weight(1))
	GridColumnConfigure(left, 0, Weight(1))

	// Right: permissions for the selected directory.
	right := pane.TFrame(Padding("2m"))
	d.pathLbl = right.TLabel(Txt(noDirSelected))
	Grid(d.pathLbl, Row(0), Column(0), Columnspan(2), Sticky("w"), Pady("1m"))
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

	Bind(d.tree, "<Key>", Command(func(e *Event) { d.typeAhead(e.Char) }))

	// A notebook unmaps the tab it leaves, so <Map> fires whenever the Directory
	// tab comes to the front: claim the keyboard so type-ahead works without
	// having to click the tree first.
	Bind(d.tree, "<Map>", Command(func() { Focus(d.tree) }))

	Bind(d.tree, "<<TreeviewSelect>>", Command(func() {
		sel := d.tree.Selection("")
		if len(sel) == 0 {
			d.sel = ""
		} else {
			d.sel = d.paths[sel[0]]
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
	Bind(widgets.nb, "<<NotebookTabChanged>>", Command(func() {
		if root := widgets.root.Textvariable(); root != d.shownRoot {
			d.rebuild(root, false)
		}
	}))
}

// rebuild reconstructs the tree from root. With keepState the expansion,
// selection and scroll position are carried over; without it (root changed)
// they are dropped and stale grants are pruned.
func (d *dirTab) rebuild(root string, keepState bool) {
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
	if root == "" {
		d.updateSelection()
		return
	}

	rootID := d.insert("", filepath.Clean(root), filepath.Clean(root))
	d.expand(rootID)
	d.restoreExpansion(rootID)
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
	d.tree.Insert(id, "end", Txt(""))
	return id
}

// fill replaces a node's placeholder with its subdirectories.
func (d *dirTab) fill(id string) {
	d.loaded[id] = true
	for _, kid := range d.tree.Children(id) {
		d.forget(kid)
	}

	dir := d.paths[id]
	entries, err := os.ReadDir(dir)
	if err != nil {
		// Unreadable directories are ordinary (permissions), and browsing near
		// /root or /etc would otherwise mean a dialog per node. Grey the row
		// instead, so "empty" stays distinguishable from "can't look".
		d.tree.TagAdd("unreadable", id)
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
	for _, name := range names {
		d.insert(id, filepath.Join(dir, name), name)
	}
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
	// IdentifyItem yields nothing.
	for y := 0; y < 200; y++ {
		if id := d.tree.IdentifyItem(4, y); id != "" {
			return d.paths[id]
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

// updateSelection redraws the right pane. A permission already granted globally
// or by an ancestor is shown checked and disabled: ghfs grants are additive, so
// letting the user clear such a box would promise a revocation that cannot happen.
func (d *dirTab) updateSelection() {
	if d.sel == "" {
		d.pathLbl.Configure(Txt(noDirSelected))
		for i := range d.checks {
			setChecked(d.vars[i], false)
			d.checks[i].Configure(State("disabled"))
			d.hints[i].Configure(Txt(""))
		}
		return
	}

	d.pathLbl.Configure(Txt(d.sel))
	own := d.perms.get(d.sel)
	inherited, from := d.perms.inherited(d.sel)

	var globals [4]bool
	if d.globals != nil {
		globals = d.globals()
	}

	for i, bit := range permOrder {
		state, hint := "normal", ""
		checked := own&bit != 0
		switch {
		case globals[i]:
			state, hint, checked = "disabled", "(granted globally)", true
		case inherited&bit != 0:
			state, hint, checked = "disabled", "(inherited from "+from[bit]+")", true
		}
		if d.locked {
			state = "disabled"
		}
		setChecked(d.vars[i], checked)
		d.checks[i].Configure(State(state))
		d.hints[i].Configure(Txt(hint))
	}
}

// setDirTabLocked freezes the right pane while the server runs. The tree and
// refresh button are handled by lockedControls; the checkbuttons need this hook
// because updateSelection recomputes their state from scratch.
func (d *dirTab) setLocked(locked bool) {
	d.locked = locked
	d.updateSelection()
}
