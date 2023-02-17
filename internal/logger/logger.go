package logger

import (
	"github.com/jwping/logger"
)

func NewLogger(logLevel int) *logger.Logger {
	return logger.NewLogger(logger.Options{
		Lt:        logger.JSON,
		Level:     logger.Level(logLevel),
		AddSource: true,
	})
}
