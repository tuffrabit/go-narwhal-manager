package view

import (
	tk "modernc.org/tk9.0"
)

type View interface {
	Show(parent *tk.Window)
	Hide()
}
