package log

import (
	"os"

	"github.com/oops-reader/oops-reader-backend/internal/platform/config"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func NewLogger(cfg *config.Config) *zap.Logger {
	var encoder zapcore.Encoder

	if cfg.Log.Format == "console" {
		encoderConfig := zap.NewDevelopmentEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
		encoder = zapcore.NewConsoleEncoder(encoderConfig)
	} else {
		encoderConfig := zap.NewProductionEncoderConfig()
		encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
		encoder = zapcore.NewJSONEncoder(encoderConfig)
	}

	var writer zapcore.WriteSyncer
	if cfg.Log.OutputPath == "stdout" {
		writer = zapcore.AddSync(os.Stdout)
	} else {
		file, err := os.OpenFile(cfg.Log.OutputPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
		if err != nil {
			panic(err)
		}
		writer = zapcore.AddSync(file)
	}

	level := zapcore.InfoLevel
	switch cfg.Log.Level {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	}

	core := zapcore.NewCore(encoder, writer, level)
	logger := zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1))

	return logger
}
