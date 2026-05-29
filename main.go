package main

import (
	. "modernc.org/tk9.0"
	_ "modernc.org/tk9.0/themes/azure"
)

func main() {
	applySystemTheme()
	widgets := newUI()
	loadPreference(widgets)
	attachHandlers(widgets)
	App.Wait()
	savePreference(widgets)
}
