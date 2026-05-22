package settings

import (
	"fmt"

	"CompeteAI/internal/pkg/logger"
	"github.com/fsnotify/fsnotify"
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

func Init(filePath string) (err error) {
	viper.SetConfigFile(filePath)
	err = viper.ReadInConfig()
	if err != nil {
		fmt.Printf("viper.ReadInConfig failed, err:%v\n", err)
		return
	}

	if err := viper.Unmarshal(Conf); err != nil {
		fmt.Printf("viper.Unmarshal failed, err:%v\n", err)
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