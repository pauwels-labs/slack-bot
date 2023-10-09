package config

type SlackConfig struct {
	SigningKey string `mapstructure:"signingkey"`
}

type Config struct {
	Port  uint16      `mapstructure:"port"`
	Slack SlackConfig `mapstructure:"slack"`
}
