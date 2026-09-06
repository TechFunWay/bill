package logger

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestLoggerWritesDatePartitionedFilesWithoutStdout(t *testing.T) {
	SetRetentionFunc(nil)
	logDir := t.TempDir()
	if err := Init(logDir, 30); err != nil {
		t.Fatalf("init logger: %v", err)
	}

	stdout := os.Stdout
	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create stdout pipe: %v", err)
	}
	os.Stdout = writer
	Info("application started")
	Error("request failed")
	_ = writer.Close()
	os.Stdout = stdout
	output, _ := io.ReadAll(reader)
	_ = reader.Close()
	Close()

	if len(output) != 0 {
		t.Fatalf("logger wrote to stdout: %q", output)
	}

	today := time.Now()
	date := today.Format("20060102")
	month := today.Format("200601")
	for _, level := range []string{"info", "error"} {
		path := filepath.Join(logDir, month, level+"_"+date+".log")
		content, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s log: %v", level, err)
		}
		if !strings.Contains(string(content), "["+strings.ToUpper(level)+"]") {
			t.Fatalf("%s log has unexpected content: %q", level, content)
		}
	}
}

func TestCleanOldLogsRemovesFilesOutsideRetentionWindow(t *testing.T) {
	logDir := t.TempDir()
	today := time.Now()
	oldDate := today.AddDate(0, 0, -31)
	recentDate := today.AddDate(0, 0, -2)

	oldPath := writeTestLog(t, logDir, oldDate, "info")
	recentPath := writeTestLog(t, logDir, recentDate, "error")
	cleanOldLogs(logDir, 30)

	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("old log still exists: %v", err)
	}
	if _, err := os.Stat(recentPath); err != nil {
		t.Fatalf("recent log was removed: %v", err)
	}
}

func writeTestLog(t *testing.T, logDir string, date time.Time, level string) string {
	t.Helper()
	monthDir := filepath.Join(logDir, date.Format("200601"))
	if err := os.MkdirAll(monthDir, 0755); err != nil {
		t.Fatalf("create month directory: %v", err)
	}
	path := filepath.Join(monthDir, level+"_"+date.Format("20060102")+".log")
	if err := os.WriteFile(path, []byte("test\n"), 0644); err != nil {
		t.Fatalf("write test log: %v", err)
	}
	return path
}
