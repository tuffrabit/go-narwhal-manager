package main

import (
	tk "modernc.org/tk9.0"
)

type View interface {
	Show(parent *tk.Window)
	Hide()
}

type AppManager struct {
	window  *tk.Window
	current View
}

func (m *AppManager) SwitchTo(view View) {
	if m.current != nil {
		m.current.Hide()
	}
	m.current = view
	view.Show(m.window)
}
