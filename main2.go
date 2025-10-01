package main

import (
	"bufio"
	"crypto/md5"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"os"
	path/filepath"
	"regexp"
	"sort"
	"time"
)

type WXAnalyticsLogRecordEntry struct {
	TimeMS             int64  `json:"timeMs"`
	StreamID           string `json:"streamId"`
	TotalBytesReceived int64  `json:"totalByteReceived"`
	BytesTransferred   int64  `json:"byteTransferred"`
	DurationMS         int64  `json:"durationMs"`
	Width              int    `json:"width"`
	Height             int    `json:"height"`
           


ype Config struct {
	LogFilePath   string                 `json:"logFilePath"`
	OutputFiles   map[string]string      `json:"outputFiles"`
	EventFilters  map[string]interface{} `json:"eventFilters"`
	BatchInterval string                 `json:"batchInterval"`
	MonitorPeriod string                 `json:"monitorPeriod"`
}


const applicationLogsFile = "applicationlogs.log"

var lastChecksum string
var lastReadPosition int64 = 0
var seenEntries = make(map[string]bool)

func main() {

	configFilePath := flag.String("config", "", "Path to the JSON configuration file")
	flag.Parse()
	if *configFilePath == "" {
		fmt.Println("Error: Configuration file path must be specified using the -config flag.")
		os.Exit(1)
	}

	
	config, err := loadConfig(*configFilePath)
	if err != nil {
		fmt.Printf("Error loading configuration: %v\n", err)
		return
	}

	
	logMessage(fmt.Sprintf("Started monitoring log file: %s", config.LogFilePath))

	
	file, err := os.Open(config.LogFilePath)
	if err != nil {
		logMessage(fmt.Sprintf("Error opening file: %v", err))
		return
	}
	defer file.Close()
	monitorPeriod, err := time.ParseDuration(config.MonitorPeriod)
	if err != nil {
		logMessage(fmt.Sprintf("Invalid monitor period: %v", err))
		return
	}
	ticker := time.NewTicker(monitorPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			processLogFile(file, config)
		}
	}
}

func loadConfig(filePath string) (*Config, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return nil, fmt.Errorf("error opening config file: %v", err)
	}
	defer file.Close()

	var config Config
	decoder := json.NewDecoder(file)
	err = decoder.Decode(&config)
	if err != nil {
		return nil, fmt.Errorf("error decoding config file: %v", err)
	}
	return &config, nil
}


func calculateChecksum(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", fmt.Errorf("error opening log file for checksum calculation: %v", err)
	}
	defer file.Close()

	hash := md5.New()
	_, err = io.Copy(hash, file)
	if err != nil {
		return "", fmt.Errorf("error reading log file for checksum: %v", err)
	}

	return fmt.Sprintf("%x", hash.Sum(nil)), nil
}

func processLogFile(file *os.File, config *Config) {
	checksum, err := calculateChecksum(config.LogFilePath)
	if err != nil {
		logMessage(fmt.Sprintf("Error calculating checksum: %v", err))
		return
	}

	if checksum == lastChecksum {
		logMessage("No changes detected in the log file, skipping processing.")
		return
	}
	lastChecksum = checksum

	_, err = file.Seek(lastReadPosition, io.SeekStart)
	if err != nil {
		logMessage(fmt.Sprintf("Error seeking to last read position: %v", err))
		return
	}

	entries, err := parse(file)
	if err != nil {
		logMessage(fmt.Sprintf("Error parsing log file: %v", err))
		return
	}

	sort.Slice(entries, func(i, j int) bool {
		return entries[i].TimeMS < entries[j].TimeMS
	})

	err = categorize(entries, config)
	if err != nil {
		logMessage(fmt.Sprintf("Error categorizing log entries: %v", err))
	}


	seenEntries = make(map[string]bool)

	lastReadPosition, err = file.Seek(0, io.SeekCurrent)
	if err != nil {
		logMessage(fmt.Sprintf("Error updating last read position: %v", err))
	}
}


func parse(file *os.File) ([]WXAnalyticsLogRecordEntry, error) {
	var entries []WXAnalyticsLogRecordEntry
	scanner := bufio.NewScanner(file)
	jsonRegex := regexp.MustCompile(`\{.*\}`)

	for scanner.Scan() {
		line := scanner.Text()
		matches := jsonRegex.FindString(line)
		if matches == "" {
			continue
		}
		var entry WXAnalyticsLogRecordEntry
		err := json.Unmarshal([]byte(matches), &entry)
		if err != nil {
			fmt.Printf("Error parsing line: %s, error: %v\n", line, err)
			continue
		}

		key := fmt.Sprintf("%d-%s-%s", entry.TimeMS, entry.StreamID, entry.EventType)
		if seenEntries[key] {
			logMessage(fmt.Sprintf("Duplicate entry detected and skipped: %v", entry))
			continue
		}
		seenEntries[key] = true
		entries = append(entries, entry)
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading file: %v", err)
	}
	return entries, nil
}


func categorize(entries []WXAnalyticsLogRecordEntry, config *Config) error {
	for eventType, filePath := range config.OutputFiles {
		filter, err := createFilter(eventType, config.EventFilters)
		if err != nil {
			logMessage(fmt.Sprintf("Error creating filter for %s: %v", eventType, err))
			continue
		}

		var filteredEntries []WXAnalyticsLogRecordEntry
		for _, entry := range entries {
			if filter(entry) {
				filteredEntries = append(filteredEntries, entry)
			}
		}

		if len(filteredEntries) > 0 {
			err := writeToFile(filePath, filteredEntries)
			if err != nil {
				logMessage(fmt.Sprintf("Error writing to file %s: %v", filePath, err))
			}
		}
	}
	return nil
}

func writeToFile(filePath string, entries []WXAnalyticsLogRecordEntry) error {
	dir := filepath.Dir(filePath)
	if err := os.MkdirAll(dir, os.ModePerm); err != nil {
		return fmt.Errorf("failed to create directories for file %s: %w", filePath, err)
	}

	file, err := os.OpenFile(filePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return fmt.Errorf("error opening file %s: %w", filePath, err)
	}
	defer file.Close()

	for _, entry := range entries {
		output, err := json.Marshal(entry)
		if err != nil {
			return fmt.Errorf("error marshalling entry: %w", err)
		}
		_, err = file.WriteString(string(output) + "\n")
		if err != nil {
			return fmt.Errorf("error writing to file %s: %w", filePath, err)
		}
	}
	return nil
}


func createFilter(eventType string, filters map[string]interface{}) (func(WXAnalyticsLogRecordEntry) bool, error) {
	filter, ok := filters[eventType]
	if !ok {
		return nil, fmt.Errorf("no filter found for event type: %s", eventType)
	}
	switch v := filter.(type) {
	case string:
		return func(e WXAnalyticsLogRecordEntry) bool { return e.EventType == v }, nil
	case []interface{}:
		allowed := make(map[string]bool)
		for _, ev := range v {
			allowed[ev.(string)] = true
		}
		return func(e WXAnalyticsLogRecordEntry) bool { return allowed[e.EventType] }, nil
	default:
		return nil, fmt.Errorf("unsupported filter type for event type: %s", eventType)
	}
}


func logMessage(message string) {
	timestamp := time.Now().Format(time.RFC3339)
	logEntry := fmt.Sprintf("[%s] %s\n", timestamp, message)

	file, err := os.OpenFile(applicationLogsFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Printf("Error writing to application logs: %v\n", err)
		return
	}
	defer file.Close()

	_, err = file.WriteString(logEntry)
	if err != nil {
		fmt.Printf("Error writing to application logs: %v\n", err)
	}
}
