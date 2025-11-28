package logger

import (
	"io"
	"log"
	"os"
)

var DebugMode bool

func Init() {
	if os.Getenv("DEBUG") == "true" {
		DebugMode = true
	} else {
		// By default, discard all logs to prevent TUI corruption
		log.SetOutput(io.Discard)
	}
}

// SetOutput sets the output destination for the standard logger
func SetOutput(w io.Writer) {
	log.SetOutput(w)
}

func Debug(format string, v ...interface{}) {
	if DebugMode {
		log.Printf("[DEBUG] "+format, v...)
	}
}

func Info(format string, v ...interface{}) {
	// Always log info if debug is enabled, or if we have a specific InfoMode?
	// For now, let's just use standard log which goes to wherever SetOutput pointed it
	log.Printf("[INFO] "+format, v...)
}
