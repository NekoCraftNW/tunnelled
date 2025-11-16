package ip

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
	"tunnelled/internal/router"
)

type IPUpdateRequest struct {
	Endpoints []string `json:"endpoints"`
	NewIP     string   `json:"new-ip"`
}

type IPUpdateResponse struct {
	Success bool   `json:"success"`
	Message string `json:"message"`
}

type IPNotifier struct {
	routeManager   *router.Manager
	clientEndpoint string
	bearerToken    string
	httpClient     *http.Client
}

func NewIPNotifier(routeManager *router.Manager, clientEndpoint, bearerToken string) *IPNotifier {
	return &IPNotifier{
		routeManager:   routeManager,
		clientEndpoint: clientEndpoint,
		bearerToken:    bearerToken,
		httpClient: &http.Client{
			Timeout: 15 * time.Second,
		},
	}
}

// NotifyClientOfIPChange sends IP update to client with list of endpoints to update
func (n *IPNotifier) NotifyClientOfIPChange(newIP string) error {
	// Collect all route IDs from RouterManager
	var endpoints []string
	n.routeManager.Routes.Range(func(key, value any) bool {
		route, ok := value.(*router.Route)
		if ok {
			endpoints = append(endpoints, route.RouteID)
		}
		return true
	})

	if len(endpoints) == 0 {
		return fmt.Errorf("no routes found to update")
	}

	updateReq := IPUpdateRequest{
		Endpoints: endpoints,
		NewIP:     newIP,
	}

	jsonData, err := json.Marshal(updateReq)
	if err != nil {
		return fmt.Errorf("failed to marshal IP update request: %v", err)
	}

	// Send to client with Bearer token
	url := fmt.Sprintf("%s/api/ip/update", n.clientEndpoint)
	req, err := http.NewRequest("POST", url, bytes.NewBuffer(jsonData))
	if err != nil {
		return fmt.Errorf("failed to create request: %v", err)
	}
	
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", n.bearerToken)
	
	resp, err := n.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to send IP update to client: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("client returned status %d for IP update", resp.StatusCode)
	}

	var updateResp IPUpdateResponse
	err = json.NewDecoder(resp.Body).Decode(&updateResp)
	if err != nil {
		return fmt.Errorf("failed to decode client response: %v", err)
	}

	if !updateResp.Success {
		return fmt.Errorf("client rejected IP update: %s", updateResp.Message)
	}

	fmt.Printf("Successfully notified client of IP change. Updated endpoints: %v -> %s\n", endpoints, newIP)
	return nil
}