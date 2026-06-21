package app

import (
	"CompeteAI/internal/bus"
	"CompeteAI/internal/event"
	"CompeteAI/internal/memory"
	"CompeteAI/internal/orchestrator"
	"CompeteAI/internal/outbox"
	"CompeteAI/internal/repository"
	"CompeteAI/internal/repository/dao"
	"CompeteAI/internal/service"
	"CompeteAI/internal/workflow"
	"CompeteAI/ioc"
	"CompeteAI/settings"
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/redis/go-redis/v9"
)

// SharedDeps 三类进程共享的基础设施。
type SharedDeps struct {
	Redis     redis.Cmdable
	Bus       bus.Bus
	Engine    *workflow.Engine
	Runtime   *orchestrator.Runtime
	Events    *event.Publisher
	Tasks     repository.TaskRepository
	Store     *orchestrator.Store
	Outbox    *outbox.Publisher
	MemorySvc memory.MemoryService
}

// BuildSharedDeps 组装 Engine + Bus + 持久化组件。
func BuildSharedDeps(consumerID string) (*SharedDeps, error) {
	redisClient := ioc.InitRedis()
	db := ioc.InitDB()

	taskDao := dao.NewTaskDao(db)
	reportDao := dao.NewReportDao(db)
	traceDao := dao.NewTraceDao(db)
	eventDao := dao.NewEventLogDao(db)
	msgDao := dao.NewMessageLogDao(db)
	outboxDao := dao.NewOutboxDao(db)
	cpDao := dao.NewCheckpointDao(db)
	dlqDao := dao.NewDeadLetterDao(db)

	tasks := repository.NewTaskRepository(taskDao)
	reports := repository.NewReportRepository(reportDao)
	traces := repository.NewTraceRepository(traceDao)

	toolRegistry := ioc.InitFirecrawlTools()
	chatService := ioc.InitChatService(toolRegistry)
	registry := service.NewAgentRegistry(chatService, toolRegistry)
	hub := event.NewHybridHub(dao.NewEventLogDao(db))

	var messageBus bus.Bus
	if consumerID != "" {
		messageBus = bus.InitConsumer(redisClient, consumerID)
	} else {
		messageBus = bus.InitPublisher(redisClient)
	}
	router := bus.NewRouter(messageBus)

	useRedis := true
	maxRetry, maxRounds := 3, 0
	if cfg := settings.Conf.WorkflowConfig; cfg != nil {
		useRedis = cfg.UseRedisBlackboard
		if cfg.MaxAgentRetries > 0 {
			maxRetry = cfg.MaxAgentRetries
		}
		maxRounds = cfg.MaxRounds
	}
	memSvc := memory.NewService(dao.NewMemoryDao(db), settings.Conf.MemoryConfig)
	engine := workflow.NewEngine(registry, tasks, reports, traces, hub, messageBus, router, redisClient, useRedis, maxRetry, maxRounds, memSvc)


	eventsPub := event.NewPublisher(eventDao)
	store := orchestrator.NewStore(cpDao, redisClient)
	dlqHandler := bus.NewDLQHandler(dlqDao, messageBus)
	outboxPub := outbox.NewPublisher(outboxDao, messageBus)
	runtime := orchestrator.NewRuntime(orchestrator.Config{
		Engine: engine, Bus: messageBus, Store: store,
		Events: eventsPub, MsgLog: msgDao, DLQ: dlqHandler, Tasks: tasks,
		WorkerID: WorkerID(), Role: WorkerRole(), UseRedis: useRedis,
	})

	return &SharedDeps{
		Redis: redisClient, Bus: messageBus, Engine: engine,
		Runtime: runtime, Events: eventsPub, Tasks: tasks, Store: store,
		Outbox: outboxPub, MemorySvc: memSvc,
	}, nil
}

// RunWorker 按 WORKER_ROLE 启动消费者。configFile 为空时使用默认路径。
func RunWorker(configFile ...string) error {
	cf := ""
	if len(configFile) > 0 {
		cf = configFile[0]
	}
	if err := LoadConfigFile(cf); err != nil {
		return err
	}
	deps, err := BuildSharedDeps(WorkerID())
	if err != nil {
		return err
	}
	deps.Runtime.RegisterHandlers()
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	defer func() {
		// 进程退出时关闭 Milvus 连接和后台 goroutine
		if deps.MemorySvc != nil {
			_ = deps.MemorySvc.Close()
		}
	}()
	go deps.Outbox.Run(ctx)

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sig
		cancel()
	}()

	log.Printf("[worker] role=%s id=%s consuming", WorkerRole(), WorkerID())
	return deps.Bus.Run(ctx)
}

// RunAll 一个进程同时跑 API + Worker（开发用，省去启两个终端）。
// 用法: go run . all  或  go run . all config/dev.yaml
func RunAll(configFile string) error {
	if err := LoadConfigFile(configFile); err != nil {
		return err
	}

	errCh := make(chan error, 2)

	// ── API goroutine ────────────────────────────────────────────
	go func() {
		server := InitWebServer()
		addr := fmt.Sprintf(":%d", settings.Conf.Port)
		log.Printf("[all-in-one] API listening %s", addr)
		errCh <- server.Run(addr)
	}()

	// ── Worker goroutine ─────────────────────────────────────────
	go func() {
		deps, err := BuildSharedDeps(WorkerID())
		if err != nil {
			errCh <- fmt.Errorf("worker init: %w", err)
			return
		}
		deps.Runtime.RegisterHandlers()
		ctx, cancel := context.WithCancel(context.Background())
		defer cancel()
		defer func() {
			if deps.MemorySvc != nil {
				_ = deps.MemorySvc.Close()
			}
		}()
		go deps.Outbox.Run(ctx)

		sig := make(chan os.Signal, 1)
		signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
		go func() { <-sig; cancel() }()

		log.Printf("[all-in-one] Worker role=%s id=%s consuming", WorkerRole(), WorkerID())
		errCh <- deps.Bus.Run(ctx)
	}()

	return <-errCh
}

// RunRecovery 定时扫描 stale 任务并 reclaim pending 消息。
func RunRecovery(configFile ...string) error {
	cf := ""
	if len(configFile) > 0 {
		cf = configFile[0]
	}
	if err := LoadConfigFile(cf); err != nil {
		return err
	}
	deps, err := BuildSharedDeps("recovery-" + WorkerID())
	if err != nil {
		return err
	}
	rec := orchestrator.NewRecovery(deps.Tasks, deps.Bus, deps.Store, deps.Engine)
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	log.Printf("[recovery] started")
	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	for {
		select {
		case <-sig:
			return nil
		case <-ticker.C:
			rec.RunOnce(context.Background())
		}
	}
}
