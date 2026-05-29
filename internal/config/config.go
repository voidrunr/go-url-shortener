package config

type Config struct {
	Addr string
	Port int
}

func GetDefault () Config {
	return Config {
		Addr: "http://localhost",
		Port: 8080,
	}
}
