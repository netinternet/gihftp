package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"gih-ftp/internal/config"
	ftpclient "gih-ftp/internal/ftp"
	"gih-ftp/internal/gihapi"
	"gih-ftp/internal/logger"
	"gih-ftp/internal/merger"
	sftpclient "gih-ftp/internal/sftp"
)

const (
	ExitSuccess      = 0
	ExitConfigError  = 1
	ExitFetchError   = 2
	ExitMergeError   = 3
	ExitUploadError  = 4
	ExitPartialError = 5
)

func main() {
	// Load configuration (from flags or config file)
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Configuration error: %v\n", err)
		fmt.Fprintf(os.Stderr, "\nUsage examples:\n")
		fmt.Fprintf(os.Stderr, "  Using config file:\n")
		fmt.Fprintf(os.Stderr, "    %s --config=/etc/gihftp.conf\n\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "  Using command-line flags:\n")
		fmt.Fprintf(os.Stderr, "    %s --gih-servers=dns1.example.com,dns2.example.com \\\n", os.Args[0])
		fmt.Fprintf(os.Stderr, "      --gih-api-port=2035 \\\n")
		fmt.Fprintf(os.Stderr, "      --ftp-host=127.0.0.1 \\\n")
		fmt.Fprintf(os.Stderr, "      --ftp-user=upload_user \\\n")
		fmt.Fprintf(os.Stderr, "      --ftp-log-dir=/var/log/uploads/ \\\n")
		fmt.Fprintf(os.Stderr, "      --ssh-key=/root/.ssh/id_rsa \\\n")
		fmt.Fprintf(os.Stderr, "      --work-dir=/tmp/logmerger\n\n")
		fmt.Fprintf(os.Stderr, "  Password can be provided via FTP_PASSWORD environment variable\n")
		os.Exit(ExitConfigError)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Configuration validation failed: %v\n", err)
		os.Exit(ExitConfigError)
	}

	// Initialize logger
	logger.Init(cfg.LogLevel)

	logger.Info("GIH-FTP Service Starting",
		"version", "2.0.0",
		"gih_servers", fmt.Sprintf("%v", cfg.GIHServers),
		"ftp_host", cfg.FTPHost,
		"work_dir", cfg.WorkDir,
	)

	// Run main process
	exitCode := run(cfg)

	if exitCode == ExitSuccess {
		logger.Info("GIH-FTP Service completed successfully")
	} else {
		logger.Error("GIH-FTP Service completed with errors", "exit_code", exitCode)
	}

	os.Exit(exitCode)
}

func run(cfg *config.Config) int {
	startTime := time.Now()

	// Create GIH API client
	apiClient := gihapi.NewClient(cfg.InsecureSkipVerify)
	defer apiClient.Close()

	startDate, endDate := getLastWeekRange()
	logger.Info("Fetching logs for last week",
		"start_date", startDate,
		"end_date", endDate,
	)

	m := merger.New(cfg.WorkDir)

	successCount := 0
	failureCount := 0

	for _, host := range cfg.GIHServers {
		err := fetchFromServerWeekly(apiClient, m, host, cfg.GIHAPIPort, startDate, endDate)
		if err != nil {
			logger.Error("Weekly fetch failed",
				"host", host,
				"error", err)
			failureCount++
		} else {
			successCount++
		}
	}

	if successCount == 0 {
		logger.Error("No successful fetch from any server")
		return ExitFetchError
	}

	stats := m.GetStats()
	logger.Info("Weekly merge statistics",
		"week_start", startDate,
		"week_end", endDate,
		"unique_domains", stats["unique_domains"],
		"total_requests", stats["total_requests"],
		"top_domain", stats["top_domain"],
		"top_domain_hits", stats["top_domain_hits"],
	)

	uploadDate := time.Now().Format("20060102")
	filename := fmt.Sprintf("NETINTERNET-GIH-DNS_250k-%s.txt", uploadDate)
	outputPath, err := m.SaveToFile(filename)
	if err != nil {
		logger.Error("Failed to save weekly merged file", "error", err)
		return ExitMergeError
	}

	logger.Info("Weekly merged file created",
		"file", outputPath,
		"week_start", startDate,
		"week_end", endDate,
	)

	if err := uploadToFTP(cfg, outputPath); err != nil {
		logger.Error("FTP upload failed",
			"file", outputPath,
			"error", err)
		return ExitUploadError
	}

	if cfg.CleanupAfter {
		if err := os.Remove(outputPath); err != nil {
			logger.Warn("Failed to remove temp file", "file", outputPath)
		} else {
			logger.Info("Temp file removed", "file", outputPath)
		}
	}

	duration := time.Since(startTime)
	logger.Info("Weekly processing completed",
		"duration_seconds", duration.Seconds(),
		"servers_success", successCount,
		"servers_failed", failureCount,
	)

	if failureCount > 0 {
		return ExitPartialError
	}

	return ExitSuccess
}

func fetchFromServerWeekly(apiClient *gihapi.Client, m *merger.Merger, host, port, startDate, endDate string) error {
	logger.Info("Fetching weekly logs from server",
		"host", host,
		"start_date", startDate,
		"end_date", endDate,
	)

	files, err := apiClient.FetchLogFiles(host, port, startDate, endDate)
	if err != nil {
		return fmt.Errorf("failed to fetch weekly log list: %w", err)
	}

	if len(files) == 0 {
		logger.Warn("No weekly log files found",
			"host", host,
			"start_date", startDate,
			"end_date", endDate)
		return nil
	}

	logger.Info("Found log files for week",
		"host", host,
		"file_count", len(files),
	)

	for _, file := range files {
		logger.Debug("Downloading log file",
			"host", host,
			"filename", file.Filename,
		)

		content, err := apiClient.DownloadFile(host, port, file.DownloadURL)
		if err != nil {
			logger.Error("Failed to download log",
				"host", host,
				"filename", file.Filename,
				"error", err)
			continue
		}

		if err := m.AddContent(content); err != nil {
			logger.Error("Failed to merge log",
				"host", host,
				"filename", file.Filename,
				"error", err)
			continue
		}
	}

	return nil
}

func uploadToSFTP(cfg *config.Config, localPath string) error {
	logger.Info("Uploading to SFTP server")

	// Create SFTP client
	sftpClient := sftpclient.NewClient(
		cfg.FTPHost,
		cfg.FTPUser,
		cfg.FTPPassword,
		cfg.SSHKeyPath,
		cfg.InsecureSkipVerify,
	)

	// Build remote path
	filename := filepath.Base(localPath)
	remotePath := filepath.Join(cfg.FTPLogDir, filename)

	// Upload file
	if err := sftpClient.Upload(localPath, remotePath); err != nil {
		return fmt.Errorf("SFTP upload failed: %w", err)
	}

	logger.Info("SFTP upload successful",
		"local_path", localPath,
		"remote_path", remotePath,
	)

	return nil
}

func uploadToFTP(cfg *config.Config, localPath string) error {
	logger.Info("Uploading to FTP server")

	ftpClient := ftpclient.NewClient(
		normalizeFTPHost(cfg.FTPHost),
		cfg.FTPUser,
		cfg.FTPPassword,
	)

	filename := filepath.Base(localPath)
	remotePath := filepath.Join(cfg.FTPLogDir, filename)

	if err := ftpClient.Upload(localPath, remotePath); err != nil {
		return fmt.Errorf("FTP upload failed: %w", err)
	}

	logger.Info("FTP upload successful",
		"local_path", localPath,
		"remote_path", remotePath,
	)

	return nil
}

func normalizeFTPHost(host string) string {
	if !strings.Contains(host, ":") {
		return host + ":21"
	}
	return host
}

func getLastWeekRange() (startDate, endDate string) {
	now := time.Now()

	yesterday := now.AddDate(0, 0, -1)
	endDate = yesterday.Format("20060102")

	weekAgo := now.AddDate(0, 0, -7)
	startDate = weekAgo.Format("20060102")

	return startDate, endDate
}
