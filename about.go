package main

import (
	"fmt"

	. "modernc.org/tk9.0"
)

const (
	urlAppRepo = "https://github.com/mjpclab/go-http-file-server-gui"
	urlGhfs    = "https://github.com/mjpclab/go-http-file-server"
	urlTk      = "https://pkg.go.dev/modernc.org/tk9.0"
)

// newAboutTab builds the About tab: the application icon and name, its own
// version and the versions of the two dependencies that shape what it can do,
// plus the toolchain and platform a bug report needs.
//
// Everything here is static, so the frame is all the caller gets back — no
// widget outside it is ever reconfigured, and the tab takes no part in the
// running-state lock or in preference persistence.
func newAboutTab(parent *Window) *TFrameWidget {
	about := parent.TFrame(Padding("2m"))
	info := collectAbout()

	header := about.TFrame()
	// Icon.png is 32x32; displayed at its native size, since upscaling a
	// 2-bit colormap image only makes it blocky.
	Pack(header.TLabel(Image(NewPhoto(Data(iconPNG)))), Pady("1m"))

	nameFont := NewFont(Family("TkDefaultFont"), Size(12), Weight("bold"), Underline(true))
	name := header.TLabel(Txt(appName), Cursor("hand2"), Style("Link.TLabel"), Font(nameFont))
	bindLink(name, urlAppRepo)
	Pack(name)
	Pack(header.TLabel(Txt(info.app)), Pady("1m"))

	Grid(header, Row(0), Column(0), Columnspan(2), Pady("2m"))

	rows := []struct {
		label, url, version string
	}{
		{modGhfs, urlGhfs, info.ghfs},
		{modTk, urlTk, info.tk},
		{"Go", "", info.goVer},
		{"Platform", "", info.platform},
	}
	for i, r := range rows {
		var label *TLabelWidget
		if r.url != "" {
			label = about.TLabel(Txt(r.label), Cursor("hand2"), Style("Link.TLabel"))
			bindLink(label, r.url)
		} else {
			label = about.TLabel(Txt(r.label))
		}
		Grid(label, Row(i+1), Column(0), Sticky("w"), Padx("1m"), Pady("1m"))
		Grid(about.TLabel(Txt(r.version)), Row(i+1), Column(1), Sticky("w"), Padx("1m"), Pady("1m"))
	}
	GridColumnConfigure(about, 1, Weight(1))

	return about
}

// bindLink makes a Link.TLabel open url on click. Shared with the Links tab.
func bindLink(lbl *TLabelWidget, url string) {
	Bind(lbl, "<Button-1>", Command(func() {
		if err := openBrowser(url); err != nil {
			fmt.Println(err)
		}
	}))
}
