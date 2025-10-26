package net

import (
	"errors"
	"fmt"
	"sync"
	"time"
	"tunnelled/internal/net/dialer"
	"tunnelled/internal/router"

	"github.com/panjf2000/gnet/v2"
)

type Manager struct {
	Listeners map[string]*Listener
}

// Global connection manager for server mode
var (
	ActiveConnections = make(map[string]*Connection)
	ConnectionsMutex  sync.RWMutex
)

func RegisterConnection(connectionID string, conn *Connection) {
	ConnectionsMutex.Lock()
	defer ConnectionsMutex.Unlock()
	ActiveConnections[connectionID] = conn
	fmt.Printf("Registered connection %s\n", connectionID)
}

func UnregisterConnection(connectionID string) {
	ConnectionsMutex.Lock()
	defer ConnectionsMutex.Unlock()
	delete(ActiveConnections, connectionID)
	fmt.Printf("Unregistered connection %s\n", connectionID)
}

func GetConnection(connectionID string) (*Connection, bool) {
	ConnectionsMutex.RLock()
	defer ConnectionsMutex.RUnlock()
	conn, exists := ActiveConnections[connectionID]
	return conn, exists
}

type Listener struct {
	gnet.BuiltinEventEngine

	eng      gnet.Engine
	Route    *router.Route
	IsServer bool
}

// FireUp starts the listener to accept incoming connections
// and forward them to the backend server.
// If isClient is true, it indicates that this listener is running on the client side.
// If not, we'll check if the first packet is "magic", which means that it contains
// the connection id for later routing.
func (l *Listener) FireUp() {
	bind := fmt.Sprintf("tcp://%s:%d", l.Route.BindIP, l.Route.BindPort)
	err := gnet.Run(l, bind, gnet.WithMulticore(true))
	if err != nil {
		panic(errors.Join(errors.New(fmt.Sprintf("failed to start listener %s over %s", l.Route.RouteID, bind)), err))
	}
}

func (l *Listener) OnBoot(eng gnet.Engine) gnet.Action {
	l.eng = eng
	fmt.Printf("Listener %s is now listening on %s:%d\n", l.Route.RouteID, l.Route.BindIP, l.Route.BindPort)
	return gnet.None
}

func (l *Listener) OnOpen(conn gnet.Conn) (out []byte, action gnet.Action) {
	fmt.Printf("New connection from %s on listener %s\n", conn.RemoteAddr().String(), l.Route.RouteID)

	if l.IsServer {
		// In server mode, incoming connections are from tunnelled-client
		// We don't create a user connection yet, just wait for ConnectionID packet
		fmt.Printf("Server mode: waiting for ConnectionID from client\n")
		return nil, gnet.None
	}

	// Client mode: this is a real user connection
	connection := NewConnection(l, conn)
	conn.SetContext(connection)

	th := &ReverseTrafficHandler{
		Connection: connection,
	}

	l.attemptBackendConnection(connection, th)
	return nil, gnet.None
}

func (l *Listener) attemptBackendConnection(connection *Connection, th *ReverseTrafficHandler) {
	_, err := dialer.GlobalClient.DialContext("tcp", fmt.Sprintf("%s:%d", l.Route.BackendIP, l.Route.BackendPort), th)
	if err != nil {
		fmt.Printf("Failed to connect to backend for listener %s: %v\n", l.Route.RouteID, err)
		connection.IsConnected = false

		// Only reconnect if we're in client mode
		if !l.IsServer {
			go l.scheduleReconnect(connection, th)
		}
		return
	}
	connection.IsConnected = true
	connection.ReconnectAttempts = 0
}

func (l *Listener) scheduleReconnect(connection *Connection, th *ReverseTrafficHandler) {
	// Never reconnect in server mode
	if l.IsServer || connection.ClientConn == nil {
		fmt.Println("Ignoring reconnect attempt in server mode or no client connection")
		return
	}

	delay := connection.GetReconnectDelay()
	connection.ReconnectAttempts++
	connection.LastReconnectTime = time.Now()

	fmt.Printf("Scheduling reconnect attempt %d in %v for listener %s\n",
		connection.ReconnectAttempts, delay, l.Route.RouteID)

	time.Sleep(delay)

	if connection.ClientConn != nil {
		l.attemptBackendConnection(connection, th)
	}
}

func (l *Listener) OnClose(conn gnet.Conn, err error) (action gnet.Action) {
	fmt.Printf("Client disconnected from listener %s: %v\n", l.Route.RouteID, err)

	connection, ok := conn.Context().(*Connection)
	if !ok || connection == nil {
		return gnet.None
	}

	// Unregister connection in server mode
	if l.IsServer {
		UnregisterConnection(connection.ConnectionID)
	}

	// Close backend connection when client disconnects
	if connection.BackendConn != nil {
		fmt.Printf("Closing backend connection for listener %s\n", l.Route.RouteID)
		connection.BackendConn.Close()
		connection.BackendConn = nil
	}

	connection.IsConnected = false
	connection.ClientConn = nil

	return gnet.None
}

type ReverseTrafficHandler struct {
	dialer.TrafficHandler
	Connection *Connection
}

func (l *Listener) OnTraffic(clientConn gnet.Conn) (action gnet.Action) {
	gnetBuffer, _ := clientConn.Next(-1)
	data := make([]byte, len(gnetBuffer))
	copy(data, gnetBuffer)

	if l.IsServer {
		// Server mode: check if this is a connection ID packet from client
		isIDPacket, connectionID := (&Connection{}).IsConnectionIDPacket(data)
		if isIDPacket {
			fmt.Printf("Received connection ID: %s from client\n", connectionID)

			// Create a new connection representing this user session
			connection := &Connection{
				Listener:          l,
				ConnectionID:      connectionID,
				ClientConn:        clientConn, // The connection from tunnelled-client
				IsConnected:       false,      // Not connected to backend yet
				PacketQueue:       make([][]byte, 0),
				MaxReconnectDelay: 30 * time.Second,
				MaxQueueSize:      1000,
			}

			clientConn.SetContext(connection)
			RegisterConnection(connectionID, connection)
			fmt.Printf("Created connection for ID %s, connecting to backend\n", connectionID)

			// Connect to actual backend (BungeeCord)
			th := &ReverseTrafficHandler{
				Connection: connection,
			}
			l.attemptBackendConnection(connection, th)

			return gnet.None
		}

		// Regular traffic in server mode - forward to backend (BungeeCord)
		conn, ok := clientConn.Context().(*Connection)
		if !ok || conn == nil {
			return gnet.Close
		}

		if conn.BackendConn != nil {
			conn.BackendConn.Write(data)
		}
		return gnet.None
	}

	// Client mode traffic handling
	fmt.Println("client sent traffic, forwarding to backend...")
	conn, ok := clientConn.Context().(*Connection)
	if !ok || conn == nil {
		return gnet.Close
	}

	if conn.IsConnected && conn.BackendConn != nil {
		conn.BackendConn.Write(data)
	} else {
		fmt.Println("Backend disconnected, queuing packet...")
		conn.QueuePacket(data)
	}

	return gnet.None
}

func (rth *ReverseTrafficHandler) HandleTraffic(gnetConn gnet.Conn, data []byte) gnet.Action {
	fmt.Println("backend sent traffic, forwarding to client...")
	rth.Connection.ClientConn.Write(data)
	return gnet.None
}

func (rth *ReverseTrafficHandler) OnConnection(gnetConn gnet.Conn) {
	rth.Connection.BackendConn = gnetConn
	rth.Connection.IsConnected = true
	rth.Connection.ReconnectAttempts = 0
	fmt.Printf("Backend connected for listener %s (ConnectionID: %s)\n",
		rth.Connection.Listener.Route.RouteID, rth.Connection.ConnectionID)

	// Send connection ID as first packet if in client mode
	if !rth.Connection.Listener.IsServer {
		magicPacket := rth.Connection.SendConnectionID()
		gnetConn.Write(magicPacket)
		fmt.Printf("Sent connection ID %s to server\n", rth.Connection.ConnectionID)
	}

	rth.Connection.FlushQueue()
}

func (rth *ReverseTrafficHandler) OnDisconnection(gnetConn gnet.Conn, err error) {
	fmt.Printf("Backend disconnected for listener %s: %v\n", rth.Connection.Listener.Route.RouteID, err)
	rth.Connection.IsConnected = false
	rth.Connection.BackendConn = nil

	if rth.Connection.Listener.IsServer {
		// In server mode: backend disconnect (BungeeCord) should close client connection
		if rth.Connection.ClientConn != nil {
			fmt.Printf("Closing client connection due to backend disconnect in server mode for listener %s\n", rth.Connection.Listener.Route.RouteID)
			rth.Connection.ClientConn.Close()
			rth.Connection.ClientConn = nil
		}
	} else {
		// In client mode: keep client alive and try to reconnect to server
		fmt.Printf("Keeping client alive, will try to reconnect to server for listener %s\n", rth.Connection.Listener.Route.RouteID)
		go rth.Connection.Listener.scheduleReconnect(rth.Connection, rth)
	}
}
