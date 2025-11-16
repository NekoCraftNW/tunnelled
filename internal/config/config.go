package config

import (
	"encoding/json"
	"errors"
	"os"
)

type ClientConfig struct {
	HTTPPort int `json:"http_port"`
}

type ServerConfig struct {
	ClientEndpoint  string `json:"client_endpoint"`  // HTTP endpoint of tunnelled-client
	IPCheckInterval int    `json:"ip_check_interval"` // in seconds
}

var clientConfigFile = "config.json"
var serverConfigFile = "config.json"

func LoadClientConfig() (*ClientConfig, error) {
	config := &ClientConfig{
		HTTPPort: 8080, // Default
	}

	if _, err := os.Stat(clientConfigFile); os.IsNotExist(err) {
		// Create default config
		return config, SaveClientConfig(config)
	}

	data, err := os.ReadFile(clientConfigFile)
	if err != nil {
		return config, err
	}

	err = json.Unmarshal(data, config)
	if err != nil {
		return config, errors.Join(errors.New("failed to parse client config"), err)
	}

	return config, nil
}

func SaveClientConfig(config *ClientConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(clientConfigFile, data, 0644)
}

func LoadServerConfig() (*ServerConfig, error) {
	config := &ServerConfig{
		ClientEndpoint:  "http://YOUR_VPS_IP:8080", // Default - needs to be configured
		IPCheckInterval: 300,                       // 5 minutes default
	}

	if _, err := os.Stat(serverConfigFile); os.IsNotExist(err) {
		// Create default config
		return config, SaveServerConfig(config)
	}

	data, err := os.ReadFile(serverConfigFile)
	if err != nil {
		return config, err
	}

	err = json.Unmarshal(data, config)
	if err != nil {
		return config, errors.Join(errors.New("failed to parse server config"), err)
	}

	return config, nil
}

func SaveServerConfig(config *ServerConfig) error {
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(serverConfigFile, data, 0644)
}