package logging

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
)

type zapLogger struct {
	logger *zap.Logger
}

// ZapConfig defines the options for the Zap rolling logger
type ZapConfig struct {
	// Filename is the file to write logs to.
	Filename string
	// MaxSize is the maximum size in megabytes of the log file before it gets rotated.
	MaxSize int
	// MaxBackups is the maximum number of old log files to retain.
	MaxBackups int
	// MaxAge is the maximum number of days to retain old log files.
	MaxAge int
	// Compress determines if the rotated log files should be compressed using gzip.
	Compress bool
	// Console specifies if logs should also be printed to standard output.
	Console bool
}

// DefaultZapLogger creates a high-performance logger using uber-go/zap with lumberjack for log rotation.
func DefaultZapLogger(cfg ZapConfig) Logger {
	if cfg.Filename == "" {
		cfg.Filename = "logs/gorag.log"
	}
	if cfg.MaxSize == 0 {
		cfg.MaxSize = 100 // default 100 MB
	}
	if cfg.MaxAge == 0 {
		cfg.MaxAge = 30 // default 30 days
	}
	if cfg.MaxBackups == 0 {
		cfg.MaxBackups = 7 // default 7 backups
	}

	// Lumberjack hook for log rotation
	lumberJackLogger := &lumberjack.Logger{
		Filename:   cfg.Filename,
		MaxSize:    cfg.MaxSize,
		MaxBackups: cfg.MaxBackups,
		MaxAge:     cfg.MaxAge,
		Compress:   cfg.Compress,
	}

	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder

	// Create a core that writes to the lumberjack rotating file
	fileWriter := zapcore.AddSync(lumberJackLogger)
	fileCore := zapcore.NewCore(
		zapcore.NewJSONEncoder(encoderConfig),
		fileWriter,
		zap.DebugLevel,
	)

	cores := []zapcore.Core{fileCore}

	// Add console output if requested
	if cfg.Console {
		consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
		consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zap.DebugLevel)
		cores = append(cores, consoleCore)
	}

	// Combine cores
	core := zapcore.NewTee(cores...)

	return &zapLogger{
		logger: zap.New(core, zap.AddCaller(), zap.AddCallerSkip(1)),
	}
}

func (l *zapLogger) Info(msg string, fields ...map[string]any) {
	l.logger.Info(msg, toZapFields(fields)...)
}

func (l *zapLogger) Error(msg string, err error, fields ...map[string]any) {
	zFields := toZapFields(fields)
	if err != nil {
		zFields = append(zFields, zap.Error(err))
	}
	l.logger.Error(msg, zFields...)
}

func (l *zapLogger) Debug(msg string, fields ...map[string]any) {
	l.logger.Debug(msg, toZapFields(fields)...)
}

func (l *zapLogger) Warn(msg string, fields ...map[string]any) {
	l.logger.Warn(msg, toZapFields(fields)...)
}

func toZapFields(maps []map[string]any) []zap.Field {
	if len(maps) == 0 {
		return nil
	}
	
	var fields []zap.Field
	for _, m := range maps {
		for k, v := range m {
			fields = append(fields, zap.Any(k, v))
		}
	}
	return fields
}