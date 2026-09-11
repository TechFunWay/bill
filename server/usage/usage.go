// Package usage implements optional anonymous usage statistics plus the
// donate-support event counter, mirroring the lottery app's behaviour:
// a device-level heartbeat is reported once per day at a random time, and
// each "已支持" tap sends one donate_support event. Nothing user-specific
// (username, bills, IP) is ever sent.
package usage

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"math/rand"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"smallgo/server/config"
	"smallgo/server/version"
)

// StatsService reports anonymous device statistics.
type StatsService struct {
	apiURL     string
	version    string
	deviceID   string
	deviceType string
	os         string
	arch       string
	enabled    bool
	stopChan   chan struct{}
	once       sync.Once
	mu         sync.RWMutex
}

type statsRequest struct {
	AppName    string `json:"app_name,omitempty"`
	Version    string `json:"version"`
	DeviceID   string `json:"device_id,omitempty"`
	DeviceType string `json:"device_type,omitempty"`
	OS         string `json:"os,omitempty"`
	Arch       string `json:"arch,omitempty"`
	Event      string `json:"event,omitempty"` // empty for heartbeat, donate_support for a support tap
}

const defaultStatsAPI = "https://techfunway.wycto.cn/api/apps.online/refresh"

var svc = &StatsService{
	enabled:  os.Getenv("DISABLE_STATS") != "true",
	stopChan: make(chan struct{}),
}

// Init prepares the stats service: picks up the endpoint (STATS_ENDPOINT
// overrides the default), derives the stable device ID and stores it under
// dataDir. Failures are silent — statistics must never block startup.
func Init(dataDir string) {
	apiURL := os.Getenv("STATS_ENDPOINT")
	if apiURL == "" {
		apiURL = defaultStatsAPI
	}
	svc.mu.Lock()
	svc.apiURL = apiURL
	svc.version = version.Version
	svc.os = runtime.GOOS
	svc.arch = runtime.GOARCH
	svc.mu.Unlock()

	if config.C.FnOSApp {
		svc.SetDeviceType("fnos")
	} else {
		svc.SetDeviceType("docker")
	}

	if id, err := deviceID(dataDir); err == nil && id != "" {
		svc.mu.Lock()
		svc.deviceID = id
		svc.mu.Unlock()
	}
}

// Start launches the daily heartbeat goroutine. No-op when disabled.
func Start() {
	svc.mu.RLock()
	ok := svc.apiURL != "" && svc.enabled
	svc.mu.RUnlock()
	if !ok {
		return
	}
	go svc.send("")
	go svc.dailyLoop()
}

// Stop terminates the heartbeat loop.
func Stop() {
	svc.once.Do(func() { close(svc.stopChan) })
}

// SendEvent reports a single event synchronously and reports whether the
// endpoint accepted it (non-2xx counts as failure).
func SendEvent(event string) bool {
	svc.mu.RLock()
	ok := svc.apiURL != "" && svc.enabled
	svc.mu.RUnlock()
	if !ok {
		return false
	}
	return svc.send(event)
}

// DeviceID returns the current anonymous device identifier.
func DeviceID() string {
	svc.mu.RLock()
	defer svc.mu.RUnlock()
	return svc.deviceID
}

func (s *StatsService) SetDeviceType(t string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deviceType = t
}

func (s *StatsService) send(event string) bool {
	s.mu.RLock()
	req := statsRequest{
		AppName:    version.AppName,
		Version:    s.version,
		DeviceID:   s.deviceID,
		DeviceType: s.deviceType,
		OS:         s.os,
		Arch:       s.arch,
		Event:      event,
	}
	apiURL := s.apiURL
	s.mu.RUnlock()

	body, err := json.Marshal(req)
	if err != nil {
		return false
	}
	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Post(apiURL, "application/json", bytes.NewReader(body))
	if err != nil {
		return false
	}
	_, _ = io.Copy(io.Discard, resp.Body)
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}

// dailyLoop sends one heartbeat per day at a random moment.
func (s *StatsService) dailyLoop() {
	for {
		now := time.Now()
		tomorrow := now.AddDate(0, 0, 1)
		midnight := time.Date(tomorrow.Year(), tomorrow.Month(), tomorrow.Day(), 0, 0, 0, 0, tomorrow.Location())
		nextRun := midnight.Add(time.Duration(rand.Intn(24)) * time.Hour)

		timer := time.NewTimer(time.Until(nextRun))
		select {
		case <-timer.C:
			go s.send("")
		case <-s.stopChan:
			timer.Stop()
			return
		}
	}
}

// deviceID returns the persisted anonymous device ID, creating it from
// machine fingerprints (hashed, never stored in plaintext) on first run.
func deviceID(dataDir string) (string, error) {
	file := filepath.Join(dataDir, "device_id.txt")
	if data, err := os.ReadFile(file); err == nil {
		if id := strings.TrimSpace(string(data)); id != "" {
			return id, nil
		}
	}

	var info strings.Builder
	fmt.Fprintf(&info, "OS:%s Arch:%s", runtime.GOOS, runtime.GOARCH)
	switch runtime.GOOS {
	case "darwin":
		if serial, err := macSerial(); err == nil {
			fmt.Fprintf(&info, " Serial:%s", serial)
		}
	case "linux":
		if mid, err := os.ReadFile("/etc/machine-id"); err == nil {
			fmt.Fprintf(&info, " MachineID:%s", strings.TrimSpace(string(mid)))
		}
		if uuid, err := os.ReadFile("/sys/class/dmi/id/product_uuid"); err == nil {
			fmt.Fprintf(&info, " ProductUUID:%s", strings.TrimSpace(string(uuid)))
		}
	case "windows":
		if guid, err := exec.Command("wmic", "csproduct", "get", "uuid").Output(); err == nil {
			lines := strings.Fields(string(guid))
			if len(lines) >= 2 {
				fmt.Fprintf(&info, " MachineGUID:%s", lines[1])
			}
		}
	}

	hash := md5.Sum([]byte(info.String()))
	id := hex.EncodeToString(hash[:])
	if err := os.WriteFile(file, []byte(id), 0644); err != nil && !os.IsExist(err) {
		// Persisting is best-effort; the hashed ID is still usable.
		return id, nil
	}
	return id, nil
}

func macSerial() (string, error) {
	out, err := exec.Command("ioreg", "-l").Output()
	if err != nil {
		return "", err
	}
	for _, line := range strings.Split(string(out), "\n") {
		if strings.Contains(line, "IOPlatformSerialNumber") {
			parts := strings.Split(line, "\"")
			if len(parts) >= 2 {
				return parts[1], nil
			}
		}
	}
	return "", fmt.Errorf("serial number not found")
}
