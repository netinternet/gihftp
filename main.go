package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"gih-ftp/internal/config"
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
		fmt.Fprintf(os.Stderr, "      --ftp-host=ftp.btk.gov.tr \\\n")
		fmt.Fprintf(os.Stderr, "      --ftp-user=username \\\n")
		fmt.Fprintf(os.Stderr, "      --ftp-log-dir=/var/log/gih/ \\\n")
		fmt.Fprintf(os.Stderr, "      --ssh-key=/root/.ssh/id_rsa \\\n")
		fmt.Fprintf(os.Stderr, "      --work-dir=/tmp/gihftp\n\n")
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

	// Get date range for log files
	startDate, endDate := gihapi.GetLastWeekDates()
	logger.Info("Fetching logs for date range",
		"start_date", startDate,
		"end_date", endDate,
	)

	// Create merger
	m := merger.New(cfg.WorkDir)

	// Fetch logs from all GIH servers
	successCount := 0
	failureCount := 0

	for _, host := range cfg.GIHServers {
		if err := fetchFromServer(apiClient, m, host, cfg.GIHAPIPort, startDate, endDate); err != nil {
			logger.Error("Failed to fetch from server",
				"host", host,
				"error", err,
			)
			failureCount++
		} else {
			successCount++
		}
	}

	if successCount == 0 {
		logger.Error("Failed to fetch logs from any server")
		return ExitFetchError
	}

	if failureCount > 0 {
		logger.Warn("Some servers failed",
			"success", successCount,
			"failures", failureCount,
		)
	}

	// Log merge statistics
	stats := m.GetStats()
	logger.Info("Merge statistics",
		"unique_domains", stats["unique_domains"],
		"total_requests", stats["total_requests"],
		"top_domain", stats["top_domain"],
		"top_domain_hits", stats["top_domain_hits"],
	)

	// Save merged data to file
	outputFilename := fmt.Sprintf("MERGED_WEEK_%s.log", time.Now().Format("20060102"))
	outputPath, err := m.SaveToFile(outputFilename)
	if err != nil {
		logger.Error("Failed to save merged file", "error", err)
		return ExitMergeError
	}

	logger.Info("Merged file created", "path", outputPath)

	// Upload to SFTP server
	if err := uploadToSFTP(cfg, outputPath); err != nil {
		logger.Error("Failed to upload to SFTP", "error", err)
		return ExitUploadError
	}

	// Cleanup temporary files if requested
	if cfg.CleanupAfter {
		if err := os.Remove(outputPath); err != nil {
			logger.Warn("Failed to remove temporary file", "file", outputPath, "error", err)
		} else {
			logger.Info("Temporary file removed", "file", outputPath)
		}
	}

	duration := time.Since(startTime)
	logger.Info("Processing completed",
		"duration_seconds", duration.Seconds(),
	)

	if failureCount > 0 {
		return ExitPartialError
	}

	return ExitSuccess
}

func fetchFromServer(apiClient *gihapi.Client, m *merger.Merger, host, port, startDate, endDate string) error {
	logger.Info("Fetching logs from server", "host", host)

	// Get list of log files
	files, err := apiClient.FetchLogFiles(host, port, startDate, endDate)
	if err != nil {
		return fmt.Errorf("failed to fetch log file list: %w", err)
	}

	if len(files) == 0 {
		logger.Warn("No log files found", "host", host)
		return nil
	}

	logger.Info("Found log files",
		"host", host,
		"count", len(files),
	)

	// Download and merge each file
	for _, file := range files {
		logger.Debug("Downloading log file",
			"host", host,
			"filename", file.Filename,
			"size_bytes", file.Size,
		)

		content, err := apiClient.DownloadFile(host, port, file.DownloadURL)
		if err != nil {
			logger.Error("Failed to download file",
				"host", host,
				"filename", file.Filename,
				"error", err,
			)
			continue
		}

		if err := m.AddContent(content); err != nil {
			logger.Error("Failed to merge file content",
				"host", host,
				"filename", file.Filename,
				"error", err,
			)
			continue
		}

		logger.Debug("File merged successfully",
			"host", host,
			"filename", file.Filename,
		)
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
