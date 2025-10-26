package router

import (
	"encoding/json"
	"errors"
	"os"
	"sync"
)

type Manager struct {
	Routes *sync.Map
}

var routesFile = "routes.json"

func NewManager() *Manager {
	m := &Manager{
		Routes: &sync.Map{},
	}

	if _, err := os.Stat(routesFile); os.IsNotExist(err) {
		m.LoadDefault()
		return m
	}
	data, err := os.ReadFile(routesFile)
	if err != nil {
		return m
	}
	var routes []*Route
	err = json.Unmarshal(data, &routes)
	if err != nil {
		panic(errors.Join(errors.New("cannot unmarshal routes file"), err))
		return m
	}

	for _, route := range routes {
		m.Routes.Store(route.RouteID, route)
	}

	return m
}

func (m *Manager) LoadDefault() {
	route := &Route{
		RouteID:  "default",
		BindIP:   "localhost",
		BindPort: 8080,
		HAProxy:  HAProxyOFF,
	}
	m.Routes.Store("default", route)
	_ = m.SaveRoutesToFile()
}

func (m *Manager) SaveRoutesToFile() error {
	var routes []*Route
	m.Routes.Range(func(key, value any) bool {
		route, ok := value.(*Route)
		if ok {
			routes = append(routes, route)
		}
		return true
	})

	data, err := json.MarshalIndent(routes, "", "  ")
	if err != nil {
		return err
	}

	return os.WriteFile(routesFile, data, 0644)
}

type HAProxyVersion string

const (
	HAProxyV1  HAProxyVersion = "v1"
	HAProxyV2  HAProxyVersion = "v2"
	HAProxyOFF HAProxyVersion = "off"
)

type Route struct {
	RouteID  string `json:"route_id"`
	BindIP   string `json:"bind_ip"`
	BindPort int    `json:"bind_port"`

	HAProxy HAProxyVersion `json:"haproxy"`

	BackendIP   string `json:"backend_ip"`
	BackendPort int    `json:"backend_port"`
}
