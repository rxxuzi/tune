package command

import (
	"fmt"
	"strings"

	"github.com/rxxuzi/tune/internal/logger"
	"golang.org/x/crypto/ssh"
)

func ExecuteCommand(client *ssh.Client, cmd string) (string, error) {
	session, err := client.NewSession()
	if err != nil {
		logger.Err("Failed to create SSH session: %v", err)
		return "", err
	}
	defer session.Close()

	output, err := session.CombinedOutput(cmd)
	if err != nil {
		logger.Err("Failed to execute command '%s'. Output: %s, Error: %v", cmd, string(output), err)
		return "", fmt.Errorf("command failed: %v, output: %s", err, string(output))
	}

	return strings.TrimSpace(string(output)), nil
}
