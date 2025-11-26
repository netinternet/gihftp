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

	// Get date range for log files
	startDate, endDate := gihapi.GetLastWeekDates()
	logger.Info("Fetching logs for date range",
		"start_date", startDate,
		"end_date", endDate,
	)

	days := getLast7Days()

	totalFailures := 0

	for _, day := range days {

		dateStr := day.Format("20060102")

		logger.Info("Processing day", "date", dateStr)

		m := merger.New(cfg.WorkDir)

		daySuccess := 0
		dayFailure := 0

		for _, host := range cfg.GIHServers {

			err := fetchFromServerDaily(apiClient, m, host, cfg.GIHAPIPort, dateStr)
			if err != nil {
				logger.Error("Daily fetch failed",
					"host", host,
					"date", dateStr,
					"error", err)
				dayFailure++
			} else {
				daySuccess++
			}
		}

		if daySuccess == 0 {
			logger.Warn("No successful fetch for this day", "date", dateStr)
			totalFailures++
			continue
		}

		stats := m.GetStats()

		logger.Info("Daily merge statistics",
			"date", dateStr,
			"unique_domains", stats["unique_domains"],
			"total_requests", stats["total_requests"],
			"top_domain", stats["top_domain"],
			"top_domain_hits", stats["top_domain_hits"],
		)

		filename := fmt.Sprintf("NETINTERNET-GIH-DNS_250k-%s.txt", dateStr)
		outputPath, err := m.SaveToFile(filename)
		if err != nil {
			logger.Error("Failed to save daily merged file",
				"date", dateStr,
				"error", err)
			totalFailures++
			continue
		}

		logger.Info("Daily merged file created", "file", outputPath)

		if err := uploadToFTP(cfg, outputPath); err != nil {
			logger.Error("Daily FTP upload failed",
				"date", dateStr,
				"file", outputPath,
				"error", err)
			totalFailures++
			continue
		}

		if cfg.CleanupAfter {
			if err := os.Remove(outputPath); err != nil {
				logger.Warn("Failed to remove temp file", "file", outputPath)
			} else {
				logger.Info("Temp file removed", "file", outputPath)
			}
		}
	}

	duration := time.Since(startTime)
	logger.Info("Daily processing completed",
		"duration_seconds", duration.Seconds(),
	)

	if totalFailures > 0 {
		return ExitPartialError
	}

	return ExitSuccess
}

func fetchFromServerDaily(apiClient *gihapi.Client, m *merger.Merger, host, port, date string) error {

	logger.Info("Fetching daily logs from server",
		"host", host,
		"date", date,
	)

	files, err := apiClient.FetchLogFiles(host, port, date, date)
	if err != nil {
		return fmt.Errorf("failed to fetch daily log list: %w", err)
	}

	if len(files) == 0 {
		logger.Warn("No daily log files found",
			"host", host,
			"date", date)
		return nil
	}

	for _, file := range files {
		logger.Debug("Downloading daily log file",
			"host", host,
			"date", date,
			"filename", file.Filename,
		)

		content, err := apiClient.DownloadFile(host, port, file.DownloadURL)
		if err != nil {
			logger.Error("Failed to download daily log",
				"host", host,
				"date", date,
				"filename", file.Filename,
				"error", err)
			continue
		}

		if err := m.AddContent(content); err != nil {
			logger.Error("Failed to merge daily log",
				"host", host,
				"date", date,
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

func getLast7Days() []time.Time {
	days := []time.Time{}
	today := time.Now().AddDate(0, 0, -1)

	for i := 0; i < 7; i++ {
		days = append(days, today.AddDate(0, 0, -i))
	}

	return days
}
