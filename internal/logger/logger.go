package logger

import (
	"log"
	"os"
	"sync"
)

var (
	once   sync.Once
	logger *log.Logger
)

func Init() {
	once.Do(func() {
		logger = log.New(os.Stdout, "APP_LOG: ", log.LstdFlags|log.Lshortfile)
	})
}

func Info(message string, v ...interface{}) {
	if logger == nil {
		Init()
	}
	logger.Printf("INFO: "+message, v...)
}

func Error(message string, v ...interface{}) {
	if logger == nil {
		Init()
	}
	logger.Printf("ERROR: "+message, v...)
}

func Debug(message string, v ...interface{}) {
	if logger == nil {
		Init()
	}
	logger.Printf("DEBUG: "+message, v...)
}
