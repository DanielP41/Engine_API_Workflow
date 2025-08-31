package logger

import (
	"log"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

// Logger wrapper compatible con main.go
type Logger struct {
	zap *zap.Logger
}

// New crea un nuevo logger compatible con main.go
func New(logLevel, environment string) *Logger {
	var zapLogger *zap.Logger
	var err error

	// Configurar nivel de log
	level := zapcore.InfoLevel
	switch strings.ToLower(logLevel) {
	case "debug":
		level = zapcore.DebugLevel
	case "info":
		level = zapcore.InfoLevel
	case "warn", "warning":
		level = zapcore.WarnLevel
	case "error":
		level = zapcore.ErrorLevel
	case "fatal":
		level = zapcore.FatalLevel
	}

	// Configurar logger según ambiente
	if environment == "production" {
		config := zap.NewProductionConfig()
		config.Level = zap.NewAtomicLevelAt(level)
		zapLogger, err = config.Build()
	} else {
		config := zap.NewDevelopmentConfig()
		config.Level = zap.NewAtomicLevelAt(level)
		zapLogger, err = config.Build()
	}

	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}

	return &Logger{zap: zapLogger}
}

// Info log con campos adicionales - CORREGIDO
func (l *Logger) Info(msg string, keysAndValues ...interface{}) {
	if len(keysAndValues) == 0 {
		l.zap.Sugar().Info(msg)
		return
	}

	if len(keysAndValues)%2 != 0 {
		// CORREGIDO: usar Infow para argumentos variádicos
		l.zap.Sugar().Infow(msg, keysAndValues...)
		return
	}

	fields := make([]zap.Field, 0, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, zap.Any(key, keysAndValues[i+1]))
	}

	l.zap.Info(msg, fields...)
}

// Error log con campos adicionales - CORREGIDO
func (l *Logger) Error(msg string, keysAndValues ...interface{}) {
	if len(keysAndValues) == 0 {
		l.zap.Sugar().Error(msg)
		return
	}

	if len(keysAndValues)%2 != 0 {
		// CORREGIDO: usar Errorw para argumentos variádicos
		l.zap.Sugar().Errorw(msg, keysAndValues...)
		return
	}

	fields := make([]zap.Field, 0, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, zap.Any(key, keysAndValues[i+1]))
	}

	l.zap.Error(msg, fields...)
}

// Fatal log con campos adicionales - CORREGIDO
func (l *Logger) Fatal(msg string, keysAndValues ...interface{}) {
	if len(keysAndValues) == 0 {
		l.zap.Sugar().Fatal(msg)
		return
	}

	if len(keysAndValues)%2 != 0 {
		// CORREGIDO: usar Fatalw para argumentos variádicos
		l.zap.Sugar().Fatalw(msg, keysAndValues...)
		return
	}

	fields := make([]zap.Field, 0, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, zap.Any(key, keysAndValues[i+1]))
	}

	l.zap.Fatal(msg, fields...)
}

// Debug log con campos adicionales - CORREGIDO
func (l *Logger) Debug(msg string, keysAndValues ...interface{}) {
	if len(keysAndValues) == 0 {
		l.zap.Sugar().Debug(msg)
		return
	}

	if len(keysAndValues)%2 != 0 {
		// CORREGIDO: usar Debugw para argumentos variádicos
		l.zap.Sugar().Debugw(msg, keysAndValues...)
		return
	}

	fields := make([]zap.Field, 0, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, zap.Any(key, keysAndValues[i+1]))
	}

	l.zap.Debug(msg, fields...)
}

// Warn log con campos adicionales - CORREGIDO
func (l *Logger) Warn(msg string, keysAndValues ...interface{}) {
	if len(keysAndValues) == 0 {
		l.zap.Sugar().Warn(msg)
		return
	}

	if len(keysAndValues)%2 != 0 {
		// CORREGIDO: usar Warnw para argumentos variádicos
		l.zap.Sugar().Warnw(msg, keysAndValues...)
		return
	}

	fields := make([]zap.Field, 0, len(keysAndValues)/2)
	for i := 0; i < len(keysAndValues); i += 2 {
		key, ok := keysAndValues[i].(string)
		if !ok {
			continue
		}
		fields = append(fields, zap.Any(key, keysAndValues[i+1]))
	}

	l.zap.Warn(msg, fields...)
}

// Sync sincroniza el logger
func (l *Logger) Sync() error {
	return l.zap.Sync()
}
