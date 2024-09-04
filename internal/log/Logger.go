package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger is a wrapper around zap.SugaredLogger
type Logger struct {
    *zap.SugaredLogger
}

// NewLogger creates a new Logger instance
func NewLogger(development, debug bool) (*Logger, error) {
    var config zap.Config
    if development {
        config = zap.NewDevelopmentConfig()
    } else {
        config = zap.NewProductionConfig()
    }
    if debug {
        config.Level = zap.NewAtomicLevelAt(zap.DebugLevel)
    } else {
        config.Level = zap.NewAtomicLevelAt(zap.InfoLevel)
    }        

    config.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
    config.OutputPaths = []string{"web-server.log"}
    config.ErrorOutputPaths = []string{"web-server.log"}

    zapLogger, err := config.Build()
    if err != nil {
        return nil, err
    }

    sugar := zapLogger.Sugar()
    return &Logger{sugar}, nil
}

// Sync flushes any buffered log entries
func (l *Logger) Sync() error {
    return l.SugaredLogger.Sync()
}