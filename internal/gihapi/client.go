package gihapi

import (
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"gih-ftp/internal/logger"
)

type LogFile struct {
	Date        string `json:"date"`
	Filename    string `json:"filename"`
	DownloadURL string `json:"download_url"`
	Size        int    `json:"size"`
}

type APIResponse struct {
	Status  bool   `json:"status"`
	Message string `json:"message"`
	Data    struct {
		Count     int       `json:"count"`
		StartDate string    `json:"start_date"`
		EndDate   string    `json:"end_date"`
		Files     []LogFile `json:"files"`
	} `json:"data"`
}

type Client struct {
	httpClient         *http.Client
	insecureSkipVerify bool
}

func NewClient(insecureSkipVerify bool) *Client {
	tlsConfig := &tls.Config{
		InsecureSkipVerify: insecureSkipVerify,
	}

	// Try to load system CA certificates if not skipping verification
	if !insecureSkipVerify {
		if certPool, err := x509.SystemCertPool(); err == nil {
			tlsConfig.RootCAs = certPool
		} else {
			logger.Warn("Failed to load system CA certificates, using default pool", "error", err)
		}
	}

	if insecureSkipVerify {
		logger.Warn("TLS certificate verification is DISABLED - this is insecure!")
	}

	transport := &http.Transport{
		TLSClientConfig:     tlsConfig,
		MaxIdleConns:        10,
		IdleConnTimeout:     30 * time.Second,
		DisableCompression:  false,
		DisableKeepAlives:   false,
		MaxIdleConnsPerHost: 2,
	}

	return &Client{
		httpClient: &http.Client{
			Timeout:   30 * time.Second,
			Transport: transport,
		},
		insecureSkipVerify: insecureSkipVerify,
	}
}

func (c *Client) FetchLogFiles(host, port, startDate, endDate string) ([]LogFile, error) {
	apiURL := fmt.Sprintf("https://%s:%s/api/dns/query/logs?start=%s&end=%s",
		host, port, startDate, endDate)

	logger.Debug("Fetching log files", "url", apiURL)

	respBytes, err := c.httpGet(apiURL)
	if err != nil {
		return nil, fmt.Errorf("API request failed: %w", err)
	}

	var apiResp APIResponse
	if err := json.Unmarshal(respBytes, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w", err)
	}

	if !apiResp.Status {
		return nil, fmt.Errorf("API returned error: %s", apiResp.Message)
	}

	logger.Info("Fetched log files",
		"host", host,
		"count", apiResp.Data.Count,
		"start_date", apiResp.Data.StartDate,
		"end_date", apiResp.Data.EndDate,
	)

	return apiResp.Data.Files, nil
}

func (c *Client) DownloadFile(host, port, downloadURL string) ([]byte, error) {
	fullURL := fmt.Sprintf("https://%s:%s%s", host, port, downloadURL)

	logger.Debug("Downloading file", "url", fullURL)

	content, err := c.httpGet(fullURL)
	if err != nil {
		return nil, fmt.Errorf("download failed: %w", err)
	}

	logger.Debug("Downloaded file", "size_bytes", len(content))

	return content, nil
}

func (c *Client) httpGet(url string) ([]byte, error) {
	resp, err := c.httpClient.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(body))
	}

	return io.ReadAll(resp.Body)
}

func GetLastWeekDates() (startDate, endDate string) {
	// End date is yesterday (most recent)
	yesterday := time.Now().AddDate(0, 0, -1)
	endDate = yesterday.Format("20060102")

	// Start date is 7 days before yesterday
	weekAgo := yesterday.AddDate(0, 0, -6)
	startDate = weekAgo.Format("20060102")

	return startDate, endDate
}

func GetDateRange(daysBack int) (startDate, endDate string) {
	if daysBack <= 0 {
		daysBack = 7
	}

	yesterday := time.Now().AddDate(0, 0, -1)
	endDate = yesterday.Format("20060102")

	startDay := yesterday.AddDate(0, 0, -(daysBack - 1))
	startDate = startDay.Format("20060102")

	return startDate, endDate
}

func (c *Client) Close() {
	c.httpClient.CloseIdleConnections()
}

// EnableDebugLogging enables HTTP request/response debugging
func (c *Client) EnableDebugLogging() {
	if file, err := os.OpenFile("/tmp/gihftp-http-debug.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0600); err == nil {
		transport := c.httpClient.Transport.(*http.Transport)
		transport.Proxy = http.ProxyFromEnvironment
		// Note: For full request/response logging, consider using httputil.DumpRequest/DumpResponse
		logger.Info("HTTP debug logging enabled", "logfile", file.Name())
	}
}
