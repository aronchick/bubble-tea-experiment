package main

import (
	"context"
	"fmt"
	"math/rand"
	"os"
	"strings"
	"time"

	"github.com/aronchick/bubble-tea-experiment/pkg/models"
	"github.com/aronchick/bubble-tea-experiment/pkg/testutils"
	tea "github.com/charmbracelet/bubbletea"
)

const LogFilePath = "bubble-tea-experiment.log"

var LogFile *os.File

const statusLength = 30

var words = []string{
	"apple", "banana", "cherry", "date", "elderberry",
	"fig", "grape", "honeydew", "kiwi", "lemon",
	"mango", "nectarine", "orange", "papaya", "quince",
	"raspberry", "strawberry", "tangerine", "ugli", "watermelon",
}

func getRandomWords(n int) string {
	rand.Shuffle(len(words), func(i, j int) {
		words[i], words[j] = words[j], words[i]
	})
	return strings.Join(words[:n], " ")
}

func main() {
	// Create the log file
	if err := os.WriteFile(LogFilePath, []byte{}, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "Error creating log file: %v\n", err)
		os.Exit(1)
	}

	// Truncate the log file
	if err := os.Truncate(LogFilePath, 0); err != nil {
		fmt.Fprintf(os.Stderr, "Error truncating log file: %v\n", err)
		os.Exit(1)
	}

	var err error
	LogFile, err = os.OpenFile(LogFilePath, os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Error opening log file: %v\n", err)
		os.Exit(1)
	}
	defer LogFile.Close()

	// Ensure we flush the log file before exiting
	defer LogFile.Sync()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go testutils.StartLogGenerator(ctx, LogFilePath)
	if err := runTestDisplay(cancel); err != nil {
		fmt.Fprintf(os.Stderr, "Error running display: %v\n", err)
		os.Exit(1)
	}
}

func runTestDisplay(cancel context.CancelFunc) error {
	m := GetGlobalModel()
	m.Cancel = cancel
	p := tea.NewProgram(m, tea.WithAltScreen())

	done := make(chan struct{})
	fmt.Fprintf(LogFile, "Starting runTestDisplay\n")
	LogFile.Sync()

	go func() {
		defer close(done)
		fmt.Fprintf(LogFile, "Starting background goroutine\n")
		LogFile.Sync()
		totalTasks := 5
		statuses := make([]*models.DisplayStatus, totalTasks)
		for i := 0; i < totalTasks; i++ {
			newDisplayStatus := models.NewDisplayVMStatus(
				fmt.Sprintf("testVM%d", i+1),
				models.AzureResourceStateNotStarted,
			)
			newDisplayStatus.Location = testutils.RandomRegion()
			newDisplayStatus.StatusMessage = "Initializing"
			newDisplayStatus.DetailedStatus = "Starting"
			newDisplayStatus.ElapsedTime = 0
			newDisplayStatus.InstanceID = fmt.Sprintf("test%d", i+1)

			if i%2 == 0 {
				newDisplayStatus.Orchestrator = false
				newDisplayStatus.SSH = models.ServiceStateSucceeded
				newDisplayStatus.Docker = models.ServiceStateFailed
				newDisplayStatus.Bacalhau = models.ServiceStateSucceeded
			} else {
				newDisplayStatus.Orchestrator = true
				newDisplayStatus.SSH = models.ServiceStateFailed
				newDisplayStatus.Docker = models.ServiceStateSucceeded
				newDisplayStatus.Bacalhau = models.ServiceStateFailed
			}
			newDisplayStatus.PublicIP = testutils.RandomIP()
			newDisplayStatus.PrivateIP = testutils.RandomIP()
			statuses[i] = newDisplayStatus
			p.Send(models.StatusUpdateMsg{Status: statuses[i]})
		}

		wordTicker := time.NewTicker(1 * time.Second)
		timeTicker := time.NewTicker(100 * time.Millisecond)
		defer wordTicker.Stop()
		defer timeTicker.Stop()

		for {
			select {
			case <-wordTicker.C:
				for i := 0; i < totalTasks; i++ {
					rawStatus := getRandomWords(3)
					if len(rawStatus) > statusLength {
						statuses[i].StatusMessage = rawStatus[:statusLength]
					} else {
						statuses[i].StatusMessage = fmt.Sprintf("%-*s", statusLength, rawStatus)
					}
					statuses[i].Progress = (statuses[i].Progress + 1) % 7
					p.Send(models.StatusUpdateMsg{Status: statuses[i]})
				}
			case <-timeTicker.C:
				p.Send(models.TimeUpdateMsg{})
			case <-done:
				return
			}
		}
	}()

	if _, err := p.Run(); err != nil {
		return fmt.Errorf("Error running program: %v", err)
	}

	close(done)
	return nil
}
