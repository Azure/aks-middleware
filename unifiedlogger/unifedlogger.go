package logger

import (
	"context"
	"log/slog"
	"os"

	"github.com/sirupsen/logrus"
)

type Logger interface {
	Debug(msg string, fields map[string]interface{})
	Info(msg string, fields map[string]interface{})
	Warn(msg string, fields map[string]interface{})
	Error(msg string, fields map[string]interface{})
	Fatal(msg string, fields map[string]interface{})
}

type LogrusLogger struct {
	logger *logrus.Logger
	ctx    context.Context
}

func NewLogrusLogger(loggerType string, ctx context.Context) *LogrusLogger {
	logger := logrus.New()
	logger.Out = os.Stdout

	return &LogrusLogger{logger: logger, ctx: ctx}
}

func (l *LogrusLogger) Debug(msg string, fields map[string]interface{}) {
	l.logger.WithFields(fields).Debug(msg)
}

func (l *LogrusLogger) Info(msg string, fields map[string]interface{}) {
	l.logger.WithFields(fields).Info(msg)
}

func (l *LogrusLogger) Warn(msg string, fields map[string]interface{}) {
	l.logger.WithFields(fields).Warn(msg)
}

func (l *LogrusLogger) Error(msg string, fields map[string]interface{}) {
	l.logger.WithFields(fields).Error(msg)
}

func (l *LogrusLogger) Fatal(msg string, fields map[string]interface{}) {
	l.logger.WithFields(fields).Fatal(msg)
}

type SlogLogger struct {
	logger *slog.Logger
	ctx    context.Context
}

func NewSlogLogger(loggerType string, ctx context.Context) *SlogLogger {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	return &SlogLogger{logger: logger, ctx: ctx}
}

func (l *SlogLogger) Debug(msg string, fields map[string]interface{}) {
	l.logger.Debug(msg, slog.Any("args", fields))
}

func (l *SlogLogger) Info(msg string, fields map[string]interface{}) {
	l.logger.Info(msg, slog.Any("args", fields))
}

func (l *SlogLogger) Warn(msg string, fields map[string]interface{}) {
	l.logger.Warn(msg, slog.Any("args", fields))
}

func (l *SlogLogger) Error(msg string, fields map[string]interface{}) {
	l.logger.Error(msg, slog.Any("args", fields))
}

func (l *SlogLogger) Fatal(msg string, fields map[string]interface{}) {
	l.logger.Error(msg, slog.Any("args", fields))
	os.Exit(1)
}

type LoggerWrapper struct {
	logger Logger
}

func NewLoggerWrapper(loggerType string, ctx context.Context) *LoggerWrapper {
	var logger Logger

	switch loggerType {
	case "logrus":
		logger = NewLogrusLogger(loggerType, ctx)
	case "slog":
		logger = NewSlogLogger(loggerType, ctx)
	default:
		logger = NewLogrusLogger(loggerType, ctx)
	}

	return &LoggerWrapper{logger: logger}
}

func (lw *LoggerWrapper) Debug(msg string, fields map[string]interface{}) {
	lw.logger.Debug(msg, fields)
}

func (lw *LoggerWrapper) Info(msg string, fields map[string]interface{}) {
	lw.logger.Info(msg, fields)
}

func (lw *LoggerWrapper) Warn(msg string, fields map[string]interface{}) {
	lw.logger.Warn(msg, fields)
}

func (lw *LoggerWrapper) Error(msg string, fields map[string]interface{}) {
	lw.logger.Error(msg, fields)
}

func (lw *LoggerWrapper) Fatal(msg string, fields map[string]interface{}) {
	lw.logger.Fatal(msg, fields)
}
