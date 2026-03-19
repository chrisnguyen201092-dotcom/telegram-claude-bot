package store

import (
	"fmt"
	"path/filepath"
	"strings"
)

type McpServer struct {
	Name      string            `json:"name"`
	Type      string            `json:"type"`
	Command   string            `json:"command,omitempty"`
	Args      []string          `json:"args,omitempty"`
	URL       string            `json:"url,omitempty"`
	Env       map[string]string `json:"env,omitempty"`
	IsActive  bool              `json:"is_active"`
	CreatedAt string            `json:"created_at"`
}

func mcpPath(name string) string {
	return filepath.Join(DataDir, "mcp", SafeFilename(name)+".json")
}

func AddMcpServer(server *McpServer) error {
	path := mcpPath(server.Name)
	unlock := lockFile(path)
	defer unlock()
	if server.CreatedAt == "" {
		server.CreatedAt = NowUTC()
	}
	return WriteJSON(path, server)
}

func RemoveMcpServer(name string) (bool, error) {
	path := mcpPath(name)
	// M11: Lock first to prevent TOCTOU race
	unlock := lockFile(path)
	defer unlock()
	if !FileExists(path) {
		return false, nil
	}
	return true, DeleteFile(path)
}

func ToggleMcpServer(name string) (found bool, isActive bool, err error) {
	path := mcpPath(name)
	// M11: Lock first to prevent TOCTOU race
	unlock := lockFile(path)
	defer unlock()
	if !FileExists(path) {
		return false, false, nil
	}
	server, err := ReadJSON[McpServer](path)
	if err != nil {
		return false, false, err
	}
	server.IsActive = !server.IsActive
	err = WriteJSON(path, server)
	return true, server.IsActive, err
}

func ListMcpServers() ([]*McpServer, error) {
	dir := filepath.Join(DataDir, "mcp")
	names, err := ListJSONFiles(dir)
	if err != nil {
		return nil, err
	}
	var servers []*McpServer
	for _, name := range names {
		s, err := ReadJSON[McpServer](filepath.Join(dir, name+".json"))
		if err != nil {
			continue
		}
		servers = append(servers, &s)
	}
	return servers, nil
}

func ListActiveMcpServers() ([]*McpServer, error) {
	all, err := ListMcpServers()
	if err != nil {
		return nil, err
	}
	var active []*McpServer
	for _, s := range all {
		if s.IsActive {
			active = append(active, s)
		}
	}
	return active, nil
}

func BuildMcpConfigs() (map[string]any, error) {
	servers, err := ListActiveMcpServers()
	if err != nil {
		return nil, err
	}
	configs := make(map[string]any)
	for _, s := range servers {
		switch s.Type {
		case "stdio":
			entry := map[string]any{
				"command": s.Command,
				"args":    s.Args,
			}
			if len(s.Env) > 0 {
				entry["env"] = s.Env
			}
			configs[s.Name] = entry
		case "sse", "http":
			configs[s.Name] = map[string]any{
				"url": s.URL,
			}
		}
	}
	return configs, nil
}

func FormatServerList(servers []*McpServer) string {
	if len(servers) == 0 {
		return "No MCP servers configured."
	}
	var sb strings.Builder
	for _, s := range servers {
		status := "on"
		if !s.IsActive {
			status = "off"
		}
		detail := ""
		if s.Type == "stdio" {
			detail = fmt.Sprintf("%s %s", s.Command, strings.Join(s.Args, " "))
		} else {
			detail = s.URL
		}
		sb.WriteString(fmt.Sprintf("- **%s** [%s/%s]: %s\n", s.Name, s.Type, status, detail))
	}
	return sb.String()
}
