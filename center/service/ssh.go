package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/ccfos/nightingale/v6/models"
	"github.com/ccfos/nightingale/v6/pkg/ctx"
	"github.com/pkg/errors"
	"golang.org/x/crypto/ssh"
)

// SSHTarget represents a target with SSH connection details
type SSHTarget struct {
	Host         string
	Port         int
	User         string
	AuthMethod   string // "key" or "password"
	Credential   string // Private key content or password
	SudoRequired bool
	Timeout      time.Duration
}

// TestSSHConnection tests SSH connection to a target
func TestSSHConnection(ctx *ctx.Context, targetIdent string) error {
	// Get managed host details
	managedHost, err := models.ManagedHostGetByIdent(ctx, targetIdent)
	if err != nil {
		return errors.Wrap(err, "failed to get managed host")
	}
	if managedHost == nil {
		return errors.New("managed host not found")
	}

	// Get target details
	target, err := models.TargetGetByIdent(ctx, targetIdent)
	if err != nil {
		return errors.Wrap(err, "failed to get target")
	}
	if target == nil {
		return errors.New("target not found")
	}

	// Determine SSH host
	sshHost := managedHost.SSHIp
	if sshHost == "" {
		sshHost = target.HostIp
	}
	if sshHost == "" {
		return errors.New("no SSH host available")
	}

	// Get credential
	credential, err := GetSSHCredential(ctx, targetIdent, managedHost.AuthMethod)
	if err != nil {
		return errors.Wrap(err, "failed to get SSH credential")
	}

	// Create SSH target
	sshTarget := &SSHTarget{
		Host:         sshHost,
		Port:         managedHost.SSHPort,
		User:         managedHost.SSHUser,
		AuthMethod:   managedHost.AuthMethod,
		Credential:   credential,
		SudoRequired: managedHost.SudoRequired,
		Timeout:      10 * time.Second,
	}

	// Test connection
	return testSSHConnection(sshTarget)
}

// testSSHConnection performs the actual SSH connection test
func testSSHConnection(target *SSHTarget) error {
	// Prepare SSH client configuration
	config := &ssh.ClientConfig{
		User:            target.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Insecure, but acceptable for testing
		Timeout:         target.Timeout,
	}

	// Set up authentication based on method
	switch target.AuthMethod {
	case "key":
		signer, err := ssh.ParsePrivateKey([]byte(target.Credential))
		if err != nil {
			return errors.Wrap(err, "failed to parse private key")
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	case "password":
		config.Auth = []ssh.AuthMethod{ssh.Password(target.Credential)}
	default:
		return errors.New("invalid auth method")
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", target.Host, target.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return errors.Wrap(err, "failed to connect to SSH server")
	}
	defer client.Close()

	// Test executing a simple command
	session, err := client.NewSession()
	if err != nil {
		return errors.Wrap(err, "failed to create SSH session")
	}
	defer session.Close()

	// Execute a simple command to verify connection
	output, err := session.Output("echo 'SSH connection test successful'")
	if err != nil {
		return errors.Wrap(err, "failed to execute test command")
	}

	// Check output
	if !strings.Contains(string(output), "SSH connection test successful") {
		return errors.New("unexpected output from test command")
	}

	// If sudo is required, test sudo access
	if target.SudoRequired {
		sudoSession, err := client.NewSession()
		if err != nil {
			return errors.Wrap(err, "failed to create SSH session for sudo test")
		}
		defer sudoSession.Close()

		// Try to execute a sudo command
		_, err = sudoSession.Output("sudo -n echo 'sudo test successful'")
		if err != nil {
			return errors.Wrap(err, "failed to execute sudo test command")
		}
	}

	return nil
}

// GetSSHClient creates and returns an SSH client for a managed host
func GetSSHClient(ctx *ctx.Context, targetIdent string) (*ssh.Client, error) {
	// Get managed host details
	managedHost, err := models.ManagedHostGetByIdent(ctx, targetIdent)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get managed host")
	}
	if managedHost == nil {
		return nil, errors.New("managed host not found")
	}

	// Get target details
	target, err := models.TargetGetByIdent(ctx, targetIdent)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get target")
	}
	if target == nil {
		return nil, errors.New("target not found")
	}

	// Determine SSH host
	sshHost := managedHost.SSHIp
	if sshHost == "" {
		sshHost = target.HostIp
	}
	if sshHost == "" {
		return nil, errors.New("no SSH host available")
	}

	// Get credential
	credential, err := GetSSHCredential(ctx, targetIdent, managedHost.AuthMethod)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get SSH credential")
	}

	// Create SSH target
	sshTarget := &SSHTarget{
		Host:         sshHost,
		Port:         managedHost.SSHPort,
		User:         managedHost.SSHUser,
		AuthMethod:   managedHost.AuthMethod,
		Credential:   credential,
		SudoRequired: managedHost.SudoRequired,
		Timeout:      30 * time.Second,
	}

	// Prepare SSH client configuration
	config := &ssh.ClientConfig{
		User:            sshTarget.User,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(), // Insecure for internal use
		Timeout:         sshTarget.Timeout,
	}

	// Set up authentication based on method
	switch sshTarget.AuthMethod {
	case "key":
		signer, err := ssh.ParsePrivateKey([]byte(sshTarget.Credential))
		if err != nil {
			return nil, errors.Wrap(err, "failed to parse private key")
		}
		config.Auth = []ssh.AuthMethod{ssh.PublicKeys(signer)}
	case "password":
		config.Auth = []ssh.AuthMethod{ssh.Password(sshTarget.Credential)}
	default:
		return nil, errors.New("invalid auth method")
	}

	// Connect to SSH server
	addr := fmt.Sprintf("%s:%d", sshTarget.Host, sshTarget.Port)
	client, err := ssh.Dial("tcp", addr, config)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to SSH server")
	}

	return client, nil
}
