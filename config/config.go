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
	From string `mapstructure:"from"`
}

type NodeConfig struct {
	RPC  string     `mapstructure:"rpc"`
	Sync SyncConfig `mapstructure:"sync"`
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
	Dir       string     `mapstrucuture:"dir"`
	Contracts []Contract `mapstructure:"contracts"`
}

type Contract struct {
	Name    string `mapstructure:"name"`
	Address string `mapstructure:"address"`
}
