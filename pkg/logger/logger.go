package logger

import (
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type Config struct {
	Level      string `yaml:"level"`
	Format     string `yaml:"format"`
	Output     string `yaml:"output"`
	TimeFormat string `yaml:"time_format"`
}

type Logger struct {
	logger zerolog.Logger
}

func New(config Config) *Logger {
	level := parseLevel(config.Level)
	zerolog.SetGlobalLevel(level)

	var output io.Writer = os.Stdout
	if config.Output != "" && config.Output != "stdout" {
		if file, err := os.OpenFile(config.Output, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666); err == nil {
			output = file
		}
	}

	var logger zerolog.Logger
	if config.Format == "console" {
		logger = zerolog.New(zerolog.ConsoleWriter{
			Out:        output,
			TimeFormat: getTimeFormat(config.TimeFormat),
		}).With().Timestamp().Logger()
	} else {
		logger = zerolog.New(output).With().Timestamp().Logger()
	}

	return &Logger{logger: logger}
}

func (l *Logger) Debug(msg string) {
	l.logger.Debug().Msg(msg)
}

func (l *Logger) Info(msg string) {
	l.logger.Info().Msg(msg)
}

func (l *Logger) Warn(msg string) {
	l.logger.Warn().Msg(msg)
}

func (l *Logger) Error(msg string) {
	l.logger.Error().Msg(msg)
}

func (l *Logger) Fatal(msg string) {
	l.logger.Fatal().Msg(msg)
}

func (l *Logger) WithField(key string, value interface{}) *Logger {
	return &Logger{logger: l.logger.With().Interface(key, value).Logger()}
}

func (l *Logger) WithFields(fields map[string]interface{}) *Logger {
	logger := l.logger.With()
	for k, v := range fields {
		logger = logger.Interface(k, v)
	}
	return &Logger{logger: logger.Logger()}
}

func (l *Logger) WithError(err error) *Logger {
	return &Logger{logger: l.logger.With().Err(err).Logger()}
}

func parseLevel(level string) zerolog.Level {
	switch level {
	case "debug":
		return zerolog.DebugLevel
	case "info":
		return zerolog.InfoLevel
	case "warn":
		return zerolog.WarnLevel
	case "error":
		return zerolog.ErrorLevel
	case "fatal":
		return zerolog.FatalLevel
	default:
		return zerolog.InfoLevel
	}
}

func getTimeFormat(format string) string {
	if format != "" {
		return format
	}
	return time.RFC3339
}

func SetGlobalLogger(logger *Logger) {
	log.Logger = logger.logger
}