package config

import (
	"fmt"
	"time"

	"github.com/spf13/viper"
)

type Config struct {
	Server    ServerConfig    `mapstructure:"server"`
	Database  DatabaseConfig  `mapstructure:"database"`
	Redis     RedisConfig     `mapstructure:"redis"`
	JWT       JWTConfig       `mapstructure:"jwt"`
	Log       LogConfig       `mapstructure:"log"`
	TTS       TTSConfig       `mapstructure:"tts"`
	Community CommunityConfig `mapstructure:"community"`
}

type ServerConfig struct {
	Port            int           `mapstructure:"port"`
	Mode            string        `mapstructure:"mode"`
	ReadTimeout     time.Duration `mapstructure:"read_timeout"`
	WriteTimeout    time.Duration `mapstructure:"write_timeout"`
	ShutdownTimeout time.Duration `mapstructure:"shutdown_timeout"`
}

type DatabaseConfig struct {
	Host            string        `mapstructure:"host"`
	Port            int           `mapstructure:"port"`
	User            string        `mapstructure:"user"`
	Password        string        `mapstructure:"password"`
	Database        string        `mapstructure:"database"`
	MaxOpenConns    int           `mapstructure:"max_open_conns"`
	MaxIdleConns    int           `mapstructure:"max_idle_conns"`
	ConnMaxLifetime time.Duration `mapstructure:"conn_max_lifetime"`
	ConnMaxIdleTime time.Duration `mapstructure:"conn_max_idle_time"`
}

type RedisConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	Password string `mapstructure:"password"`
	DB       int    `mapstructure:"db"`
}

type JWTConfig struct {
	Secret           string        `mapstructure:"secret"`
	AccessTokenExpiry time.Duration `mapstructure:"access_token_expiry"`
	RefreshTokenExpiry time.Duration `mapstructure:"refresh_token_expiry"`
}

type LogConfig struct {
	Level      string `mapstructure:"level"`
	Format     string `mapstructure:"format"`
	OutputPath string `mapstructure:"output_path"`
}

type TTSConfig struct {
	DefaultProvider string        `mapstructure:"default_provider"`
	Edge            TTSEdgeConfig `mapstructure:"edge"`
	MiMo            TTSMiMoConfig `mapstructure:"mimo"`
	RateLimit       TTSRateLimit  `mapstructure:"rate_limit"`
	StreamRateLimit TTSRateLimit  `mapstructure:"stream_rate_limit"`
}

type TTSRateLimit struct {
	Enabled bool    `mapstructure:"enabled"`
	Rate    float64 `mapstructure:"rate"` // requests per minute per user
	Burst   int     `mapstructure:"burst"`
}

type TTSEdgeConfig struct {
	BaseURL string `mapstructure:"base_url"`
	Token   string `mapstructure:"token"`
}

type TTSMiMoConfig struct {
	APIKey  string `mapstructure:"api_key"`
	BaseURL string `mapstructure:"base_url"`
	Model   string `mapstructure:"model"`
}

type CommunityConfig struct {
	Images CommunityImagesConfig `mapstructure:"images"`
}

type CommunityImagesConfig struct {
	LocalPath string `mapstructure:"local_path"`
	PublicURL string `mapstructure:"public_url"`
}

func Load() (*Config, error) {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/oops-reader-backend")

	setDefaults()

	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); ok {
		} else {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	viper.AutomaticEnv()

	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	return &cfg, nil
}

func setDefaults() {
	viper.SetDefault("server.port", 8080)
	viper.SetDefault("server.mode", "debug")
	viper.SetDefault("server.read_timeout", 30*time.Second)
	viper.SetDefault("server.write_timeout", 30*time.Second)
	viper.SetDefault("server.shutdown_timeout", 30*time.Second)

	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 3306)
	viper.SetDefault("database.max_open_conns", 100)
	viper.SetDefault("database.max_idle_conns", 10)
	viper.SetDefault("database.conn_max_lifetime", 1*time.Hour)
	viper.SetDefault("database.conn_max_idle_time", 10*time.Minute)

	viper.SetDefault("redis.host", "localhost")
	viper.SetDefault("redis.port", 6379)
	viper.SetDefault("redis.db", 0)

	viper.SetDefault("jwt.access_token_expiry", 1*time.Hour)
	viper.SetDefault("jwt.refresh_token_expiry", 7*24*time.Hour)

	viper.SetDefault("log.level", "info")
	viper.SetDefault("log.format", "json")
	viper.SetDefault("log.output_path", "stdout")

	viper.SetDefault("tts.default_provider", "edge")
	viper.SetDefault("tts.edge.base_url", "http://8.136.58.109:80")
	viper.SetDefault("tts.edge.token", "")
	viper.SetDefault("tts.rate_limit.enabled", true)
	viper.SetDefault("tts.rate_limit.rate", 30) // 30 requests per minute
	viper.SetDefault("tts.rate_limit.burst", 5) // burst of 5
	viper.SetDefault("tts.stream_rate_limit.enabled", true)
	viper.SetDefault("tts.stream_rate_limit.rate", 120) // 120 streaming requests per minute
	viper.SetDefault("tts.stream_rate_limit.burst", 20) // allow a playback buffer to fill

	viper.SetDefault("community.images.local_path", "./data/community/images")
	viper.SetDefault("community.images.public_url", "/community/images")
}

func (c *Config) GetDSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.Database.User,
		c.Database.Password,
		c.Database.Host,
		c.Database.Port,
		c.Database.Database,
	)
}

func (c *Config) GetRedisAddr() string {
	return fmt.Sprintf("%s:%d", c.Redis.Host, c.Redis.Port)
}
