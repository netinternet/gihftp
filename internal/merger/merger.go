package merger

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"gih-ftp/internal/logger"
)

type DomainStats struct {
	Domain string
	Count  int
}

type Merger struct {
	data    map[string]int
	workDir string
}

func New(workDir string) *Merger {
	return &Merger{
		data:    make(map[string]int),
		workDir: workDir,
	}
}

func (m *Merger) AddContent(content []byte) error {
	scanner := bufio.NewScanner(strings.NewReader(string(content)))
	linesProcessed := 0
	linesSkipped := 0

	for scanner.Scan() {
		line := scanner.Text()

		if line == "" {
			continue
		}

		parts := strings.Split(line, "|")
		if len(parts) != 2 {
			linesSkipped++
			logger.Debug("Skipping invalid line", "line", line)
			continue
		}

		domain := strings.TrimSpace(parts[0])
		countStr := strings.TrimSpace(parts[1])

		count, err := strconv.Atoi(countStr)
		if err != nil {
			linesSkipped++
			logger.Debug("Skipping line with invalid count", "line", line, "error", err)
			continue
		}

		m.data[domain] += count
		linesProcessed++
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading content: %w", err)
	}

	logger.Debug("Processed content",
		"lines_processed", linesProcessed,
		"lines_skipped", linesSkipped,
		"unique_domains", len(m.data),
	)

	return nil
}

func (m *Merger) GetSortedStats() []DomainStats {
	stats := make([]DomainStats, 0, len(m.data))

	for domain, count := range m.data {
		stats = append(stats, DomainStats{
			Domain: domain,
			Count:  count,
		})
	}

	// Sort by count (descending)
	sort.Slice(stats, func(i, j int) bool {
		return stats[i].Count > stats[j].Count
	})

	return stats
}

func (m *Merger) SaveToFile(filename string) (string, error) {
	// Ensure work directory exists
	if m.workDir != "" && m.workDir != "." {
		if err := os.MkdirAll(m.workDir, 0755); err != nil {
			return "", fmt.Errorf("failed to create work directory: %w", err)
		}
	}

	// Generate filename with timestamp if not provided
	if filename == "" {
		filename = fmt.Sprintf("MERGED_WEEK_%s.log", time.Now().Format("20060102_150405"))
	}

	// Build full path
	fullPath := filepath.Join(m.workDir, filename)

	// Create file
	file, err := os.Create(fullPath)
	if err != nil {
		return "", fmt.Errorf("failed to create file: %w", err)
	}
	defer file.Close()

	// Get sorted stats
	stats := m.GetSortedStats()

	// Write to file
	for _, stat := range stats {
		if _, err := fmt.Fprintf(file, "%s|%d\n", stat.Domain, stat.Count); err != nil {
			return "", fmt.Errorf("failed to write to file: %w", err)
		}
	}

	logger.Info("Merge completed",
		"file", fullPath,
		"unique_domains", len(stats),
		"total_requests", m.getTotalRequests(stats),
	)

	return fullPath, nil
}

func (m *Merger) getTotalRequests(stats []DomainStats) int {
	total := 0
	for _, stat := range stats {
		total += stat.Count
	}
	return total
}

func (m *Merger) GetStats() map[string]interface{} {
	stats := m.GetSortedStats()
	return map[string]interface{}{
		"unique_domains":  len(stats),
		"total_requests":  m.getTotalRequests(stats),
		"top_domain":      m.getTopDomain(stats),
		"top_domain_hits": m.getTopDomainHits(stats),
	}
}

func (m *Merger) getTopDomain(stats []DomainStats) string {
	if len(stats) > 0 {
		return stats[0].Domain
	}
	return "N/A"
}

func (m *Merger) getTopDomainHits(stats []DomainStats) int {
	if len(stats) > 0 {
		return stats[0].Count
	}
	return 0
}

func (m *Merger) Clear() {
	m.data = make(map[string]int)
}

func (m *Merger) GetDomainCount() int {
	return len(m.data)
}
