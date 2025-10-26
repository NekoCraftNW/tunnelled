package net

import (
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/panjf2000/gnet/v2"
)

type Connection struct {
	Listener *Listener

	FirstPacketSent bool
	ConnectionID    string // Unique ID for this connection

	ClientConn  gnet.Conn
	BackendConn gnet.Conn

	// Reconnection logic
	IsConnected       bool
	PacketQueue       [][]byte
	QueueMutex        sync.RWMutex
	ReconnectAttempts int
	LastReconnectTime time.Time
	MaxReconnectDelay time.Duration
	MaxQueueSize      int
}

func NewConnection(listener *Listener, clientConn gnet.Conn) *Connection {
	return &Connection{
		Listener:          listener,
		ClientConn:        clientConn,
		ConnectionID:      generateConnectionID(),
		IsConnected:       false,
		PacketQueue:       make([][]byte, 0),
		MaxReconnectDelay: 30 * time.Second,
		MaxQueueSize:      1000,
	}
}

func generateConnectionID() string {
	bytes := make([]byte, 16)
	rand.Read(bytes)
	return hex.EncodeToString(bytes)
}

func (c *Connection) QueuePacket(data []byte) {
	c.QueueMutex.Lock()
	defer c.QueueMutex.Unlock()

	if len(c.PacketQueue) >= c.MaxQueueSize {
		c.PacketQueue = c.PacketQueue[1:]
	}

	dataCopy := make([]byte, len(data))
	copy(dataCopy, data)
	c.PacketQueue = append(c.PacketQueue, dataCopy)
}

func (c *Connection) SendConnectionID() []byte {
	// Create magic packet with connection ID
	// Format: "TUNNELLED_ID:" + ConnectionID + "\n"
	magicPacket := []byte("TUNNELLED_ID:" + c.ConnectionID + "\n")
	return magicPacket
}

func (c *Connection) IsConnectionIDPacket(data []byte) (bool, string) {
	packetStr := string(data)
	if len(packetStr) > 13 && packetStr[:13] == "TUNNELLED_ID:" {
		if endIdx := len(packetStr) - 1; packetStr[endIdx] == '\n' {
			connectionID := packetStr[13:endIdx]
			return true, connectionID
		}
	}
	return false, ""
}

func (c *Connection) FlushQueue() {
	c.QueueMutex.Lock()
	defer c.QueueMutex.Unlock()

	for _, packet := range c.PacketQueue {
		if c.BackendConn != nil {
			c.BackendConn.Write(packet)
		}
	}
	c.PacketQueue = c.PacketQueue[:0]
}

func (c *Connection) GetReconnectDelay() time.Duration {
	baseDelay := time.Second
	multiplier := 1 << uint(c.ReconnectAttempts)
	if multiplier > 30 {
		multiplier = 30
	}
	delay := baseDelay * time.Duration(multiplier)
	if delay > c.MaxReconnectDelay {
		delay = c.MaxReconnectDelay
	}
	return delay
}
