package utils

import (
	"fmt"
	"os"
	"strings"

	"github.com/rs/zerolog"
)

type LogInterface interface {
	Debug(message interface{}, args ...interface{})
	Info(message interface{}, args ...interface{})
	Warn(message interface{}, args ...interface{})
	Error(message interface{}, args ...interface{})
	Fatal(message interface{}, args ...interface{})
}

type Logger struct {
	logger *zerolog.Logger
}

func NewLogger(level string) LogInterface {
	var l zerolog.Level

	switch strings.ToLower(level) {
	case "debug":
		l = zerolog.DebugLevel
	case "info":
		l = zerolog.InfoLevel
	case "warn":
		l = zerolog.WarnLevel
	case "error":
		l = zerolog.ErrorLevel
	case "fatal":
		l = zerolog.FatalLevel
	default:
		l = zerolog.InfoLevel
	}

	zerolog.SetGlobalLevel(l)
	skipFrameCount := 3
	logger := zerolog.New(os.Stdout).With().Timestamp().CallerWithSkipFrameCount(zerolog.CallerSkipFrameCount + skipFrameCount).Logger()

	return &Logger{logger: &logger}
}

func (l *Logger) Debug(message interface{}, args ...interface{}) {
	l.logger.Debug().Msgf(fmt.Sprintf("%v", message), args...)
}

func (l *Logger) Info(message interface{}, args ...interface{}) {
	l.logger.Info().Msgf(fmt.Sprintf("%v", message), args...)
}

func (l *Logger) Warn(message interface{}, args ...interface{}) {
	l.logger.Warn().Msgf(fmt.Sprintf("%v", message), args...)
}

func (l *Logger) Error(message interface{}, args ...interface{}) {

	l.logger.Error().Msgf(fmt.Sprintf("%v", message), args...)
}

func (l *Logger) Fatal(message interface{}, args ...interface{}) {
	l.logger.Fatal().Msgf(fmt.Sprintf("%v", message), args...)
}
