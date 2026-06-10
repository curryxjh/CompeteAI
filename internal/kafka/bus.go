package kafka

import "CompeteAI/internal/bus"

// 兼容层：新代码请直接使用 internal/bus。
type Bus = bus.Bus

var (
	NewMemoryBus      = bus.NewMemoryBus
	NewRedisStreamBus = bus.NewRedisStreamBus
	NewKafkaBus       = bus.NewKafkaBus
	InitBus           = bus.InitBus
	InitPublisher     = bus.InitPublisher
	InitConsumer      = bus.InitConsumer
)

const (
	TopicTaskCreate        = bus.TopicTaskCreate
	TopicCoordinatorInput  = bus.TopicAgentCoordinator
	TopicCollectorInput    = bus.TopicAgentCollector
	TopicAnalystInput      = bus.TopicAgentAnalyst
	TopicWriterInput       = bus.TopicAgentWriter
	TopicQAInput           = bus.TopicAgentQA
	TopicQAQuery           = bus.TopicQAQuery
	TopicAnalystResponse   = bus.TopicAnalystReply
	TopicUserClarify       = bus.TopicTaskClarify
)

func ProcessedKey(messageID string) string { return bus.DedupKey(messageID) }
