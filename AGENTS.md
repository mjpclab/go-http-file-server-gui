# AGENTS.md

Guidance for coding agents working in this repository.

## Project Overview

A desktop GUI that wraps the `mjpclab.dev/ghfs` (go-http-file-server) library. The window lets users configure and launch an HTTP file server — root dir, listen address, optional TLS, archive/upload/mkdir/delete permissions — from a single Tk window.

The UI is built on `modernc.org/tk9.0`, a CGo-free Tk binding that embeds the Tcl/Tk runtime. There is no C toolchain dependency for builds (CGO is off everywhere).

## Common Commands

```sh
# Run from source. On Linux an X11 display must be available at runtime.
go run .

# Plain build (CGO off — the Tcl/Tk runtime is embedded by modernc.org/tk9.0
# and extracted to ~/.cache/modernc.org on first launch).
CGO_ENABLED=0 go build .

# Single self-contained binary for the current platform (-trimpath, -s -w).
bash build/build-current.sh

# Cross-compile every target (linux/{amd64,arm64}, windows/{386,amd64,arm64},
# darwin/{amd64,arm64}) into output/. Windows builds add `-H windowsgui` to
# suppress the console window.
bash build/build-all.sh

# Wrap the linux binaries from output/ into .deb/.rpm/Arch/apk packages (nfpm).
bash build/pack-linux.sh
```

```sh
# Tests. Only the Tk-free logic is covered (dirperm_test.go, version_test.go,
# multivalue_test.go, hidefilter_test.go) —
# the widget code needs a display and is not tested.
go test ./...
go test -run TestPrune ./...   # single test
go vet ./...
```

## Architecture

Single `main` package. Cooperating files share a `*uiWidgets` struct as the wiring backbone:

- `main.go` — six-line entry point: `initTcl()` → `applySystemTheme()` → `newUI()` → `loadPreference()` → `attachHandlers()` → `App.Wait()` → `savePreference()`. Order matters: the theme must be active before widgets are built, and preferences must load before handlers attach so the embedded ghfs app sees the restored values. `savePreference` runs after `App.Wait()` returns (i.e. when the window closes) — and a second time from the Start handler, so a crash or a kill while serving does not lose the settings that were just used.
- `ui.go` — builds the entire Tk widget tree using tk9's dot-imported DSL (`. "modernc.org/tk9.0"`) and returns `*uiWidgets`. Widgets are constructed here with no behavior — `Command(...)` callbacks are wired up later in `handler.go`. The form lives in a `TNotebook` with six tabs: **General** (root, listen, the global permission checkbuttons, dir index, hide, the Access/Error log switches), **Directory** (per-directory permissions, built by `newDirTab` in `dirtree.go`), **Advanced** (listen plain/TLS, TLS cert/key), **Links** (clickable access URLs), **Logs** (the server's access and error logs, built by `newLogsTab` in `logs.go`) and **About** (version information, built by `newAboutTab` in `about.go`). Roughly the first half of the file is not widget construction at all but theme patching (`useLargerThemeTiles`, `useBuiltinTreeIndicator` and their tables) — see "Theming". Also defines `applySystemTheme` (see below), `newMainWindow` (see "Window identity" below), `formRow`/`formEntryRow` (label : entry [...] grid helpers, the second spanning the pick button's column so entries of both kinds end flush), and `resizeWindow` (sizes and re-centers the window from a caller-supplied size, no event-loop round-trip; the `winWidth/winHeight` defaults must stay above `minWidth/minHeight` or the window opens already clamped). The icon is `//go:embed Icon.png` and applied via `IconPhoto`.
- `preference.go` — load/save the form fields to `~/<UserConfigDir>/ghfs-gui/preference.json` via `encoding/json`. The default listen address is `8080`. Any field added to `uiWidgets` that should persist needs explicit entries in **both** `loadPreference` and `savePreference`, plus a JSON-tagged field on the `preference` struct. Per-directory grants round-trip through `dirPerms.toJSON`/`dirPermsFromJSON` and are pruned against Root on both load and save. The window size is persisted too (see below).
- `handler.go` — attaches five handler groups: window size tracking (`attachWindowHandlers`), browse buttons (`ChooseDirectory` for root, `GetOpenFile` for TLS cert/key), global permission checkbuttons (they re-render the Directory tab), the Directory tab (`attachDirHandlers`, in `dirtree.go`), and start/stop. `attachStartStopHandlers` holds the running `*app.App` instance in a closure; Start launches `appInst.Open()` on a goroutine and toggles UI state; Stop calls `appInst.Close()`. Because `Open` blocks while serving, post-stop UI updates are marshaled back to the GUI thread via `PostEvent(..., false)`.
- `multivalue.go` — `parseMultiValues`, the one place a form field holding several values is split into a `[]string` for ghfs. Spaces and commas both separate, in any combination and any number, so `8080 8081`, `8080,8081` and `8080, 8081` are the same input. A value that *opens* with a double quote runs to the closing quote, so its separators are literal: `"My Docs*"` is one `Hide` wildcard, `"cache-control:public, max-age=0"` one header entry. Three things about that rule are deliberate and easy to "fix" wrongly:

  - **Only the opening position is special.** A quote anywhere else is an ordinary character, which is the entire escape story — there is no backslash and no doubled-quote form. The payoff is that a header value which is itself quoted round-trips as typed: `etag:"abc123"` yields exactly that, verified end to end against ghfs (`Etag: "abc123"` on the wire). Adding an escape character would take that away, and a backslash in particular would collide with the Windows paths that `Hide` and `Index Page` carry.
  - **The cost is the mirror image:** a value cannot *begin* with a literal quote. Accepted knowingly — no ghfs field has a value starting with `"`.
  - **Degenerate quoting is lossless, not an error.** `parseMultiValues` has no error channel and is called on every Start, so text after the closing quote joins the same value (`"a"b` → `ab`) and an unclosed quote runs to the end of the field (`"a b` → `a b`). Dropping the input instead would fail silently, which is exactly the trap the Headers field already sets (see below).

    That `"a"b` → `ab` coincides with POSIX shell quote removal, but **do not generalize it to the shell's full rule**: the shell also folds `a"b"` into `ab`, and here the leading case deliberately does not (`a"b` stays `a"b`). The asymmetry *is* the escape mechanism from the first bullet — making the opening position ordinary too would take `etag:"abc123"` away.

  An empty field yields `nil`, not `[""]`, which is what makes an empty `Listen` fall back to ghfs's `:80`/`:443` instead of failing to bind an empty address. Free of any Tk dependency and tested by `multivalue_test.go`.
- `hidefilter.go` — `newHideFilter`, which compiles the Hide wildcards into the regexp ghfs matches names against so the Directory tab can mark the rows the server will leave out of its listings. It mirrors `serverHandler.wildcardToRegexp` (unexported), sharing its one meaningful step, the exported `util.WildcardToStrRegexp` — that anchors each pattern at both ends and carries the build tags making the match case-insensitive on windows/darwin, so the tree agrees with the server per platform for free. `nil` means "nothing is filtered", and a pattern that will not compile yields `nil` too rather than an error: Start surfaces ghfs's own complaint, and until then marking nothing beats marking the wrong rows. Free of any Tk dependency and tested by `hidefilter_test.go`.
- `dirperm.go` — the permission model, free of any Tk dependency (one of the four tested files, with `version.go`, `multivalue.go` and `hidefilter.go`). `perm` is a bit set (`permArchive/permUpload/permMkdir/permDelete/permCors/permList`, `permCount` wide); `permOrder` fixes the order used by every parallel array and by the tree's abbreviation column. `dirPerms` maps a cleaned absolute path to its explicit grants and answers `get`/`inherited`/`abbr`/`dirsWith`/`prune`. `hasPathPrefix` deliberately mirrors ghfs's `util.HasFsPrefixDir` (separator-boundary rule, case-insensitive on windows/darwin) — `dirperm_test.go` cross-checks the two, and they must stay in step or the UI's inheritance display would disagree with what the server enforces. Also holds `nativePath` (see "Path spelling" below).
- `dirtree.go` — the Directory tab: a `TPanedwindow` with a lazily populated `TTreeview` of the directories under Root on the left (children read on `<<TreeviewOpen>>`, symlinks not followed, unreadable dirs greyed via the `unreadable` tag instead of raising a dialog) and the per-directory checkbuttons on the right. Details worth knowing before touching it:
  - **Root has no row.** The top-level rows are Root's subdirectories, so the tree has as many roots as Root has children. In code this makes `""`, the treeview's own root, a node like any other: `fill("")` reads Root, `dirOf` maps `""` back to `shownRoot`, and the `unreadable` tag is skipped for it since there is no row to tag — only its hint row reports the failure.
  - **Header.** Above the tree, a readonly `TEntry` showing the Root the tree was built from, with the refresh button beside it — labelled `↻` plus a `Tooltip` rather than the word, so it leaves that entry as much width as possible.
  - **The right pane shows the selected path relative to Root** via `relPath`, since the header states the prefix once for every row. Only the display is relative; `dirPerms` keys stay absolute.
  - **Readonly entries, not labels.** Both the header's Root and the right pane's selected path are readonly `TEntry`s so a path wider than the pane can be scrolled sideways and copied; `setEntry` scrolls one back to its start via `tclEval` (tk9.0 wraps `xview` only for `TextWidget`).
  - **Label ("hint") rows.** A directory with nothing to list still gets a child row (`hintEmpty` / `hintUnreadable`), because ttk::treeview derives "is a leaf" from the child list alone and would otherwise drop the expand arrow from an expanded empty directory. Label rows carry no `dirTab.paths` entry — that absence, not the `hintTag`, is what tells them apart from directories throughout the file.
  - **Binding overrides.** `overrideTreeBindings` installs raw-Tcl bindings on the widget's own bindtag (which Tk runs before the class one) for `<Button-1>`, `<Up>`/`<Down>` and `<Prior>`/`<Next>`: they must step over label rows, must start somewhere when no row has focus yet (Tk's `Keynav` returns immediately then, leaving the arrow keys dead on a freshly opened tab), and Tk's stock Page keys only scroll, sliding the selection off screen. Tcl rather than Go because tk9.0's `Bind` cannot return `break`. `swallowExtraClicks` separately breaks the third and following clicks of a rapid `<Button-1>` sequence, which Tk would otherwise keep matching against `<Double-Button-1>`, toggling the row once per extra click.
  - **Hidden rows.** A directory the Hide patterns keep out of its parent's listing is drawn in italic (`hiddenTag`, whose font is derived from the real `TkDefaultFont` via `font actual` so only the slant changes). Italic and not grey: `#888888` already means `unreadable` *and* "label row" here, and neither is what this is. The row stays selectable and its checkbuttons stay live **on purpose** — see "Hide is a listing filter, not access control" below. `insert` marks new rows, `refreshHidden` re-marks loaded ones after the pattern changes (`tag remove` with no items clears the tree in one call; tk9.0 wraps `TagAdd` but not its removal). The pattern is picked up in the `<<NotebookTabChanged>>` handler *before* the root comparison, since a rebuild tags rows as it inserts them.
  - **Right-pane diffing.** `updateSelection` computes a `paneState` and `applyPane` writes only what changed. Not just round-trip count: the hint labels have no fixed width, so rewriting one can resize a grid column → the frame → the paned window, relaying out the whole tree while an arrow key is held.
  - Also includes OS-file-browser-style type-ahead (`typeAhead`, 500 ms buffer matching Tk's own iconlist) and scroll/expansion/selection restoration across `rebuild`.
- `version.go` — version resolution, free of any Tk dependency and tested by `version_test.go`. Holds `appName` (shared by the window title and the About heading) and `aboutFrom`, the pure function behind `collectAbout`. See "Version reporting" below.
- `about.go` — the About tab: `newAboutTab` builds the icon/name/version header and the module rows, and `bindLink` (shared with `links.go`) wires a `Link.TLabel` to `openBrowser`.
- `links.go` — the Links tab (`linksTab`): one clickable `Link.TLabel` per access URL, `showPlaceholder` for the stopped state. After Start, `createLinks` (in `handler.go`) hands `appInst.GetAccessibleUrls(false)` to `linksTab.show` and brings the tab forward. The labels are packed into a plain frame that a `Canvas` carries as a single window item, since ttk has no scrollable container — `layout` stretches that frame to the canvas width, tracks the scrollregion and grids the `TScrollbar` only while the content is taller than the viewport, so a short list looks exactly like the bare frame this used to be. Toggling the scrollbar changes the canvas *width*, and the labels do not wrap, so the visibility test cannot oscillate. `bindWheel` is applied to **every widget covering part of the tab** — the tab frame, the canvas, the body frame and each label: Tk routes an event to the widget, its class, its toplevel and `all`, never to the enclosing frames, and a label is only as wide as its text, so binding the labels alone leaves the wheel dead everywhere except directly on a URL.
- `logs.go` — the Logs tab (`logsTab`): one disabled `Text` (still selectable) filling the tab, access and error lines interleaved in arrival order, error lines tagged in `logErrorColor` (set per theme by `applySystemTheme`). ghfs writes each line to an `io.Writer` from its own goroutine; `logWriter.Write` only queues it, and at most one `PostEvent(flush)` is in flight. Keep it that way: `PostEvent` blocks on a full Tk queue, and Stop runs `app.Close()` on the GUI thread, which waits for ghfs to drain its log queues into these writers — a post per line would deadlock under load. `accessLog`/`errorLog` must stay distinct values, since ghfs's `WriterMan` shares one queue between loggers handed the same writer. The view follows the tail only while it is already scrolled to the bottom, and the text is capped at `maxLogLines`.
- `winstate.go` — maximize/restore support plus `tclEval`, the raw-Tcl escape hatch used across the whole package. See "Window geometry persistence" and "Raw Tcl" below.
- `url_linux.go` / `url_darwin.go` / `url_windows.go` — platform-split `openBrowser`, selected by build tag via filename suffix. Linux uses `xdg-open`, macOS uses `open`, Windows uses `rundll32 url.dll,FileProtocolHandler`.

### Theming

`applySystemTheme` (in `ui.go`) initializes the `themedetector` tk9 extension, queries the OS light/dark preference, and activates the bundled `azure` theme (`_ "modernc.org/tk9.0/themes/azure"` is imported in `main.go`). It also configures the `Link.TLabel` style (underlined, blue) used for the access-URL links, and — in dark mode only — overrides the disabled `TButton` foreground via `StyleMap` so disabled buttons stay legible against the dark window background.

A third `StyleMap` greys the `TEntry` foreground in the `readonly` state, which is what makes the running-state lock visible: azure draws a readonly entry exactly like an editable one. The text is the only channel available — the field is an image element, not a `-fieldbackground`, so a greyed background would mean synthesizing a recoloured copy of `box-basic`. The map reaches every readonly entry, including the Directory tab's two permanently readonly display entries (the Root header and the selected path), where the same grey reads as "display only".

It then applies two patches to azure, both written as raw Tcl because tk9.0 exposes no API for style elements. Neither is load-bearing: `tclEval` swallows errors and the Tcl side re-checks its assumptions, so a moved-on theme just keeps drawing itself the old way.

- `useLargerThemeTiles` — a ttk image element with a `-border` is *tiled*, not stretched, so azure repaints an entry, a button, the notebook body, the tree and the tree heading from 20x20/50x50 images: hundreds of blits per widget per resize step. The five elements are restated over private copies grown to `tileSize` once at startup (13ms → 3.8ms per resize step at 1200x900). Three constraints are encoded in `tiledElements` and easy to break: each element name keeps the **last dotted component** of the one it replaces (`Ttk_FindElement` matches only the tail, so a `client` element must still be named `….client`); `-width`/`-height` pin the natural size back to the original, since a widget adds an element's image size to its own request; and the copies are private because azure shares images between elements (`box-basic` also backs the checkbutton indicator, which draws it whole). An element definition **replaces** azure's wholesale, so a state left out of the table stops rendering.
- `useBuiltinTreeIndicator` — the Directory tree's expand arrows. azure keys `Treeitem.indicator` on `user1`/`user2`, which is how Tk 8.6 spelled open/leaf; Tk 9.0 moved both to unnamed state bits, so the map never matches and every arrow points right. Tk's own C element reads the new bits, so it is borrowed under `treeIndicator` and spliced into the item layout. The name is constrained: no dot (`Ttk_GetElement` retries after each dot and would find azure's element again) and it must end in a lowercase `indicator`, which `ttk::treeview::Press` glob-matches to decide whether a click toggles a row.

### Raw Tcl

`tclEval` (in `winstate.go`) is the general escape hatch, not a window-state detail. It is used wherever tk9.0's typed API does not reach: style elements and layouts (`ui.go`), bindings that must return `break` or call into `ttk::treeview::` internals (`dirtree.go`), and `wm state` (`winstate.go`). It swallows Tcl errors on purpose — tk9.0's default `ErrorMode` panics — so every caller has to tolerate a no-op.

### Per-directory permissions

The Directory tab grants Archive/Upload/Mkdir/Delete/CORS/List on individual directories under Root, on top of the General tab's global switches. Two properties of ghfs shape the whole design and are easy to break:

- **Grants are additive and inherited by prefix.** A grant on a directory covers every descendant, and nothing can revoke it further down. The UI does **not** turn that into a restriction: a checkbutton always shows what the selected directory grants on its own and stays clickable when the same permission already arrives globally or from an ancestor, adding only a `(global)` / `(inherited)` hint (`hintGlobal`/`hintInherited`, kept terse because they share a paned-window-sized column with the checkbutton they annotate). ghfs merges the grants, so a redundant one costs nothing and the explicit grant survives the wider one being turned off — don't reintroduce the earlier checked-and-disabled behaviour. The hints show with no directory selected too, since a global switch says the same thing about every directory.
- **Grants name absolute filesystem paths.** They stop meaning anything once Root moves, hence `dirPerms.prune(root)` on load, on save, and on a Root change: a stale grant re-applied to a same-named directory elsewhere would silently expose different files.

The tree's second column shows an abbreviation per row — uppercase (`A U M D C L`) for a grant made on that directory, lowercase for one inherited. Globally granted permissions are deliberately excluded there, or every row would carry the same letters. Toggling one directory can change what its descendants inherit, so `refreshAbbrs` redraws every loaded row.

Rebuilding the tree is driven by `<<NotebookTabChanged>>` and only when Root actually changed, keeping `os.ReadDir` out of the typing path. The tree is never built for users who don't open the tab.

### Hide is a listing filter, not access control

Worth knowing before anyone "improves" the Directory tab's italic rows into greyed-out, non-interactive ones. `FilterItems` (ghfs `serverHandler/filter.go`) is called on one thing only: the `subItems` of the directory being listed (`sessionData.go`). It takes no part in access control. Verified against v1.21.7 with `hide=secret` and an upload+archive grant on that same directory:

| request | result |
|---|---|
| `GET /` | listing omits `secret` |
| `GET /secret/` | `200`, and lists `inner`, `a.txt` in full |
| ” | `canUpload: true`, `canArchive: true` — the grant is at full strength |
| `GET /secret/a.txt`, `/secret/inner/b.txt` | `200`, contents served |

So a hidden directory loses nothing but its line in the parent's listing. Disabling its checkbuttons would deny a grant that demonstrably works. Hence: italic marks it, nothing else changes.

The one place filtering does reach further is `archive.go`, which filters the children it packs, so a zip excludes hidden entries.

`Show` was in the General tab briefly and was removed. It is the same filter with the test inverted, which makes it a trap on directories: names like `docs` or `build` match no file-oriented wildcard, so `show=*.txt` empties the root listing entirely while every directory stays perfectly reachable by URL. `param.Param.Shows` is simply left unset.

### Path spelling

Tk's file dialogs return Tcl-style paths (`D:/Downloads` on Windows) while everything on the Go side — `dirPerms` keys, the tree rows, `hasPathPrefix`'s separator test — goes through `filepath`. Every path that enters a widget is therefore normalized with `nativePath` (`filepath.Clean`, except that `""` stays `""` rather than becoming `.`): both file pickers, `loadPreference`, `savePreference`, `dirTab.rebuild` and the `<<NotebookTabChanged>>` comparison. Skipping it anywhere shows two spellings of the same path on screen and, in the tab-change comparison, rebuilds the tree on every tab switch because a hand-typed `D:/x` never equals the cleaned `shownRoot`.

### Running-state lock

While the server is running, `setInputsEnabled` (in `handler.go`) puts text entries in `lockedInputs` into `"readonly"` (greyed text, still selectable/copyable — see "Theming" for where the grey comes from) and the controls in `lockedControls` (pick buttons, checkbuttons) into `"disabled"`. Both slices are populated in `newUI`. Stop reverses this. Two Directory tab widgets are intentionally *not* in those slices: `ttk::treeview` has no `-state` option (configuring one raises a Tcl error, which tk9.0 turns into a panic), and the per-directory checkbuttons are driven by `dirTab.setLocked` → `updateSelection` instead, because their enabled state also depends on whether a directory is selected.

### Version reporting

The About tab shows the app's own version, `mjpclab.dev/ghfs`, `modernc.org/tk9.0`, the Go toolchain and `GOOS/GOARCH`.

Everything but the app's own version comes from `runtime/debug.ReadBuildInfo()`, which survives `-trimpath` — no build-script support needed. The app's own version does need help: the VCS-derived `Main.Version` degrades to a pseudo-version (`v0.0.9-0.20260808044334-888df4e7986a+dirty`) whenever HEAD is not an exact, clean tag, and to `(devel)` with no VCS stamp at all. So `build-current.sh`/`build-all.sh` inject `-X main.appVersion=…`, and `aboutFrom` falls back to `Main.Version` when the variable is empty (`go run .`, plain `go build`).

Four scripts derive a version from git, in three ways that differ on purpose:

| Script | Expression | Why |
|---|---|---|
| `build-current.sh`, `build-all.sh` | `git describe --tags`, `sed 's/-[0-9]*-g/-/'` | full precision for display; nothing parses it, so the leading `v` is kept |
| `gen_syso.sh` | same, plus strip `v` | `goversioninfo` requires a leading digit |
| `pack-linux.sh` | `--abbrev=0`, strip `v` | rpm and apk reject `-` in a version |

The `sed` drops both git's commit count and the `g` it prefixes to the hash: `v0.0.9-3-g888df4e` becomes `v0.0.9-888df4e`. It is a no-op on an exact tag. `git describe` failures are swallowed (`2>/dev/null`), so a source tarball without git history still builds and falls back to the build info.

The About tab is static: it is built eagerly in `newUI` (unlike the Directory tab there is no `os.ReadDir` to defer — build info is an in-binary table), takes no part in `lockedInputs`/`lockedControls` (nothing there is editable) and persists nothing.

### Window identity

The form does not live in `.` (`App`) but in a child toplevel built by `newMainWindow`, kept as `uiWidgets.win`. The reason is `WM_CLASS`: Tk derives `.`'s from the interpreter's `argv0` at `Tk_Init`, tk9.0 sets neither and initializes Tk during package init, so there is no way in — `.` reports `"tk"/"Tk"`, and a second instance on the same X display gets `"tk #2"`. A desktop shell then sees two unrelated applications: KDE files each instance under its own task entry and falls back to guessing an icon. A child toplevel accepts `-class` (`wmClass`, `"ghfs-gui"`), which is identical in every instance and matches `StartupWMClass` in `build/ghfs-gui.desktop` — **the two constants have to stay equal**. Lowercase, against the X11 convention of a capitalized class, because it then equals the desktop entry's id exactly: Plasma reaches the entry by lowercasing the class and looking up `<class>.desktop`, so an exact match needs no case folding from whatever shell is asking. Verified on KDE/Plasma 6 (XWayland), with an installed desktop entry that carried no `StartupWMClass` at all: the taskbar groups both instances and takes the icon from the entry. Windows and macOS are untested.

`.` stays alive, withdrawn, as the root `App.Wait` waits on, which imposes two rules on new code:

- New toplevel widgets belong to `win`, not to the package-level constructors (`TButton()` and friends build children of `.`).
- Dialogs need an explicit `Parent(widgets.win)`; their default parent is `.`, which is off-screen.

`newMainWindow` also wires `WM_DELETE_WINDOW` to `Destroy(App)` — without it, closing the visible window would leave `App.Wait` blocking on a live `.` — and sets the icon on both windows, since tk9.0's `Wait` applies its own default icon to `.` unless something already claimed it.

### Window geometry persistence

`preference.json` carries `width`/`height`/`maximized`. `loadPreference` applies the size via `resizeWindow` (clamped to `minWidth/minHeight`, so a hand-edited file cannot produce an unusable window; zero means "never saved" and the default size is kept), then maximizes on top of it — so un-maximizing after a restart lands on the size the user had chosen.

Both are *tracked* by a `<Configure>` binding on `widgets.win` (`attachWindowHandlers`) rather than read back at save time: `savePreference` runs after `App.Wait()` returns, when the window no longer exists and any `winfo` call would panic. Two things the binding has to account for: it also fires for child widgets (their bindtags include the toplevel), hence the `e.EventWindow != widgets.win` filter, and while maximized the reported size is the screen's, so the size is recorded only when not maximized.

`winstate.go` holds the platform handling, because there is no single spelling for "maximized": X11 uses `wm attributes -zoomed`, Windows and macOS use `wm state zoomed`, and each errors out on the other platform. tk9.0 wraps `wm attributes` but not `wm state`, and evaluating Tcl directly is only exposed to extensions — hence `tclExtension`, a do-nothing extension registered solely to capture an `ExtensionContext`, and `tclEval`, which swallows errors so an unsupported window-manager request degrades instead of panicking (tk9.0's default `ErrorMode` panics on Tcl errors). `initTcl` must run from the main package: `InitializeExtension` walks the call stack and refuses otherwise. Restoring is deferred to `<Map>` (`maximizeWhenMapped`) since a WM may ignore a state request for a window it has not put on screen yet.

The X11 path is verified end to end; the `wm state` path for Windows/macOS is not exercised by any test here.

### Integration with ghfs

The GUI never touches HTTP itself. `createApp` in `handler.go` translates the form into a single-entry `[]param.Param`, then calls `param.NewParams(...)` followed by `app.NewWriterLogApp(params, …)`, handing it the Logs tab's writers for whichever of the General tab's **Log** switches are on (an untyped `nil` otherwise — a nil `*logWriter` in the interface would pass ghfs's nil test). `NewWriterLogApp` is not in a tagged ghfs release yet; the build resolves it through the `../go.work` workspace, which points at a local `go-http-file-server` checkout. The only default baked in is `DefaultSort: "/n"`. `GlobalList` is the "may list a directory" permission, driven by the General tab's **List** checkbutton, which is the one option that defaults to *on*: `loadPreference` seeds `List: true` so a preference file written before the option existed keeps the behaviour it had. It has nothing to do with the General tab's **Index Page** (`DirIndexes`), which names the file served *instead of* that listing — the similar names are ghfs's, not a typo. Every multi-value field (`Listens`, `ListensPlain`, `ListensTLS`, `DirIndexes`, `Hides`, `GlobalHeaders`) goes through `parseMultiValues`. An empty **Root** sets `EmptyRoot: true`: `param.NewParams` would otherwise run `filepath.Abs("")` and serve the working directory the GUI was launched from, while `EmptyRoot` rewrites Root to `os.DevNull` and maps `/` to it, so the server answers with an empty listing. TLS is enabled only when **both** cert and key paths are non-empty. The Advanced tab's **Headers** field adds one step after that split: each value is a `<name>:<value>` entry, turned into `[][2]string` by ghfs's exported `param.EntriesToKVs` — the same function `--global-header` uses, so the GUI and the CLI accept the same spelling and drop a malformed entry the same silent way. A value containing a space or comma is written with the quoting `parseMultiValues` provides — `"cache-control:public, max-age=0"`, quotes around the *whole* entry, not around the value alone. That silent dropping is a real trap: `cache-control: public,max-age=0` (note the space after the colon) parses to three tokens, none of them a valid entry, so Start succeeds and sets no header at all. Nothing in the UI reports it yet. Per-directory grants are passed as `ArchiveDirs`/`UploadDirs`/`MkdirDirs`/`DeleteDirs`/`CorsDirs`/`ListDirs` (filesystem paths) rather than the `*Urls` fields — with a single root the two are equivalent today, but a filesystem path names the directory itself and stays correct if aliases or vhosts are ever added.

### Adding a new form field

Touching multiple files is unavoidable by design:
1. Add the widget to the struct and construction in `ui.go` (and to `lockedInputs`/`lockedControls` if it should lock while running).
2. Add a JSON-tagged field on `preference` plus load/save lines in `preference.go` (both directions).
3. Map it into `param.Param` inside `createApp` in `handler.go`.

If the field holds a filesystem path, every write into the widget (picker result, loaded preference) goes through `nativePath` — see "Path spelling".

If the ghfs field is a `[]string`, step 3 wraps the entry text in `parseMultiValues` while step 2 stores that same text *unparsed*: the form is the source of truth, so the preference round-trips what the user typed instead of re-spelling it with one separator.

(The About tab is the exception that proves the rule: it is display-only, so it appears in none of the three.)

## Packaging notes

The window title (`"Go HTTP File Server GUI"`) and embedded `Icon.png` are set in `ui.go`. Platform-specific resources are pre-generated and committed; regenerate them with the helper scripts only when the icon or version changes:

- `rc_windows_{386,amd64,arm64}.syso` — Windows PE resources (icon + VERSIONINFO). Regenerate with `bash build/gen_syso.sh`, which runs `goversioninfo` against `Icon.ico` and derives the version from `git describe --tags`. The target machine type comes from `goversioninfo`'s `-64`/`-arm` pair, both derived from the GOARCH name (`386` = neither, `amd64` = `-64`, `arm64` = both). These `.syso` files are picked up automatically by `go build` for the matching Windows target. Since the version string includes the commit suffix when HEAD is ahead of the tag, run this **on the release tag**.
- `Icon.icns` — macOS icon, generated from `Icon.png` by `bash build/gen_icns.sh` (uses `icnsify`).
- `build/icons/setup.ico`, `build/icons/uninstall.ico` — the NSIS installer's and uninstaller's own icons (`MUI_ICON`/`MUI_UNICON`), *not* the installed app's. Generated by `bash build/gen_setup_ico.sh`: `Icon.ico` as the base, with NSIS's stock emblem (a download arrow, a red X) badged into the bottom-right corner at half the icon's width, in 16/32/48. Needs ImageMagick 7; the stock icons are located through `makensis -HDRINFO` (`NSISDIR=`), overridable with `NSISDIR=`. Four things in there are deliberate:
  - **Bottom-right is chosen knowingly**, not by oversight: both executables ask for elevation, so Windows draws its UAC shield over that corner and hides the emblem — and with it the only thing telling the installer's icon from the uninstaller's, the base being the same app icon in both. Wherever the shield is not drawn the emblem does hold that job down to 16px, where the arrow itself is illegible but an orange dot still reads differently from a red one.
  - **The emblem is taken from the stock icons' 16px frame**, which is separate artwork: the bare emblem circle with no package drawn around it. The 32px and 48px frames would drag the package in with it.
  - **48px is upscaled from 32px with nearest neighbour**, because `Icon.ico` and `Icon.png` are both 32x32 and there is no larger master. The smooth filters turn the pixel art to mush, and omitting the frame is no better — the shell then scales the 32px one itself. A higher-resolution app icon would lift this (and `.icns`, the `.syso` files and the Linux icon with it).
  - **The `PNG32:` intermediates and `-type TrueColorAlpha` on the output are load-bearing.** `Icon.ico` carries no colour of its own, so a plain PNG intermediate is written greyscale and desaturates the emblem composited onto it; and left to itself the ICO encoder takes the palette path, rewriting every partially transparent pixel to black. Frames are read back by size, taking the **last** match, since the 4-bit and 8-bit palette versions precede the 32-bit one that has a real alpha channel.

  `MUI_ICON` and `MUI_UNICON` do not have to share a frame layout — makensis accepts a 3-frame icon against the stock 9-frame one. Verified: the installer's three frames appear byte-for-byte in the compiled `setup.exe`. The uninstaller icon is only stored compressed and patched in at runtime, so it is unverified here beyond compiling.
- `bash build/pack-darwin.sh` — wraps the `output/ghfs-gui-darwin-*` binaries (run `build-all.sh` first) into double-clickable `.app` bundles with an `Info.plist`, then zips them (store-only). Requires `Icon.icns`.
- `bash build/pack-linux.sh` — wraps the `output/ghfs-gui-linux-{amd64,arm64}` binaries (run `build-all.sh` first) into `.deb`, `.rpm`, Arch (`.pkg.tar.zst`) and `.apk` packages via [nfpm](https://github.com/goreleaser/nfpm) — no dpkg/rpmbuild/makepkg needed on the host; the script `go install`s nfpm if it isn't already on `PATH`/in `GOPATH/bin`. Package metadata lives in `build/nfpm.yaml` (one config for all four formats, `${PKG_ARCH}`/`${VERSION}` expanded from the environment) and installs `/usr/bin/ghfs-gui`, `build/ghfs-gui.desktop` into `/usr/share/applications/`, and `Icon.png` into `/usr/share/icons/hicolor/32x32/apps/`. The version comes from `git describe --tags --abbrev=0` with the leading `v` stripped (rpm/apk reject `-` in versions); override with `VERSION=…`.

- `bash build/pack-windows.sh` — wraps the `output/ghfs-gui-windows-{amd64,arm64}.exe` binaries (run `build-all.sh` first) into NSIS installers, `output/ghfs-gui-windows-<arch>-setup.exe`. Needs `makensis` on `PATH` (Debian/Ubuntu: `apt install nsis`); unlike nfpm it cannot be fetched on demand, so the script errors out instead. The installer script is `build/installer.nsi`, driven by `-D` defines (`ARCH`, `VERSION`, `VIVERSION`, `SRCEXE`, `OUTFILE`). Notes on it:
  - **All paths go through `${ROOT}` (`..`)**, because makensis resolves relative paths against the *script's* directory, not the working directory. `SRCEXE`/`OUTFILE` are therefore passed repo-root-relative.
  - The file carries a **UTF-8 BOM** so the Simplified Chinese `LangString`s survive on a Windows makensis, which would otherwise read the source in the ANSI codepage. English is declared first and is the fallback; NSIS picks the match for the user's system language with no language prompt.
  - `MultiUser.nsh` gives the per-user (`%LocalAppData%\Programs\ghfs-gui`, default) / all-users (`$PROGRAMFILES64\ghfs-gui`) choice. `MULTIUSER_EXECUTIONLEVEL Highest` means an administrator account sees one UAC prompt at startup regardless of the choice — avoiding that needs the third-party UAC plugin, which base NSIS does not ship. `SetRegView 64` keeps the uninstall entry out of `WOW6432Node`. The scope has to be remembered through **two** define pairs pointing at the same `Software\ghfs-gui\InstallDir` value: `MULTIUSER_INSTALLMODE_INSTDIR_REGISTRY_*` only restores the directory within an already-chosen scope, while `MULTIUSER_INSTALLMODE_DEFAULT_REGISTRY_*` is what preselects the scope itself. Without the second pair every run — including the uninstaller, which shares the init path — falls back to per-user and an all-users install leaks its HKLM uninstall entry and common Start Menu shortcut.
  - No license page, and deliberately **no "run now" checkbox** on the finish page: an all-users install runs elevated, and the file server would inherit administrator rights.
  - The two optional tasks — a desktop shortcut and a Windows Firewall inbound rule — are `/o` sections on a **components page placed after the directory page**, so both are decided on the last page before any file is copied (they used to be one checkbox on the finish page). The firewall section shells out to `netsh advfirewall firewall add rule` for the `.exe` itself, not a port, so the rule survives a port change in the GUI; `profile=domain,private` deliberately leaves public networks out. Its exit code is logged rather than fatal — the files are already in place, and a missing rule only means Windows asks on first listen. `netsh` writes machine-wide, so a standard user, who is never elevated here, gets the section greyed out (`SF_RO`) with the reason in its label instead of a silent failure. The uninstaller deletes the rule unconditionally (matched on name **and** program path) since nothing records whether it was created. Both init functions sit below the sections because `.onInit` references `${SecFirewall}` — a section index only exists from its definition onwards.
  - The uninstaller has an nsDialogs page with an unchecked "also delete my settings" box; it switches to `SetShellVarContext current` before touching `$APPDATA\ghfs-gui`, since an all-users uninstall would otherwise resolve `$APPDATA` to `C:\ProgramData`. A silent uninstall skips the page and keeps the settings.
  - Both the installer and uninstaller bail out via `FindWindow` on the window title if the app is running — Windows cannot overwrite a running `.exe`.
  - Installers are **not** code-signed, so SmartScreen shows an unknown-publisher warning. NSIS only emits a 32-bit x86 installer stub; on ARM64 Windows it runs under the built-in x86 emulation and unpacks a native arm64 binary.

## Release CI

`.github/workflows/release.yml` runs on a pushed `v*` tag and on `workflow_dispatch`. It resolves the tag from `GITHUB_REF_NAME` when triggered by the tag push and from `git describe --tags --abbrev=0` otherwise, checks it out (so a manual run always releases the latest tag, never `main`), runs `build/build-all.sh`, then `build/pack-all.sh` (after `apt install nsis`), which runs every other `build/pack-*.sh` and stops at the first failure, and publishes `output/*` via `gh release create --generate-notes`. Raw binaries stay in `output/` alongside the packages, so portable downloads remain available.
