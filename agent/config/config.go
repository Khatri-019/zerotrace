package config

import (
    "github.com/spf13/viper"
    "go.uber.org/zap"
)

type Config struct {
    Collector struct {
        Address string `mapstructure:"address"`
        TLS     bool   `mapstructure:"tls"`
    } `mapstructure:"collector"`
    Probes struct {
        TCP       bool `mapstructure:"tcp"`
        SSL       bool `mapstructure:"ssl"`
        HTTPParse bool `mapstructure:"http_parse"`
        Process   bool `mapstructure:"process"`
    } `mapstructure:"probes"`
    Filters struct {
        ExcludeComms []string `mapstructure:"exclude_comms"`
        ExcludePorts []int    `mapstructure:"exclude_ports"`
    } `mapstructure:"filters"`
    RingBuffer struct {
        SizeMB int `mapstructure:"size_mb"`
    } `mapstructure:"ring_buffer"`
    Export struct {
        BatchSize       int `mapstructure:"batch_size"`
        FlushIntervalMS int `mapstructure:"flush_interval_ms"`
    } `mapstructure:"export"`
}

func Load(path string, log *zap.Logger) (*Config, error) {
    v := viper.New()
    v.SetConfigFile(path)

    // Defaults — agent must not panic if config key is absent
    v.SetDefault("collector.address", "localhost:4317")
    v.SetDefault("collector.tls", false)
    v.SetDefault("probes.tcp", true)
    v.SetDefault("probes.ssl", true)
    v.SetDefault("probes.http_parse", true)
    v.SetDefault("probes.process", true)
    v.SetDefault("filters.exclude_comms", []string{"agent", "sshd", "systemd", "kworker"})
    v.SetDefault("filters.exclude_ports", []int{22, 53})
    v.SetDefault("ring_buffer.size_mb", 256)
    v.SetDefault("export.batch_size", 100)
    v.SetDefault("export.flush_interval_ms", 100)

    if err := v.ReadInConfig(); err != nil {
        log.Warn("Config file not found, using defaults", zap.String("path", path), zap.Error(err))
    }

    var cfg Config
    if err := v.Unmarshal(&cfg); err != nil {
        return nil, err
    }
    return &cfg, nil
}
