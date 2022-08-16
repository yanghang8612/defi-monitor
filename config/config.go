package config

import (
    "fmt"

    "github.com/BurntSushi/toml"
)

type Config struct {
    SlackWebhook string `toml:"slack_webhook"`
    SUN          SUNConfig
    PSM          PSMConfig
}

type SUNConfig struct {
    SwapThreshold      int64 `toml:"swap_threshold"`
    LiquidityThreshold int64 `toml:"liquidity_threshold"`
}

type PSMConfig struct {
    GemThreshold int64 `toml:"gem_threshold"`
    DaiThreshold int64 `toml:"dai_threshold"`
}

func Get() *Config {
    var config Config
    data, err := toml.DecodeFile("./config.toml", &config)
    if err != nil {
        fmt.Println(data, err)
    }
    return &config
}
