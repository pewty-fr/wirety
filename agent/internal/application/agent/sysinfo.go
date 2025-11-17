package agent

import (
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

// SystemInfo holds system information for heartbeat
type SystemInfo struct {
	Hostname         string
	SystemUptime     int64
	WireGuardUptime  int64
	ReportedEndpoint string
	PeerEndpoints    map[string]string // map of peer public key to endpoint
}

// CollectSystemInfo gathers system information for heartbeat
func CollectSystemInfo(wgInterface string) (*SystemInfo, error) {
	hostname, _ := os.Hostname()
	if hostname == "" {
		hostname = "unknown"
	}

	systemUptime := getSystemUptime()
	wgUptime := getWireGuardUptime(wgInterface)
	peerEndpoints := getWireGuardEndpoints(wgInterface)


	return &SystemInfo{
		Hostname:         hostname,
		SystemUptime:     systemUptime,
		WireGuardUptime:  wgUptime,
		PeerEndpoints:    peerEndpoints,
	}, nil
}

// getSystemUptime returns system uptime in seconds
func getSystemUptime() int64 {
	// Try to read /proc/uptime on Linux
	data, err := os.ReadFile("/proc/uptime")
	if err == nil {
		parts := strings.Fields(string(data))
		if len(parts) > 0 {
			uptime, err := strconv.ParseFloat(parts[0], 64)
			if err == nil {
				return int64(uptime)
			}
		}
	}

	// Fallback: try using uptime command
	cmd := exec.Command("uptime", "-s")
	output, err := cmd.Output()
	if err == nil {
		bootTime, err := time.Parse("2006-01-02 15:04:05", strings.TrimSpace(string(output)))
		if err == nil {
			return int64(time.Since(bootTime).Seconds())
		}
	}

	return 0
}

// getWireGuardUptime returns WireGuard interface uptime in seconds
func getWireGuardUptime(iface string) int64 {
	// Try to get interface creation time from /sys/class/net/<iface>/
	path := fmt.Sprintf("/sys/class/net/%s/operstate", iface)
	info, err := os.Stat(path)
	if err == nil {
		// Approximate: time since file was modified (interface brought up)
		return int64(time.Since(info.ModTime()).Seconds())
	}

	// Fallback: try using ip command
	cmd := exec.Command("ip", "-o", "link", "show", iface)
	output, err := cmd.Output()
	if err == nil && len(output) > 0 {
		// Interface exists, but we can't determine uptime precisely
		// Return a marker value
		return 1
	}

	return 0
}

// getWireGuardEndpoints returns a map of peer public keys to their endpoints
func getWireGuardEndpoints(iface string) map[string]string {
	// Get peer endpoints using wg show
	cmd := exec.Command("wg", "show", iface, "endpoints")
	output, err := cmd.Output()
	if err != nil {
		return make(map[string]string)
	}

	lines := strings.Split(strings.TrimSpace(string(output)), "\n")
	peerEndpoints := make(map[string]string)

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Parse line: <public-key>\t<endpoint>
		parts := strings.Fields(line)
		if len(parts) != 2 {
			continue
		}

		publicKey := parts[0]
		endpoint := parts[1]

		// Skip (none) endpoints
		if endpoint == "(none)" {
			continue
		}

		peerEndpoints[publicKey] = endpoint
	}

	return peerEndpoints
}
