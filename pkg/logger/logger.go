package logger

import (
	"fmt"
	"os"
	"sync"
)

var (
	debugMode bool
	logFile   *os.File
	mu        sync.Mutex
)

func init() {
	debugMode = os.Getenv("DEBUG_CHANNELS") == "1"
}

// SetLogFile sets the file to write debug logs to
func SetLogFile(file *os.File) {
	mu.Lock()
	defer mu.Unlock()
	logFile = file
}

// Debug logs a debug message if DEBUG_CHANNELS is set to 1
func Debug(format string, args ...interface{}) {
	if debugMode {
		mu.Lock()
		defer mu.Unlock()
		if logFile != nil {
			fmt.Fprintf(logFile, format+"\n", args...)
			logFile.Sync()
		}
	}
}
