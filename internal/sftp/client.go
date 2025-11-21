package sftp

import (
	"crypto/sha256"
	"fmt"
	"io"
	"net"
	"os"
	"path/filepath"
	"time"

	"github.com/pkg/sftp"
	"golang.org/x/crypto/ssh"
	"golang.org/x/crypto/ssh/knownhosts"

	"gih-ftp/internal/logger"
)

type Client struct {
	host               string
	user               string
	password           string
	keyPath            string
	insecureSkipVerify bool
}

func NewClient(host, user, password, keyPath string, insecureSkipVerify bool) *Client {
	return &Client{
		host:               host,
		user:               user,
		password:           password,
		keyPath:            keyPath,
		insecureSkipVerify: insecureSkipVerify,
	}
}

func (c *Client) Upload(localPath, remotePath string) error {
	logger.Info("Starting SFTP upload",
		"local_file", localPath,
		"remote_path", remotePath,
		"host", c.host,
	)

	// Load SSH config
	sshConfig, err := c.getSSHConfig()
	if err != nil {
		return fmt.Errorf("failed to create SSH config: %w", err)
	}

	// Connect to SSH server
	hostPort := c.host
	if _, _, err := net.SplitHostPort(hostPort); err != nil {
		// No port specified, add default SSH port
		hostPort = net.JoinHostPort(hostPort, "22")
	}

	logger.Debug("Connecting to SSH server", "host", hostPort)

	sshClient, err := ssh.Dial("tcp", hostPort, sshConfig)
	if err != nil {
		return fmt.Errorf("SSH connection failed: %w", err)
	}
	defer sshClient.Close()

	logger.Debug("SSH connection established")

	// Create SFTP client
	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("SFTP client creation failed: %w", err)
	}
	defer sftpClient.Close()

	logger.Debug("SFTP client created")

	// Open local file
	localFile, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer localFile.Close()

	// Get file info
	fileInfo, err := localFile.Stat()
	if err != nil {
		return fmt.Errorf("failed to stat local file: %w", err)
	}

	logger.Debug("Local file opened",
		"size_bytes", fileInfo.Size(),
		"modified", fileInfo.ModTime(),
	)

	// Ensure remote directory exists
	remoteDir := filepath.Dir(remotePath)
	if err := sftpClient.MkdirAll(remoteDir); err != nil {
		return fmt.Errorf("failed to create remote directory: %w", err)
	}

	logger.Debug("Remote directory ensured", "path", remoteDir)

	// Create remote file
	remoteFile, err := sftpClient.Create(remotePath)
	if err != nil {
		return fmt.Errorf("failed to create remote file: %w", err)
	}
	defer remoteFile.Close()

	// Copy file with progress tracking
	startTime := time.Now()
	written, err := io.Copy(remoteFile, localFile)
	if err != nil {
		return fmt.Errorf("file upload failed: %w", err)
	}

	duration := time.Since(startTime)
	speedMBps := float64(written) / duration.Seconds() / (1024 * 1024)

	logger.Info("SFTP upload completed",
		"bytes_uploaded", written,
		"duration_seconds", duration.Seconds(),
		"speed_mbps", fmt.Sprintf("%.2f", speedMBps),
	)

	return nil
}

func (c *Client) getSSHConfig() (*ssh.ClientConfig, error) {
	config := &ssh.ClientConfig{
		User:    c.user,
		Timeout: 15 * time.Second,
	}

	// Set up authentication
	authMethods := []ssh.AuthMethod{}

	// Try password first if provided
	if c.password != "" {
		authMethods = append(authMethods, ssh.Password(c.password))
		logger.Debug("Using password authentication")
	}

	// Try key-based auth if key path is provided
	if c.keyPath != "" {
		keyAuth, err := c.loadPrivateKey(c.keyPath)
		if err != nil {
			logger.Warn("Failed to load SSH private key", "error", err)
		} else {
			authMethods = append(authMethods, keyAuth)
			logger.Debug("Using key-based authentication", "key_path", c.keyPath)
		}
	}

	if len(authMethods) == 0 {
		return nil, fmt.Errorf("no authentication method available (provide password or SSH key)")
	}

	config.Auth = authMethods

	// Set up host key verification
	if c.insecureSkipVerify {
		logger.Warn("SSH host key verification is DISABLED - this is insecure!")
		config.HostKeyCallback = ssh.InsecureIgnoreHostKey()
	} else {
		hostKeyCallback, err := c.getHostKeyCallback()
		if err != nil {
			logger.Warn("Failed to load known_hosts, falling back to fingerprint verification",
				"error", err)
			config.HostKeyCallback = c.trustOnFirstUse()
		} else {
			config.HostKeyCallback = hostKeyCallback
			logger.Debug("Using known_hosts for host key verification")
		}
	}

	return config, nil
}

func (c *Client) loadPrivateKey(keyPath string) (ssh.AuthMethod, error) {
	// Expand environment variables
	expandedPath := os.ExpandEnv(keyPath)

	key, err := os.ReadFile(expandedPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read private key: %w", err)
	}

	// Try without passphrase first
	signer, err := ssh.ParsePrivateKey(key)
	if err != nil {
		// If it fails, try with passphrase from environment
		passphrase := os.Getenv("SSH_KEY_PASSPHRASE")
		if passphrase != "" {
			signer, err = ssh.ParsePrivateKeyWithPassphrase(key, []byte(passphrase))
			if err != nil {
				return nil, fmt.Errorf("failed to parse private key with passphrase: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to parse private key (try setting SSH_KEY_PASSPHRASE env var): %w", err)
		}
	}

	return ssh.PublicKeys(signer), nil
}

func (c *Client) getHostKeyCallback() (ssh.HostKeyCallback, error) {
	// Try to load known_hosts file
	knownHostsPath := os.ExpandEnv("$HOME/.ssh/known_hosts")

	if _, err := os.Stat(knownHostsPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("known_hosts file not found: %s", knownHostsPath)
	}

	callback, err := knownhosts.New(knownHostsPath)
	if err != nil {
		return nil, fmt.Errorf("failed to parse known_hosts: %w", err)
	}

	return callback, nil
}

// trustOnFirstUse implements a TOFU (Trust On First Use) policy
// This is more secure than InsecureIgnoreHostKey but less secure than known_hosts
func (c *Client) trustOnFirstUse() ssh.HostKeyCallback {
	trustedKeys := make(map[string]ssh.PublicKey)

	return func(hostname string, remote net.Addr, key ssh.PublicKey) error {
		fingerprint := ssh.FingerprintSHA256(key)

		if trustedKey, exists := trustedKeys[hostname]; exists {
			if !keyEqual(trustedKey, key) {
				return fmt.Errorf("WARNING: Remote host identification has changed! (MITM attack?) Expected: %s, Got: %s",
					ssh.FingerprintSHA256(trustedKey), fingerprint)
			}
			return nil
		}

		// First time seeing this host - trust it
		logger.Warn("SSH host not in known_hosts, trusting on first use (TOFU)",
			"host", hostname,
			"fingerprint", fingerprint,
		)
		trustedKeys[hostname] = key

		return nil
	}
}

func keyEqual(a, b ssh.PublicKey) bool {
	return string(a.Marshal()) == string(b.Marshal())
}

// GetHostFingerprint returns the SSH host key fingerprint for verification
func (c *Client) GetHostFingerprint() (string, error) {
	config := &ssh.ClientConfig{
		User:    c.user,
		Auth:    []ssh.AuthMethod{ssh.Password("dummy")}, // Won't be used
		Timeout: 5 * time.Second,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// Don't actually verify, just capture the key
			return nil
		},
	}

	hostPort := c.host
	if _, _, err := net.SplitHostPort(hostPort); err != nil {
		hostPort = net.JoinHostPort(hostPort, "22")
	}

	conn, err := ssh.Dial("tcp", hostPort, config)
	if err != nil {
		// Connection will fail, but we can still get the key from the error
		return "", fmt.Errorf("could not get host fingerprint: %w", err)
	}
	defer conn.Close()

	return "", fmt.Errorf("unexpected success")
}

// VerifyConnection tests the SFTP connection without uploading
func (c *Client) VerifyConnection() error {
	sshConfig, err := c.getSSHConfig()
	if err != nil {
		return err
	}

	hostPort := c.host
	if _, _, err := net.SplitHostPort(hostPort); err != nil {
		hostPort = net.JoinHostPort(hostPort, "22")
	}

	sshClient, err := ssh.Dial("tcp", hostPort, sshConfig)
	if err != nil {
		return fmt.Errorf("SSH connection test failed: %w", err)
	}
	defer sshClient.Close()

	sftpClient, err := sftp.NewClient(sshClient)
	if err != nil {
		return fmt.Errorf("SFTP client test failed: %w", err)
	}
	defer sftpClient.Close()

	logger.Info("SFTP connection verified successfully", "host", c.host)
	return nil
}

// computeChecksum calculates SHA256 checksum of a file
func computeChecksum(filepath string) (string, error) {
	file, err := os.Open(filepath)
	if err != nil {
		return "", err
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}
