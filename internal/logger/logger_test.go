package logger

import (
	"bytes"
	"log"
	"strings"
	"sync"
	"testing"
)

func TestLoggerFunctionsCalled(t *testing.T) {
	var buf bytes.Buffer

	// Reset singleton
	logger = nil
	once = sync.Once{}

	// Redirect package logger to buffer
	logger = log.New(&buf, "", log.LstdFlags|log.Lshortfile)

	Info("info message")
	Error("error message")
	Debug("debug message")

	output := buf.String()
	if !strings.Contains(output, "info message") ||
		!strings.Contains(output, "error message") ||
		!strings.Contains(output, "debug message") {
		t.Errorf("logger functions not called, output: %s", output)
	}
}
