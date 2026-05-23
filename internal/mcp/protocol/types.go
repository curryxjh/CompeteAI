package protocol

// ServerConfig MCP Server 连接配置
type ServerConfig struct {
	Name      string `yaml:"name"`      // 服务名称
	Transport string `yaml:"transport"` // 传输方式: sse / stdio
	URL       string `yaml:"url"`       // SSE 传输时的 URL
	Command   string `yaml:"command"`   // stdio 传输时的命令
	Args      []string `yaml:"args"`    // stdio 传输时的参数
}

// ClientConfig MCP Client 全局配置
type ClientConfig struct {
	Enabled        bool           `yaml:"enabled"`
	Servers        []ServerConfig `yaml:"servers"`
	ConnectTimeout int            `yaml:"connect_timeout"` // 连接超时（秒）
	RequestTimeout int            `yaml:"request_timeout"` // 请求超时（秒）
	MaxRetries     int            `yaml:"max_retries"`     // 最大重试次数
}
