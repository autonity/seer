package config

// Config holds the application configuration
type Config struct {
	Seer     SeerConfig     `mapstructure:"seer"`
	Node     NodeConfig     `mapstructure:"node"`
	InfluxDB InfluxDBConfig `mapstructure:"db"`
	ABIs     ABIConfig      `mapstructure:"abi"`
}

type SeerConfig struct {
	LogLevel string `mapstructure:"logLevel"`
}

type SyncConfig struct {
	History bool `mapstructure:"history"`
}

type NodeConfig struct {
	RPC  Conn       `mapstructure:"rpc"`
	WS   Conn       `mapstructure:"ws"`
	Sync SyncConfig `mapstructure:"sync"`
}

type Conn struct {
	MaxConnections int      `mapstructure:"maxConnections"`
	URLs           []string `mapstructure:"urls"`
}

type InfluxDBConfig struct {
	URL      string `mapstructure:"url"`
	Token    string `mapstructure:"token"`
	Org      string `mapstructure:"org"`
	Bucket   string `mapstructure:"bucket"`
	User     string `mapstructure:"user"`
	password string `mapstructure:"password"`
}

type ABIConfig struct {
	Dir string `mapstrucuture:"dir"`
}
