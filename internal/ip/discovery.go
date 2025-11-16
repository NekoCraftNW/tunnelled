package ip

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type DiscoveryService struct {
	currentIP     string
	lastChecked   time.Time
	checkInterval time.Duration
	httpClient    *http.Client
}

func NewDiscoveryService(checkIntervalSeconds int) *DiscoveryService {
	return &DiscoveryService{
		checkInterval: time.Duration(checkIntervalSeconds) * time.Second,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
		},
	}
}

// GetPublicIP fetches the public IP from checkip.amazonaws.com
func (d *DiscoveryService) GetPublicIP() (string, error) {
	resp, err := d.httpClient.Get("https://checkip.amazonaws.com")
	if err != nil {
		return "", fmt.Errorf("failed to fetch public IP: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("checkip.amazonaws.com returned status %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("failed to read response body: %v", err)
	}

	ip := strings.TrimSpace(string(body))
	if ip == "" {
		return "", fmt.Errorf("empty IP response from checkip.amazonaws.com")
	}

	return ip, nil
}

// CheckAndUpdateIP checks if IP has changed and returns the new IP if it has
func (d *DiscoveryService) CheckAndUpdateIP() (string, bool, error) {
	// Check if we should check IP based on interval
	if time.Since(d.lastChecked) < d.checkInterval && d.currentIP != "" {
		return d.currentIP, false, nil
	}

	newIP, err := d.GetPublicIP()
	if err != nil {
		return d.currentIP, false, err
	}

	d.lastChecked = time.Now()

	// Check if IP has changed
	if newIP != d.currentIP {
		oldIP := d.currentIP
		d.currentIP = newIP
		fmt.Printf("Public IP changed: %s -> %s\n", oldIP, newIP)
		return newIP, true, nil
	}

	return d.currentIP, false, nil
}

// GetCurrentIP returns the currently known IP without checking
func (d *DiscoveryService) GetCurrentIP() string {
	return d.currentIP
}

// ForceCheck forces an immediate IP check regardless of interval
func (d *DiscoveryService) ForceCheck() (string, bool, error) {
	d.lastChecked = time.Time{} // Reset last checked to force update
	return d.CheckAndUpdateIP()
}