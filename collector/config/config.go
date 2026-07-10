package config

import (
	"strings"

	"github.com/spf13/viper"
	"go.uber.org/zap"
)

type Config struct {
	GRPC struct {
		Address string `mapstructure:"address"`
	} `mapstructure:"grpc"`
	HTTP struct {
		Address string `mapstructure:"address"`
	} `mapstructure:"http"`
	Storage struct {
		Path string `mapstructure:"path"`
	} `mapstructure:"storage"`
	Retention struct {
		Hours int `mapstructure:"hours"`
	} `mapstructure:"retention"`
}

func Load(path string, log *zap.Logger) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)

	v.SetDefault("grpc.address", "0.0.0.0:4317")
	v.SetDefault("http.address", "0.0.0.0:8080")
	v.SetDefault("storage.path", "/data/badger")
	v.SetDefault("retention.hours", 24)

	// Allow environment variable overrides:
	// e.g. ZEROTRACE_STORAGE_PATH=/tmp/badger overrides storage.path
	v.SetEnvPrefix("ZEROTRACE")
	v.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	v.AutomaticEnv()

	if err := v.ReadInConfig(); err != nil {
		log.Warn("Config file not found, using defaults", zap.String("path", path), zap.Error(err))
	}

	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
