package logger

import (
	"log"
	"os"
)

var (
	Info  = log.New(os.Stdout, "[INFO]  ", log.LstdFlags|log.Lmsgprefix)
	Warn  = log.New(os.Stdout, "[WARN]  ", log.LstdFlags|log.Lmsgprefix)
	Error = log.New(os.Stderr, "[ERROR] ", log.LstdFlags|log.Lmsgprefix)
	Debug = log.New(os.Stdout, "[DEBUG] ", log.LstdFlags|log.Lmsgprefix)
)

// SetDebugMode enables or disables debug logging
func SetDebugMode(enabled bool) {
	if !enabled {
		Debug.SetOutput(os.NewFile(0, os.DevNull))
	} else {
		Debug.SetOutput(os.Stdout)
	}
}
