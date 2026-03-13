package main

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"syscall"
	"time"
)

const (
	defaultPublicChatBaseURL = "https://chat.gimble.dev"
	tunnelWaitTimeout        = 15 * time.Second
	publicReadyTimeout       = 60 * time.Second
)

var tryCloudflareURLPattern = regexp.MustCompile(`https://[a-zA-Z0-9\-]+\.trycloudflare\.com`)

type tunnelState struct {
	PID           int    `json:"pid"`
	UserID        string `json:"user_id"`
	Username      string `json:"username"`
	SessionID     string `json:"session_id"`
	TunnelURL     string `json:"tunnel_url"`
	PublicURL     string `json:"public_url"`
	BrokerEnabled bool   `json:"broker_enabled"`
	UpdatedAt     string `json:"updated_at"`
}

type publicTunnelInfo struct {
	PID           int
	Username      string
	SessionID     string
	TunnelURL     string
	PublicURL     string
	BrokerEnabled bool
	BrokerError   string
}

func stopPreviousChatTunnel() error {
	state, err := loadTunnelState()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if state.PID > 0 {
		_ = killProcessGroupOrPID(state.PID, syscall.SIGTERM)
		for i := 0; i < 12; i++ {
			if err := syscall.Kill(state.PID, 0); err == syscall.ESRCH {
				break
			}
			time.Sleep(50 * time.Millisecond)
		}
		_ = killProcessGroupOrPID(state.PID, syscall.SIGKILL)
	}
	if err := unregisterPublicSession(state); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to unregister public session %s/%s: %v\n", state.Username, state.SessionID, err)
	}
	_ = clearTunnelState()
	return nil
}

func startPublicChatTunnel(localPort int) (*publicTunnelInfo, error) {
	cloudflaredPath, err := ensureCloudflaredBinary()
	if err != nil {
		return nil, err
	}

	username := normalizedLocalUsername()
	sessionID, err := randomSessionID()
	if err != nil {
		return nil, err
	}
	baseURL := strings.TrimRight(chatBrokerSetting("GIMBLE_CHAT_PUBLIC_BASE", defaultPublicChatBaseURL), "/")
	publicURL := fmt.Sprintf("%s/%s/%s", baseURL, username, sessionID)
	localURL := fmt.Sprintf("http://127.0.0.1:%d", localPort)

	logPath, startOffset, err := chatTunnelLogPathAndOffset()
	if err != nil {
		return nil, err
	}

	namedName := strings.TrimSpace(chatBrokerSetting("GIMBLE_NAMED_TUNNEL_NAME", ""))
	namedHost := strings.TrimSpace(chatBrokerSetting("GIMBLE_NAMED_TUNNEL_HOSTNAME", ""))
	tunnelURL := strings.TrimSpace(chatBrokerSetting("GIMBLE_NAMED_TUNNEL_URL", ""))
	if tunnelURL == "" && namedHost != "" {
		h := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(namedHost, "https://"), "http://"))
		if h != "" {
			tunnelURL = "https://" + h
		}
	}

	pid := 0

	if namedName != "" && namedHost != "" {
		h := strings.TrimSpace(strings.TrimPrefix(strings.TrimPrefix(namedHost, "https://"), "http://"))
		cmd := exec.Command(cloudflaredPath,
			"tunnel",
			"--no-autoupdate",
			"--logfile", logPath,
			"--url", localURL,
			"--hostname", h,
			"--name", namedName,
		)
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start named tunnel (%s): %w", namedName, err)
		}
		pid = cmd.Process.Pid
		_ = cmd.Process.Release()
		if tunnelURL == "" {
			tunnelURL = "https://" + h
		}

		if readyErr := waitForPublicURLReady(tunnelURL, 12*time.Second); readyErr != nil {
			_ = killProcessGroupOrPID(pid, syscall.SIGTERM)
			return nil, fmt.Errorf("named tunnel did not become ready quickly: %v; run once: %s tunnel login", readyErr, cloudflaredPath)
		}
	} else if tunnelURL != "" {
		if !strings.HasPrefix(tunnelURL, "https://") {
			return nil, fmt.Errorf("GIMBLE_NAMED_TUNNEL_URL must start with https://")
		}
		// External named tunnel connector mode: caller keeps tunnel process running.
	} else {
		cmd := exec.Command(cloudflaredPath, "tunnel", "--url", localURL, "--no-autoupdate", "--logfile", logPath)
		cmd.Stdin = nil
		cmd.Stdout = nil
		cmd.Stderr = nil
		cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

		if err := cmd.Start(); err != nil {
			return nil, fmt.Errorf("failed to start cloudflared tunnel: %w", err)
		}
		pid = cmd.Process.Pid
		_ = cmd.Process.Release()

		tunnelURL, err = waitForTryCloudflareURL(logPath, startOffset, pid, tunnelWaitTimeout)
		if err != nil {
			_ = killProcessGroupOrPID(pid, syscall.SIGTERM)
			return nil, err
		}
	}

	info := &publicTunnelInfo{
		PID:       pid,
		Username:  username,
		SessionID: sessionID,
		TunnelURL: strings.TrimRight(tunnelURL, "/"),
		PublicURL: publicURL,
	}

	brokerEndpoint := chatBrokerSetting("GIMBLE_CHAT_BROKER_ENDPOINT", baseURL+"/api/register")
	attempts := 30
	if pid == 0 {
		attempts = 10
	}
	if err := registerPublicSessionWithRetry(brokerEndpoint, info, attempts, 1*time.Second); err == nil {
		if readyErr := waitForPublicURLReady(info.PublicURL, publicReadyTimeout); readyErr == nil {
			info.BrokerEnabled = true
		} else {
			// Registration succeeded, but edge propagation can lag.
			// Keep the chat.gimble.dev URL as primary and retain the direct tunnel as fallback.
			info.BrokerEnabled = true
			info.BrokerError = fmt.Sprintf("public URL is still propagating: %v", readyErr)
		}
	} else {
		info.BrokerEnabled = false
		info.BrokerError = err.Error()
		info.PublicURL = info.TunnelURL
	}

	if err := saveTunnelState(tunnelState{
		PID:           info.PID,
		UserID:        info.Username,
		Username:      info.Username,
		SessionID:     info.SessionID,
		TunnelURL:     info.TunnelURL,
		PublicURL:     info.PublicURL,
		BrokerEnabled: info.BrokerEnabled,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to persist tunnel state: %v\n", err)
	}

	return info, nil
}

func registerPublicSessionWithRetry(endpoint string, info *publicTunnelInfo, attempts int, delay time.Duration) error {
	if attempts < 1 {
		attempts = 1
	}
	var lastErr error
	for i := 0; i < attempts; i++ {
		if err := registerPublicSession(endpoint, info); err == nil {
			return nil
		} else {
			lastErr = err
		}
		if i < attempts-1 {
			time.Sleep(delay)
		}
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("registration failed")
	}
	return lastErr
}

func registerPublicSession(endpoint string, info *publicTunnelInfo) error {
	payload := map[string]any{
		"username":           info.Username,
		"session_id":         info.SessionID,
		"tunnel_url":         info.TunnelURL,
		"public_url":         info.PublicURL,
		"expires_in_seconds": 21600,
		"created_at":         time.Now().UTC().Format(time.RFC3339),
	}
	body, _ := json.Marshal(payload)

	// Cloudflare Bot protections may block Go's default TLS/client fingerprint on some zones.
	// Prefer curl transport when present; fallback to net/http.
	if _, err := exec.LookPath("curl"); err == nil {
		cmd := exec.Command(
			"curl",
			"-sS",
			"-A", "Mozilla/5.0 (Gimble; +https://gimble.dev)",
			"-H", "Content-Type: application/json",
			"-H", "Accept: application/json",
			"--max-time", "8",
			"--data", string(body),
			"--write-out", "\n%{http_code}",
			endpoint,
		)
		out, err := cmd.CombinedOutput()
		if err == nil {
			raw := strings.TrimSpace(string(out))
			idx := strings.LastIndex(raw, "\n")
			if idx > -1 {
				respBody := strings.TrimSpace(raw[:idx])
				statusCode := strings.TrimSpace(raw[idx+1:])
				if len(statusCode) == 3 && statusCode[0] == '2' {
					var parsed struct {
						PublicURL string `json:"public_url"`
					}
					_ = json.Unmarshal([]byte(respBody), &parsed)
					if strings.TrimSpace(parsed.PublicURL) != "" {
						info.PublicURL = strings.TrimSpace(parsed.PublicURL)
					}
					return nil
				}
				return fmt.Errorf("broker registration failed: %s (%s)", statusCode, strings.TrimSpace(respBody))
			}
		}
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Gimble; +https://gimble.dev)")
	resp, err := (&http.Client{Timeout: 6 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("broker registration failed: %s (%s)", resp.Status, strings.TrimSpace(string(raw)))
	}
	var out struct {
		PublicURL string `json:"public_url"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if strings.TrimSpace(out.PublicURL) != "" {
		info.PublicURL = strings.TrimSpace(out.PublicURL)
	}
	return nil
}

func unregisterPublicSession(state tunnelState) error {
	userID := strings.TrimSpace(state.UserID)
	if userID == "" {
		userID = normalizedLocalUsername()
	}
	sessionID := strings.TrimSpace(strings.ToLower(state.SessionID))
	if userID == "" || sessionID == "" {
		return nil
	}

	endpoint := ""
	if cloudBase := strings.TrimSpace(cloudAPIBase()); cloudBase != "" {
		endpoint = strings.TrimRight(cloudBase, "/") + "/v1/sessions/disconnect"
	} else {
		baseURL := strings.TrimRight(chatBrokerSetting("GIMBLE_CHAT_PUBLIC_BASE", defaultPublicChatBaseURL), "/")
		endpoint = chatBrokerSetting("GIMBLE_CHAT_BROKER_UNREGISTER_ENDPOINT", baseURL+"/api/unregister")
	}
	payload := map[string]any{"user_id": userID, "session_id": sessionID}
	body, _ := json.Marshal(payload)
	token := cloudAPIToken()

	if _, err := exec.LookPath("curl"); err == nil {
		cmd := exec.Command(
			"curl",
			"-sS",
			"-A", "Mozilla/5.0 (Gimble; +https://gimble.dev)",
			"-H", "Content-Type: application/json",
			"-H", "Accept: application/json",
			"--max-time", "6",
			"-H", "X-Gimble-Token: "+token,
			"-H", "X-Gimble-Device: "+cloudDeviceID(),
			"--data", string(body),
			"--write-out", "\n%{http_code}",
			endpoint,
		)
		out, err := cmd.CombinedOutput()
		if err == nil {
			raw := strings.TrimSpace(string(out))
			idx := strings.LastIndex(raw, "\n")
			if idx > -1 {
				statusCode := strings.TrimSpace(raw[idx+1:])
				if len(statusCode) == 3 && (statusCode[0] == '2' || statusCode == "404") {
					return nil
				}
				respBody := strings.TrimSpace(raw[:idx])
				return fmt.Errorf("broker unregister failed: %s (%s)", statusCode, respBody)
			}
		}
	}

	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "Mozilla/5.0 (Gimble; +https://gimble.dev)")
	if token != "" {
		req.Header.Set("X-Gimble-Token", token)
	}
	req.Header.Set("X-Gimble-Device", cloudDeviceID())
	resp, err := (&http.Client{Timeout: 6 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusNotFound {
		return nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return fmt.Errorf("broker unregister failed: %s (%s)", resp.Status, strings.TrimSpace(string(raw)))
	}
	return nil
}

func waitForPublicURLReady(publicURL string, timeout time.Duration) error {
	if strings.TrimSpace(publicURL) == "" {
		return fmt.Errorf("public URL is empty")
	}
	deadline := time.Now().Add(timeout)
	client := &http.Client{Timeout: 3 * time.Second}
	for time.Now().Before(deadline) {
		req, _ := http.NewRequest(http.MethodGet, publicURL, nil)
		req.Header.Set("User-Agent", "Mozilla/5.0 (Gimble readiness; +https://gimble.dev)")
		resp, err := client.Do(req)
		if err == nil {
			body, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
			_ = resp.Body.Close()
			b := strings.ToLower(string(body))
			if resp.StatusCode >= 200 && resp.StatusCode < 500 && !strings.Contains(b, "error 1016") && !strings.Contains(b, "origin dns error") {
				return nil
			}
		}
		time.Sleep(400 * time.Millisecond)
	}
	return fmt.Errorf("public URL not ready before timeout")
}

func waitForTryCloudflareURL(logPath string, offset int64, pid int, timeout time.Duration) (string, error) {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			return "", fmt.Errorf("cloudflared process exited before tunnel URL was ready")
		}
		content, err := os.ReadFile(logPath)
		if err == nil && int64(len(content)) > offset {
			tail := string(content[offset:])
			if match := tryCloudflareURLPattern.FindString(tail); match != "" {
				return match, nil
			}
		}
		time.Sleep(180 * time.Millisecond)
	}
	return "", fmt.Errorf("timed out waiting for cloudflared tunnel URL")
}

func ensureCloudflaredBinary() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("GIMBLE_CLOUDFLARED_BIN")); explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit, nil
		}
		return "", fmt.Errorf("GIMBLE_CLOUDFLARED_BIN points to a missing file: %s", explicit)
	}

	if p, err := exec.LookPath("cloudflared"); err == nil {
		return p, nil
	}

	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user config dir: %w", err)
	}
	binDir := filepath.Join(base, "gimble", "bin")
	if err := os.MkdirAll(binDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create gimble bin dir: %w", err)
	}
	binPath := filepath.Join(binDir, "cloudflared")
	if runtime.GOOS == "windows" {
		binPath += ".exe"
	}
	if st, err := os.Stat(binPath); err == nil && st.Mode().Perm()&0o111 != 0 {
		return binPath, nil
	}

	downloadURL, err := cloudflaredDownloadURL(runtime.GOOS, runtime.GOARCH)
	if err != nil {
		return "", err
	}

	resp, err := (&http.Client{Timeout: 120 * time.Second}).Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("failed to download cloudflared: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("failed to download cloudflared: %s", resp.Status)
	}

	tmpFile := filepath.Join(binDir, "cloudflared-download.tgz")
	f, err := os.OpenFile(tmpFile, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", err
	}
	if _, err := io.Copy(f, resp.Body); err != nil {
		_ = f.Close()
		return "", err
	}
	_ = f.Close()

	if err := extractCloudflaredTarGz(tmpFile, binPath); err != nil {
		return "", err
	}
	_ = os.Remove(tmpFile)
	if err := os.Chmod(binPath, 0o755); err != nil {
		return "", err
	}
	return binPath, nil
}

func cloudflaredDownloadURL(goos, arch string) (string, error) {
	key := goos + "-" + arch
	name := map[string]string{
		"darwin-amd64": "cloudflared-darwin-amd64.tgz",
		"darwin-arm64": "cloudflared-darwin-arm64.tgz",
		"linux-amd64":  "cloudflared-linux-amd64.tgz",
		"linux-arm64":  "cloudflared-linux-arm64.tgz",
	}[key]
	if name == "" {
		return "", fmt.Errorf("cloudflared auto-install is not supported on %s/%s", goos, arch)
	}
	return "https://github.com/cloudflare/cloudflared/releases/latest/download/" + name, nil
}

func extractCloudflaredTarGz(tarGzPath string, outPath string) error {
	f, err := os.Open(tarGzPath)
	if err != nil {
		return err
	}
	defer f.Close()
	gz, err := gzip.NewReader(f)
	if err != nil {
		return err
	}
	defer gz.Close()
	tr := tar.NewReader(gz)

	for {
		hdr, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}
		if hdr.Typeflag != tar.TypeReg {
			continue
		}
		if filepath.Base(hdr.Name) != "cloudflared" {
			continue
		}
		out, err := os.OpenFile(outPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o755)
		if err != nil {
			return err
		}
		if _, err := io.Copy(out, tr); err != nil {
			_ = out.Close()
			return err
		}
		_ = out.Close()
		return nil
	}
	return fmt.Errorf("cloudflared binary not found in downloaded archive")
}

func normalizedLocalUsername() string {
	candidate := strings.TrimSpace(os.Getenv("USER"))
	if candidate == "" {
		candidate = strings.TrimSpace(os.Getenv("USERNAME"))
	}
	if candidate == "" {
		candidate = "developer"
	}
	candidate = strings.ToLower(candidate)
	var b strings.Builder
	for _, r := range candidate {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_':
			b.WriteRune(r)
		}
	}
	out := strings.Trim(b.String(), "-_")
	if out == "" {
		return "developer"
	}
	return out
}

func chatBrokerSetting(key string, fallback string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	path, err := chatEnvPath()
	if err == nil {
		if vals, err := loadKeyValueEnv(path); err == nil {
			if v := strings.TrimSpace(vals[key]); v != "" {
				return v
			}
		}
	}
	return strings.TrimSpace(fallback)
}

func randomSessionID() (string, error) {
	return randomHexNBytes(4)
}

func randomHexNBytes(n int) (string, error) {
	if n <= 0 {
		return "", fmt.Errorf("invalid random byte length: %d", n)
	}
	buf := make([]byte, n)
	if _, err := rand.Read(buf); err != nil {
		return "", fmt.Errorf("failed to generate random token: %w", err)
	}
	return strings.ToLower(hex.EncodeToString(buf)), nil
}

func chatTunnelStatePath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user config dir for tunnel state: %w", err)
	}
	dir := filepath.Join(base, "gimble")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create gimble state dir: %w", err)
	}
	return filepath.Join(dir, "chat-tunnel-state.json"), nil
}

func loadTunnelState() (tunnelState, error) {
	path, err := chatTunnelStatePath()
	if err != nil {
		return tunnelState{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return tunnelState{}, err
	}
	var state tunnelState
	if err := json.Unmarshal(data, &state); err != nil {
		return tunnelState{}, fmt.Errorf("failed to parse tunnel state: %w", err)
	}
	return state, nil
}

func saveTunnelState(state tunnelState) error {
	path, err := chatTunnelStatePath()
	if err != nil {
		return err
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(path, data, 0o600)
}

func clearTunnelState() error {
	path, err := chatTunnelStatePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func chatTunnelLogPathAndOffset() (string, int64, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", 0, err
	}
	dir := filepath.Join(base, "gimble")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", 0, err
	}
	path := filepath.Join(dir, "chat-tunnel.log")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return "", 0, err
	}
	_ = f.Close()
	st, err := os.Stat(path)
	if err != nil {
		return "", 0, err
	}
	return path, st.Size(), nil
}

func killProcessGroupOrPID(pid int, sig syscall.Signal) error {
	if pid <= 0 {
		return nil
	}
	if err := syscall.Kill(-pid, sig); err != nil && err != syscall.ESRCH {
		if err2 := syscall.Kill(pid, sig); err2 != nil && err2 != syscall.ESRCH {
			return err2
		}
	}
	return nil
}
