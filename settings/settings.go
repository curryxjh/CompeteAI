package settings

import (
	"fmt"
	"os"
	"strings"

	"CompeteAI/internal/pkg/logger"
	"github.com/fsnotify/fsnotify"
	"github.com/joho/godotenv"
	"github.com/spf13/viper"
	"go.uber.org/zap"
)

var Conf = new(AppConfig)

type AppConfig struct {
	Name      string `mapstructure:"name"`
	Mode      string `mapstructure:"mode"`
	Version   string `mapstructure:"version"`
	StartTime string `mapstructure:"start_time"`
	MachineID int64  `mapstructure:"machine_id"`
	Port      int    `mapstructure:"port"`

	*LogConfig   `mapstructure:"log"`
	*MySQLConfig `mapstructure:"mysql"`
	*RedisConfig `mapstructure:"redis"`
	*LLMConfig          `mapstructure:"llm"`
	*FirecrawlMCPConfig  `mapstructure:"firecrawl_mcp"`
}

type LLMConfig struct {
	BaseURL string `mapstructure:"base_url"`
	APIKey  string `mapstructure:"api_key"`
	Model   string `mapstructure:"model"`
}

type FirecrawlMCPConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Command string   `mapstructure:"command"`
	Args    []string `mapstructure:"args"`
}

type MySQLConfig struct {
	Host         string `mapstructure:"host"`
	User         string `mapstructure:"user"`
	Password     string `mapstructure:"password"`
	DB           string `mapstructure:"dbname"`
	Port         int    `mapstructure:"port"`
	MaxOpenConns int    `mapstructure:"max_open_conns"`
	MaxIdleConns int    `mapstructure:"max_idle_conns"`
}

func (c *MySQLConfig) DSN() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		c.User, c.Password, c.Host, c.Port, c.DB)
}

type RedisConfig struct {
	Host         string `mapstructure:"host"`
	Password     string `mapstructure:"password"`
	Port         int    `mapstructure:"port"`
	DB           int    `mapstructure:"db"`
	PoolSize     int    `mapstructure:"pool_size"`
	MinIdleConns int    `mapstructure:"min_idle_conns"`
}

func (c *RedisConfig) Addr() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

type LogConfig struct {
	Level      string `mapstructure:"level"`
	Filename   string `mapstructure:"filename"`
	MaxSize    int    `mapstructure:"max_size"`
	MaxAge     int    `mapstructure:"max_age"`
	MaxBackups int    `mapstructure:"max_backups"`
	
	// 新增完整logger配置
	Mode       string `mapstructure:"mode"`
	TimeFormat string `mapstructure:"time_format"`
	
	// 文件输出配置
	FileOutputEnabled    bool   `mapstructure:"file_output_enabled"`
	FileOutputCompress   bool   `mapstructure:"file_output_compress"`
	
	// 控制台输出配置
	ConsoleOutputEnabled bool   `mapstructure:"console_output_enabled"`
	ConsoleOutputColorful bool  `mapstructure:"console_output_colorful"`
	ConsoleOutputTimeFormat string `mapstructure:"console_output_time_format"`
}

// InitLogger 初始化日志系统
func InitLogger() error {
	config := &logger.Config{
		Level:      Conf.LogConfig.Level,
		Mode:       Conf.LogConfig.Mode,
		TimeFormat: Conf.LogConfig.TimeFormat,
		FileOutput: logger.FileOutputConfig{
			Enabled:    Conf.LogConfig.FileOutputEnabled,
			Filename:   Conf.LogConfig.Filename,
			MaxSize:    Conf.LogConfig.MaxSize,
			MaxBackups: Conf.LogConfig.MaxBackups,
			MaxAge:     Conf.LogConfig.MaxAge,
			Compress:   Conf.LogConfig.FileOutputCompress,
		},
		ConsoleOutput: logger.ConsoleOutputConfig{
			Enabled:    Conf.LogConfig.ConsoleOutputEnabled,
			Colorful:   Conf.LogConfig.ConsoleOutputColorful,
			TimeFormat: Conf.LogConfig.ConsoleOutputTimeFormat,
		},
	}

	// 创建日志管理器
	manager, err := logger.NewManager(config)
	if err != nil {
		return err
	}

	// 设置全局日志实例
	zapLogger := manager.GetLogger()
	logger.SetGlobalLogger(logger.NewZapLogger(zapLogger))

	// 替换zap全局实例
	zap.ReplaceGlobals(zapLogger)

	// 记录初始化成功日志
	logger.L().Info("logger initialized successfully", 
		logger.String("level", config.Level),
		logger.String("mode", config.Mode),
		logger.Bool("file_output", config.FileOutput.Enabled),
		logger.Bool("console_output", config.ConsoleOutput.Enabled),
	)

	return nil
}

func loadEnvFile() {
	if err := godotenv.Load(".env"); err != nil && !os.IsNotExist(err) {
		fmt.Printf("load .env failed, err:%v\n", err)
	}
}

func Init(filePath string) (err error) {
	loadEnvFile()

	viper.SetConfigFile(filePath)
	err = viper.ReadInConfig()
	if err != nil {
		fmt.Printf("viper.ReadInConfig failed, err:%v\n", err)
		return
	}

	if err := viper.Unmarshal(Conf); err != nil {
		fmt.Printf("viper.Unmarshal failed, err:%v\n", err)
	}

	localPath := strings.TrimSuffix(filePath, ".yaml") + ".local.yaml"
	if _, statErr := os.Stat(localPath); statErr == nil {
		localViper := viper.New()
		localViper.SetConfigFile(localPath)
		if readErr := localViper.ReadInConfig(); readErr != nil {
			fmt.Printf("read local config failed, err:%v\n", readErr)
		} else {
			var overlay struct {
				LLM *LLMConfig `mapstructure:"llm"`
			}
			if unmarshalErr := localViper.Unmarshal(&overlay); unmarshalErr != nil {
				fmt.Printf("unmarshal local config failed, err:%v\n", unmarshalErr)
			} else if overlay.LLM != nil {
				if Conf.LLMConfig == nil {
					Conf.LLMConfig = &LLMConfig{}
				}
				if overlay.LLM.BaseURL != "" {
					Conf.LLMConfig.BaseURL = overlay.LLM.BaseURL
				}
				if overlay.LLM.Model != "" {
					Conf.LLMConfig.Model = overlay.LLM.Model
				}
				if overlay.LLM.APIKey != "" {
					Conf.LLMConfig.APIKey = overlay.LLM.APIKey
				}
			}
		}
	}

	if key := os.Getenv("ARK_API_KEY"); key != "" && Conf.LLMConfig != nil {
		Conf.LLMConfig.APIKey = key
	}

	viper.WatchConfig()
	viper.OnConfigChange(func(in fsnotify.Event) {
		fmt.Printf("config file changed:%v\n", in.Name)
		if err := viper.Unmarshal(Conf); err != nil {
			fmt.Printf("viper.Unmarshal failed, err:%v\n", err)
			return
		}
	})
	return
}
