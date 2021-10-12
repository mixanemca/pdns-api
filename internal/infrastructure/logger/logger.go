package logger

import (
	"github.com/sirupsen/logrus"
	"os"
)

func NewLogger(logFilePath string, logLevel string) *logrus.Logger {
	var logger = logrus.New()
	logger.SetFormatter(&logrus.JSONFormatter{})
	// default log output
	logger.Out = os.Stdout
	file, err := os.OpenFile(logFilePath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0664)
	if err == nil {
		logger.Out = file
	} else {
		logger.Warn("failed to log to file, using default stdout")
	}
	// Log level
	level, err := logrus.ParseLevel(logLevel)
	if err == nil {
		logger.SetLevel(level)
	} else {
		// Default info level
		logger.SetLevel(logrus.InfoLevel)
		logger.Warnf("failed to parse log level %s, using default info", logLevel)
	}

	return logger
}
