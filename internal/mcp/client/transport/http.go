package transport

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"CompeteAI/internal/mcp/protocol"
)

// HTTPTransport 通过 HTTP POST 与 MCP Server 通信（适用于 Firecrawl 托管 MCP）
type HTTPTransport struct {
	url        string
	httpClient *http.Client
	connected  bool
}

func NewHTTPTransport(url string, timeout time.Duration) *HTTPTransport {
	return &HTTPTransport{
		url: url,
		httpClient: &http.Client{
			Timeout: timeout,
		},
	}
}

func (t *HTTPTransport) Connect() error {
	t.connected = true
	return nil
}

func (t *HTTPTransport) Send(req *protocol.Request) (*protocol.Response, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	httpReq, err := http.NewRequest(http.MethodPost, t.url, bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("create HTTP request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Accept", "application/json")

	httpResp, err := t.httpClient.Do(httpReq)
	if err != nil {
		return nil, fmt.Errorf("send request: %w", err)
	}
	defer httpResp.Body.Close()

	if httpResp.StatusCode < 200 || httpResp.StatusCode >= 300 {
		return nil, fmt.Errorf("HTTP %d from MCP server", httpResp.StatusCode)
	}

	var resp protocol.Response
	if err := json.NewDecoder(httpResp.Body).Decode(&resp); err != nil {
		return nil, fmt.Errorf("decode response: %w", err)
	}
	return &resp, nil
}

func (t *HTTPTransport) SendNotification(notif *protocol.Notification) error {
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

func (t *HTTPTransport) Close() error {
	t.connected = false
	return nil
}

func (t *HTTPTransport) IsConnected() bool {
	return t.connected
}
