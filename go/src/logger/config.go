package logger

// Config 定义日志组件初始化参数。
type Config struct {
	Level     string
	Format    string
	AddSource bool
	Service   string
	Env       string
}

func defaultConfig() Config {
	return Config{
		Level:     "info",
		Format:    "text",
		AddSource: false,
		Service:   "goster-iot",
		Env:       "dev",
	}
}
