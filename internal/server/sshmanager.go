package server

import (
	"sync"

	"golang.org/x/crypto/ssh"
)

// SSHManager manages SSH clients associated with session IDs
type SSHManager struct {
	mu      sync.RWMutex
	clients map[string]*ssh.Client
}

// NewSSHManager creates a new SSHManager
func NewSSHManager() *SSHManager {
	return &SSHManager{
		clients: make(map[string]*ssh.Client),
	}
}

// AddClient associates the SSH client with the given session ID
func (sm *SSHManager) AddClient(sessionID string, client *ssh.Client) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	sm.clients[sessionID] = client
}

// GetClient retrieves the SSH client associated with the given session ID
func (sm *SSHManager) GetClient(sessionID string) (*ssh.Client, bool) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	client, exists := sm.clients[sessionID]
	return client, exists
}

// RemoveClient removes the SSH client associated with the given session ID
func (sm *SSHManager) RemoveClient(sessionID string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	if client, exists := sm.clients[sessionID]; exists {
		client.Close()
		delete(sm.clients, sessionID)
	}
}

var sshManager = NewSSHManager()
