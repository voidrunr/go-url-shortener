package config

type Config struct {
	Addr string
	Port int
}

func GetDefault () Config {
	return Config {
		Addr: "localhost",
		Port: 8080,
	}
}
