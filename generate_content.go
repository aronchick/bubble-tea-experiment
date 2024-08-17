package main

import (
	"context"
	rand "math/rand/v2"
	"time"

	"github.com/aronchick/bubble-tea-experiment/pkg/models"
	"github.com/aronchick/bubble-tea-experiment/pkg/testutils"
)

var totalTasks = 20

func generateEvents(ctx context.Context, logChan chan<- string) {
	statuses := make(map[string]*models.DisplayStatus)
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for i := 0; i < totalTasks; i++ {
		newStatus := testutils.CreateRandomStatus()
		statuses[newStatus.ID] = newStatus
	}

	logTicker := time.NewTicker(2 * time.Second)
	defer logTicker.Stop()

	for {
		select {
		case <-ticker.C:
			if Quitting {
				return
			}
			if status := testutils.GetRandomStatus(statuses); status != nil {
				updateRandomStatus(status)
			}
		case <-logTicker.C:
			if Quitting {
				return
			}
			logEntry := testutils.GenerateRandomLogEntry()
			select {
			case logChan <- logEntry:
			case <-ctx.Done():
				return
			}
		case <-ctx.Done():
			return
		}
	}
}
func updateRandomStatus(status *models.DisplayStatus) bool {
	oldStatus := *status
	status.ElapsedTime += time.Duration(rand.IntN(10)) * time.Second
	status.StatusMessage = testutils.RandomStatus()
	status.DetailedStatus = testutils.GetRandomDetailedStatus(status.StatusMessage)
	return oldStatus != *status // Return true if there's a change
}
