package transport

import (
	"bufio"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"

	"CompeteAI/internal/mcp/protocol"
)

// SSETransport 通过 HTTP SSE 与远程 MCP Server 通信
type SSETransport struct {
	url        string
	httpClient *http.Client
	eventCh    chan *protocol.Response
	closeCh    chan struct{}
	connected  bool
}

func NewSSETransport(url string, timeout time.Duration) *SSETransport {
	return &SSETransport{
		url: url,
		httpClient: &http.Client{
			Timeout: timeout,
		},
		eventCh: make(chan *protocol.Response, 100),
		closeCh: make(chan struct{}),
	}
}

func (t *SSETransport) Connect() error {
	req, err := http.NewRequest("GET", t.url, nil)
	if err != nil {
		return fmt.Errorf("create SSE request: %w", err)
	}
	req.Header.Set("Accept", "text/event-stream")
	req.Header.Set("Cache-Control", "no-cache")

	resp, err := t.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("connect SSE: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("SSE connection failed: status %d", resp.StatusCode)
	}

	t.connected = true

	go func() {
		defer resp.Body.Close()
		scanner := bufio.NewScanner(resp.Body)
		var dataBuf strings.Builder

		for scanner.Scan() {
			select {
			case <-t.closeCh:
				return
			default:
			}

			line := scanner.Text()
			switch {
			case strings.HasPrefix(line, "data: "):
				dataBuf.WriteString(strings.TrimPrefix(line, "data: "))
			case line == "" && dataBuf.Len() > 0:
				var r protocol.Response
				if err := json.Unmarshal([]byte(dataBuf.String()), &r); err == nil {
					t.eventCh <- &r
				}
				dataBuf.Reset()
			}
		}
	}()

	return nil
}

func (t *SSETransport) Send(req *protocol.Request) (*protocol.Response, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest("POST", t.url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")

	httpResp, err := t.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()

	var r protocol.Response
	if err := json.NewDecoder(httpResp.Body).Decode(&r); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &r, nil
}

func (t *SSETransport) SendNotification(notif *protocol.Notification) error {
	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	resp, err := t.httpClient.Post(t.url, "application/json", bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("send notification: %w", err)
	}
	resp.Body.Close()
	return nil
}

func (t *SSETransport) Close() error {
	close(t.closeCh)
	t.connected = false
	return nil
}

func (t *SSETransport) IsConnected() bool {
	return t.connected
}

// Events 返回 SSE 事件通道（用于接收服务端推送）
func (t *SSETransport) Events() <-chan *protocol.Response {
	return t.eventCh
}
