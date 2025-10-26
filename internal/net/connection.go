package net

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"sync"
	"time"
	"tunnelled/internal/haproxy"

	"github.com/panjf2000/gnet/v2"
)

type Connection struct {
	Listener *Listener

	FirstPacketSent bool
	ConnectionID    string // Unique ID for this connection

	ClientConn  gnet.Conn
	BackendConn gnet.Conn

	// HAProxy protocol support
	ProxyInfo         *haproxy.ProxyInfo
	HAProxyProcessed  bool
	PendingData       []byte

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
		HAProxyProcessed:  false,
		PendingData:       make([]byte, 0),
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
	// Create magic packet with connection ID and proxy info
	// Format: "TUNNELLED_ID:" + ConnectionID + "|PROXY_INFO:" + encoded_proxy_info + "\n"
	magicPacket := "TUNNELLED_ID:" + c.ConnectionID
	
	// Always add proxy info (either extracted from HAProxy or inferred from connection)
	var proxyInfo *haproxy.ProxyInfo
	if c.ProxyInfo != nil {
		proxyInfo = c.ProxyInfo
	} else {
		// Infer proxy info from client connection
		proxyInfo = c.inferProxyInfo()
	}
	
	if proxyInfo != nil {
		proxyStr := fmt.Sprintf("%s:%d->%s:%d", 
			proxyInfo.SrcIP.String(), proxyInfo.SrcPort,
			proxyInfo.DstIP.String(), proxyInfo.DstPort)
		magicPacket += "|PROXY_INFO:" + proxyStr
	}
	
	magicPacket += "\n"
	return []byte(magicPacket)
}

// inferProxyInfo creates proxy info from the client connection when no HAProxy header was present
func (c *Connection) inferProxyInfo() *haproxy.ProxyInfo {
	if c.ClientConn == nil {
		return nil
	}
	
	remoteAddr := c.ClientConn.RemoteAddr()
	localAddr := c.ClientConn.LocalAddr()
	
	if remoteAddr == nil || localAddr == nil {
		return nil
	}
	
	// Parse remote address (source)
	remoteHost, remotePortStr, err1 := net.SplitHostPort(remoteAddr.String())
	localHost, localPortStr, err2 := net.SplitHostPort(localAddr.String())
	
	if err1 != nil || err2 != nil {
		return nil
	}
	
	srcIP := net.ParseIP(remoteHost)
	dstIP := net.ParseIP(localHost)
	if srcIP == nil || dstIP == nil {
		return nil
	}
	
	srcPort, err1 := strconv.ParseUint(remotePortStr, 10, 16)
	dstPort, err2 := strconv.ParseUint(localPortStr, 10, 16)
	if err1 != nil || err2 != nil {
		return nil
	}
	
	return &haproxy.ProxyInfo{
		SrcIP:   srcIP,
		DstIP:   dstIP,
		SrcPort: uint16(srcPort),
		DstPort: uint16(dstPort),
		Version: 1,
	}
}

func (c *Connection) IsConnectionIDPacket(data []byte) (bool, string) {
	packetStr := string(data)
	if len(packetStr) > 13 && packetStr[:13] == "TUNNELLED_ID:" {
		if endIdx := len(packetStr) - 1; packetStr[endIdx] == '\n' {
			content := packetStr[13:endIdx]
			return true, content
		}
	}
	return false, ""
}

func (c *Connection) ParseConnectionIDPacket(content string) (string, *haproxy.ProxyInfo) {
	parts := strings.Split(content, "|")
	connectionID := parts[0]
	
	var proxyInfo *haproxy.ProxyInfo
	for _, part := range parts[1:] {
		if strings.HasPrefix(part, "PROXY_INFO:") {
			proxyStr := part[11:] // Remove "PROXY_INFO:" prefix
			proxyInfo = c.parseProxyInfo(proxyStr)
		}
	}
	
	return connectionID, proxyInfo
}

func (c *Connection) parseProxyInfo(proxyStr string) *haproxy.ProxyInfo {
	// Format: "srcIP:srcPort->dstIP:dstPort"
	parts := strings.Split(proxyStr, "->")
	if len(parts) != 2 {
		return nil
	}
	
	srcParts := strings.Split(parts[0], ":")
	dstParts := strings.Split(parts[1], ":")
	
	if len(srcParts) != 2 || len(dstParts) != 2 {
		return nil
	}
	
	srcIP := net.ParseIP(srcParts[0])
	dstIP := net.ParseIP(dstParts[0])
	if srcIP == nil || dstIP == nil {
		return nil
	}
	
	srcPort, err1 := strconv.ParseUint(srcParts[1], 10, 16)
	dstPort, err2 := strconv.ParseUint(dstParts[1], 10, 16)
	if err1 != nil || err2 != nil {
		return nil
	}
	
	return &haproxy.ProxyInfo{
		SrcIP:   srcIP,
		DstIP:   dstIP,
		SrcPort: uint16(srcPort),
		DstPort: uint16(dstPort),
		Version: 1, // Default to v1
	}
}

// ProcessHAProxyData processes incoming data for HAProxy protocol headers
func (c *Connection) ProcessHAProxyData(data []byte) ([]byte, error) {
	if c.HAProxyProcessed {
		return data, nil
	}

	// Append to pending data
	c.PendingData = append(c.PendingData, data...)

	// Check if we have enough data to determine if it's HAProxy
	if len(c.PendingData) < 5 {
		return nil, nil // Need more data
	}

	// Check if this looks like HAProxy protocol
	isHAProxy, version := haproxy.IsHAProxyHeader(c.PendingData)
	if !isHAProxy {
		// Not HAProxy, mark as processed and return all pending data
		c.HAProxyProcessed = true
		result := make([]byte, len(c.PendingData))
		copy(result, c.PendingData)
		c.PendingData = nil
		return result, nil
	}

	// Parse HAProxy header
	var proxyInfo *haproxy.ProxyInfo
	var headerSize int
	var err error

	if version == 1 {
		proxyInfo, headerSize, err = haproxy.ParseV1(c.PendingData)
	} else if version == 2 {
		proxyInfo, headerSize, err = haproxy.ParseV2(c.PendingData)
	} else {
		return nil, fmt.Errorf("unsupported HAProxy version: %d", version)
	}

	if err != nil {
		// Check if we need more data
		if len(c.PendingData) < 512 { // Reasonable buffer size
			return nil, nil // Need more data
		}
		return nil, err
	}

	c.ProxyInfo = proxyInfo
	c.HAProxyProcessed = true

	// Return remaining data after HAProxy header
	remainingData := c.PendingData[headerSize:]
	c.PendingData = nil

	return remainingData, nil
}

// GenerateHAProxyHeader generates HAProxy header to send to backend
func (c *Connection) GenerateHAProxyHeader() []byte {
	if c.ProxyInfo == nil {
		return nil
	}

	if c.Listener.Route.HAProxy == "v1" {
		return c.ProxyInfo.GenerateV1()
	} else if c.Listener.Route.HAProxy == "v2" {
		return c.ProxyInfo.GenerateV2()
	}

	return nil
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
