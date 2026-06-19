package config

import "github.com/spf13/viper"

type Config struct {
	Server struct {
		Address string
	}
	Security struct {
		JWTIssuer  string
		AccessTTL  string
		RefreshTTL string
		JWTSecret  string
	}
	Database struct {
		DSN string
	}
	XDP struct {
		Interface   string
		AttachMode  string
		ProgramName string
	}
	NFT struct {
		TableFamily string
		TableName   string
		SetName     string
	}
}

func Load(path string) (*Config, error) {
	v := viper.New()
	v.SetConfigFile(path)
	if err := v.ReadInConfig(); err != nil {
		return nil, err
	}
	var cfg Config
	if err := v.Unmarshal(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}
