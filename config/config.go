package config

// Config holds the application configuration
type Config struct {
	Node      NodeConfig       `mapstructure:"node"`
	InfluxDB  InfluxDBConfig   `mapstructure:"db"`
	Contracts []ContractConfig `mapstructure:"contracts"`
	Logging   LoggingConfig    `mapstructure:"logging"`
}

type NodeConfig struct {
	RPC string `mapstructure:"rpc"`
}

type InfluxDBConfig struct {
	URL      string `mapstructure:"url"`
	Token    string `mapstructure:"token"`
	Org      string `mapstructure:"org"`
	Bucket   string `mapstructure:"bucket"`
	User     string `mapstructure:"user"`
	password string `mapstructure:"password"`
}

type ContractConfig struct {
	Address string `mapstructure:"address"`
	path    string `mapstructure:"path"`
}

type LoggingConfig struct {
	Level string `mapstructure:"level"`
}
