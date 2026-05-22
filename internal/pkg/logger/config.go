package logger

// Config 统一的日志配置
type Config struct {
	// 基础配置
	Level      string `yaml:"level" json:"level"`             // 日志级别: debug, info, warn, error
	Mode       string `yaml:"mode" json:"mode"`               // 运行模式: dev, prod
	TimeFormat string `yaml:"time_format" json:"time_format"` // 时间格式

	// 文件输出配置
	FileOutput FileOutputConfig `yaml:"file_output" json:"file_output"`

	// 控制台输出配置
	ConsoleOutput ConsoleOutputConfig `yaml:"console_output" json:"console_output"`
}

// FileOutputConfig 文件输出配置
type FileOutputConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`         // 是否启用文件输出
	Filename   string `yaml:"filename" json:"filename"`       // 日志文件路径
	MaxSize    int    `yaml:"max_size" json:"max_size"`       // 单个文件最大大小(MB)
	MaxBackups int    `yaml:"max_backups" json:"max_backups"` // 最大备份文件数
	MaxAge     int    `yaml:"max_age" json:"max_age"`         // 最大保存天数
	Compress   bool   `yaml:"compress" json:"compress"`       // 是否压缩备份文件
}

// ConsoleOutputConfig 控制台输出配置
type ConsoleOutputConfig struct {
	Enabled    bool   `yaml:"enabled" json:"enabled"`         // 是否启用控制台输出
	Colorful   bool   `yaml:"colorful" json:"colorful"`       // 是否启用彩色输出
	TimeFormat string `yaml:"time_format" json:"time_format"` // 控制台时间格式
}
