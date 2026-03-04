package mpv

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"
)

type IPCClient struct {
	conn net.Conn
	r    *bufio.Reader
}

func DialIPC(ctx context.Context, socketPath string) (*IPCClient, error) {
	d := net.Dialer{}
	conn, err := d.DialContext(ctx, "unix", socketPath)
	if err != nil {
		return nil, err
	}
	return &IPCClient{conn: conn, r: bufio.NewReader(conn)}, nil
}

func (c *IPCClient) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	return c.conn.Close()
}

// Command envia um comando IPC do mpv no formato array JSON.
func (c *IPCClient) Command(ctx context.Context, args ...any) error {
	if c == nil || c.conn == nil {
		return errors.New("mpv: IPC não conectado")
	}

	payload := map[string]any{
		"command": args,
	}
	b, err := json.Marshal(payload)
	if err != nil {
		return err
	}
	b = append(b, '\n')

	_ = c.conn.SetWriteDeadline(time.Now().Add(2 * time.Second))
	if _, err := c.conn.Write(b); err != nil {
		return err
	}

	// Opcionalmente, poderíamos ler a resposta e validar "error":"success".
	// Para o MVP, apenas tenta consumir uma linha quando possível.
	_ = c.conn.SetReadDeadline(time.Now().Add(500 * time.Millisecond))
	line, err := c.r.ReadBytes('\n')
	if err != nil {
		// Timeout é aceitável para comandos que não respondem imediatamente.
		var ne net.Error
		if errors.As(err, &ne) && ne.Timeout() {
			return nil
		}
		return nil
	}

	var resp struct {
		Error string `json:"error"`
	}
	_ = json.Unmarshal(line, &resp)
	if resp.Error != "" && resp.Error != "success" {
		return fmt.Errorf("mpv: erro IPC: %s", resp.Error)
	}
	return nil
}

