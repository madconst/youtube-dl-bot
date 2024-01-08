package main

import (
	"os"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Token          string `yaml:"token"`
	DownloaderPath string `yaml:"downloader_path"`
	BaseUrl        string `yaml:"base_url"`
	StorageDir     string `yaml:"storage_dir"`
	Debug          bool   `yaml:"debug"`
	HttpServer     string `yaml:"http_server"`
}

func Parse(data []byte) (*Config, error) {
	cfg := &Config{}
	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, err
	}
	return cfg, nil
}

func LoadConfig(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return Parse(data)
}
