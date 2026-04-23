package services

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"strings"
	"time"
)

// ContainerInfo represents a Docker container.
type ContainerInfo struct {
	ID      string          `json:"id"`
	Name    string          `json:"name"`
	Image   string          `json:"image"`
	State   string          `json:"state"`
	Status  string          `json:"status"`
	Created int64           `json:"created"`
	Ports   []ContainerPort `json:"ports"`
}

// ContainerPort represents a port mapping.
type ContainerPort struct {
	Private uint16 `json:"private"`
	Public  uint16 `json:"public"`
	Type    string `json:"type"`
}

// DockerService manages Docker containers via the Docker socket HTTP API.
type DockerService struct {
	client *http.Client
}

// NewDockerService creates a new Docker service connected via the Unix socket.
func NewDockerService() (*DockerService, error) {
	transport := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			return net.DialTimeout("unix", "/var/run/docker.sock", 5*time.Second)
		},
	}
	client := &http.Client{
		Transport: transport,
		Timeout:   30 * time.Second,
	}

	// Test connectivity
	resp, err := client.Get("http://docker/version")
	if err != nil {
		return nil, fmt.Errorf("cannot connect to Docker socket: %w", err)
	}
	resp.Body.Close()

	return &DockerService{client: client}, nil
}

// dockerAPIContainer is the raw structure from Docker's API.
type dockerAPIContainer struct {
	ID      string `json:"Id"`
	Names   []string
	Image   string
	State   string
	Status  string
	Created int64
	Ports   []struct {
		PrivatePort uint16 `json:"PrivatePort"`
		PublicPort  uint16 `json:"PublicPort"`
		Type        string
	}
}

// ListContainers returns all Docker containers (running and stopped).
func (s *DockerService) ListContainers() ([]ContainerInfo, error) {
	resp, err := s.client.Get("http://docker/containers/json?all=true")
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}
	defer resp.Body.Close()

	var raw []dockerAPIContainer
	if err := json.NewDecoder(resp.Body).Decode(&raw); err != nil {
		return nil, fmt.Errorf("failed to decode containers: %w", err)
	}

	var result []ContainerInfo
	for _, c := range raw {
		name := ""
		if len(c.Names) > 0 {
			name = strings.TrimPrefix(c.Names[0], "/")
		}

		var ports []ContainerPort
		for _, p := range c.Ports {
			ports = append(ports, ContainerPort{
				Private: p.PrivatePort,
				Public:  p.PublicPort,
				Type:    p.Type,
			})
		}

		id := c.ID
		if len(id) > 12 {
			id = id[:12]
		}

		result = append(result, ContainerInfo{
			ID:      id,
			Name:    name,
			Image:   c.Image,
			State:   c.State,
			Status:  c.Status,
			Created: c.Created,
			Ports:   ports,
		})
	}

	return result, nil
}

// StartContainer starts a stopped container.
func (s *DockerService) StartContainer(id string) error {
	resp, err := s.client.Post(
		fmt.Sprintf("http://docker/containers/%s/start", id),
		"application/json",
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker error (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// StopContainer stops a running container.
func (s *DockerService) StopContainer(id string) error {
	resp, err := s.client.Post(
		fmt.Sprintf("http://docker/containers/%s/stop?t=10", id),
		"application/json",
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker error (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// RestartContainer restarts a container.
func (s *DockerService) RestartContainer(id string) error {
	resp, err := s.client.Post(
		fmt.Sprintf("http://docker/containers/%s/restart?t=10", id),
		"application/json",
		nil,
	)
	if err != nil {
		return fmt.Errorf("failed to restart container: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker error (%d): %s", resp.StatusCode, string(body))
	}
	return nil
}

// GetContainerLogs returns the last N lines of container logs.
func (s *DockerService) GetContainerLogs(id string, lines int) (string, error) {
	resp, err := s.client.Get(
		fmt.Sprintf("http://docker/containers/%s/logs?stdout=true&stderr=true&tail=%d&timestamps=true", id, lines),
	)
	if err != nil {
		return "", fmt.Errorf("failed to get logs: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	// Clean Docker log headers (8-byte prefix per line)
	return cleanDockerLogs(string(data)), nil
}

// cleanDockerLogs strips the 8-byte Docker multiplex header from each line.
func cleanDockerLogs(raw string) string {
	var lines []string
	for _, line := range strings.Split(raw, "\n") {
		if len(line) > 8 {
			lines = append(lines, line[8:])
		} else if len(line) > 0 {
			lines = append(lines, line)
		}
	}
	return strings.Join(lines, "\n")
}
