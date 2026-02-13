package view

import (
	"bufio"
	"io"

	"go.bug.st/serial"
	tk "modernc.org/tk9.0"
)

type MainView struct {
	serialPort serial.Port
	
	// Main container
	mainFrame *tk.TFrameWidget
	
	// Console overlay
	consoleFrame    *tk.TFrameWidget
	consoleOutput   *tk.TextWidget
	consoleInput    *tk.TEntryWidget
	consoleScroll   *tk.TScrollbarWidget
	
	// State
	consoleVisible  bool
	stopReading     chan struct{}
}

func NewMainView(port serial.Port) *MainView {
	return &MainView{
		serialPort:  port,
		stopReading: make(chan struct{}),
	}
}

func (m *MainView) Show(parent *tk.Window) {
	// Create main container frame
	m.mainFrame = tk.TFrame()
	tk.Grid(m.mainFrame, tk.Row(0), tk.Column(0), tk.Sticky("nsew"))
	tk.GridRowConfigure(parent, 0, tk.Weight(1))
	tk.GridColumnConfigure(parent, 0, tk.Weight(1))
	
	// Configure main frame grid
	tk.GridRowConfigure(m.mainFrame, 0, tk.Weight(1))
	tk.GridColumnConfigure(m.mainFrame, 0, tk.Weight(1))
	
	// Create placeholder content (for future profiles UI)
	placeholderFrame := m.createPlaceholderContent()
	tk.Grid(placeholderFrame, tk.Row(0), tk.Column(0), tk.Sticky("nsew"))
	
	// Create console overlay (initially hidden)
	m.createConsoleOverlay()
}

func (m *MainView) Hide() {
	// Stop serial reading goroutine
	if m.stopReading != nil {
		close(m.stopReading)
		m.stopReading = nil
	}
	
	// Destroy main frame
	if m.mainFrame != nil {
		tk.Destroy(m.mainFrame)
		m.mainFrame = nil
	}
	
	// Reset state
	m.consoleVisible = false
	m.consoleFrame = nil
	m.consoleOutput = nil
	m.consoleInput = nil
	m.consoleScroll = nil
}

func (m *MainView) createPlaceholderContent() *tk.TFrameWidget {
	frame := m.mainFrame.TFrame()
	
	// Title label
	title := frame.TLabel(tk.Txt("Gamepad Profiles"))
	tk.Grid(title, tk.Row(0), tk.Column(0), tk.Pady(20))
	
	// Placeholder text
	placeholder := frame.TLabel(
		tk.Txt("Profile management UI will go here.\nClick the button below to open the serial console."),
		tk.Justify("center"),
	)
	tk.Grid(placeholder, tk.Row(1), tk.Column(0), tk.Pady(10))
	
	// Serial Console button
	consoleBtn := frame.TButton(
		tk.Txt("Open Serial Console"),
		tk.Command(m.showConsole),
	)
	tk.Grid(consoleBtn, tk.Row(2), tk.Column(0), tk.Pady(20))
	
	return frame
}

func (m *MainView) createConsoleOverlay() {
	// Console frame - will be shown/hidden as overlay
	m.consoleFrame = m.mainFrame.TFrame(
		tk.Relief("flat"),
	)
	
	// Configure console frame grid
	tk.GridRowConfigure(m.consoleFrame, 0, tk.Weight(0))    // Header
	tk.GridRowConfigure(m.consoleFrame, 1, tk.Weight(1))    // Output text
	tk.GridRowConfigure(m.consoleFrame, 2, tk.Weight(0))    // Input area
	tk.GridColumnConfigure(m.consoleFrame, 0, tk.Weight(1))
	tk.GridColumnConfigure(m.consoleFrame, 1, tk.Weight(0)) // Scrollbar
	
	// Header frame with Back and Clear buttons
	headerFrame := m.consoleFrame.TFrame()
	tk.Grid(headerFrame, tk.Row(0), tk.Column(0), tk.Columnspan(2), tk.Sticky("ew"), tk.Padx(5), tk.Pady(5))
	
	backBtn := headerFrame.TButton(
		tk.Txt("← Back"),
		tk.Command(m.hideConsole),
	)
	tk.Grid(backBtn, tk.Row(0), tk.Column(0), tk.Sticky("w"))
	
	clearBtn := headerFrame.TButton(
		tk.Txt("Clear"),
		tk.Command(m.clearConsole),
	)
	tk.Grid(clearBtn, tk.Row(0), tk.Column(1), tk.Sticky("e"), tk.Padx(5))
	tk.GridColumnConfigure(headerFrame, 0, tk.Weight(1))
	
	// Output text widget with scrollbar
	m.consoleOutput = m.consoleFrame.Text(
		tk.Wrap("word"),
		tk.State("disabled"), // Read-only by default
		tk.Width(80),
		tk.Height(30),
		tk.Font("Consolas", 10),
	)
	
	// Scrollbar
	m.consoleScroll = m.consoleFrame.TScrollbar(
		tk.Command(func(e *tk.Event) { e.Yview(m.consoleOutput) }),
	)
	
	// Connect text widget to scrollbar
	m.consoleOutput.Configure(
		tk.Yscrollcommand(func(e *tk.Event) { e.ScrollSet(m.consoleScroll) }),
	)
	
	tk.Grid(m.consoleOutput, tk.Row(1), tk.Column(0), tk.Sticky("nsew"), tk.Padx(5))
	tk.Grid(m.consoleScroll, tk.Row(1), tk.Column(1), tk.Sticky("ns"), tk.Pady(5))
	
	// Input area frame
	inputFrame := m.consoleFrame.TFrame()
	tk.Grid(inputFrame, tk.Row(2), tk.Column(0), tk.Columnspan(2), tk.Sticky("ew"), tk.Padx(5), tk.Pady(5))
	
	tk.GridColumnConfigure(inputFrame, 0, tk.Weight(0))  // Label
	tk.GridColumnConfigure(inputFrame, 1, tk.Weight(1))  // Entry
	tk.GridColumnConfigure(inputFrame, 2, tk.Weight(0))  // Button
	
	// Input label
	inputLabel := inputFrame.TLabel(tk.Txt("Command:"))
	tk.Grid(inputLabel, tk.Row(0), tk.Column(0), tk.Sticky("w"))
	
	// Input entry - starts empty
	m.consoleInput = inputFrame.TEntry(
		tk.Textvariable(""),
		tk.Width(60),
	)
	tk.Grid(m.consoleInput, tk.Row(0), tk.Column(1), tk.Sticky("ew"), tk.Padx(5))
	
	// Send button
	sendBtn := inputFrame.TButton(
		tk.Txt("Send"),
		tk.Command(m.sendCommand),
	)
	tk.Grid(sendBtn, tk.Row(0), tk.Column(2))
	
	// Bind Enter key to send command
	tk.Bind(m.consoleInput, "<Return>", tk.Command(func() {
		m.sendCommand()
	}))
	
	// Note: Console is initially hidden (not gridded)
}

func (m *MainView) showConsole() {
	if m.consoleVisible || m.consoleFrame == nil {
		return
	}
	
	// Show console overlay
	tk.Grid(m.consoleFrame, tk.Row(0), tk.Column(0), tk.Sticky("nsew"))
	m.consoleVisible = true
	
	// Focus input
	tk.Focus(m.consoleInput)
	
	// Start reading from serial port
	m.stopReading = make(chan struct{})
	go m.readSerialLoop()
	
	// Add welcome message
	m.appendToConsole("=== Serial Console Connected ===\n")
}

func (m *MainView) hideConsole() {
	if !m.consoleVisible || m.consoleFrame == nil {
		return
	}
	
	// Stop reading from serial port
	if m.stopReading != nil {
		close(m.stopReading)
		m.stopReading = nil
	}
	
	// Hide console overlay
	tk.GridForget(m.consoleFrame.Window)
	m.consoleVisible = false
}

func (m *MainView) clearConsole() {
	if m.consoleOutput == nil {
		return
	}
	
	m.consoleOutput.Configure(tk.State("normal"))
	m.consoleOutput.Delete("1.0", "end")
	m.consoleOutput.Configure(tk.State("disabled"))
}

func (m *MainView) sendCommand() {
	if m.consoleInput == nil || m.serialPort == nil {
		return
	}
	
	// Get current text from entry using Textvariable() method
	cmd := m.consoleInput.Textvariable()
	if cmd == "" {
		return
	}
	
	// Append sent command to console
	m.appendToConsole("> " + cmd + "\n")
	
	// Send to serial port with newline
	cmdWithNewline := cmd + "\n"
	_, err := m.serialPort.Write([]byte(cmdWithNewline))
	if err != nil {
		m.appendToConsole("[ERROR sending: " + err.Error() + "]\n")
	}
	
	// Clear input by setting empty textvariable
	m.consoleInput.Configure(tk.Textvariable(""))
	
	tk.Focus(m.consoleInput)
}

func (m *MainView) appendToConsole(text string) {
	if m.consoleOutput == nil {
		return
	}
	
	// Use PostEvent to update UI from any goroutine
	tk.PostEvent(func() {
		if m.consoleOutput == nil {
			return
		}
		
		// Enable editing temporarily
		m.consoleOutput.Configure(tk.State("normal"))
		
		// Insert text at end
		m.consoleOutput.Insert("end", text)
		
		// Auto-scroll to bottom
		m.consoleOutput.MarkSet("insert", "end")
		m.consoleOutput.See("end")
		
		// Disable editing again
		m.consoleOutput.Configure(tk.State("disabled"))
	}, false)
}

func (m *MainView) readSerialLoop() {
	if m.serialPort == nil {
		return
	}
	
	reader := bufio.NewReader(m.serialPort)
	
	for {
		select {
		case <-m.stopReading:
			return
		default:
		}
		
		// Read line from serial port
		line, err := reader.ReadString('\n')
		if err != nil {
			if err == io.EOF {
				// Serial port closed
				m.appendToConsole("\n=== Serial port disconnected ===\n")
				return
			}
			// Other error - check if we should stop
			select {
			case <-m.stopReading:
				return
			default:
				m.appendToConsole("\n[ERROR reading: " + err.Error() + "]\n")
				return
			}
		}
		
		if line != "" {
			m.appendToConsole(line)
		}
	}
}
