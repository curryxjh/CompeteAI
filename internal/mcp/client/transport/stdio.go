package transport

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"

	"CompeteAI/internal/mcp/protocol"
)

// StdioTransport 通过子进程的 stdin/stdout 通信
type StdioTransport struct {
	cmd    *exec.Cmd
	stdin  io.WriteCloser
	stdout *bufio.Scanner
	env    map[string]string
}

func NewStdioTransport(command string, args []string, env map[string]string) *StdioTransport {
	return &StdioTransport{
		cmd: exec.Command(command, args...),
		env: env,
	}
}

func (t *StdioTransport) Connect() error {
	if len(t.env) > 0 {
		t.cmd.Env = append(os.Environ(), flattenEnv(t.env)...)
	}
	stdin, err := t.cmd.StdinPipe()
	if err != nil {
		return fmt.Errorf("get stdin pipe: %w", err)
	}
	stdout, err := t.cmd.StdoutPipe()
	if err != nil {
		return fmt.Errorf("get stdout pipe: %w", err)
	}

	if err := t.cmd.Start(); err != nil {
		return fmt.Errorf("start command: %w", err)
	}

	t.stdin = stdin
	t.stdout = bufio.NewScanner(stdout)
	return nil
}

func (t *StdioTransport) Send(req *protocol.Request) (*protocol.Response, error) {
	data, err := json.Marshal(req)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	if _, err := fmt.Fprintf(t.stdin, "%s\n", data); err != nil {
		return nil, fmt.Errorf("write request: %w", err)
	}

	if !t.stdout.Scan() {
		return nil, fmt.Errorf("read response: %w", t.stdout.Err())
	}

	var resp protocol.Response
	if err := json.Unmarshal(t.stdout.Bytes(), &resp); err != nil {
		return nil, fmt.Errorf("unmarshal response: %w", err)
	}
	return &resp, nil
}

func (t *StdioTransport) SendNotification(notif *protocol.Notification) error {
	data, err := json.Marshal(notif)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}
	_, err = fmt.Fprintf(t.stdin, "%s\n", data)
	return err
}

func (t *StdioTransport) Close() error {
	if t.stdin != nil {
		t.stdin.Close()
	}
	if t.cmd.Process != nil {
		return t.cmd.Process.Kill()
	}
	return nil
}

func (t *StdioTransport) IsConnected() bool {
	return t.cmd.Process != nil
}

func flattenEnv(env map[string]string) []string {
	out := make([]string, 0, len(env))
	for k, v := range env {
		out = append(out, k+"="+v)
	}
	return out
}
