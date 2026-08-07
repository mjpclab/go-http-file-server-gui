package main

import (
	"fmt"
	"runtime"

	. "modernc.org/tk9.0"
)

// tk9.0 wraps `wm attributes` but not `wm state`, and evaluating Tcl directly
// is only exposed to extensions. Registering a do-nothing extension purely to
// capture the context is the supported way in — it is how the bundled
// extensions get theirs. Initialization has to happen from the main package
// (tk9.0 walks the call stack to check), which initTcl satisfies.
const tclExtensionName = "ghfs-gui-tcl"

type tclExtension struct{ ctx ExtensionContext }

var tcl = &tclExtension{}

func init() {
	_, _ = RegisterExtension(tclExtensionName, tcl)
}

func (e *tclExtension) Initialize(ctx ExtensionContext) error {
	e.ctx = ctx
	return nil
}

func initTcl() {
	_ = InitializeExtension(tclExtensionName)
}

// tclEval runs a Tcl script and returns "" if it fails. Errors are swallowed on
// purpose: everything routed through here is cosmetic window state, and window
// managers differ in what they support — tk9.0's default error mode would turn
// an unsupported request into a panic.
func tclEval(script string) string {
	if tcl.ctx == nil {
		return ""
	}
	r, err := tcl.ctx.Eval(script)
	if err != nil {
		return ""
	}
	return r
}

// X11 exposes the maximized state as a window manager attribute, Windows and
// macOS as a window state; neither spelling exists on the other platform.
var zoomViaAttribute = runtime.GOOS != "windows" && runtime.GOOS != "darwin"

func isMaximized(w *Window) bool {
	if zoomViaAttribute {
		return tclEval(fmt.Sprintf("wm attributes %s -zoomed", w)) == "1"
	}
	return tclEval(fmt.Sprintf("wm state %s", w)) == "zoomed"
}

// maximizeWhenMapped maximizes the window once the window manager has put it on
// screen. Asking before that is unreliable — a WM is free to ignore a state
// request for a window it has not mapped yet — so the request is deferred to
// <Map>, and made only once, since a toplevel is re-mapped on deiconify.
func maximizeWhenMapped(w *Window) {
	done := false
	Bind(w, "<Map>", Command(func(e *Event) {
		// Child widgets carry their toplevel in their bindtags, so this fires
		// for them too.
		if done || e.EventWindow != w {
			return
		}
		done = true
		setMaximized(w, true)
	}))
}

func setMaximized(w *Window, on bool) {
	if zoomViaAttribute {
		flag := "0"
		if on {
			flag = "1"
		}
		tclEval(fmt.Sprintf("wm attributes %s -zoomed %s", w, flag))
		return
	}
	state := "normal"
	if on {
		state = "zoomed"
	}
	tclEval(fmt.Sprintf("wm state %s %s", w, state))
}
