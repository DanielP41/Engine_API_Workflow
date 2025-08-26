package logger

import (
	"log"

	"go.uber.org/zap"
)

var logger *zap.SugaredLogger

func Init(environment string) {
	var zapLogger *zap.Logger
	var err error

	if environment == "production" {
		zapLogger, err = zap.NewProduction()
	} else {
		zapLogger, err = zap.NewDevelopment()
	}

	if err != nil {
		log.Fatal("Failed to initialize logger:", err)
	}

	logger = zapLogger.Sugar()
}

func Info(msg string) {
	if logger != nil {
		logger.Info(msg)
	} else {
		log.Println("INFO:", msg)
	}
}

func Error(msg string) {
	if logger != nil {
		logger.Error(msg)
	} else {
		log.Println("ERROR:", msg)
	}
}

func Fatal(msg string) {
	if logger != nil {
		logger.Fatal(msg)
	} else {
		log.Fatal("FATAL:", msg)
	}
}

func Debug(msg string) {
	if logger != nil {
		logger.Debug(msg)
	} else {
		log.Println("DEBUG:", msg)
	}
}

func Warn(msg string) {
	if logger != nil {
		logger.Warn(msg)
	} else {
		log.Println("WARN:", msg)
	}
}
