package testutils

import (
	"sync"
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

func AppendToLogBuffer(logLine string) {
	logBufferMutex.Lock()
	defer logBufferMutex.Unlock()

	logBuffer[logBufferIndex] = logLine
	logBufferIndex = (logBufferIndex + 1) % logBufferSize
}
