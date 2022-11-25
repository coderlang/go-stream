package websocketc

import (
  "context"
  "github.com/gorilla/websocket"
  "github.com/xpwu/go-log/log"
  conn2 "github.com/xpwu/go-stream/conn"
  "github.com/xpwu/go-xnet/connid"
  "net"
  "sync"
  "time"
)

type conn struct {
  conn2.Base
  c       *websocket.Conn
  mu      chan struct{}
  ctx     context.Context
  cancelF context.CancelFunc
  closed  bool
  once    sync.Once
  id      connid.Id
}

func newConn(ctx context.Context, c *websocket.Conn) *conn {

  ret := &conn{
    c:  c,
    mu: make(chan struct{}, 1),
    id: connid.New(),
  }
  ret.ctx, ret.cancelF = context.WithCancel(ctx)

  return ret
}

func (c *conn) GetVar(name string) string {
  return ""
}

func (c *conn) Id() connid.Id {
  return c.id
}

func (c *conn) Write(buffers net.Buffers) error {
  // 只能一个goroutines 访问
  c.mu <- struct{}{}
  defer func() {
    <-c.mu
  }()

  err := c.c.SetWriteDeadline(time.Now().Add(5 * time.Second))
  if err != nil {
    return err
  }

  writer, err := c.c.NextWriter(websocket.BinaryMessage)
  if err != nil {
    return err
  }

  for _, d := range buffers {
    if _, err = writer.Write(d); err != nil {
      return err
    }
  }

  return writer.Close()
}

func (c *conn) CloseWith(err error) {
  c.once.Do(func() {
    _, logger := log.WithCtx(c.ctx)
    c.Base.Close()
    c.cancelF()
    if err != nil {
      logger.Error(err)
    }
    logger.Info("close connection")
    _ = c.c.Close()
  })
}

func (c *conn) context() context.Context {
  return c.ctx
}
