package view

import (
	"bufio"
	"encoding/hex"
	"fmt"
	"io"
	"strings"
	"time"

	"go.bug.st/serial"
	tk "modernc.org/tk9.0"

	"github.com/tuffrabit/go-narwhal-manager/protocol"
)

type MainView struct {
	serialPort serial.Port
	onDisconnect func() // Called when device disconnects
	
	// Main container
	mainFrame *tk.TFrameWidget
	
	// Console overlay
	consoleFrame    *tk.TFrameWidget
	consoleOutput   *tk.TextWidget
	consoleScroll   *tk.TScrollbarWidget
	
	// Command input controls
	cmdCombo       *tk.TComboboxWidget
	payloadInput   *tk.TEntryWidget
	
	// State
	consoleVisible  bool
	stopReading     chan struct{}
	stopPingLoop    chan struct{}
}

// commandInfo holds metadata for each supported command
type commandInfo struct {
	Code        byte
	Name        string
	PayloadHint string
	NeedsPayload bool
}

// availableCommands defines all protocol commands for the dropdown
var availableCommands = []commandInfo{
	{protocol.CmdPing, "Ping", "any data (optional)", false},
	{protocol.CmdGetDeviceConfig, "GetDeviceConfig", "none", false},
	{protocol.CmdSetDeviceConfig, "SetDeviceConfig", "12 bytes (device config)", true},
	{protocol.CmdGetProfile, "GetProfile", "slot (1 byte)", true},
	{protocol.CmdSetProfile, "SetProfile", "slot + 286 bytes", true},
	{protocol.CmdDeleteProfile, "DeleteProfile", "slot (1 byte)", true},
	{protocol.CmdListProfiles, "ListProfiles", "none", false},
	{protocol.CmdGetStorageStats, "GetStorageStats", "none", false},
	{protocol.CmdFactoryReset, "FactoryReset", "none", false},
	{protocol.CmdGetVersion, "GetVersion", "none", false},
	{protocol.CmdDiscover, "Discover", "none", false},
}

func NewMainView(port serial.Port, onDisconnect func()) *MainView {
	return &MainView{
		serialPort:   port,
		onDisconnect: onDisconnect,
		stopReading:  make(chan struct{}),
		stopPingLoop: make(chan struct{}),
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
	
	// Start the ping monitor to detect disconnects
	m.stopPingLoop = make(chan struct{})
	go m.pingLoop()
}

func (m *MainView) Hide() {
	// Stop serial reading goroutine
	if m.stopReading != nil {
		close(m.stopReading)
		m.stopReading = nil
	}
	
	// Stop ping loop
	if m.stopPingLoop != nil {
		close(m.stopPingLoop)
		m.stopPingLoop = nil
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
	m.consoleScroll = nil
	m.cmdCombo = nil
	m.payloadInput = nil
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
	
	tk.GridColumnConfigure(inputFrame, 0, tk.Weight(0))  // Command label
	tk.GridColumnConfigure(inputFrame, 1, tk.Weight(0))  // Command dropdown
	tk.GridColumnConfigure(inputFrame, 2, tk.Weight(0))  // Payload label
	tk.GridColumnConfigure(inputFrame, 3, tk.Weight(1))  // Payload entry
	tk.GridColumnConfigure(inputFrame, 4, tk.Weight(0))  // Send button
	
	// Command dropdown label
	cmdLabel := inputFrame.TLabel(tk.Txt("Command:"))
	tk.Grid(cmdLabel, tk.Row(0), tk.Column(0), tk.Sticky("w"), tk.Padx(2))
	
	// Command dropdown - populate with command names
	cmdNames := make([]string, len(availableCommands))
	for i, cmd := range availableCommands {
		cmdNames[i] = cmd.Name
	}
	m.cmdCombo = inputFrame.TCombobox(
		tk.Values(cmdNames),
		tk.State("readonly"),
		tk.Width(18),
		tk.Textvariable("Ping"), // Default selection
	)
	tk.Grid(m.cmdCombo, tk.Row(0), tk.Column(1), tk.Sticky("w"), tk.Padx(2))
	
	// Bind selection change to update payload hint
	tk.Bind(m.cmdCombo, "<<ComboboxSelected>>", tk.Command(m.updatePayloadHint))
	
	// Payload label
	payloadLabel := inputFrame.TLabel(tk.Txt("Payload (hex):"))
	tk.Grid(payloadLabel, tk.Row(0), tk.Column(2), tk.Sticky("w"), tk.Padx(5))
	
	// Payload entry - accepts hex bytes
	m.payloadInput = inputFrame.TEntry(
		tk.Width(40),
	)
	tk.Grid(m.payloadInput, tk.Row(0), tk.Column(3), tk.Sticky("ew"), tk.Padx(2))
	
	// Send button
	sendBtn := inputFrame.TButton(
		tk.Txt("Send"),
		tk.Command(m.sendCommand),
	)
	tk.Grid(sendBtn, tk.Row(0), tk.Column(4), tk.Padx(2))
	
	// Bind Enter key to send command
	tk.Bind(m.payloadInput, "<Return>", tk.Command(func() {
		m.sendCommand()
	}))
	
	// Set initial hint
	m.updatePayloadHint()
	
	// Note: Console is initially hidden (not gridded)
}

func (m *MainView) showConsole() {
	if m.consoleVisible || m.consoleFrame == nil {
		return
	}
	
	// Show console overlay
	tk.Grid(m.consoleFrame, tk.Row(0), tk.Column(0), tk.Sticky("nsew"))
	m.consoleVisible = true
	
	// Focus payload input
	tk.Focus(m.payloadInput)
	
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

func (m *MainView) updatePayloadHint() {
	if m.payloadInput == nil {
		return
	}
	
	// Clear the payload input when command changes
	// (The hint text was being treated as actual input)
	m.payloadInput.Configure(tk.Textvariable(""))
}

func (m *MainView) sendCommand() {
	if m.serialPort == nil {
		return
	}
	
	// Get selected command
	selectedName := m.cmdCombo.Textvariable()
	if selectedName == "" {
		return
	}
	
	// Find command info
	var cmdInfo *commandInfo
	for i := range availableCommands {
		if availableCommands[i].Name == selectedName {
			cmdInfo = &availableCommands[i]
			break
		}
	}
	if cmdInfo == nil {
		return
	}
	
	// Parse payload from hex input
	payloadHex := m.payloadInput.Textvariable()
	payloadHex = strings.TrimSpace(payloadHex)
	
	// Show hint if payload is empty and command expects one
	if payloadHex == "" && cmdInfo.NeedsPayload {
		m.appendToConsole(fmt.Sprintf("[HINT: %s expects payload: %s]\n", cmdInfo.Name, cmdInfo.PayloadHint))
	}
	
	payload, err := parseHexString(payloadHex)
	if err != nil {
		m.appendToConsole(fmt.Sprintf("[ERROR: invalid hex payload: %v]\n", err))
		return
	}
	
	// Build the frame (handles length + CRC automatically)
	frame := protocol.BuildFrame(cmdInfo.Code, payload)
	
	// Display what we're sending
	m.appendToConsole(fmt.Sprintf("> %s\n", formatHexBytes(frame)))
	m.appendToConsole(fmt.Sprintf("  → %s [payload: %d bytes]\n", cmdInfo.Name, len(payload)))
	
	// Send raw bytes to serial port
	_, err = m.serialPort.Write(frame)
	if err != nil {
		m.appendToConsole(fmt.Sprintf("[ERROR sending: %v]\n", err))
		return
	}
	
	// Clear payload input (but keep command selection)
	m.payloadInput.Configure(tk.Textvariable(""))
	m.updatePayloadHint()
	
	tk.Focus(m.payloadInput)
}

// parseHexString converts a space-separated hex string to bytes
// Accepts: "01 02 AB" or "0102AB" or "01,02,ab"
func parseHexString(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	if s == "" || s == "none" {
		return nil, nil
	}
	
	// Remove common separators
	s = strings.ReplaceAll(s, ",", " ")
	s = strings.ReplaceAll(s, "0x", " ")
	s = strings.ReplaceAll(s, "0X", " ")
	
	// If no spaces, try to parse as continuous hex
	if !strings.Contains(s, " ") {
		// Check if it looks like a hint text (contains letters other than hex)
		if len(s) > 0 && s[0] >= 'a' && s[0] <= 'z' || s[0] >= 'A' && s[0] <= 'Z' {
			// It's a hint text, treat as empty payload
			return nil, nil
		}
		// Continuous hex string - ensure even length
		if len(s)%2 != 0 {
			s = "0" + s
		}
		return hex.DecodeString(s)
	}
	
	// Space-separated: normalize and decode
	parts := strings.Fields(s)
	var result []byte
	for _, part := range parts {
		if part == "" {
			continue
		}
		b, err := hex.DecodeString(part)
		if err != nil {
			return nil, fmt.Errorf("invalid hex '%s': %w", part, err)
		}
		result = append(result, b...)
	}
	return result, nil
}

// formatHexBytes formats bytes as space-separated hex
func formatHexBytes(data []byte) string {
	if len(data) == 0 {
		return "<empty>"
	}
	var parts []string
	for _, b := range data {
		parts = append(parts, fmt.Sprintf("%02X", b))
	}
	return strings.Join(parts, " ")
}

// parseFrameAndDisplay attempts to parse a received frame and display both raw and parsed
func (m *MainView) parseFrameAndDisplay(data []byte) {
	if len(data) < protocol.MinFrameSize {
		m.appendToConsole(fmt.Sprintf("< %s\n", formatHexBytes(data)))
		m.appendToConsole("  ← [incomplete frame]\n")
		return
	}
	
	// Check sync byte
	if data[0] != protocol.SyncByte {
		m.appendToConsole(fmt.Sprintf("< %s\n", formatHexBytes(data)))
		m.appendToConsole(fmt.Sprintf("  ← [invalid sync: 0x%02X]\n", data[0]))
		return
	}
	
	status, payload, err := protocol.ParseFrame(data)
	if err != nil {
		m.appendToConsole(fmt.Sprintf("< %s\n", formatHexBytes(data)))
		m.appendToConsole(fmt.Sprintf("  ← [parse error: %v]\n", err))
		return
	}
	
	// Successfully parsed
	m.appendToConsole(fmt.Sprintf("< %s\n", formatHexBytes(data)))
	
	// Format parsed response
	statusStr := formatStatusCode(status)
	payloadDesc := formatPayload(status, payload)
	m.appendToConsole(fmt.Sprintf("  ← %s %s\n", statusStr, payloadDesc))
}

// formatStatusCode returns a human-readable status name
func formatStatusCode(status byte) string {
	switch status {
	case protocol.StatusOK:
		return "OK"
	case protocol.StatusError:
		return "ERROR"
	case protocol.StatusInvalidCmd:
		return "INVALID_CMD"
	case protocol.StatusInvalidData:
		return "INVALID_DATA"
	case protocol.StatusNotFound:
		return "NOT_FOUND"
	case protocol.StatusNoSpace:
		return "NO_SPACE"
	case protocol.StatusVersionMismatch:
		return "VERSION_MISMATCH"
	case protocol.StatusCRCError:
		return "CRC_ERROR"
	default:
		return fmt.Sprintf("UNKNOWN(0x%02X)", status)
	}
}

// formatPayload provides a description of the payload based on status/command context
func formatPayload(status byte, payload []byte) string {
	if len(payload) == 0 {
		return ""
	}
	
	// For error responses, just show byte count
	if status != protocol.StatusOK {
		return fmt.Sprintf("[%d bytes]", len(payload))
	}
	
	// For OK responses, try to interpret based on known response types
	if len(payload) == 12 {
		// Likely DeviceConfig
		var cfg protocol.DeviceConfig
		if err := cfg.UnmarshalBinary(payload); err == nil {
			return fmt.Sprintf("[DeviceConfig v%d, profile=%d, brightness=%d]",
				cfg.Version, cfg.ActiveProfile, cfg.Brightness)
		}
	}
	
	if len(payload) == 286 {
		return "[Profile: 286 bytes]"
	}
	
	if len(payload) == 16 {
		// Likely StorageStats
		var stats protocol.StorageStats
		if err := stats.UnmarshalBinary(payload); err == nil {
			return fmt.Sprintf("[Storage: %d/%d blocks used]", stats.UsedBlocks, stats.TotalBlocks)
		}
	}
	
	if len(payload) == 4 {
		// Likely GetVersion response
		return fmt.Sprintf("[Version: fw=%d.%d, cfg=v%d]", payload[0], payload[1], payload[2]|payload[3]<<8)
	}
	
	// Check for "tuffpad" discovery response
	if string(payload) == "tuffpad" {
		return "[Discover: tuffpad]"
	}
	
	// Check for profile list response
	if len(payload) > 0 && len(payload) < 20 {
		// Could be profile list: [count][slot1][slot2]...
		return fmt.Sprintf("[ProfileList: %d slots]", len(payload)-1)
	}
	
	// Default: show as hex preview
	preview := formatHexBytes(payload)
	if len(preview) > 40 {
		preview = preview[:37] + "..."
	}
	return fmt.Sprintf("[%s]", preview)
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
		
		// Read frame from serial port
		// First read sync byte
		syncByte, err := reader.ReadByte()
		if err != nil {
			if err == io.EOF {
				m.appendToConsole("\n=== Serial port disconnected ===\n")
				return
			}
			select {
			case <-m.stopReading:
				return
			default:
				continue // Try to resync
			}
		}
		
		// Check if it's a sync byte, otherwise we might be out of sync
		if syncByte != protocol.SyncByte {
			// Not a frame start, append as raw byte (might be debug output)
			m.appendToConsole(string(syncByte))
			continue
		}
		
		// Read header (cmd + len: 3 bytes)
		header := make([]byte, 3)
		if _, err := io.ReadFull(reader, header); err != nil {
			select {
			case <-m.stopReading:
				return
			default:
				m.appendToConsole("\n[ERROR reading header: " + err.Error() + "]\n")
				continue
			}
		}
		
		// Extract length (little-endian uint16)
		length := uint16(header[1]) | uint16(header[2])<<8
		
		// Sanity check on length
		if length > 4096 {
			m.appendToConsole("\n[ERROR: invalid frame length]\n")
			continue
		}
		
		// Read payload + CRC
		remaining := make([]byte, int(length)+2) // payload + 2-byte CRC
		if _, err := io.ReadFull(reader, remaining); err != nil {
			select {
			case <-m.stopReading:
				return
			default:
				m.appendToConsole("\n[ERROR reading payload: " + err.Error() + "]\n")
				continue
			}
		}
		
		// Reconstruct full frame
		frame := append([]byte{syncByte}, header...)
		frame = append(frame, remaining...)
		
		// Parse and display the frame
		m.parseFrameAndDisplay(frame)
	}
}

// pingLoop runs a background health check that pings the device periodically.
// If the device stops responding, it triggers the onDisconnect callback.
func (m *MainView) pingLoop() {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.stopPingLoop:
			return
		case <-ticker.C:
			if !m.pingDevice() {
				// Ping failed - device disconnected
				if m.onDisconnect != nil {
					tk.PostEvent(m.onDisconnect, false)
				}
				return
			}
		}
	}
}

// pingDevice sends a ping and waits for a response.
// Returns true if successful, false if the device is not responding.
func (m *MainView) pingDevice() bool {
	if m.serialPort == nil {
		return false
	}
	
	// Build ping frame with empty payload
	pingFrame := protocol.BuildFrame(protocol.CmdPing, nil)
	
	// Clear any pending input
	m.serialPort.ResetInputBuffer()
	
	// Send ping
	if _, err := m.serialPort.Write(pingFrame); err != nil {
		return false
	}
	
	// Wait for response with timeout
	buf := make([]byte, 32)
	m.serialPort.SetReadTimeout(200 * time.Millisecond)
	
	n, err := m.serialPort.Read(buf)
	if err != nil || n == 0 {
		return false
	}
	
	// Try to parse response - just check for valid sync byte and OK status
	if n >= protocol.MinFrameSize && buf[0] == protocol.SyncByte && buf[1] == protocol.StatusOK {
		return true
	}
	
	return false
}
