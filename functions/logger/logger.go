package logger

import (
	"log"
	"os"
)

var logger *log.Logger

func GetLogger() *log.Logger {
	if logger == nil {
		logger = log.New(os.Stderr, "", log.LstdFlags|log.Llongfile)
	}
	return logger
}
