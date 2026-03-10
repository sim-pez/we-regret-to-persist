package config

import (
	"log/slog"
	"reflect"

	"github.com/joho/godotenv"
	"github.com/kelseyhightower/envconfig"
)

type DBConf struct {
	PostgresHost     string `required:"true" split_words:"true" mask:"false"`
	PostgresPort     int    `required:"true" split_words:"true" mask:"false"`
	PostgresUser     string `required:"true" split_words:"true" mask:"true"`
	PostgresDB       string `required:"false" split_words:"true" mask:"true"`
	PostgresPassword string `required:"true" split_words:"true" mask:"true"`
	ClaudeAPIKey     string `required:"true" split_words:"true" mask:"true"`
	KafkaBroker      string `required:"true" split_words:"true" mask:"false"`
	KafkaTopic       string `required:"true" split_words:"true" mask:"false"`
	KafkaGroupID     string `required:"true" split_words:"true" mask:"false"`
}

var OperatorRole string

func (m *DBConf) ShowLoadedVariables() {
	maskedConfig := *m
	configValue := reflect.ValueOf(&maskedConfig).Elem()
	configType := configValue.Type()

	args := []any{}
	for i := 0; i < configValue.NumField(); i++ {
		field := configType.Field(i)
		val := configValue.Field(i).Interface()
		if maskTag, ok := field.Tag.Lookup("mask"); ok && maskTag == "true" {
			val = "****"
		}
		args = append(args, field.Name, val)
	}

	slog.Info("loaded config", args...)
}

func LoadConfig() (*DBConf, error) {
	// .env is optional — in production, vars are injected via the environment
	_ = godotenv.Load(".env")
	var conf DBConf
	err := envconfig.Process("", &conf)
	if err != nil {
		return nil, err
	}

	return &conf, nil
}
