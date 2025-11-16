package http

import (
	"fmt"
	"math/rand"
	"os"
	"sync"
	"time"
	"tunnelled/internal/config"
	"tunnelled/internal/ip"
	"tunnelled/internal/router"

	"github.com/gin-gonic/gin"
)

type UpdateRequest struct {
	RouteID string `json:"route_id"`
	IP      string `json:"ip"`
	Port    int    `json:"port"`
}

// Server state to store current public IP
var (
	currentPublicIP string
	publicIPMutex   sync.RWMutex
)

func SetPublicIP(ip string) {
	publicIPMutex.Lock()
	defer publicIPMutex.Unlock()
	currentPublicIP = ip
}

func GetPublicIP() string {
	publicIPMutex.RLock()
	defer publicIPMutex.RUnlock()
	return currentPublicIP
}

func NewHTTPServer(manager *router.Manager) {
	// Load client config for HTTP port
	clientConfig, err := config.LoadClientConfig()
	if err != nil {
		panic(fmt.Errorf("failed to load client config: %v", err))
	}

	r := gin.Default()
	bearerToken := "Bearer " + ReadToken()

	// Health check endpoint
	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(200, gin.H{
			"status":    "ok",
			"timestamp": time.Now(),
		})
	})

	// Endpoint to receive IP updates from server
	r.POST("/api/ip/update", func(c *gin.Context) {
		// Check bearer token
		token := c.GetHeader("Authorization")
		if token != bearerToken {
			c.JSON(401, gin.H{
				"success": false,
				"message": "unauthorized",
			})
			return
		}

		var updateReq ip.IPUpdateRequest
		if err := c.BindJSON(&updateReq); err != nil {
			c.JSON(400, gin.H{
				"success": false,
				"message": "invalid request format",
			})
			return
		}

		// Update backend IP for specified endpoints
		updatedCount := 0
		for _, routeID := range updateReq.Endpoints {
			value, ok := manager.Routes.Load(routeID)
			if !ok {
				fmt.Printf("Warning: route %s not found\n", routeID)
				continue
			}

			route, ok := value.(*router.Route)
			if !ok {
				fmt.Printf("Warning: invalid route type for %s\n", routeID)
				continue
			}

			route.BackendIP = updateReq.NewIP
			updatedCount++
			fmt.Printf("Updated route %s backend IP to %s\n", routeID, updateReq.NewIP)
		}

		// Save updated routes to file
		if updatedCount > 0 {
			err := manager.SaveRoutesToFile()
			if err != nil {
				c.JSON(500, gin.H{
					"success": false,
					"message": fmt.Sprintf("failed to save routes: %v", err),
				})
				return
			}
		}

		c.JSON(200, gin.H{
			"success": true,
			"message": fmt.Sprintf("updated %d routes", updatedCount),
		})
	})

	// Existing route update endpoint
	r.POST("/update", func(c *gin.Context) {
		// read if the request has the bearer token
		token := c.GetHeader("Authorization")
		if token != bearerToken {
			c.JSON(401, gin.H{"error": "unauthorized"})
			return
		}

		var req UpdateRequest
		if err := c.BindJSON(&req); err != nil {
			c.JSON(400, gin.H{"error": "bad request"})
			return
		}

		value, ok := manager.Routes.Load(req.RouteID)
		if !ok {
			c.JSON(404, gin.H{"error": "route not found"})
			return
		}

		route, ok := value.(*router.Route)
		if !ok {
			c.JSON(500, gin.H{"error": "internal server error"})
			return
		}

		route.BindIP = req.IP
		route.BindPort = req.Port

		err := manager.SaveRoutesToFile()
		if err != nil {
			c.JSON(500, gin.H{"error": "failed to save routes"})
			return
		}

		c.JSON(200, gin.H{"success": true})
	})

	// Start server on configured port
	address := fmt.Sprintf(":%d", clientConfig.HTTPPort)
	fmt.Printf("Starting HTTP server on %s\n", address)
	err = r.Run(address)
	if err != nil {
		panic(fmt.Errorf("failed to start HTTP server: %v", err))
	}
}

func ReadToken() string {
	tokenFile := ".token"
	if _, err := os.Stat(tokenFile); os.IsNotExist(err) {
		token := generateRandomToken(32)
		err := os.WriteFile(tokenFile, []byte(token), 0600)
		if err != nil {
			panic(err)
		}
		return token
	}
	data, err := os.ReadFile(tokenFile)
	if err != nil {
		panic(err)
	}
	return string(data)
}

func generateRandomToken(length int) string {
	seededRand := rand.New(rand.NewSource(time.Now().UnixNano()))
	const charset = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789"
	b := make([]byte, length)
	for i := range b {
		b[i] = charset[seededRand.Intn(len(charset))]
	}
	return string(b)
}
