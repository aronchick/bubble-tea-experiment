package testutils

import (
	"context"
	"sync"
	"time"
)

const logBufferSize = 10

var logBuffer = make([]string, logBufferSize)
var logBufferIndex = 0
var logBufferMutex sync.Mutex

// GetLastLogLines returns the last logBufferSize log lines from the circular buffer
func GetLastLogLines() []string {
	logBufferMutex.Lock()
	defer logBufferMutex.Unlock()

	var logLines []string
	for i := 0; i < logBufferSize; i++ {
		index := (logBufferIndex + i) % logBufferSize
		if logBuffer[index] != "" {
			logLines = append(logLines, logBuffer[index])
		}
	}

	return logLines
}

func StartLogGenerator(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				logEntry := GenerateRandomLogEntry()
				logBufferMutex.Lock()
				logBuffer[logBufferIndex] = logEntry
				logBufferIndex = (logBufferIndex + 1) % logBufferSize
				logBufferMutex.Unlock()
			}
		}
	}()
}
