package main

import (
	tk "modernc.org/tk9.0"

	"github.com/tuffrabit/go-narwhal-manager/view"
)

type AppManager struct {
	window  *tk.Window
	current view.View
}

func (m *AppManager) SwitchTo(view view.View) {
	if m.current != nil {
		m.current.Hide()
	}
	m.current = view
	view.Show(m.window)
}
