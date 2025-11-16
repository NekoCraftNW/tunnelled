package ip

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type UpdateRequest struct {
	PublicIP  string    `json:"public_ip"`
	Timestamp time.Time `json:"timestamp"`
}

type UpdateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type NotificationClient struct {
	clientEndpoint string
	httpClient     *http.Client
}

func NewNotificationClient(clientEndpoint string) *NotificationClient {
	return &NotificationClient{
		clientEndpoint: clientEndpoint,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// NotifyIPChange sends the new public IP to the tunnelled-client
func (n *NotificationClient) NotifyIPChange(publicIP string) error {
	updateReq := UpdateRequest{
		PublicIP:  publicIP,
		Timestamp: time.Now(),
	}

	jsonData, err := json.Marshal(updateReq)
	if err != nil {
		return fmt.Errorf("failed to marshal IP update request: %v", err)
	}

	// Send to /api/ip/update endpoint
	url := fmt.Sprintf("%s/api/ip/update", n.clientEndpoint)
	resp, err := n.httpClient.Post(url, "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to send IP update to client: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("client returned status %d for IP update", resp.StatusCode)
	}

	var updateResp UpdateResponse
	err = json.NewDecoder(resp.Body).Decode(&updateResp)
	if err != nil {
		return fmt.Errorf("failed to decode client response: %v", err)
	}

	if !updateResp.Success {
		return fmt.Errorf("client rejected IP update: %s", updateResp.Message)
	}

	fmt.Printf("Successfully notified client of IP change: %s\n", publicIP)
	return nil
}

// TestConnection tests if the client endpoint is reachable
func (n *NotificationClient) TestConnection() error {
	url := fmt.Sprintf("%s/api/health", n.clientEndpoint)
	resp, err := n.httpClient.Get(url)
	if err != nil {
		return fmt.Errorf("failed to reach client endpoint: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("client health check returned status %d", resp.StatusCode)
	}

	return nil
}