package view

import (
	tk "modernc.org/tk9.0"
)

type LoadingView struct {
	label *tk.LabelWidget
}

func (v *LoadingView) Show(parent *tk.Window) {
	v.label = tk.Label(tk.Txt("Loading..."))
	tk.Pack(v.label, tk.Expand(true))
}

func (v *LoadingView) Hide() {
	tk.Destroy(v.label)
}
