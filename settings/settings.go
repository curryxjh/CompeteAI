package settings

import (
	"fmt"
	"os"
	"strings"

	"CompeteAI/internal/pkg/logger"
	"github.com/fsnotify/fsnotify"
	"github.com/go-sql-driver/mysql"
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
	*KafkaConfig         `mapstructure:"kafka"`
	*WorkflowConfig      `mapstructure:"workflow"`
	*MemoryConfig        `mapstructure:"memory"`
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

type KafkaConfig struct {
	Enabled bool     `mapstructure:"enabled"`
	Brokers []string `mapstructure:"brokers"`
	GroupID string   `mapstructure:"group_id"`
}

type WorkflowConfig struct {
	MaxAgentRetries    int  `mapstructure:"max_agent_retries"`
	MaxRounds          int  `mapstructure:"max_rounds"`
	UseRedisBlackboard bool `mapstructure:"use_redis_blackboard"`
}

type MemoryConfig struct {
	Enabled      bool   `mapstructure:"enabled"`
	ExplicitFile string `mapstructure:"explicit_file"`
	ProjectID    string `mapstructure:"project_id"`
	WorkspaceID  string `mapstructure:"workspace_id"`

	Embedder    *EmbedderConfig         `mapstructure:"embedder"`
	Milvus      *MilvusConfig           `mapstructure:"milvus"`
	Retrieval   MemoryRetrievalConfig   `mapstructure:"retrieval"`
	WritePolicy MemoryWritePolicyConfig `mapstructure:"write_policy"`
	Governance  MemoryGovernanceConfig  `mapstructure:"governance"`
}

// MilvusConfig 向量数据库配置。
type MilvusConfig struct {
	// Enabled 总开关；false 时退回到 MySQL 纯关键词模式
	Enabled          bool   `mapstructure:"enabled"`
	// Address gRPC 地址，如 "localhost:19530"
	Address          string `mapstructure:"address"`
	// CollectionPrefix collection 名称前缀，实际名 = prefix + "_embeddings"
	CollectionPrefix string `mapstructure:"collection_prefix"`
	// Dim 向量维度，需与 Embedder.Dim 一致，默认 1024
	Dim              int    `mapstructure:"dim"`
	// IndexType 索引类型，默认 "HNSW"
	IndexType        string `mapstructure:"index_type"`
	// MetricType 距离类型，默认 "COSINE"
	MetricType       string `mapstructure:"metric_type"`
	// HNSWM HNSW 图连接数，默认 16
	HNSWM            int    `mapstructure:"hnsw_m"`
	// HNSWEfConstruct HNSW 构建时 ef 参数，默认 200
	HNSWEfConstruct  int    `mapstructure:"hnsw_ef_construct"`
}

// EmbedderConfig Embedding API 配置。
type EmbedderConfig struct {
	// Type: "volcano"（豆包/火山引擎）| "hash"（本地哈希，降级用）
	Type     string `mapstructure:"type"`
	APIKey   string `mapstructure:"api_key"`
	Endpoint string `mapstructure:"endpoint"`
	Model    string `mapstructure:"model"`
	// Dim 向量维度，volcano-large 为 1024，小模型为 256
	Dim int `mapstructure:"dim"`
}

type MemoryRetrievalConfig struct {
	TopKFacts    int  `mapstructure:"top_k_facts"`
	TopKEpisodes int  `mapstructure:"top_k_episodes"`
	TopKEvidence int  `mapstructure:"top_k_evidence"`
	Rerank       bool `mapstructure:"rerank"`
}

type MemoryWritePolicyConfig struct {
	MinConfidence          float64 `mapstructure:"min_confidence"`
	AllowPendingInference  bool    `mapstructure:"allow_pending_inference"`
	RequireSourceForFact   bool    `mapstructure:"require_source_for_fact"`
	EpisodeOnCompletionOnly bool   `mapstructure:"episode_on_completion_only"`
}

type MemoryGovernanceConfig struct {
	DecayAfterDays   int `mapstructure:"decay_after_days"`
	StaleAfterDays   int `mapstructure:"stale_after_days"`
	InvalidAfterDays int `mapstructure:"invalid_after_days"`
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
	cfg := mysql.Config{
		User:   c.User,
		Passwd: c.Password,
		Net:    "tcp",
		Addr:   fmt.Sprintf("%s:%d", c.Host, c.Port),
		DBName: c.DB,
		Params: map[string]string{
			"charset":   "utf8mb4",
			"parseTime": "True",
			"loc":       "Local",
		},
	}
	return cfg.FormatDSN()
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

// envInt 将环境变量字符串转为 int，解析失败返回 0。
func envInt(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
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

	// ── MySQL 地址覆盖 ──────────────────────────────────────────────
	// MYSQL_HOST / MYSQL_PORT / MYSQL_USER / MYSQL_PASSWORD / MYSQL_DB
	if Conf.MySQLConfig == nil {
		Conf.MySQLConfig = &MySQLConfig{}
	}
	if v := os.Getenv("MYSQL_HOST"); v != "" {
		Conf.MySQLConfig.Host = v
	}
	if v := os.Getenv("MYSQL_PORT"); v != "" {
		if p := envInt(v); p > 0 {
			Conf.MySQLConfig.Port = p
		}
	}
	if v := os.Getenv("MYSQL_USER"); v != "" {
		Conf.MySQLConfig.User = v
	}
	if v := os.Getenv("MYSQL_PASSWORD"); v != "" {
		Conf.MySQLConfig.Password = v
	}
	if v := os.Getenv("MYSQL_DB"); v != "" {
		Conf.MySQLConfig.DB = v
	}

	// ── Redis 地址覆盖 ──────────────────────────────────────────────
	// REDIS_HOST / REDIS_PORT / REDIS_PASSWORD
	if Conf.RedisConfig == nil {
		Conf.RedisConfig = &RedisConfig{}
	}
	if v := os.Getenv("REDIS_HOST"); v != "" {
		Conf.RedisConfig.Host = v
	}
	if v := os.Getenv("REDIS_PORT"); v != "" {
		if p := envInt(v); p > 0 {
			Conf.RedisConfig.Port = p
		}
	}
	if v := os.Getenv("REDIS_PASSWORD"); v != "" {
		Conf.RedisConfig.Password = v
	}

	// ── Embedding API key 覆盖 ──────────────────────────────────────
	if key := os.Getenv("VOLCANO_EMBED_API_KEY"); key != "" {
		if Conf.MemoryConfig == nil {
			Conf.MemoryConfig = &MemoryConfig{}
		}
		if Conf.MemoryConfig.Embedder == nil {
			Conf.MemoryConfig.Embedder = &EmbedderConfig{}
		}
		Conf.MemoryConfig.Embedder.APIKey = key
	}

	// ── Milvus 地址覆盖 ─────────────────────────────────────────────
	// MILVUS_ADDRESS=host:19530  或  MILVUS_HOST + MILVUS_PORT
	if Conf.MemoryConfig == nil {
		Conf.MemoryConfig = &MemoryConfig{}
	}
	if Conf.MemoryConfig.Milvus == nil {
		Conf.MemoryConfig.Milvus = &MilvusConfig{}
	}
	if addr := os.Getenv("MILVUS_ADDRESS"); addr != "" {
		Conf.MemoryConfig.Milvus.Address = addr
	} else if host := os.Getenv("MILVUS_HOST"); host != "" {
		port := "19530"
		if p := os.Getenv("MILVUS_PORT"); p != "" {
			port = p
		}
		Conf.MemoryConfig.Milvus.Address = host + ":" + port
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
