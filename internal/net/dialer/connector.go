package dialer

import (
	"fmt"
	"time"

	"github.com/panjf2000/gnet/v2"
)

type TrafficHandler interface {
	HandleTraffic(gnetConn gnet.Conn, data []byte) gnet.Action
	OnConnection(gnetConn gnet.Conn)
	OnDisconnection(gnetConn gnet.Conn, err error)
}

type clientEventHandler struct {
	*gnet.BuiltinEventEngine
}

var GlobalClient *gnet.Client

func FireUpClient() (*gnet.Client, error) {
	handler := &clientEventHandler{}

	client, err := gnet.NewClient(handler,
		gnet.WithMulticore(true),

		gnet.WithNumEventLoop(16),

		gnet.WithLockOSThread(true),

		gnet.WithTCPKeepAlive(time.Second*30),
		gnet.WithTCPNoDelay(gnet.TCPNoDelay),

		gnet.WithReusePort(true),

		gnet.WithLoadBalancing(gnet.LeastConnections),
	)
	if err != nil {
		return nil, err
	}

	if err := client.Start(); err != nil {
		return nil, err
	}

	GlobalClient = client
	return client, nil
}

func (eh *clientEventHandler) OnBoot(_ gnet.Engine) (action gnet.Action) {
	fmt.Println("Network > gnet global client started")
	return gnet.None
}

func (eh *clientEventHandler) OnOpen(c gnet.Conn) (out []byte, action gnet.Action) {
	handler, ok := c.Context().(TrafficHandler)
	if !ok || handler == nil {
		return nil, gnet.Close
	}

	handler.OnConnection(c)
	return nil, gnet.None
}

func (eh *clientEventHandler) OnClose(c gnet.Conn, err error) (action gnet.Action) {
	handler, ok := c.Context().(TrafficHandler)
	if !ok || handler == nil {
		return gnet.Close
	}

	handler.OnDisconnection(c, err)
	return gnet.None
}

func (eh *clientEventHandler) OnTraffic(c gnet.Conn) (action gnet.Action) {
	handler, ok := c.Context().(TrafficHandler)
	if !ok || handler == nil {
		return gnet.Close
	}

	gnetBuffer, _ := c.Next(-1)

	data := make([]byte, len(gnetBuffer))
	copy(data, gnetBuffer)

	handler.HandleTraffic(c, data)
	return gnet.None
}
