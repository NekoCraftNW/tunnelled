package http

import (
	"math/rand"
	"os"
	"time"
	"tunnelled/internal/router"

	"github.com/gin-gonic/gin"
)

type UpdateRequest struct {
	RouteID string `json:"route_id"`
	IP      string `json:"ip"`
	Port    int    `json:"port"`
}

func NewHTTPServer(manager *router.Manager) {
	r := gin.Default()

	bearerToken := "Bearer " + ReadToken()

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
	})

	r.Run(":8080")
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
