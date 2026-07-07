package config

import (
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Config struct {
	Ingest struct {
		Address string `mapstructure:"address"`
	} `mapstructure:"ingest"`
	API struct {
		Address string `mapstructure:"address"`
	} `mapstructure:"api"`
	Storage struct {
		DataPath string `mapstructure:"data_path"`
		TTLHours int    `mapstructure:"ttl_hours"`
	} `mapstructure:"storage"`
}

func Load(path string, log *zap.Logger) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	v.SetDefault("ingest.address", ":4317")
	v.SetDefault("api.address", ":8080")
	v.SetDefault("storage.data_path", "/data/badger")
	v.SetDefault("storage.ttl_hours", 24)

	if err := v.ReadInConfig(); err != nil {
		log.Warn("Config file not found, using defaults", zap.String("path", path), zap.Error(err))
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
