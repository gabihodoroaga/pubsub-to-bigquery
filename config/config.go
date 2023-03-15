package config

import (
	"errors"
	"os"
)

var config *Config

// Config hold entire app configuration seetings. All value are read from environment variables
type Config struct {
	Port               string
	LogLevel           string
	TraceSample        string
	PubsubProject string
	PubSubSubscription string
	BigQueryProject    string
	BigQueryDataset    string
	BigQueryTable      string
}

// Setup read all the environment variables and validate the configuration
func Setup() error {
	config = &Config{}

	config.LogLevel = os.Getenv("LOG_LEVEL")
	config.Port = os.Getenv("PORT")
	config.TraceSample = os.Getenv("TRACE_SAMPLE")
	config.PubsubProject = os.Getenv("PUBSUB_PROJECT")
	if config.PubsubProject == "" {
		return errors.New("PUBSUB_PROJECT environment variable is required")
	}
	config.PubSubSubscription = os.Getenv("PUBSUB_SUBSCRIPTION")
	if config.PubSubSubscription == "" {
		return errors.New("PUBSUB_SUBSCRIPTION environment variable is required")
	}
	config.BigQueryProject = os.Getenv("BIGQUERY_PROJECT")
	if config.BigQueryProject == "" {
		return errors.New("BIGQUERY_PROJECT environment variable is required")
	}
	config.BigQueryDataset = os.Getenv("BIGQUERY_DATASET")
	if config.BigQueryDataset == "" {
		return errors.New("BIGQUERY_DATASET environment variable is required")
	}
	config.BigQueryTable = os.Getenv("BIGQUERY_TABLE")
	if config.BigQueryTable == "" {
		return errors.New("BIGQUERY_TABLE environment variable is required")
	}
	return nil
}

// GetConfig returns the current app configuration
func GetConfig() *Config {
	return config
}
