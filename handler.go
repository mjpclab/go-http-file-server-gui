package main

import (
	"crypto/tls"
	"errors"
	"fmt"
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/dialog"
	"fyne.io/fyne/v2/storage"
	"fyne.io/fyne/v2/widget"
	"mjpclab.dev/ghfs/src/app"
	"mjpclab.dev/ghfs/src/goVirtualHost"
	"mjpclab.dev/ghfs/src/param"
	"mjpclab.dev/ghfs/src/setting"
	"net/url"
	"path/filepath"
)

func attachHandlers(widgets *uiWidgets) {
	attachDropHandlers(widgets)
	attachBrowseHandlers(widgets)
	attachStartStopHandlers(widgets)
}

func attachDropHandlers(widgets *uiWidgets) {
	widgets.window.SetOnDropped(func(pos fyne.Position, uris []fyne.URI) {
		if len(uris) > 0 {
			widgets.root.SetText(uris[0].Path())
		}
	})
}

func attachBrowseHandlers(widgets *uiWidgets) {
	_attachBrowseFolderHandler(widgets.window, widgets.root, widgets.rootPick)
	_attachBrowseFileHandler(widgets.window, widgets.tlsCert, widgets.tlsCertPick)
	_attachBrowseFileHandler(widgets.window, widgets.tlsKey, widgets.tlsKeyPick)
}

func _attachBrowseFolderHandler(window fyne.Window, entry *widget.Entry, button *widget.Button) {
	dlg := dialog.NewFolderOpen(func(uri fyne.ListableURI, err error) {
		if uri == nil || err != nil {
			return
		}
		entry.SetText(uri.Path())
	}, window)

	button.OnTapped = func() {
		uri := storage.NewFileURI(entry.Text)
		luri, err := storage.ListerForURI(uri)
		if err == nil {
			dlg.SetLocation(luri)
		}
		dlg.Show()
	}
}

func _attachBrowseFileHandler(window fyne.Window, entry *widget.Entry, button *widget.Button) {
	dlg := dialog.NewFileOpen(func(uri fyne.URIReadCloser, err error) {
		if uri == nil || err != nil {
			return
		}
		entry.SetText(uri.URI().Path())
	}, window)

	button.OnTapped = func() {
		uri := storage.NewFileURI(filepath.Dir(entry.Text))
		luri, err := storage.ListerForURI(uri)
		if err == nil {
			dlg.SetLocation(luri)
		}
		dlg.Show()
	}
}

func attachStartStopHandlers(widgets *uiWidgets) {
	var appInst *app.App
	start := widgets.start
	stop := widgets.stop

	start.OnTapped = func() {
		var errs []error
		appInst, errs = _createApp(widgets)
		if len(errs) > 0 {
			return
		}
		start.Disable()
		stop.Enable()
		_createLinks(appInst, widgets.links)
		go func() {
			errs = appInst.Open()
			if len(errs) > 0 {
				fmt.Println(errs)
				dialog.ShowError(errors.Join(errs...), widgets.window)
			}
			widgets.links.RemoveAll()
			stop.Disable()
			start.Enable()
			appInst = nil
		}()
	}

	stop.OnTapped = func() {
		if appInst != nil {
			appInst.Close()
		}
	}
}

func _createApp(widgets *uiWidgets) (appInst *app.App, errs []error) {
	var certificates []tls.Certificate
	if len(widgets.tlsCert.Text) > 0 && len(widgets.tlsKey.Text) > 0 {
		cert, err := goVirtualHost.LoadCertificate(widgets.tlsCert.Text, widgets.tlsKey.Text)
		if err == nil {
			certificates = append(certificates, cert)
		}
	}

	params, errs := param.NewParams([]param.Param{{
		IndexUrls:     []string{"/"},
		Root:          widgets.root.Text,
		Listens:       []string{widgets.listen.Text},
		GlobalArchive: widgets.archive.Checked,
		GlobalUpload:  widgets.upload.Checked,
		GlobalMkdir:   widgets.mkdir.Checked,
		GlobalDelete:  widgets.del.Checked,
		Certificates:  certificates,
	}})
	if len(errs) > 0 {
		fmt.Println(errs)
		return
	}

	// setting
	setting := &setting.Setting{
		Quiet:   false,
		PidFile: "",
	}

	// app
	appInst, errs = app.NewApp(params, setting)
	if len(errs) > 0 {
		fmt.Println(errs)
	}
	return
}

func _createLinks(appInst *app.App, container *fyne.Container) {
	accessOrigins := appInst.GetAccessibleOrigins(false)
	if len(accessOrigins) == 0 {
		return
	}
	for _, origin := range accessOrigins[0] {
		originUrl, err := url.Parse(origin)
		if err == nil {
			container.Add(widget.NewHyperlink(origin, originUrl))
		}
	}
}
