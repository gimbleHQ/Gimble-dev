package main

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gimble-dev/gimble/internal/profile"
)

type cloudSessionCreateRequest struct {
	UserID        string            `json:"user_id"`
	Username      string            `json:"username"`
	Source        string            `json:"source"`
	SessionConfig map[string]string `json:"session_config"`
}

type cloudSessionCreateResponse struct {
	SessionID      string `json:"session_id"`
	PublicURL      string `json:"public_url"`
	IngestEndpoint string `json:"ingest_endpoint"`
}

const defaultCloudAPIBase = "https://chat.gimble.dev"

func shouldUseCloudMode() bool {
	return strings.TrimSpace(cloudAPIBase()) != ""
}

func cloudAPIBase() string {
	if v := strings.TrimSpace(os.Getenv("GIMBLE_CLOUD_API_BASE")); v != "" {
		return strings.TrimRight(v, "/")
	}
	if path, err := chatEnvPath(); err == nil {
		if vals, err := loadKeyValueEnv(path); err == nil {
			if v := strings.TrimSpace(vals["GIMBLE_CLOUD_API_BASE"]); v != "" {
				return strings.TrimRight(v, "/")
			}
		}
	}
	return defaultCloudAPIBase
}

func cloudAPIToken() string {
	if v := strings.TrimSpace(os.Getenv("GIMBLE_CLOUD_API_TOKEN")); v != "" {
		return v
	}
	if path, err := chatEnvPath(); err == nil {
		if vals, err := loadKeyValueEnv(path); err == nil {
			if v := strings.TrimSpace(vals["GIMBLE_CLOUD_API_TOKEN"]); v != "" {
				return v
			}
		}
	}
	return defaultCloudAPIBase
}

func resolveSessionLogPath() (string, error) {
	if p := strings.TrimSpace(os.Getenv("GIMBLE_SESSION_LOG_PATH")); p != "" {
		return p, nil
	}
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	latest := filepath.Join(base, "gimble", "session-logs", "session-latest.log")
	if _, err := os.Stat(latest); err == nil {
		return latest, nil
	}
	return "", fmt.Errorf("could not resolve active session log path")
}

func findCloudUploaderScript() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("GIMBLE_CLOUD_UPLOADER")); explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit, nil
		}
		return "", fmt.Errorf("GIMBLE_CLOUD_UPLOADER points to a missing file: %s", explicit)
	}

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	candidates := []string{
		filepath.Join("python", "cloud_ingest_uploader.py"),
		filepath.Join(exeDir, "..", "share", "gimble", "python", "cloud_ingest_uploader.py"),
		filepath.Join(exeDir, "python", "cloud_ingest_uploader.py"),
		filepath.Join(exeDir, "..", "python", "cloud_ingest_uploader.py"),
	}
	for _, c := range candidates {
		if _, err := os.Stat(c); err == nil {
			return c, nil
		}
	}
	return "", fmt.Errorf("could not locate python cloud uploader script")
}

func buildSessionConfig() map[string]string {
	cfg := map[string]string{}
	profileCfg, err := profile.Load()
	if err != nil {
		return cfg
	}
	name, p, ok := profileCfg.Active()
	if !ok {
		return cfg
	}
	if strings.TrimSpace(name) != "" {
		cfg["profile"] = name
	}
	if strings.TrimSpace(p.Name) != "" {
		cfg["name"] = p.Name
	}
	if strings.TrimSpace(p.Email) != "" {
		cfg["email"] = p.Email
	}
	if strings.TrimSpace(p.GitHub) != "" {
		cfg["github"] = "@" + strings.TrimPrefix(p.GitHub, "@")
	}
	if len(p.WorkspaceRoots) > 0 {
		cfg["roots"] = strings.Join(p.WorkspaceRoots, ", ")
	}
	rosBits := []string{}
	if strings.TrimSpace(p.ROSType) != "" {
		rosBits = append(rosBits, p.ROSType)
	}
	if strings.TrimSpace(p.ROSDistro) != "" {
		rosBits = append(rosBits, p.ROSDistro)
	}
	if strings.TrimSpace(p.ROSWorkspace) != "" {
		rosBits = append(rosBits, p.ROSWorkspace)
	}
	if len(rosBits) > 0 {
		cfg["ros"] = strings.Join(rosBits, " | ")
	}
	obsBits := []string{}
	if strings.TrimSpace(p.ObsGrafanaURL) != "" {
		obsBits = append(obsBits, "Grafana: "+p.ObsGrafanaURL)
	}
	if strings.TrimSpace(p.ObsSentryURL) != "" {
		obsBits = append(obsBits, "Sentry: "+p.ObsSentryURL)
	}
	if len(obsBits) > 0 {
		cfg["obs"] = strings.Join(obsBits, " | ")
	}
	if strings.TrimSpace(p.SystemPromptProfile) != "" {
		cfg["prompt"] = p.SystemPromptProfile
	}
	if strings.TrimSpace(p.NotificationPreference) != "" {
		cfg["notify"] = p.NotificationPreference
	}
	return cfg
}

func sendIngestHeartbeat(ingestURL, token, sessionID, userID string) error {
	if ingestURL == "" {
		return nil
	}
	ts := time.Now().UnixMilli()
	seed := fmt.Sprintf("%s:%s:%d", sessionID, userID, ts)
	h := sha1.Sum([]byte(seed))
	eventID := hex.EncodeToString(h[:])[:20]
	payload := map[string]any{
		"event_id":   eventID,
		"session_id": sessionID,
		"user_id":    userID,
		"ts_unix_ms": ts,
		"sequence":   0,
		"source":     "session_start",
		"severity":   "info",
		"text":       "session started",
		"metadata":   map[string]any{"kind": "session_start"},
	}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, ingestURL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(token) != "" {
		req.Header.Set("X-Gimble-Token", token)
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("ingest heartbeat failed: %s", resp.Status)
	}
	return nil
}

func createCloudSession(apiBase, token, userID, username string, sessionConfig map[string]string) (*cloudSessionCreateResponse, error) {
	endpoint := strings.TrimRight(apiBase, "/") + "/v1/sessions"
	payload := cloudSessionCreateRequest{UserID: userID, Username: username, Source: "gimble-cli", SessionConfig: sessionConfig}
	body, _ := json.Marshal(payload)
	req, err := http.NewRequest(http.MethodPost, endpoint, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	if token != "" {
		req.Header.Set("X-Gimble-Token", token)
	}
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("cloud session create failed: %s", resp.Status)
	}
	var out cloudSessionCreateResponse
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, err
	}
	if strings.TrimSpace(out.SessionID) == "" || strings.TrimSpace(out.PublicURL) == "" || strings.TrimSpace(out.IngestEndpoint) == "" {
		return nil, fmt.Errorf("cloud session response missing required fields")
	}
	return &out, nil
}

func runCloudChat() error {
	if existingURL := activeChatPublicURL(); existingURL != "" {
		fmt.Printf("Gimble Chat Agent is already running. Reuse this live link: %s\n", makeHyperlink(existingURL))
		return nil
	}
	if err := stopAndClearChatServer(); err != nil {
		return err
	}
	if err := stopPreviousChatTunnel(); err != nil {
		return err
	}

	apiBase := cloudAPIBase()
	if apiBase == "" {
		return fmt.Errorf("cloud mode requested but GIMBLE_CLOUD_API_BASE is empty")
	}
	token := cloudAPIToken()
	if strings.TrimSpace(token) == "" {
		return fmt.Errorf("GIMBLE_CLOUD_API_TOKEN is required for cloud mode. Set it in chat.env or env vars")
	}
	userID := normalizedLocalUsername()
	username := userID
	if v := strings.TrimSpace(os.Getenv("GIMBLE_USER_GITHUB")); v != "" {
		username = strings.ToLower(strings.TrimSpace(strings.TrimPrefix(v, "@")))
	}

	sessionConfig := buildSessionConfig()
	sess, err := createCloudSession(apiBase, token, userID, username, sessionConfig)
	if err != nil {
		return err
	}
	if err := sendIngestHeartbeat(sess.IngestEndpoint, token, sess.SessionID, userID); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to send ingest heartbeat: %v\n", err)
	}

	pythonExe, err := findPythonInterpreter()
	if err != nil {
		return err
	}
	script, err := findCloudUploaderScript()
	if err != nil {
		return err
	}
	logPath, err := resolveSessionLogPath()
	if err != nil {
		return err
	}

	logFile, _, err := openChatServerLogFile()
	if err != nil {
		return err
	}
	cmd := exec.Command(
		pythonExe,
		script,
		"--log-path", logPath,
		"--ingest-url", sess.IngestEndpoint,
		"--token", token,
		"--session-id", sess.SessionID,
		"--user-id", userID,
	)
	cmd.Stdin = nil
	cmd.Stdout = logFile
	cmd.Stderr = logFile
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		_ = logFile.Close()
		return fmt.Errorf("failed to start cloud uploader: %w", err)
	}
	pid := cmd.Process.Pid
	_ = cmd.Process.Release()
	_ = logFile.Close()

	if err := saveChatServerState(pid); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to persist chat server state: %v\n", err)
	}
	if err := saveTunnelState(tunnelState{
		PID:           0,
		Username:      username,
		SessionID:     sess.SessionID,
		TunnelURL:     sess.PublicURL,
		PublicURL:     sess.PublicURL,
		BrokerEnabled: true,
		UpdatedAt:     time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to persist cloud session state: %v\n", err)
	}

	fmt.Println("✓ Starting Gimble cloud session")
	fmt.Printf("Chat with Gimble Agents at: %s\n", makeHyperlink(sess.PublicURL))
	return nil
}
