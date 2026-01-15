// Package logging provides structured logging utilities.
package logging

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var (
	// Logger is the global logger instance
	Logger *zap.Logger

	// Sugar is the sugared logger for convenience
	Sugar *zap.SugaredLogger
)

// Config contains logging configuration
type Config struct {
	// Level is the minimum log level
	Level string `json:"level"`

	// Format is the output format (json, console)
	Format string `json:"format"`

	// Output is the output destination (stdout, stderr, file path)
	Output string `json:"output"`

	// Development enables development mode
	Development bool `json:"development"`
}

// DefaultConfig returns sensible defaults
func DefaultConfig() Config {
	return Config{
		Level:       "info",
		Format:      "console",
		Output:      "stderr",
		Development: false,
	}
}

// Initialize sets up the global logger
func Initialize(cfg Config) error {
	level, err := zapcore.ParseLevel(cfg.Level)
	if err != nil {
		level = zapcore.InfoLevel
	}

	var encoder zapcore.Encoder
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.TimeKey = "timestamp"
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	if cfg.Format == "console" {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	var writeSyncer zapcore.WriteSyncer
	switch cfg.Output {
	case "stdout":
		writeSyncer = zapcore.AddSync(os.Stdout)
	case "stderr":
		writeSyncer = zapcore.AddSync(os.Stderr)
	default:
		file, err := os.OpenFile(cfg.Output, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
		if err != nil {
			return err
		}
		writeSyncer = zapcore.AddSync(file)
	}

	core := zapcore.NewCore(encoder, writeSyncer, level)

	if cfg.Development {
		Logger = zap.New(core, zap.Development(), zap.AddCaller(), zap.AddStacktrace(zapcore.ErrorLevel))
	} else {
		Logger = zap.New(core, zap.AddCaller())
	}

	Sugar = Logger.Sugar()
	return nil
}

// InitializeDefault sets up the logger with default configuration
func InitializeDefault() {
	_ = Initialize(DefaultConfig())
}

// Sync flushes the logger
func Sync() {
	if Logger != nil {
		_ = Logger.Sync()
	}
}

// With returns a logger with additional fields
func With(fields ...zap.Field) *zap.Logger {
	return Logger.With(fields...)
}

// Debug logs at debug level
func Debug(msg string, fields ...zap.Field) {
	Logger.Debug(msg, fields...)
}

// Info logs at info level
func Info(msg string, fields ...zap.Field) {
	Logger.Info(msg, fields...)
}

// Warn logs at warn level
func Warn(msg string, fields ...zap.Field) {
	Logger.Warn(msg, fields...)
}

// Error logs at error level
func Error(msg string, fields ...zap.Field) {
	Logger.Error(msg, fields...)
}

// Fatal logs at fatal level and exits
func Fatal(msg string, fields ...zap.Field) {
	Logger.Fatal(msg, fields...)
}

func init() {
	InitializeDefault()
}
