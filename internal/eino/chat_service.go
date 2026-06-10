package eino

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"CompeteAI/internal/llm"

	"github.com/cloudwego/eino/components/model"
	"github.com/cloudwego/eino/schema"
)

const maxToolRoundTrips = 5

type ChatService struct {
	chatModel model.ToolCallingChatModel
	modelName string
	tools     *ToolRegistry
}

func NewChatService(chatModel model.ToolCallingChatModel, modelName string, tools *ToolRegistry) *ChatService {
	return &ChatService{
		chatModel: chatModel,
		modelName: modelName,
		tools:     tools,
	}
}

func (s *ChatService) Model() string {
	return s.modelName
}

func (s *ChatService) ToolsEnabled() bool {
	return s.tools != nil && s.tools.Enabled()
}

func (s *ChatService) Chat(ctx context.Context, messages []llm.Message) (string, error) {
	var content string
	err := s.StreamChat(ctx, messages, func(ev StreamEvent) error {
		if ev.Type == StreamEventContent {
			content += ev.Content
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if content == "" {
		return "", errors.New("empty response from model")
	}
	return content, nil
}

func (s *ChatService) StreamChat(ctx context.Context, messages []llm.Message, onEvent func(StreamEvent) error) error {
	if s.ToolsEnabled() {
		return s.generateWithEvents(ctx, ToSchemaMessages(messages), onEvent)
	}

	reader, err := s.chatModel.Stream(ctx, ToSchemaMessages(messages))
	if err != nil {
		return err
	}
	defer reader.Close()

	for {
		chunk, err := reader.Recv()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if chunk == nil {
			continue
		}
		if chunk.ReasoningContent != "" {
			if err := onEvent(StreamEvent{
				Type:    StreamEventThinking,
				Content: chunk.ReasoningContent,
			}); err != nil {
				return err
			}
		}
		if chunk.Content != "" {
			if err := onEvent(StreamEvent{
				Type:    StreamEventContent,
				Content: chunk.Content,
			}); err != nil {
				return err
			}
		}
	}
}

func (s *ChatService) generateWithEvents(
	ctx context.Context,
	messages []*schema.Message,
	onEvent func(StreamEvent) error,
) error {
	chatModel := s.chatModel
	if s.ToolsEnabled() {
		toolModel, err := s.bindTools(ctx)
		if err != nil {
			return err
		}
		chatModel = toolModel
	}

	current := append([]*schema.Message(nil), messages...)
	for round := 0; round < maxToolRoundTrips; round++ {
		resp, err := chatModel.Generate(ctx, current)
		if err != nil {
			return err
		}
		if resp == nil {
			return errors.New("empty response from model")
		}

		if resp.ReasoningContent != "" {
			if err := onEvent(StreamEvent{
				Type:    StreamEventThinking,
				Content: resp.ReasoningContent,
			}); err != nil {
				return err
			}
		}

		if len(resp.ToolCalls) == 0 || !s.ToolsEnabled() {
			if resp.Content != "" {
				return onEvent(StreamEvent{
					Type:    StreamEventContent,
					Content: resp.Content,
				})
			}
			return nil
		}

		if resp.Content != "" {
			if err := onEvent(StreamEvent{
				Type:    StreamEventContent,
				Content: resp.Content,
			}); err != nil {
				return err
			}
		}

		current = append(current, resp)
		for _, call := range resp.ToolCalls {
			if err := onEvent(StreamEvent{
				Type:     StreamEventToolCall,
				ToolName: call.Function.Name,
				ToolArgs: FormatToolArgsForUI(call.Function.Arguments),
				Status:   "running",
			}); err != nil {
				return err
			}

			toolMsg, toolResult := s.callTool(ctx, call)
			if err := onEvent(StreamEvent{
				Type:       StreamEventToolResult,
				ToolName:   call.Function.Name,
				ToolArgs:   FormatToolArgsForUI(call.Function.Arguments),
				ToolResult: FormatToolResultForUI(call.Function.Name, toolResult),
				Status:     "done",
			}); err != nil {
				return err
			}
			current = append(current, toolMsg)
		}
	}
	return fmt.Errorf("exceeded max tool round trips (%d)", maxToolRoundTrips)
}

func (s *ChatService) bindTools(ctx context.Context) (model.ToolCallingChatModel, error) {
	infos, err := s.tools.ListToolInfos(ctx)
	if err != nil {
		return nil, fmt.Errorf("list tool infos: %w", err)
	}
	if len(infos) == 0 {
		return s.chatModel, nil
	}
	return s.chatModel.WithTools(infos)
}

func (s *ChatService) callTool(ctx context.Context, call schema.ToolCall) (*schema.Message, string) {
	args := map[string]interface{}{}
	if strings.TrimSpace(call.Function.Arguments) != "" {
		if err := json.Unmarshal([]byte(call.Function.Arguments), &args); err != nil {
			text := fmt.Sprintf("parse tool arguments: %v", err)
			return &schema.Message{
				Role:       schema.Tool,
				Content:    text,
				ToolCallID: call.ID,
			}, text
		}
	}

	text, err := s.tools.Invoke(ctx, call.Function.Name, args)
	if err != nil {
		text = err.Error()
	}

	return &schema.Message{
		Role:       schema.Tool,
		Content:    text,
		ToolCallID: call.ID,
	}, text
}
