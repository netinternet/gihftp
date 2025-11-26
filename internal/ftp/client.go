package ftpclient

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gih-ftp/internal/logger"

	"github.com/jlaffaye/ftp"
)

type Client struct {
	host     string
	user     string
	password string
}

func NewClient(host, user, password string) *Client {
	return &Client{
		host:     host,
		user:     user,
		password: password,
	}
}

func (c *Client) Upload(localPath, remotePath string) error {
	logger.Info("Starting FTP upload",
		"local_file", localPath,
		"remote_path", remotePath,
		"host", c.host,
	)

	conn, err := ftp.Dial(c.host,
		ftp.DialWithTimeout(10*time.Second),
	)
	if err != nil {
		return fmt.Errorf("FTP connect failed: %w", err)
	}
	defer conn.Quit()

	if err := conn.Login(c.user, c.password); err != nil {
		return fmt.Errorf("FTP login failed: %w", err)
	}

	file, err := os.Open(localPath)
	if err != nil {
		return fmt.Errorf("failed to open local file: %w", err)
	}
	defer file.Close()

	remoteDir := remotePath[:len(remotePath)-len(filepath.Base(remotePath))]
	conn.MakeDir(remoteDir)

	if err := conn.Stor(remotePath, file); err != nil {
		return fmt.Errorf("FTP upload failed: %w", err)
	}

	logger.Info("FTP upload completed successfully",
		"remote_path", remotePath,
	)

	return nil
}
