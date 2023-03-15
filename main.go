package main

import (
	"fmt"
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/gabihodoroga/pubsub-to-bigquery/config"
	"github.com/gabihodoroga/pubsub-to-bigquery/server"
)

func main() {
	
	if err := config.Setup(); err != nil {
		fmt.Printf("main: error initializing config %v\n", err)
		os.Exit(1)
	}

	logger, loggerLevel, err := initLogger()
	if err != nil {
		fmt.Printf("main: error initializing logger %v\n", err)
		os.Exit(1)
	}

	logger.Info("main: start")
	server.Start(logger, loggerLevel)
}


func initLogger() (*zap.Logger, zap.AtomicLevel, error) {
	loggerConfig := zap.NewProductionConfig()

	loggerConfig.Level.SetLevel(resolveLogLevel())
	loggerConfig.EncoderConfig.LevelKey = "severity"
	loggerConfig.EncoderConfig.MessageKey = "message"
	logger, err := loggerConfig.Build()
	if err != nil {
		return nil, zap.AtomicLevel{}, err
	}

	zap.ReplaceGlobals(logger)
	return logger, loggerConfig.Level, nil
}

func resolveLogLevel() zapcore.Level {
	if config.GetConfig().LogLevel != "" {
		var l zapcore.Level
		if err := l.UnmarshalText([]byte(config.GetConfig().LogLevel)); err != nil {
			return zap.InfoLevel
		}
		return l
	}
	return zap.InfoLevel
}




