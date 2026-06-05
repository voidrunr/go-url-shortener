package config

import "flag"

type Config struct {
	ServerAddress string
	BaseURL       string
}

func Parse() Config {
	cfg := Config{}

	flag.StringVar(&cfg.ServerAddress, "a", "localhost:8080", "address to run HTTP server on")
	flag.StringVar(&cfg.BaseURL, "b", "http://localhost:8080", "base URL for shortened links")
	flag.Parse()

	return cfg
}
