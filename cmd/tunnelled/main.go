package main

import (
	"errors"
	"flag"
	"fmt"
	"time"
	"tunnelled/internal/config"
	"tunnelled/internal/http"
	"tunnelled/internal/ip"
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
		fireUpServer(rm)
	}

	if *appType == "client" {
		fireUpClient(rm)
	}
}

func fireUpServer(rm *router.Manager) {
	// Load server configuration
	serverConfig, err := config.LoadServerConfig()
	if err != nil {
		panic(fmt.Errorf("failed to load server config: %v", err))
	}

	// Initialize IP discovery service
	discoveryService := ip.NewDiscoveryService(serverConfig.IPCheckInterval)

	// Get client bearer token (should be same as client's .token file)
	clientBearerToken := "Bearer " + http.ReadToken()

	// Initialize IP notifier
	notifier := ip.NewIPNotifier(rm, serverConfig.ClientEndpoint, clientBearerToken)

	// Start IP monitoring goroutine
	go func() {
		ticker := time.NewTicker(time.Duration(serverConfig.IPCheckInterval) * time.Second)
		defer ticker.Stop()

		// Initial IP check
		currentIP, changed, err := discoveryService.ForceCheck()
		if err != nil {
			fmt.Printf("Initial IP check failed: %v\n", err)
		} else if changed {
			fmt.Printf("Initial public IP: %s\n", currentIP)
			err = notifier.NotifyClientOfIPChange(currentIP)
			if err != nil {
				fmt.Printf("Failed to notify client of initial IP: %v\n", err)
			}
		}

		// Periodic IP checks
		for range ticker.C {
			newIP, changed, err := discoveryService.CheckAndUpdateIP()
			if err != nil {
				fmt.Printf("IP check failed: %v\n", err)
				continue
			}

			if changed {
				err = notifier.NotifyClientOfIPChange(newIP)
				if err != nil {
					fmt.Printf("Failed to notify client of IP change: %v\n", err)
				}
			}
		}
	}()

	// in server mode, we need to lock down the process to keep gnet running
	select {}
}

func fireUpClient(rm *router.Manager) {
	fmt.Printf("Router > Loaded %d routes\n", util.LenSyncMap(rm.Routes))
	http.NewHTTPServer(rm)
}
