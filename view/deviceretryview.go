package view

import (
	"fmt"

	tk "modernc.org/tk9.0"
)

type DeviceRetryView struct {
	err     error
	onRetry func()
	frame   *tk.FrameWidget
}

func NewDeviceRetryView(err error, onRetry func()) *DeviceRetryView {
	return &DeviceRetryView{err: err, onRetry: onRetry}
}

func (v *DeviceRetryView) Show(parent *tk.Window) {
	v.frame = tk.Frame()
	tk.Pack(v.frame, tk.Expand(true))

	label := v.frame.Label(tk.Txt(fmt.Sprintf("Error: %v", v.err)))
	btn := v.frame.Button(tk.Txt("Retry"), tk.Command(v.onRetry))

	tk.Pack(label, tk.Pady("1m"))
	tk.Pack(btn, tk.Pady("1m"))
}

func (v *DeviceRetryView) Hide() {
	tk.Destroy(v.frame)
}
