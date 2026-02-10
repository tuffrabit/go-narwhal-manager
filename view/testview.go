package view

import (
	tk "modernc.org/tk9.0"
)

type TestView struct {
	label *tk.LabelWidget
}

func (v *TestView) Show(parent *tk.Window) {
	v.label = tk.Label(tk.Txt("TEST TEST TEST"))
	tk.Pack(v.label, tk.Expand(true))
}

func (v *TestView) Hide() {
	tk.Destroy(v.label)
}
