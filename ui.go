package main

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/widget"
)

type uiWidgets struct {
	application fyne.App
	window      fyne.Window

	root     *widget.Entry
	rootPick *widget.Button
	listen   *widget.Entry
	archive  *widget.Check
	upload   *widget.Check
	mkdir    *widget.Check
	del      *widget.Check

	tlsCert     *widget.Entry
	tlsCertPick *widget.Button
	tlsKey      *widget.Entry
	tlsKeyPick  *widget.Button

	links *fyne.Container

	start *widget.Button
	stop  *widget.Button
}

func newUI() *uiWidgets {
	application := app.NewWithID("net.mjpclab.go-http-file-server-gui")
	window := application.NewWindow("Go HTTP File Server GUI")
	window.Resize(fyne.NewSize(650, 400))

	// root
	rootLabel := widget.NewLabel("Root")

	root := widget.NewEntry()
	root.SetPlaceHolder("Root directory")

	rootPick := widget.NewButton("...", nil)
	rootContainer := container.NewBorder(nil, nil, nil, rootPick, root)

	// listen
	listenLabel := widget.NewLabel("Listen")

	listen := widget.NewEntry()
	listen.SetPlaceHolder("IP/Port/IP:Port")

	// perms
	optionsLabel := widget.NewLabel("Options")

	archive := widget.NewCheck("Archive", nil)
	upload := widget.NewCheck("Upload", nil)
	mkdir := widget.NewCheck("Mkdir", nil)
	del := widget.NewCheck("Delete", nil)
	optionsContainer := container.NewHBox(archive, upload, mkdir, del)

	// form general
	formGeneral := container.New(layout.NewFormLayout(),
		rootLabel, rootContainer,
		listenLabel, listen,
		optionsLabel, optionsContainer,
	)

	// tls cert
	tlsCertLabel := widget.NewLabel("TLS Certificate")
	tlsCert := widget.NewEntry()
	tlsCert.SetPlaceHolder("TLS Certificate File PEM format")
	tlsCertPick := widget.NewButton("...", nil)
	tlsCertContainer := container.NewBorder(nil, nil, nil, tlsCertPick, tlsCert)

	// tls key
	tlsKeyLabel := widget.NewLabel("TLS Key")
	tlsKey := widget.NewEntry()
	tlsKey.SetPlaceHolder("TLS Key File PEM format")
	tlsKeyPick := widget.NewButton("...", nil)
	tlsKeyContainer := container.NewBorder(nil, nil, nil, tlsKeyPick, tlsKey)

	// form advanced
	formAdvanced := container.New(layout.NewFormLayout(),
		tlsCertLabel, tlsCertContainer,
		tlsKeyLabel, tlsKeyContainer,
	)

	// tabs
	tabs := container.NewAppTabs(
		container.NewTabItem("General", formGeneral),
		container.NewTabItem("Advanced", formAdvanced),
	)

	// links
	links := container.NewVBox()
	linksContainer := container.NewScroll(links)

	// buttons
	start := widget.NewButton("Start server", nil)

	stop := widget.NewButton("Stop server", nil)
	stop.Disable()

	buttons := container.NewGridWithColumns(2, start, stop)

	// main border
	border := container.NewBorder(tabs, buttons, nil, nil, linksContainer)

	window.SetContent(border)

	widgets := &uiWidgets{
		application: application,
		window:      window,

		root:     root,
		rootPick: rootPick,
		archive:  archive,
		upload:   upload,
		mkdir:    mkdir,
		del:      del,
		listen:   listen,

		tlsCert:     tlsCert,
		tlsCertPick: tlsCertPick,
		tlsKey:      tlsKey,
		tlsKeyPick:  tlsKeyPick,

		links: links,

		start: start,
		stop:  stop,
	}

	return widgets
}
