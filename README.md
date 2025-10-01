# Log Analyzer

This project is a **log analyzer tool written in Go**.  
It monitors a log file in real-time, calculates checksums to detect changes, parses JSON log entries, removes duplicates, filters events by type, and writes categorized output to separate files. It also maintains an `applicationlogs.log` file to store internal logs and errors.

---

## ✨ Features
- **Configuration via JSON** (`-config` flag).
- **Periodic monitoring** of log files (using `monitorPeriod`).
- **Checksum-based change detection** (avoids reprocessing unchanged logs).
- **Duplicate detection** (unique key: `timeMs-streamId-eventType`).
- **Categorization** of log entries into different output files.
- **Application logs** for tracking processing steps and errors.

---

## 📂 Project Structure
