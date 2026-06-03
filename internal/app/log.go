package app

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	os2 "github.com/mk-5/fjira/internal/os"
)

var (
	logFile  *os.File
	logMutex sync.Mutex
	logInit  sync.Once
)

func initLogFile() {
	homeDir := os2.MustGetFjiraHomeDir()
	_ = os.MkdirAll(homeDir, 0750)
	path := filepath.Join(homeDir, "fjira.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0640)
	if err != nil {
		return
	}
	logFile = f
}

// LogError writes an error message to ~/.fjira/fjira.log with a timestamp.
func LogError(format string, args ...interface{}) {
	logMutex.Lock()
	defer logMutex.Unlock()
	logInit.Do(initLogFile)
	if logFile == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(logFile, "[%s] ERROR: %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
}

// LogInfo writes an info message to ~/.fjira/fjira.log with a timestamp.
func LogInfo(format string, args ...interface{}) {
	logMutex.Lock()
	defer logMutex.Unlock()
	logInit.Do(initLogFile)
	if logFile == nil {
		return
	}
	msg := fmt.Sprintf(format, args...)
	_, _ = fmt.Fprintf(logFile, "[%s] INFO: %s\n", time.Now().Format("2006-01-02 15:04:05"), msg)
}
