package models

import (
	"context"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
)

const StatusLength = 20

type DisplayModel struct {
	Deployment *Deployment
	TextBox    []string
	Quitting   bool
	DebugMode  bool
	cancel     context.CancelFunc
}

// DisplayMachine represents a single machine in the deployment
type DisplayMachine struct {
	Name          string
	Type          AzureResourceTypes
	Location      string
	StatusMessage string
	StartTime     time.Time
	ElapsedTime   time.Duration
	PublicIP      string
	PrivateIP     string
	Orchestrator  bool
	SSH           ServiceState
	Docker        ServiceState
	CorePackages  ServiceState
	Bacalhau      ServiceState
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
		Deployment: NewDeployment(),
		TextBox:    []string{"Resource Status Monitor"},
		DebugMode:  os.Getenv("DEBUG_DISPLAY") == "1",
	}
}

func (m *DisplayModel) updateMachineStatus(machine *Machine, status *DisplayStatus) {
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
	if status.SSH != ServiceStateUnknown {
		machine.SSH = status.SSH
	}
	if status.Docker != ServiceStateUnknown {
		machine.Docker = status.Docker
	}
	if status.CorePackages != ServiceStateUnknown {
		machine.CorePackages = status.CorePackages
	}
	if status.Bacalhau != ServiceStateUnknown {
		machine.Bacalhau = status.Bacalhau
	}
}

func (m *DisplayModel) findOrCreateMachine(status *DisplayStatus) (*Machine, bool) {
	for i, machine := range m.Deployment.Machines {
		if machine.Name == status.Name {
			return &m.Deployment.Machines[i], true
		}
	}

	if status.Name != "" && status.Type == AzureResourceTypeVM {
		newMachine := Machine{
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
