package config

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"gopkg.in/ini.v1"
)

type Config struct {
	// GIH Server settings
	GIHServers []string
	GIHAPIPort string

	// FTP/SFTP settings
	FTPHost     string
	FTPUser     string
	FTPPassword string
	FTPLogDir   string

	// SSH settings
	SSHKeyPath string

	// Working directory
	WorkDir string

	// Logging
	LogLevel string

	// Cleanup
	CleanupAfter bool

	// Security
	InsecureSkipVerify bool
}

func Load() (*Config, error) {
	cfg := &Config{}

	// Define flags
	gihServers := flag.String("gih-servers", "", "Comma-separated list of GIH server addresses (e.g., dns1.example.com,dns2.example.com)")
	gihAPIPort := flag.String("gih-api-port", "2035", "GIH API port")
	ftpHost := flag.String("ftp-host", "", "FTP/SFTP server address")
	ftpUser := flag.String("ftp-user", "root", "FTP/SFTP username")
	ftpPassword := flag.String("ftp-password", "", "FTP/SFTP password (or use FTP_PASSWORD env var)")
	ftpLogDir := flag.String("ftp-log-dir", "/var/log/gih/", "Remote directory for log files")
	sshKeyPath := flag.String("ssh-key", "$HOME/.ssh/id_rsa", "Path to SSH private key")
	workDir := flag.String("work-dir", "", "Working directory for temporary files (default: current directory)")
	logLevel := flag.String("log-level", "info", "Log level (debug, info, error)")
	cleanupAfter := flag.Bool("cleanup", true, "Remove temporary files after upload")
	insecureSkipVerify := flag.Bool("insecure-skip-verify", false, "Skip TLS/SSH certificate verification (NOT RECOMMENDED)")
	configFile := flag.String("config", "", "Path to config file (optional, for backward compatibility)")

	flag.Parse()

	// Try to load from config file first (backward compatibility)
	var iniCfg *ini.File
	var err error

	if *configFile != "" {
		iniCfg, err = ini.Load(*configFile)
		if err != nil {
			return nil, fmt.Errorf("failed to load config file %s: %w", *configFile, err)
		}
	} else {
		// Try default locations if no flags provided
		defaultConfigs := []string{"/etc/gihftp.conf", "./gihftp.conf"}
		for _, path := range defaultConfigs {
			if _, err := os.Stat(path); err == nil {
				iniCfg, _ = ini.Load(path)
				break
			}
		}
	}

	// Priority: flags > env vars > config file > defaults

	// GIH Servers
	if *gihServers != "" {
		cfg.GIHServers = strings.Split(*gihServers, ",")
		for i := range cfg.GIHServers {
			cfg.GIHServers[i] = strings.TrimSpace(cfg.GIHServers[i])
		}
	} else if iniCfg != nil {
		dns1 := iniCfg.Section("").Key("gihdns1").String()
		dns2 := iniCfg.Section("").Key("gihdns2").String()
		if dns1 != "" {
			cfg.GIHServers = append(cfg.GIHServers, dns1)
		}
		if dns2 != "" {
			cfg.GIHServers = append(cfg.GIHServers, dns2)
		}
	}

	// GIH API Port
	if *gihAPIPort != "2035" {
		cfg.GIHAPIPort = *gihAPIPort
	} else if iniCfg != nil {
		port := iniCfg.Section("").Key("gihapiport").String()
		if port != "" {
			cfg.GIHAPIPort = port
		} else {
			cfg.GIHAPIPort = "2035"
		}
	} else {
		cfg.GIHAPIPort = "2035"
	}

	// FTP Host
	if *ftpHost != "" {
		cfg.FTPHost = *ftpHost
	} else if iniCfg != nil {
		cfg.FTPHost = iniCfg.Section("").Key("ftpserver").String()
	}

	// FTP User
	cfg.FTPUser = *ftpUser
	if cfg.FTPUser == "root" && iniCfg != nil {
		if user := iniCfg.Section("").Key("ftpuser").String(); user != "" {
			cfg.FTPUser = user
		}
	}

	// FTP Password (env var preferred for security)
	if envPass := os.Getenv("FTP_PASSWORD"); envPass != "" {
		cfg.FTPPassword = envPass
	} else if *ftpPassword != "" {
		cfg.FTPPassword = *ftpPassword
	} else if iniCfg != nil {
		cfg.FTPPassword = iniCfg.Section("").Key("ftppassword").String()
	}

	// FTP Log Directory
	if *ftpLogDir != "/var/log/gih/" {
		cfg.FTPLogDir = *ftpLogDir
	} else if iniCfg != nil {
		if dir := iniCfg.Section("").Key("ftplogdir").String(); dir != "" {
			cfg.FTPLogDir = dir
		} else {
			cfg.FTPLogDir = "/var/log/gih/"
		}
	} else {
		cfg.FTPLogDir = "/var/log/gih/"
	}

	// SSH Key Path
	if *sshKeyPath != "$HOME/.ssh/id_rsa" {
		cfg.SSHKeyPath = *sshKeyPath
	} else if iniCfg != nil {
		if key := iniCfg.Section("").Key("sshkey").String(); key != "" {
			cfg.SSHKeyPath = key
		} else {
			cfg.SSHKeyPath = "$HOME/.ssh/id_rsa"
		}
	} else {
		cfg.SSHKeyPath = "$HOME/.ssh/id_rsa"
	}

	// Working Directory
	if *workDir != "" {
		cfg.WorkDir = *workDir
	} else {
		cfg.WorkDir = "."
	}

	// Other settings
	cfg.LogLevel = *logLevel
	cfg.CleanupAfter = *cleanupAfter
	cfg.InsecureSkipVerify = *insecureSkipVerify

	// Validate required fields
	if len(cfg.GIHServers) == 0 {
		return nil, fmt.Errorf("no GIH servers specified (use --gih-servers flag or config file)")
	}

	if cfg.FTPHost == "" {
		return nil, fmt.Errorf("FTP host not specified (use --ftp-host flag or config file)")
	}

	return cfg, nil
}

func (c *Config) Validate() error {
	if len(c.GIHServers) == 0 {
		return fmt.Errorf("at least one GIH server is required")
	}

	if c.FTPHost == "" {
		return fmt.Errorf("FTP host is required")
	}

	if c.GIHAPIPort == "" {
		return fmt.Errorf("GIH API port is required")
	}

	// Validate log level
	validLevels := map[string]bool{"debug": true, "info": true, "error": true}
	if !validLevels[strings.ToLower(c.LogLevel)] {
		return fmt.Errorf("invalid log level: %s (must be debug, info, or error)", c.LogLevel)
	}

	return nil
}
