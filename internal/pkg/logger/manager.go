package logger

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
)

// Manager 日志管理器，支持多输出目标
type Manager struct {
	cores  []zapcore.Core
	logger *zap.Logger
}

// NewManager 创建新的日志管理器
func NewManager(config *Config) (*Manager, error) {
	m := &Manager{}

	// 解析日志级别
	var level zapcore.Level
	err := level.UnmarshalText([]byte(config.Level))
	if err != nil {
		return nil, err
	}

	// 创建编码器
	fileEncoder := m.createFileEncoder(config)
	consoleEncoder := m.createConsoleEncoder(config)

	// 添加文件输出（如果启用）
	if config.FileOutput.Enabled {
		fileSyncer := m.createFileSyncer(&config.FileOutput)
		fileCore := zapcore.NewCore(fileEncoder, fileSyncer, level)
		m.cores = append(m.cores, fileCore)
	}

	// 添加控制台输出（如果启用）
	if config.ConsoleOutput.Enabled {
		consoleSyncer := zapcore.Lock(zapcore.AddSync(zapcore.Lock(os.Stdout)))
		consoleCore := zapcore.NewCore(consoleEncoder, consoleSyncer, level)
		m.cores = append(m.cores, consoleCore)
	}

	// 如果没有启用任何输出，使用控制台输出作为默认
	if len(m.cores) == 0 {
		consoleSyncer := zapcore.Lock(zapcore.AddSync(zapcore.Lock(os.Stdout)))
		consoleCore := zapcore.NewCore(consoleEncoder, consoleSyncer, level)
		m.cores = append(m.cores, consoleCore)
	}

	// 创建Tee核心
	core := zapcore.NewTee(m.cores...)
	m.logger = zap.New(core, zap.AddCaller())

	return m, nil
}

// GetLogger 获取底层的zap.Logger
func (m *Manager) GetLogger() *zap.Logger {
	return m.logger
}

// createFileEncoder 创建文件输出编码器
func (m *Manager) createFileEncoder(config *Config) zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.TimeKey = "time"
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	encoderConfig.EncodeDuration = zapcore.SecondsDurationEncoder
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}

// createConsoleEncoder 创建控制台输出编码器
func (m *Manager) createConsoleEncoder(config *Config) zapcore.Encoder {
	if config.Mode == "dev" {
		// 开发模式使用更友好的控制台格式
		return zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig())
	}

	// 生产模式也使用JSON格式，但可以配置颜色
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	if config.ConsoleOutput.Colorful {
		encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	return zapcore.NewConsoleEncoder(encoderConfig)
}

// createFileSyncer 创建文件同步器
func (m *Manager) createFileSyncer(config *FileOutputConfig) zapcore.WriteSyncer {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   config.Filename,
		MaxSize:    config.MaxSize,
		MaxBackups: config.MaxBackups,
		MaxAge:     config.MaxAge,
		Compress:   config.Compress,
	}
	return zapcore.AddSync(lumberJackLogger)
}
