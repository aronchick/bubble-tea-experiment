package main

import (
	"context"
	"fmt"
	"math"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/aronchick/bubble-tea-experiment/pkg/logger"
	"github.com/aronchick/bubble-tea-experiment/pkg/models"
	"github.com/aronchick/bubble-tea-experiment/pkg/testutils"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// Constants
const (
	LogLines           = 10
	AzureTotalSteps    = 7
	StatusLength       = 30
	TickerInterval     = 250 * time.Millisecond
	ProgressBarPadding = 2
)

// DisplayColumn represents a column in the display table
type DisplayColumn struct {
	TextTitle   string
	EmojiTitle  string
	Width       int
	Height      int
	EmojiColumn bool
}

// DisplayColumns defines the structure of the display table
//
//nolint:gomnd
var DisplayColumns = []DisplayColumn{
	{TextTitle: "Name", Width: 10},
	{TextTitle: "Type", Width: 6},
	{TextTitle: "Location", Width: 16},
	{TextTitle: "Status", Width: StatusLength},
	{TextTitle: "Progress", Width: 20},
	{TextTitle: "Time", Width: 8},
	{TextTitle: "Pub IP", Width: 19},
	{TextTitle: "Priv IP", Width: 19},
	{TextTitle: models.DisplayTextOrchestrator, EmojiTitle: models.DisplayEmojiOrchestrator, Width: 2, EmojiColumn: true},
	{TextTitle: models.DisplayTextSSH, EmojiTitle: models.DisplayEmojiSSH, Width: 2, EmojiColumn: true},
	{TextTitle: models.DisplayTextDocker, EmojiTitle: models.DisplayEmojiDocker, Width: 2, EmojiColumn: true},
	{TextTitle: models.DisplayTextBacalhau, EmojiTitle: models.DisplayEmojiBacalhau, Width: 2, EmojiColumn: true},
	{TextTitle: "", Width: 1},
}

var Quitting = false

func AggregateColumnWidths() int {
	width := 0
	for _, column := range DisplayColumns {
		width += column.Width
	}
	return width
}

// DisplayModel represents the main display model
type DisplayModel struct {
	Deployment *models.Deployment
	TextBox    []string
	Quitting   bool
	LastUpdate time.Time
	DebugMode  bool
	Cancel     context.CancelFunc
}

// DisplayMachine represents a single machine in the deployment
type DisplayMachine struct {
	Name          string
	Type          models.AzureResourceTypes
	Location      string
	StatusMessage string
	StartTime     time.Time
	ElapsedTime   time.Duration
	PublicIP      string
	PrivateIP     string
	Orchestrator  bool
	SSH           models.ServiceState
	Docker        models.ServiceState
	CorePackages  models.ServiceState
	Bacalhau      models.ServiceState
}

var (
	globalModelInstance *DisplayModel
	globalModelOnce     sync.Once
)

// GetGlobalModel returns the singleton instance of DisplayModel
func GetGlobalModel() *DisplayModel {
	if globalModelInstance == nil {
		globalModelOnce.Do(func() {
			globalModelInstance = InitialModel()
		})
	}
	return globalModelInstance
}

func SetGlobalModel(m *DisplayModel) {
	globalModelInstance = m
}

// InitialModel creates and returns a new DisplayModel
func InitialModel() *DisplayModel {
	return &DisplayModel{
		Deployment: models.NewDeployment(),
		TextBox:    []string{"Resource Status Monitor"},
		LastUpdate: time.Now(),
		DebugMode:  os.Getenv("DEBUG_DISPLAY") == "1",
	}
}

// Init initializes the DisplayModel
func (m *DisplayModel) Init() tea.Cmd {
	return tickCmd()
}

func CancelFunc() tea.Cmd {
	m := GetGlobalModel()
	return func() tea.Msg {
		if os.Getenv("DEBUG_CHANNELS") == "1" {
			fmt.Fprintf(LogFile, "CancelFunc called\n")
			LogFile.Sync()
		}
		if m.Cancel != nil {
			if os.Getenv("DEBUG_CHANNELS") == "1" {
				fmt.Fprintf(LogFile, "Calling cancel function\n")
				LogFile.Sync()
			}
			m.Cancel()
		}
		return nil
	}
}

type quitMsg struct{}

// Update handles updates to the DisplayModel
func (m *DisplayModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	logger.Debug("Update function called with message type: %T", msg)

	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			logger.Debug("Quit command detected")
			m.Quitting = true
			return m, tea.Sequence(
				CancelFunc(),
				tea.ClearScreen,
				tea.Quit,
			)
		}
	case quitMsg:
		logger.Debug("quitMsg received, quitting program")
		return m, tea.Quit
	case models.StatusUpdateMsg:
		logger.Debug("StatusUpdateMsg received")
		if !m.Quitting {
			m.UpdateStatus(msg.Status)
		}
	case models.TimeUpdateMsg:
		if os.Getenv("DEBUG_CHANNELS") == "1" {
			fmt.Fprintf(LogFile, "TimeUpdateMsg received\n")
			LogFile.Sync()
		}
		if !m.Quitting {
			m.LastUpdate = time.Now()
		}
	case logLinesMsg:
		if os.Getenv("DEBUG_CHANNELS") == "1" {
			fmt.Fprintf(LogFile, "logLinesMsg received\n")
			LogFile.Sync()
		}
		if !m.Quitting {
			m.TextBox = append(m.TextBox, string(msg))
			if len(m.TextBox) > LogLines {
				m.TextBox = m.TextBox[len(m.TextBox)-LogLines:]
			}
		}
	}

	if m.Quitting {
		if os.Getenv("DEBUG_CHANNELS") == "1" {
			fmt.Fprintf(LogFile, "Model is quitting, returning tea.Quit\n")
			LogFile.Sync()
		}
		return m, tea.Quit
	}
	if os.Getenv("DEBUG_CHANNELS") == "1" {
		fmt.Fprintf(LogFile, "Returning batch commands: tickCmd and updateLogCmd\n")
		LogFile.Sync()
	}
	return m, tea.Batch(tickCmd(), m.updateLogCmd())
}

// View renders the DisplayModel
func (m *DisplayModel) View() string {
	tableStyle := lipgloss.NewStyle().
		BorderStyle(lipgloss.NormalBorder()).
		BorderForeground(lipgloss.Color("240"))
	headerStyle := lipgloss.NewStyle().
		Bold(true).
		Foreground(lipgloss.Color("39")).
		Padding(0, 1)
	cellStyle := lipgloss.NewStyle().
		Padding(0, 1).
		AlignVertical(lipgloss.Center)
	textBoxStyle := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color("63")).
		Padding(1).
		Height(LogLines + 2). // Add 2 to account for the border
		Width(130)
	infoStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color("241")).
		Italic(true)

	tableStr := m.renderTable(headerStyle, cellStyle)
	logContent := strings.Join(m.TextBox, "\n")
	infoText := fmt.Sprintf(
		"Press 'q' or Ctrl+C to quit (Last Updated: %s)",
		m.LastUpdate.Format("15:04:05"),
	)

	renderedContent := lipgloss.JoinVertical(
		lipgloss.Left,
		tableStyle.Render(tableStr),
		"",
		textBoxStyle.Render(logContent),
		infoStyle.Render(infoText),
	)

	return lipgloss.NewStyle().Render(renderedContent)
}

// RenderFinalTable renders the final table
func (m *DisplayModel) RenderFinalTable() string {
	return m.View()
}

// UpdateStatus updates the status of a machine
func (m *DisplayModel) UpdateStatus(status *models.DisplayStatus) {
	if status == nil || status.Name == "" {
		return
	}

	machine, found := m.findOrCreateMachine(status)
	if found || (status.Name != "" && status.Type == models.AzureResourceTypeVM) {
		m.updateMachineStatus(machine, status)
	}
}

// Helper functions

func (m *DisplayModel) renderTable(headerStyle, cellStyle lipgloss.Style) string {
	var tableStr string
	tableStr += m.renderRow(DisplayColumns, headerStyle, true)
	if m.DebugMode {
		tableStr += strings.Repeat("-", AggregateColumnWidths()) + "\n"
	}
	for _, machine := range m.Deployment.Machines {
		if machine.Name != "" {
			tableStr += m.renderRow(m.getMachineRowData(machine), cellStyle, false)
		}
	}
	return tableStr
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func (m *DisplayModel) renderRow(data interface{}, baseStyle lipgloss.Style, isHeader bool) string {
	var rowStr string
	var cellData []string

	if isHeader {
		for _, col := range data.([]DisplayColumn) {
			if EmojisEnabled && col.EmojiColumn {
				cellData = append(cellData, col.EmojiTitle)
			} else {
				cellData = append(cellData, col.TextTitle)
			}
		}
	} else {
		cellData = data.([]string)
	}

	for i, cell := range cellData {
		cellWidth := DisplayColumns[i].Width
		style := baseStyle.Width(cellWidth)

		if DisplayColumns[i].EmojiColumn {
			if isHeader {
				style = style.Align(lipgloss.Center)
			} else {
				style = renderStyleByColumn(cell, style)
			}
		} else if isHeader {
			style = style.Align(lipgloss.Left)
		} else {
			style = style.Align(lipgloss.Left).MaxWidth(cellWidth)
		}

		renderedCell := style.Render(cell)
		if m.DebugMode {
			rowStr += fmt.Sprintf("%s[%d]", renderedCell, len(renderedCell))
		} else {
			rowStr += renderedCell
		}
	}
	return rowStr + "\n"
}

func (m *DisplayModel) getMachineRowData(machine models.Machine) []string {
	elapsedTime := time.Since(machine.StartTime).Truncate(TickerInterval)
	progress, total := machine.ResourcesComplete()
	progressBar := renderProgressBar(
		progress,
		total,
		DisplayColumns[4].Width-ProgressBarPadding,
	)

	return []string{
		machine.Name,
		machine.Type.ShortResourceName,
		machine.Location,
		machine.StatusMessage,
		progressBar,
		formatElapsedTime(elapsedTime),
		machine.PublicIP,
		machine.PrivateIP,
		ConvertToEmoji(machine.Orchestrator),
		ConvertToEmoji(machine.SSH),
		ConvertToEmoji(machine.Docker),
		ConvertToEmoji(machine.Bacalhau),
		"",
	}
}

func (m *DisplayModel) findOrCreateMachine(status *models.DisplayStatus) (*models.Machine, bool) {
	for i, machine := range m.Deployment.Machines {
		if machine.Name == status.Name {
			return &m.Deployment.Machines[i], true
		}
	}

	if status.Name != "" && status.Type == models.AzureResourceTypeVM {
		newMachine := models.Machine{
			Name:          status.Name,
			Type:          status.Type,
			Location:      status.Location,
			StatusMessage: status.StatusMessage,
			StartTime:     time.Now(),
		}
		m.Deployment.Machines = append(m.Deployment.Machines, newMachine)
		return &m.Deployment.Machines[len(m.Deployment.Machines)-1], false
	}

	return nil, false
}

func (m *DisplayModel) updateMachineStatus(machine *models.Machine, status *models.DisplayStatus) {
	if status.StatusMessage != "" {
		trimmedStatus := strings.TrimSpace(status.StatusMessage)
		if len(trimmedStatus) > StatusLength-3 {
			machine.StatusMessage = trimmedStatus[:StatusLength-3] + "…"
		} else {
			machine.StatusMessage = fmt.Sprintf("%-*s", StatusLength, trimmedStatus)
		}
	}

	if status.Location != "" {
		machine.Location = status.Location
	}
	if status.PublicIP != "" {
		machine.PublicIP = status.PublicIP
	}
	if status.PrivateIP != "" {
		machine.PrivateIP = status.PrivateIP
	}
	if status.ElapsedTime > 0 && !machine.Complete() {
		machine.ElapsedTime = status.ElapsedTime
	}
	if status.Orchestrator {
		machine.Orchestrator = status.Orchestrator
	}
	if status.SSH != models.ServiceStateUnknown {
		machine.SSH = status.SSH
		if status.SSH == models.ServiceStateSucceeded {
			go m.installDockerAndCorePackages(machine)
		}
	}
	if status.Docker != models.ServiceStateUnknown {
		machine.Docker = status.Docker
	}
	if status.CorePackages != models.ServiceStateUnknown {
		machine.CorePackages = status.CorePackages
	}
	if status.Bacalhau != models.ServiceStateUnknown {
		machine.Bacalhau = status.Bacalhau
	}
}

func (m *DisplayModel) installDockerAndCorePackages(machine *models.Machine) {
	// Install Docker
	machine.Docker = models.ServiceStateUpdating
	machine.Docker = models.ServiceStateSucceeded
	machine.CorePackages = models.ServiceStateUpdating
	machine.CorePackages = models.ServiceStateSucceeded
}

func renderStyleByColumn(status string, style lipgloss.Style) lipgloss.Style {
	style = style.Bold(true).Align(lipgloss.Center)
	switch status {
	case models.DisplayTextSuccess:
		style = style.Foreground(lipgloss.Color("#00c413"))
	case models.DisplayTextWaiting:
		style = style.Foreground(lipgloss.Color("#69acdb"))
	case models.DisplayTextNotStarted:
		style = style.Foreground(lipgloss.Color("#2e2d2d"))
	case models.DisplayTextFailed:
		style = style.Foreground(lipgloss.Color("#ff0000"))
	}
	return style
}

func renderProgressBar(progress, total, width int) string {
	if total == 0 {
		return ""
	}
	filledWidth := int(math.Ceil(float64(progress) * float64(width) / float64(total)))
	emptyWidth := width - filledWidth
	if emptyWidth < 0 {
		emptyWidth = 0
	}

	filled := lipgloss.NewStyle().
		Foreground(lipgloss.Color("42")).
		Render(strings.Repeat("█", filledWidth))
	empty := lipgloss.NewStyle().
		Foreground(lipgloss.Color("237")).
		Render(strings.Repeat("█", emptyWidth))

	return filled + empty
}

func formatElapsedTime(d time.Duration) string {
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	tenths := int(d.Milliseconds()/100) % 10

	if minutes > 0 {
		return fmt.Sprintf("%dm%02d.%ds", minutes, seconds, tenths)
	}
	return fmt.Sprintf("%2d.%ds", seconds, tenths)
}

func (m *DisplayModel) updateLogCmd() tea.Cmd {
	return func() tea.Msg {
		logLines := testutils.GetLastLogLines()
		if len(logLines) > 0 {
			return logLinesMsg(logLines[len(logLines)-1])
		}
		return nil
	}
}

type tickMsg time.Time
type logLinesMsg string

func tickCmd() tea.Cmd {
	return tea.Tick(TickerInterval, func(t time.Time) tea.Msg {
		return tickMsg(t)
	})
}

func ConvertToEmoji(value interface{}) string {
	switch v := value.(type) {
	case bool:
		if v {
			return models.DisplayTextOrchestratorNode
		}
		return models.DisplayTextWorkerNode
	case models.ServiceState:
		switch v {
		case models.ServiceStateNotStarted:
			if EmojisEnabled {
				return models.DisplayEmojiNotStarted
			}
			return models.DisplayTextNotStarted
		case models.ServiceStateSucceeded:
			if EmojisEnabled {
				return models.DisplayEmojiSuccess
			}
			return models.DisplayTextSuccess
		case models.ServiceStateUpdating:
			if EmojisEnabled {
				return models.DisplayEmojiWaiting
			}
			return models.DisplayTextWaiting
		case models.ServiceStateCreated:
			if EmojisEnabled {
				return models.DisplayEmojiCreating
			}
			return models.DisplayTextCreating
		case models.ServiceStateFailed:
			if EmojisEnabled {
				return models.DisplayEmojiFailed
			}
			return models.DisplayTextFailed
		}
	}

	if EmojisEnabled {
		return models.DisplayEmojiWaiting
	}
	return models.DisplayTextWaiting
}
