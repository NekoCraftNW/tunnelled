package haproxy

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"net"
	"strconv"
	"strings"
)

type ProxyInfo struct {
	SrcIP   net.IP
	DstIP   net.IP
	SrcPort uint16
	DstPort uint16
	Version int
}

// ParseV1 parses HAProxy protocol v1 header
// Format: "PROXY TCP4 <src_ip> <dst_ip> <src_port> <dst_port>\r\n"
func ParseV1(data []byte) (*ProxyInfo, int, error) {
	// Find the end of the header (\r\n)
	headerEnd := bytes.Index(data, []byte("\r\n"))
	if headerEnd == -1 {
		return nil, 0, fmt.Errorf("incomplete HAProxy v1 header")
	}

	headerStr := string(data[:headerEnd])
	parts := strings.Fields(headerStr)

	if len(parts) != 6 || parts[0] != "PROXY" {
		return nil, 0, fmt.Errorf("invalid HAProxy v1 header format")
	}

	protocol := parts[1]
	if protocol != "TCP4" && protocol != "TCP6" {
		return nil, 0, fmt.Errorf("unsupported protocol: %s", protocol)
	}

	srcIP := net.ParseIP(parts[2])
	dstIP := net.ParseIP(parts[3])
	if srcIP == nil || dstIP == nil {
		return nil, 0, fmt.Errorf("invalid IP addresses")
	}

	srcPort, err := strconv.ParseUint(parts[4], 10, 16)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid source port: %v", err)
	}

	dstPort, err := strconv.ParseUint(parts[5], 10, 16)
	if err != nil {
		return nil, 0, fmt.Errorf("invalid destination port: %v", err)
	}

	return &ProxyInfo{
		SrcIP:   srcIP,
		DstIP:   dstIP,
		SrcPort: uint16(srcPort),
		DstPort: uint16(dstPort),
		Version: 1,
	}, headerEnd + 2, nil // +2 for \r\n
}

// ParseV2 parses HAProxy protocol v2 header
func ParseV2(data []byte) (*ProxyInfo, int, error) {
	if len(data) < 16 {
		return nil, 0, fmt.Errorf("insufficient data for HAProxy v2 header")
	}

	// Check signature: \x0D\x0A\x0D\x0A\x00\x0D\x0A\x51\x55\x49\x54\x0A
	signature := []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A}
	if !bytes.Equal(data[:12], signature) {
		return nil, 0, fmt.Errorf("invalid HAProxy v2 signature")
	}

	// Parse version and command
	versionCmd := data[12]
	version := (versionCmd & 0xF0) >> 4
	cmd := versionCmd & 0x0F

	if version != 2 {
		return nil, 0, fmt.Errorf("invalid HAProxy version: %d", version)
	}

	if cmd != 1 { // PROXY command
		return nil, 0, fmt.Errorf("unsupported HAProxy command: %d", cmd)
	}

	// Parse family and protocol
	familyProto := data[13]
	family := (familyProto & 0xF0) >> 4
	protocol := familyProto & 0x0F

	if protocol != 1 { // TCP
		return nil, 0, fmt.Errorf("unsupported protocol: %d", protocol)
	}

	// Parse length
	length := binary.BigEndian.Uint16(data[14:16])
	totalHeaderSize := 16 + int(length)

	if len(data) < totalHeaderSize {
		return nil, 0, fmt.Errorf("insufficient data for complete HAProxy v2 header")
	}

	var srcIP, dstIP net.IP
	var srcPort, dstPort uint16

	if family == 1 { // IPv4
		if length < 12 {
			return nil, 0, fmt.Errorf("insufficient data for IPv4 addresses")
		}
		srcIP = net.IP(data[16:20])
		dstIP = net.IP(data[20:24])
		srcPort = binary.BigEndian.Uint16(data[24:26])
		dstPort = binary.BigEndian.Uint16(data[26:28])
	} else if family == 2 { // IPv6
		if length < 36 {
			return nil, 0, fmt.Errorf("insufficient data for IPv6 addresses")
		}
		srcIP = net.IP(data[16:32])
		dstIP = net.IP(data[32:48])
		srcPort = binary.BigEndian.Uint16(data[48:50])
		dstPort = binary.BigEndian.Uint16(data[50:52])
	} else {
		return nil, 0, fmt.Errorf("unsupported address family: %d", family)
	}

	return &ProxyInfo{
		SrcIP:   srcIP,
		DstIP:   dstIP,
		SrcPort: srcPort,
		DstPort: dstPort,
		Version: 2,
	}, totalHeaderSize, nil
}

// GenerateV1 generates HAProxy protocol v1 header
func (p *ProxyInfo) GenerateV1() []byte {
	protocol := "TCP4"
	if p.SrcIP.To4() == nil {
		protocol = "TCP6"
	}

	header := fmt.Sprintf("PROXY %s %s %s %d %d\r\n",
		protocol, p.SrcIP.String(), p.DstIP.String(), p.SrcPort, p.DstPort)
	return []byte(header)
}

// GenerateV2 generates HAProxy protocol v2 header
func (p *ProxyInfo) GenerateV2() []byte {
	signature := []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A}
	
	versionCmd := byte(0x21) // Version 2, PROXY command
	
	var familyProto byte
	var addressData []byte
	
	if p.SrcIP.To4() != nil {
		// IPv4
		familyProto = 0x11 // IPv4, TCP
		addressData = make([]byte, 12)
		copy(addressData[0:4], p.SrcIP.To4())
		copy(addressData[4:8], p.DstIP.To4())
		binary.BigEndian.PutUint16(addressData[8:10], p.SrcPort)
		binary.BigEndian.PutUint16(addressData[10:12], p.DstPort)
	} else {
		// IPv6
		familyProto = 0x21 // IPv6, TCP
		addressData = make([]byte, 36)
		copy(addressData[0:16], p.SrcIP.To16())
		copy(addressData[16:32], p.DstIP.To16())
		binary.BigEndian.PutUint16(addressData[32:34], p.SrcPort)
		binary.BigEndian.PutUint16(addressData[34:36], p.DstPort)
	}
	
	length := make([]byte, 2)
	binary.BigEndian.PutUint16(length, uint16(len(addressData)))
	
	header := make([]byte, 0, 16+len(addressData))
	header = append(header, signature...)
	header = append(header, versionCmd)
	header = append(header, familyProto)
	header = append(header, length...)
	header = append(header, addressData...)
	
	return header
}

// IsHAProxyHeader checks if data starts with HAProxy protocol header
func IsHAProxyHeader(data []byte) (bool, int) {
	if len(data) < 5 {
		return false, 0
	}

	// Check for v1 header
	if bytes.HasPrefix(data, []byte("PROXY")) {
		return true, 1
	}

	// Check for v2 signature
	if len(data) >= 12 {
		signature := []byte{0x0D, 0x0A, 0x0D, 0x0A, 0x00, 0x0D, 0x0A, 0x51, 0x55, 0x49, 0x54, 0x0A}
		if bytes.Equal(data[:12], signature) {
			return true, 2
		}
	}

	return false, 0
}