package main

func maintainPreference(widgets *uiWidgets) {
	loadPreference(widgets)
	widgets.application.Lifecycle().SetOnStopped(func() {
		savePreference(widgets)
	})
}

func savePreference(widgets *uiWidgets) {
	pref := widgets.application.Preferences()

	pref.SetString("root", widgets.root.Text)
	pref.SetString("listen", widgets.listen.Text)
	pref.SetBool("archive", widgets.archive.Checked)
	pref.SetBool("upload", widgets.upload.Checked)
	pref.SetBool("mkdir", widgets.mkdir.Checked)
	pref.SetBool("del", widgets.del.Checked)

	pref.SetString("cert", widgets.tlsCert.Text)
	pref.SetString("key", widgets.tlsKey.Text)
}

func loadPreference(widgets *uiWidgets) {
	pref := widgets.application.Preferences()

	widgets.root.SetText(pref.String("root"))
	widgets.listen.SetText(pref.StringWithFallback("listen", "8080"))
	widgets.archive.SetChecked(pref.Bool("archive"))
	widgets.upload.SetChecked(pref.Bool("upload"))
	widgets.mkdir.SetChecked(pref.Bool("mkdir"))
	widgets.del.SetChecked(pref.Bool("del"))

	widgets.tlsCert.SetText(pref.String("cert"))
	widgets.tlsKey.SetText(pref.String("key"))
}
