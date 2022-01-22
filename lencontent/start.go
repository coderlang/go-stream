package lencontent

import (
  "fmt"
  "github.com/xpwu/go-log/log"
  conn2 "github.com/xpwu/go-stream/conn"
  "github.com/xpwu/go-stream/fakehttp"
  "github.com/xpwu/go-stream/proxy"
  "github.com/xpwu/go-xnet/xtcp"
  "io"
  "time"
)

func Start() {
  for _, s := range configValue.Servers {
    if !s.Net.Listen.On() {
      continue
    }

    go runServer(s)
  }
}

func runServer(s *server) {
  logger := log.NewLogger()
  defer func() {
    if r := recover(); r != nil {
      logger.Fatal(r)
      logger.Error(fmt.Sprintf("%s server down! will restart after 5 seconds.", s.Net.Listen.LogString()))
      time.Sleep(5*time.Second)
      go runServer(s)
      logger.Info("server restart!")
    }
  }()

  s.checkValue(logger)

  var handler xtcp.HandlerFun = func(conn *xtcp.Conn) {

    ctx, logger := log.WithCtx(conn.Context())

    freqConn := newConn(conn, s)
    defer freqConn.CloseWith(nil)

    logger.PushPrefix(fmt.Sprintf("lencontent(conn_id=%s)", freqConn.Id()))
    logger.Debug("start processing new connection")

    if err := handshake(conn, s, freqConn.Id()); err != nil {
      logger.Error(err)
      return
    }

    for {
      logger.Debug("will read request")

      request := newRequest(conn, s)
      err := request.read()
      if err == io.EOF {
        logger.Error("connection closed by peer ")
        return
      }
      if err != nil {
        logger.Error("read message error. ", err, ", will close connection")
        return
      }

      if request.isHeartbeat() {
        logger.Debug("receive heartbeat")
        continue
      }

      conn2.TryConcurrent(ctx, freqConn.concurrent)
      logger.Debug("new fake http request")

      fhttpReq, err := fakehttp.NewRequest(freqConn, request.data)
      if err != nil {
        logger.Error(err, ", will close connection")
        return
      }

      logger.Debug(fmt.Sprintf("read request(addr=%p)", fhttpReq))

      go func() {
        proxy.Handler(ctx, fhttpReq, s.ProxyVar)
        conn2.DoneConcurrent(ctx, freqConn.concurrent)
      }()
    }
  }

  tcpServer := &xtcp.Server{
    Net:     s.Net,
    Handler: handler,
    Name:    "LenContent",
  }

  tcpServer.ServeAndBlock()
}
