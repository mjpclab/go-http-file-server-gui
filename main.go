package main

func main() {
	widgets := newUI()
	maintainPreference(widgets)
	attachHandlers(widgets)
	widgets.window.ShowAndRun()
}
