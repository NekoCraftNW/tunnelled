package main

import (
	"errors"
	"flag"
	"fmt"
	"tunnelled/internal/http"
	"tunnelled/internal/net"
	"tunnelled/internal/net/dialer"
	"tunnelled/internal/router"
	"tunnelled/internal/util"
	"tunnelled/internal/version"

	"github.com/gin-gonic/gin"
)

func main() {
	appType := flag.String("type", "", "Server app type: client or server")
	flag.Parse()

	gin.SetMode(gin.ReleaseMode)

	if *appType != "client" && *appType != "server" {
		fmt.Println("Error: -type flag must be either 'client' or 'server'")
		return
	}

	fmt.Printf("Starting %s\n", version.GetBuildInfo())
	version.PrintGnet()

	_, err := dialer.FireUpClient()
	if err != nil {
		panic(errors.Join(errors.New("failed to start dialer client"), err))
		return
	}

	isServer := *appType == "server"

	rm := router.NewManager()
	rm.Routes.Range(func(key, value any) bool {
		route, ok := value.(*router.Route)
		if !ok {
			return true
		}

		listener := &net.Listener{
			Route:    route,
			IsServer: isServer,
		}
		go listener.FireUp()
		return true
	})

	if *appType == "server" {
		fireUpServer()
	}

	if *appType == "client" {
		fireUpClient(rm)
	}
}

func fireUpServer() {
	// in server mode, we need to lock down the process to keep gnet running
	select {}
}

func fireUpClient(rm *router.Manager) {
	fmt.Printf("Router > Loaded %d routes\n", util.LenSyncMap(rm.Routes))
	http.NewHTTPServer(rm)
}
