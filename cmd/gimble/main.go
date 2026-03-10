package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/gimble-dev/gimble/internal/platform"
	"github.com/gimble-dev/gimble/internal/profile"
)

var version = "dev"

const defaultNamedTunnelName = "gimble-chat-named"
const defaultNamedTunnelHostname = "origin-chat.gimble.dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if err := platform.EnsureSupported(); err != nil {
		return err
	}
	if err := ensureChatBrokerEnvDefaults(); err != nil {
		fmt.Fprintf(os.Stderr, "warning: failed to initialize chat env defaults: %v\n", err)
	}

	inSession := os.Getenv("GIMBLE_SESSION") == "1"

	if inSession && len(args) == 0 {
		return fmt.Errorf("already inside a Gimble session; use 'gim exit' to leave")
	}

	if len(args) == 0 {
		if err := maybeRunFirstTimeSetup(); err != nil {
			return err
		}
		return runSession()
	}

	switch args[0] {
	case "__session_cmd":
		return runSessionCommand(args[1:])
	case "--version", "-version", "-v":
		fmt.Printf("gimble %s\n", version)
		return nil
	case "help", "--help", "-h":
		printHelp()
		return nil
	case "session":
		if inSession {
			return fmt.Errorf("already inside a Gimble session; use 'gim exit' to leave")
		}
		if err := maybeRunFirstTimeSetup(); err != nil {
			return err
		}
		return runSession()
	case "setup":
		return runSetupWizard()
	case "keys":
		return runKeysWizard()
	case "profile":
		return runProfile(args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], helpText())
	}
}

func runSessionCommand(args []string) error {
	if os.Getenv("GIMBLE_SESSION") != "1" {
		return fmt.Errorf("session commands are only available inside a Gimble session")
	}
	if len(args) == 0 {
		return fmt.Errorf("missing session subcommand")
	}

	switch args[0] {
	case "chat":
		return runPythonChat(args[1:])
	case "disconnect":
		if len(args) > 1 {
			return fmt.Errorf("unknown session command %q", strings.Join(args, " "))
		}
		return runExitChatCommand()
	case "exit":
		if len(args) > 1 {
			return fmt.Errorf("unknown session command %q", strings.Join(args, " "))
		}
		return runExitSessionCommand()
	default:
		return fmt.Errorf("unknown session command %q", args[0])
	}
}

func runExitChatCommand() error {
	if err := stopAndClearChatServer(); err != nil {
		return err
	}
	if err := stopPreviousChatTunnel(); err != nil {
		return err
	}
	fmt.Println("Gimble Chat Agent stopped.")
	return nil
}

func runExitSessionCommand() error {
	// Fail-safe: always stop cloud uploader + ingestion before exiting session shell.
	if err := runExitChatCommand(); err != nil {
		return err
	}
	ppid := os.Getppid()
	if err := syscall.Kill(ppid, syscall.SIGHUP); err != nil {
		if err2 := syscall.Kill(ppid, syscall.SIGTERM); err2 != nil {
			return fmt.Errorf("failed to exit Gimble session: %v", err2)
		}
	}
	return nil
}

func runPythonChat(args []string) error {
	fs := flag.NewFlagSet("chat", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	_ = fs.Int("port", 0, "preferred port (unused in cloud mode)")
	if err := fs.Parse(args); err != nil {
		return err
	}
	return runCloudChat()
}

func isPIDAlive(pid int) bool {
	if pid <= 0 {
		return false
	}
	err := syscall.Kill(pid, 0)
	return err == nil || err == syscall.EPERM
}

func activeChatPublicURL() string {
	s, err := loadChatServerState()
	if err != nil {
		return ""
	}
	if !isPIDAlive(s.PID) {
		_ = clearChatServerState()
		_ = clearTunnelState()
		return ""
	}
	t, err := loadTunnelState()
	if err != nil {
		return ""
	}
	if u := strings.TrimSpace(t.PublicURL); u != "" {
		return u
	}
	if u := strings.TrimSpace(t.TunnelURL); u != "" {
		return u
	}
	return ""
}

type chatServerState struct {
	PID       int    `json:"pid"`
	UpdatedAt string `json:"updated_at"`
}

func chatServerStatePath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user config dir for chat state: %w", err)
	}
	stateDir := filepath.Join(base, "gimble")
	if err := os.MkdirAll(stateDir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create chat state directory: %w", err)
	}
	return filepath.Join(stateDir, "chat-server-state.json"), nil
}

func loadChatServerState() (chatServerState, error) {
	path, err := chatServerStatePath()
	if err != nil {
		return chatServerState{}, err
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return chatServerState{}, err
	}
	var s chatServerState
	if err := json.Unmarshal(data, &s); err != nil {
		return chatServerState{}, fmt.Errorf("failed to parse chat server state: %w", err)
	}
	return s, nil
}

func saveChatServerState(pid int) error {
	path, err := chatServerStatePath()
	if err != nil {
		return err
	}
	s := chatServerState{
		PID:       pid,
		UpdatedAt: time.Now().UTC().Format(time.RFC3339),
	}
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode chat server state: %w", err)
	}
	data = append(data, '\n')
	if err := os.WriteFile(path, data, 0o600); err != nil {
		return fmt.Errorf("failed to write chat server state: %w", err)
	}
	return nil
}

func clearChatServerState() error {
	path, err := chatServerStatePath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

func stopAndClearChatServer() error {
	if err := stopPreviousChatServer(); err != nil {
		return err
	}
	_ = clearChatServerState()
	return nil
}

func stopPreviousChatServer() error {
	s, err := loadChatServerState()
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	if s.PID <= 0 {
		return nil
	}

	if err := syscall.Kill(-s.PID, syscall.SIGTERM); err != nil && err != syscall.ESRCH {
		if err2 := syscall.Kill(s.PID, syscall.SIGTERM); err2 != nil && err2 != syscall.ESRCH {
			return fmt.Errorf("failed to stop previous chat server pid %d: %w", s.PID, err2)
		}
	}

	for i := 0; i < 12; i++ {
		if err := syscall.Kill(s.PID, 0); err == syscall.ESRCH {
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}

	if err := syscall.Kill(-s.PID, syscall.SIGKILL); err != nil && err != syscall.ESRCH {
		if err2 := syscall.Kill(s.PID, syscall.SIGKILL); err2 != nil && err2 != syscall.ESRCH {
			return fmt.Errorf("failed to force-stop previous chat server pid %d: %w", s.PID, err2)
		}
	}
	return nil
}

func openChatServerLogFile() (*os.File, string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return nil, "", fmt.Errorf("failed to resolve user config dir for logs: %w", err)
	}
	logDir := filepath.Join(base, "gimble")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return nil, "", fmt.Errorf("failed to create log directory: %w", err)
	}
	logPath := filepath.Join(logDir, "chat-server.log")
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o600)
	if err != nil {
		return nil, "", fmt.Errorf("failed to open log file: %w", err)
	}
	if _, err := f.WriteString("\n==== gim chat start ====\n"); err != nil {
		_ = f.Close()
		return nil, "", fmt.Errorf("failed to initialize log file: %w", err)
	}
	if _, err := io.WriteString(f, "chat server process launched\n"); err != nil {
		_ = f.Close()
		return nil, "", fmt.Errorf("failed to initialize log file: %w", err)
	}
	return f, logPath, nil
}

func waitForChatServerReady(port int, pid int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	url := fmt.Sprintf("http://127.0.0.1:%d/", port)
	client := &http.Client{Timeout: 500 * time.Millisecond}
	for time.Now().Before(deadline) {
		resp, err := client.Get(url)
		if err == nil {
			_ = resp.Body.Close()
			if resp.StatusCode >= 200 && resp.StatusCode < 500 {
				return nil
			}
		}
		if err := syscall.Kill(pid, 0); err == syscall.ESRCH {
			return fmt.Errorf("python server process exited early")
		}
		time.Sleep(120 * time.Millisecond)
	}
	return fmt.Errorf("server did not become ready on time")
}

func ensurePythonChatRuntime(pythonExe string, scriptPath string) (string, error) {
	if resolved, err := findPythonInterpreter(); err == nil {
		pythonExe = resolved
	}

	if err := checkPythonRuntimeImports(pythonExe); err != nil {
		return "", fmt.Errorf("python runtime is not ready: %w\nRun: %s", err, runtimeSetupHint(scriptPath))
	}
	return pythonExe, nil
}

func checkPythonRuntimeImports(pythonExe string) error {
	cmd := exec.Command(pythonExe, "-c", "import requests")
	out, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(out))
		if trimmed == "" {
			trimmed = err.Error()
		}
		return fmt.Errorf(trimmed)
	}
	return nil
}

func runtimeSetupHint(scriptPath string) string {
	setupScript := filepath.Join(filepath.Dir(scriptPath), "setup_runtime.sh")
	return setupScript
}

func findPythonInterpreter() (string, error) {
	home, _ := os.UserHomeDir()
	if home != "" {
		venvCandidates := []string{
			filepath.Join(home, "Library", "Application Support", "gimble", "pyenv", "bin", "python3"),
			filepath.Join(home, ".config", "gimble", "pyenv", "bin", "python3"),
		}
		for _, p := range venvCandidates {
			if _, err := os.Stat(p); err == nil {
				return p, nil
			}
		}
	}

	for _, candidate := range []string{"python3", "python"} {
		if p, err := exec.LookPath(candidate); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("python not found. Install Python 3 and retry")
}

func findPythonChatServerScript() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("GIMBLE_CHAT_SERVER")); explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit, nil
		}
		return "", fmt.Errorf("GIMBLE_CHAT_SERVER points to a missing file: %s", explicit)
	}

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)

	candidates := []string{
		filepath.Join("python", "chat_server.py"),
		filepath.Join(exeDir, "..", "share", "gimble", "python", "chat_server.py"),
		filepath.Join(exeDir, "python", "chat_server.py"),
		filepath.Join(exeDir, "..", "python", "chat_server.py"),
	}

	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("could not locate python chat server. expected one of: %s", strings.Join(candidates, ", "))
}

func listenWithFallback(preferredPort int) (net.Listener, int, error) {
	if preferredPort != 0 {
		addr := fmt.Sprintf("127.0.0.1:%d", preferredPort)
		ln, err := net.Listen("tcp", addr)
		if err == nil {
			return ln, preferredPort, nil
		}
	}

	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return nil, 0, fmt.Errorf("failed to find open port: %w", err)
	}
	actual := ln.Addr().(*net.TCPAddr).Port
	return ln, actual, nil
}

// makeHyperlink returns an OSC 8 hyperlink sequence for terminals that support it.
// The returned string will display the URL as clickable text when supported.
func makeHyperlink(url string) string {
	// ESC ] 8 ; ; <url> ESC \ <text> ESC ] 8 ; ; ESC \
	return "\x1b]8;;" + url + "\x1b\\" + url + "\x1b]8;;\x1b\\"
}

func runTunnelSpinner(stop <-chan struct{}, done chan<- struct{}, label string) {
	defer close(done)
	frames := []rune{'|', '/', '-', '\\'}
	i := 0
	for {
		select {
		case <-stop:
			fmt.Print("\r")
			return
		default:
			fmt.Printf("\r[%c] %s...", frames[i%len(frames)], label)
			time.Sleep(110 * time.Millisecond)
			i++
		}
	}
}

func printSessionIntro(activeName string, p profile.Profile) {
	border := "=============================================================="
	title := []string{
		"   ██████╗  ██╗ ███╗   ███╗ ██████╗  ██╗      ███████╗",
		"  ██╔════╝  ██║ ████╗ ████║ ██╔══██╗ ██║      ██╔════╝",
		"  ██║  ███╗ ██║ ██╔████╔██║ ██████╔╝ ██║      █████╗",
		"  ██║   ██║ ██║ ██║╚██╔╝██║ ██╔══██╗ ██║      ██╔══╝",
		"  ╚██████╔╝ ██║ ██║ ╚═╝ ██║ ██████╔╝ ███████╗ ███████╗",
		"   ╚═════╝  ╚═╝ ╚═╝     ╚═╝ ╚═════╝  ╚══════╝ ╚══════╝",
	}

	fmt.Println()
	fmt.Println(styleText(border, "1;36"))
	for _, line := range title {
		fmt.Println(styleText(line, "1;35"))
	}
	fmt.Println(styleText(border, "1;36"))
	fmt.Println(styleText("Deployment Intelligence for Robotics", "1;37"))
	fmt.Println()

	if activeName != "" {
		fmt.Println(styleText("Active Profile", "1;33"))
		fmt.Printf("  %s (%s, %s) [%s]\n", p.Name, p.Email, profileAccountLabel(p), activeName)
		fmt.Println()
	}

	fmt.Println("Gimble helps you debug deployment failures, inspect runtime behavior,")
	fmt.Println("and trace issues across code, infrastructure, and robots in the field.")
	fmt.Println()
	fmt.Println(styleText("Capabilities", "1;33"))
	fmt.Println("  - Runtime log analysis")
	fmt.Println("  - ROS graph inspection")
	fmt.Println("  - Deployment history tracing")
	fmt.Println("  - Fleet anomaly detection")
	fmt.Println()
	fmt.Println(styleText("Function", "1;33"))
	fmt.Println("  gim chat       Starts the local web chat UI on an available localhost port")
	fmt.Println("  gim disconnect Stops the cloud uploader while staying inside Gimble session")
	fmt.Println("                 and continues running in the background.")
	fmt.Println()
	fmt.Println(styleText("Try Asking", "1;33"))
	fmt.Println("  > why did the perception pipeline crash?")
	fmt.Println("  > analyze latest ros logs")
	fmt.Println("  > inspect GPU usage on robot-03")
	fmt.Println("  > investigate deployment failures")
	fmt.Println()
	fmt.Println("Diagnose logs, ask a question, or connect a robot.")
	fmt.Println()
}

func styleText(s, code string) string {
	if !useANSI() {
		return s
	}
	return "\x1b[" + code + "m" + s + "\x1b[0m"
}

func useANSI() bool {
	if strings.TrimSpace(os.Getenv("NO_COLOR")) != "" {
		return false
	}
	term := strings.ToLower(strings.TrimSpace(os.Getenv("TERM")))
	return term != "" && term != "dumb"
}

func maybeRunFirstTimeSetup() error {
	cfg, err := profile.Load()
	if err != nil {
		return err
	}
	if cfg.ActiveProfile != "" || len(cfg.Profiles) > 0 {
		return nil
	}
	if !isInteractiveTerminal() {
		return fmt.Errorf("first-time setup required: run 'gimble setup' in an interactive terminal")
	}
	return runSetupWizard()
}

func isInteractiveTerminal() bool {
	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return false
	}
	_ = tty.Close()
	return true
}

func installPythonRuntimeDuringSetup() error {
	scriptPath, err := findCloudUploaderScript()
	if err != nil {
		return err
	}

	setupScript := filepath.Join(filepath.Dir(scriptPath), "setup_runtime.sh")
	if _, err := os.Stat(setupScript); err != nil {
		if os.IsNotExist(err) {
			return fmt.Errorf("missing runtime setup script: %s", setupScript)
		}
		return err
	}

	fmt.Println("Installing Python chat runtime (one-time setup)...")
	cmd := exec.Command("sh", setupScript)
	output, err := cmd.CombinedOutput()
	if err != nil {
		trimmed := strings.TrimSpace(string(output))
		if trimmed == "" {
			trimmed = err.Error()
		}
		return fmt.Errorf("failed to install python runtime: %s", trimmed)
	}
	fmt.Println("Python chat runtime is ready.")

	pythonExe, err := findPythonInterpreter()
	if err != nil {
		return err
	}
	_, err = ensurePythonChatRuntime(pythonExe, scriptPath)
	return err
}

func printSetupBanner() {
	border := "=============================================================="
	fmt.Println(styleText(border, "1;36"))
	fmt.Println(styleText("GIMBLE INITIAL SETUP", "1;35"))
	fmt.Println(styleText(border, "1;36"))
	fmt.Println("Gimble.dev configuration wizard")
	fmt.Println()
}

func printSetupSection(title string) {
	fmt.Println(styleText(title, "1;33"))
}

func parseCSVList(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		item := strings.TrimSpace(part)
		if item == "" {
			continue
		}
		out = append(out, item)
	}
	return out
}

func runSetupWizard() error {
	if !isInteractiveTerminal() {
		return fmt.Errorf("setup requires an interactive terminal")
	}

	printSetupBanner()
	printSetupSection("Profile")

	tty, err := os.OpenFile("/dev/tty", os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("setup requires a real terminal")
	}
	defer tty.Close()

	reader := bufio.NewReader(tty)

	name, err := promptRequired(reader, "Full name")
	if err != nil {
		return err
	}

	email, err := promptRequired(reader, "Email address")
	if err != nil {
		return err
	}
	if err := profile.ValidateEmail(email); err != nil {
		return err
	}

	providerChoice, err := promptChoiceInline(reader, "Code host", []string{"GitHub", "GitLab"}, 1, false)
	if err != nil {
		return err
	}
	provider := "github"
	if providerChoice == 2 {
		provider = "gitlab"
	}

	handleLabel := "GitHub username"
	if provider == "gitlab" {
		handleLabel = "GitLab username"
	}
	handle, err := promptRequired(reader, handleLabel)
	if err != nil {
		return err
	}
	handle = profile.NormalizeGitHub(handle)

	fmt.Println()
	printSetupSection("Experimental Settings (Optional)")
	workspaceRootsRaw, err := promptOptional(reader, "Workspace roots (comma-separated, Enter to skip)")
	if err != nil {
		return err
	}
	rosProfileChoice, err := promptChoiceMultiline(reader, "ROS profile", []string{"ROS1 / noetic", "ROS2 / humble", "ROS2 / jazzy", "ROS2 / rolling"}, true)
	if err != nil {
		return err
	}
	rosType := ""
	rosDistro := ""
	switch rosProfileChoice {
	case 1:
		rosType, rosDistro = "ros1", "noetic"
	case 2:
		rosType, rosDistro = "ros2", "humble"
	case 3:
		rosType, rosDistro = "ros2", "jazzy"
	case 4:
		rosType, rosDistro = "ros2", "rolling"
	}
	rosWorkspace, err := promptOptional(reader, "ROS workspace path (Enter to skip)")
	if err != nil {
		return err
	}
	grafanaURL, err := promptOptional(reader, "Grafana URL (Enter to skip)")
	if err != nil {
		return err
	}
	sentryURL, err := promptOptional(reader, "Sentry URL (Enter to skip)")
	if err != nil {
		return err
	}
	systemPromptChoice, err := promptChoiceMultiline(reader, "System prompt profile", []string{"debug-heavy", "concise", "incident-response"}, true)
	if err != nil {
		return err
	}
	systemPromptProfile := ""
	if systemPromptChoice >= 1 && systemPromptChoice <= 3 {
		systemPromptProfile = []string{"debug-heavy", "concise", "incident-response"}[systemPromptChoice-1]
	}

	cfg, err := profile.Load()
	if err != nil {
		return err
	}
	cfg.Upsert("default", profile.Profile{
		Name:                   strings.TrimSpace(name),
		Email:                  strings.TrimSpace(email),
		GitHub:                 handle,
		Provider:               profile.NormalizeProvider(provider),
		WorkspaceRoots:         parseCSVList(workspaceRootsRaw),
		ROSType:                strings.ToLower(strings.TrimSpace(rosType)),
		ROSDistro:              strings.TrimSpace(rosDistro),
		ROSWorkspace:           strings.TrimSpace(rosWorkspace),
		ObsGrafanaURL:          strings.TrimSpace(grafanaURL),
		ObsSentryURL:           strings.TrimSpace(sentryURL),
		SystemPromptProfile:    strings.TrimSpace(systemPromptProfile),
		NotificationPreference: "",
	})
	cfg.ActiveProfile = "default"
	if err := profile.Save(cfg); err != nil {
		return err
	}

	fmt.Println()
	printSetupSection("Model Providers (Optional)")
	fmt.Println("OpenAI key: https://platform.openai.com/api-keys")
	openAIKey, err := promptOptional(reader, "OpenAI API key (press Enter to skip)")
	if err != nil {
		return err
	}
	fmt.Println("Groq key: https://console.groq.com/keys")
	groqKey, err := promptOptional(reader, "Groq API key (press Enter to skip)")
	if err != nil {
		return err
	}

	fmt.Println()
	printSetupSection("Cloud Backend")
	fmt.Println("Gimble Cloud API base (default https://chat.gimble.dev)")
	cloudBase, err := promptOptional(reader, "GIMBLE_CLOUD_API_BASE (press Enter for default)")
	if err != nil {
		return err
	}
	if strings.TrimSpace(cloudBase) == "" {
		cloudBase = defaultCloudAPIBase
	}
	cloudToken, err := promptOptional(reader, "GIMBLE_CLOUD_API_TOKEN (required for cloud mode)")
	if err != nil {
		return err
	}

	if err := upsertChatEnv(openAIKey, groqKey, cloudBase, cloudToken); err != nil {
		return err
	}

	fmt.Println()
	printSetupSection("Runtime")
	if err := installPythonRuntimeDuringSetup(); err != nil {
		return err
	}

	chatPath, _ := chatEnvPath()
	fmt.Println()
	printSetupSection("Setup Complete")
	fmt.Printf("Active profile: default (%s, %s:@%s).\n", email, provider, handle)
	fmt.Printf("Local secrets file: %s\n", chatPath)
	fmt.Println("Keys are stored locally with user-only permissions and are never pushed by Gimble.")
	return nil
}

func runKeysWizard() error {
	if !isInteractiveTerminal() {
		return fmt.Errorf("keys update requires an interactive terminal")
	}
	printSetupSection("Model Providers (Optional)")
	reader := bufio.NewReader(os.Stdin)
	openAIKey, err := promptOptional(reader, "OPENAI_API_KEY (press Enter to keep current)")
	if err != nil {
		return err
	}
	groqKey, err := promptOptional(reader, "GROQ_API_KEY (press Enter to keep current)")
	if err != nil {
		return err
	}
	if err := upsertChatEnv(openAIKey, groqKey, "", ""); err != nil {
		return err
	}
	fmt.Println("API keys updated.")
	return nil
}

func promptRequired(reader *bufio.Reader, label string) (string, error) {
	for {
		fmt.Printf("%s: ", label)
		v, err := reader.ReadString('\n')
		if err != nil {
			return "", err
		}
		v = strings.TrimSpace(v)
		if v != "" {
			return v, nil
		}
		fmt.Println("Value is required.")
	}
}

func promptOptional(reader *bufio.Reader, label string) (string, error) {
	fmt.Printf("%s: ", label)
	v, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(v), nil
}

func promptChoice(reader *bufio.Reader, label string, options []string) (int, error) {
	fmt.Printf("%s\n", label)
	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}
	for {
		fmt.Printf("Select [1-%d]: ", len(options))
		v, err := reader.ReadString('\n')
		if err != nil {
			return 0, err
		}
		v = strings.TrimSpace(v)
		n, err := strconv.Atoi(v)
		if err == nil && n >= 1 && n <= len(options) {
			return n, nil
		}
		fmt.Println("Invalid selection.")
	}
}

func promptChoiceInline(reader *bufio.Reader, label string, options []string, defaultChoice int, allowSkip bool) (int, error) {
	parts := make([]string, 0, len(options))
	for i, opt := range options {
		parts = append(parts, fmt.Sprintf("%d=%s", i+1, opt))
	}
	prompt := fmt.Sprintf("%s [%s]", label, strings.Join(parts, ", "))
	if allowSkip {
		prompt += "; Enter=skip"
	} else if defaultChoice >= 1 && defaultChoice <= len(options) {
		prompt += fmt.Sprintf("; Enter=%d", defaultChoice)
	}
	prompt += ": "

	for {
		fmt.Print(prompt)
		v, err := reader.ReadString('\n')
		if err != nil {
			return 0, err
		}
		v = strings.TrimSpace(v)
		if v == "" {
			if allowSkip {
				return 0, nil
			}
			if defaultChoice >= 1 && defaultChoice <= len(options) {
				return defaultChoice, nil
			}
		}
		n, err := strconv.Atoi(v)
		if err == nil && n >= 1 && n <= len(options) {
			return n, nil
		}
		fmt.Println("Invalid selection.")
	}
}

func promptChoiceMultiline(reader *bufio.Reader, label string, options []string, allowSkip bool) (int, error) {
	fmt.Printf("%s\n", label)
	for i, opt := range options {
		fmt.Printf("  %d) %s\n", i+1, opt)
	}
	for {
		if allowSkip {
			fmt.Printf("Select [1-%d] (Enter to skip): ", len(options))
		} else {
			fmt.Printf("Select [1-%d]: ", len(options))
		}
		v, err := reader.ReadString('\n')
		if err != nil {
			return 0, err
		}
		v = strings.TrimSpace(v)
		if v == "" && allowSkip {
			return 0, nil
		}
		n, err := strconv.Atoi(v)
		if err == nil && n >= 1 && n <= len(options) {
			return n, nil
		}
		fmt.Println("Invalid selection.")
	}
}

func chatEnvPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user config dir: %w", err)
	}
	dir := filepath.Join(base, "gimble")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create gimble config dir: %w", err)
	}
	return filepath.Join(dir, "chat.env"), nil
}

func loadKeyValueEnv(path string) (map[string]string, error) {
	values := map[string]string{}
	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return values, nil
		}
		return nil, err
	}
	for _, raw := range strings.Split(string(data), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "export ") {
			line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		}
		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}
		values[strings.TrimSpace(parts[0])] = strings.TrimSpace(strings.Trim(parts[1], "\"'"))
	}
	return values, nil
}

func saveKeyValueEnv(path string, values map[string]string) error {
	keys := make([]string, 0, len(values))
	for k := range values {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	var b strings.Builder
	b.WriteString("# Gimble local chat secrets (never commit)\n")
	for _, k := range keys {
		v := strings.TrimSpace(values[k])
		if v == "" {
			continue
		}
		b.WriteString(k)
		b.WriteString("=")
		b.WriteString(v)
		b.WriteString("\n")
	}
	return os.WriteFile(path, []byte(b.String()), 0o600)
}

func ensureChatBrokerEnvDefaults() error {
	path, err := chatEnvPath()
	if err != nil {
		return err
	}
	vals, err := loadKeyValueEnv(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	if strings.TrimSpace(vals["GIMBLE_CLOUD_API_BASE"]) == "" {
		vals["GIMBLE_CLOUD_API_BASE"] = defaultCloudAPIBase
		return saveKeyValueEnv(path, vals)
	}
	return nil
}

func upsertChatEnv(openAIKey, groqKey, cloudBase, cloudToken string) error {
	path, err := chatEnvPath()
	if err != nil {
		return err
	}
	vals, err := loadKeyValueEnv(path)
	if err != nil {
		return fmt.Errorf("failed to read %s: %w", path, err)
	}
	if strings.TrimSpace(openAIKey) != "" {
		vals["OPENAI_API_KEY"] = strings.TrimSpace(openAIKey)
		if strings.TrimSpace(vals["OPENAI_MODEL"]) == "" {
			vals["OPENAI_MODEL"] = "gpt-4o-mini"
		}
	}
	if strings.TrimSpace(groqKey) != "" {
		vals["GROQ_API_KEY"] = strings.TrimSpace(groqKey)
		if strings.TrimSpace(vals["GROQ_MODEL"]) == "" {
			vals["GROQ_MODEL"] = "openai/gpt-oss-120b"
		}
	}
	if strings.TrimSpace(cloudBase) != "" {
		vals["GIMBLE_CLOUD_API_BASE"] = strings.TrimSpace(cloudBase)
	}
	if strings.TrimSpace(cloudToken) != "" {
		vals["GIMBLE_CLOUD_API_TOKEN"] = strings.TrimSpace(cloudToken)
	}
	if err := saveKeyValueEnv(path, vals); err != nil {
		return err
	}
	return ensureChatBrokerEnvDefaults()
}
func hasCloudflareTunnelAuth() bool {
	home, err := os.UserHomeDir()
	if err != nil {
		return false
	}
	cert := filepath.Join(home, ".cloudflared", "cert.pem")
	_, err = os.Stat(cert)
	return err == nil
}

func containsIgnoreCase(haystack, needle string) bool {
	return strings.Contains(strings.ToLower(haystack), strings.ToLower(needle))
}

func bootstrapNamedTunnelDuringSetup() error {
	cloudflaredPath, err := ensureCloudflaredBinary()
	if err != nil {
		return nil
	}
	if !hasCloudflareTunnelAuth() {
		// Non-blocking: setup remains usable with quick tunnels.
		return nil
	}

	listOut, listErr := exec.Command(cloudflaredPath, "tunnel", "list").CombinedOutput()
	if listErr != nil {
		// Non-blocking fallback.
		return nil
	}
	if !containsIgnoreCase(string(listOut), defaultNamedTunnelName) {
		if out, err := exec.Command(cloudflaredPath, "tunnel", "create", defaultNamedTunnelName).CombinedOutput(); err != nil {
			if !containsIgnoreCase(string(out), "already") {
				return nil
			}
		}
	}

	if out, err := exec.Command(cloudflaredPath, "tunnel", "route", "dns", defaultNamedTunnelName, defaultNamedTunnelHostname).CombinedOutput(); err != nil {
		if !(containsIgnoreCase(string(out), "already") || containsIgnoreCase(string(out), "exists")) {
			return nil
		}
	}

	path, err := chatEnvPath()
	if err != nil {
		return nil
	}
	vals, err := loadKeyValueEnv(path)
	if err != nil {
		return nil
	}
	vals["GIMBLE_NAMED_TUNNEL_NAME"] = defaultNamedTunnelName
	vals["GIMBLE_NAMED_TUNNEL_HOSTNAME"] = defaultNamedTunnelHostname
	vals["GIMBLE_NAMED_TUNNEL_URL"] = "https://" + defaultNamedTunnelHostname
	_ = saveKeyValueEnv(path, vals)
	return nil
}

func profileAccountProvider(p profile.Profile) string {
	return profile.NormalizeProvider(p.Provider)
}

func profileAccountLabel(p profile.Profile) string {
	return fmt.Sprintf("%s:@%s", profileAccountProvider(p), p.GitHub)
}

func runProfile(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("missing profile subcommand\n\n%s", profileHelpText())
	}

	switch args[0] {
	case "list":
		return profileList()
	case "show":
		return profileShow(args[1:])
	case "use":
		return profileUse(args[1:])
	case "delete":
		return profileDelete(args[1:])
	case "set":
		return profileSet(args[1:])
	case "init":
		return profileInit(args[1:])
	default:
		return fmt.Errorf("unknown profile subcommand %q\n\n%s", args[0], profileHelpText())
	}
}

func profileList() error {
	cfg, err := profile.Load()
	if err != nil {
		return err
	}

	names := cfg.ProfileNames()
	if len(names) == 0 {
		fmt.Println("No profiles configured.")
		return nil
	}

	for _, name := range names {
		prefix := " "
		if cfg.ActiveProfile == name {
			prefix = "*"
		}
		p := cfg.Profiles[name]
		fmt.Printf("%s %s	%s	%s\n", prefix, name, p.Email, profileAccountLabel(p))
	}
	return nil
}

func profileShow(args []string) error {
	cfg, err := profile.Load()
	if err != nil {
		return err
	}

	name := ""
	if len(args) > 0 {
		name = profile.NormalizeProfileName(args[0])
	} else {
		name = cfg.ActiveProfile
	}
	if name == "" {
		return fmt.Errorf("no active profile set")
	}

	p, ok := cfg.Get(name)
	if !ok {
		return fmt.Errorf("profile %q not found", name)
	}

	fmt.Printf("profile: %s\n", name)
	fmt.Printf("name:    %s\n", p.Name)
	fmt.Printf("email:   %s\n", p.Email)
	fmt.Printf("account: %s\n", profileAccountLabel(p))
	if cfg.ActiveProfile == name {
		fmt.Println("active:  yes")
	}
	return nil
}

func profileUse(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gimble profile use <profile>")
	}

	cfg, err := profile.Load()
	if err != nil {
		return err
	}

	name := profile.NormalizeProfileName(args[0])
	if err := cfg.Use(name); err != nil {
		return err
	}
	if err := profile.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("Active profile: %s\n", name)
	return nil
}

func profileDelete(args []string) error {
	if len(args) != 1 {
		return fmt.Errorf("usage: gimble profile delete <profile>")
	}

	cfg, err := profile.Load()
	if err != nil {
		return err
	}

	name := profile.NormalizeProfileName(args[0])
	if err := cfg.Delete(name); err != nil {
		return err
	}
	if err := profile.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("Deleted profile: %s\n", name)
	return nil
}

func profileSet(args []string) error {
	fs := flag.NewFlagSet("profile set", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	profileName := fs.String("profile", "", "profile name")
	name := fs.String("name", "", "full name")
	email := fs.String("email", "", "email address")
	github := fs.String("github", "", "GitHub username")
	provider := fs.String("provider", "github", "account provider: github|gitlab")

	if err := fs.Parse(args); err != nil {
		return err
	}

	normalizedName := profile.NormalizeProfileName(*profileName)
	if normalizedName == "" {
		return fmt.Errorf("--profile is required")
	}

	cfg, err := profile.Load()
	if err != nil {
		return err
	}

	p := profile.Profile{}
	if existing, ok := cfg.Get(normalizedName); ok {
		p = existing
	}

	if *name != "" {
		p.Name = strings.TrimSpace(*name)
	}
	if *email != "" {
		if err := profile.ValidateEmail(*email); err != nil {
			return err
		}
		p.Email = strings.TrimSpace(*email)
	}
	if *github != "" {
		p.GitHub = profile.NormalizeGitHub(*github)
	}
	if *provider != "" {
		p.Provider = profile.NormalizeProvider(*provider)
	}

	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Email) == "" || strings.TrimSpace(p.GitHub) == "" {
		return fmt.Errorf("profile %q must include name, email, and account handle (use --name, --email, --github)", normalizedName)
	}
	if strings.TrimSpace(p.Provider) == "" {
		p.Provider = "github"
	}

	cfg.Upsert(normalizedName, p)
	if cfg.ActiveProfile == "" {
		cfg.ActiveProfile = normalizedName
	}
	if err := profile.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("Saved profile: %s\n", normalizedName)
	if cfg.ActiveProfile == normalizedName {
		fmt.Println("Active profile unchanged.")
	}
	return nil
}

func profileInit(args []string) error {
	fs := flag.NewFlagSet("profile init", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)

	profileName := fs.String("profile", "default", "profile name")
	name := fs.String("name", "", "full name")
	email := fs.String("email", "", "email address")
	github := fs.String("github", "", "GitHub username")
	provider := fs.String("provider", "github", "account provider: github|gitlab")
	if err := fs.Parse(args); err != nil {
		return err
	}

	normalizedName := profile.NormalizeProfileName(*profileName)
	if normalizedName == "" {
		return fmt.Errorf("--profile cannot be empty")
	}
	if strings.TrimSpace(*name) == "" || strings.TrimSpace(*email) == "" || strings.TrimSpace(*github) == "" {
		return fmt.Errorf("--name, --email, and --github are required")
	}
	if err := profile.ValidateEmail(*email); err != nil {
		return err
	}

	cfg, err := profile.Load()
	if err != nil {
		return err
	}

	cfg.Upsert(normalizedName, profile.Profile{
		Name:     strings.TrimSpace(*name),
		Email:    strings.TrimSpace(*email),
		GitHub:   profile.NormalizeGitHub(*github),
		Provider: profile.NormalizeProvider(*provider),
	})
	cfg.ActiveProfile = normalizedName

	if err := profile.Save(cfg); err != nil {
		return err
	}

	fmt.Printf("Initialized profile %q and set as active.\n", normalizedName)
	return nil
}

func runSession() error {
	cfg, err := profile.Load()
	if err != nil {
		return err
	}

	shell := os.Getenv("SHELL")
	if shell == "" {
		if runtime.GOOS == "darwin" {
			shell = "/bin/zsh"
		} else {
			shell = "/bin/bash"
		}
	}

	env := append(os.Environ(), "GIMBLE_SESSION=1")
	promptPrefix := "gimble"

	cleanup, shimDir, shimErr := createSessionShimDir()
	if shimErr == nil {
		if oldPath := os.Getenv("PATH"); oldPath != "" {
			env = append(env, "PATH="+shimDir+":"+oldPath)
		} else {
			env = append(env, "PATH="+shimDir)
		}
		defer cleanup()
	}

	if activeName, p, ok := cfg.Active(); ok {
		env = append(env,
			"GIMBLE_PROFILE="+activeName,
			"GIMBLE_USER_NAME="+p.Name,
			"GIMBLE_USER_EMAIL="+p.Email,
			"GIMBLE_USER_GITHUB="+p.GitHub,
			"GIMBLE_USER_ACCOUNT_PROVIDER="+profileAccountProvider(p),
			"GIMBLE_WORKSPACE_ROOTS="+strings.Join(p.WorkspaceRoots, ","),
			"GIMBLE_ROS_TYPE="+p.ROSType,
			"GIMBLE_ROS_DISTRO="+p.ROSDistro,
			"GIMBLE_ROS_WORKSPACE="+p.ROSWorkspace,
			"GIMBLE_OBS_GRAFANA_URL="+p.ObsGrafanaURL,
			"GIMBLE_OBS_SENTRY_URL="+p.ObsSentryURL,
			"GIMBLE_SYSTEM_PROMPT_PROFILE="+p.SystemPromptProfile,
			"GIMBLE_NOTIFICATION_PREFERENCE="+p.NotificationPreference,
		)
		promptPrefix = "gimble:" + activeName
		printSessionIntro(activeName, p)
		fmt.Printf("Entering Gimble session as %s (%s, %s). Type 'gim exit' to leave.\n", p.Name, p.Email, profileAccountLabel(p))
	} else {
		printSessionIntro("", profile.Profile{})
		fmt.Printf("Entering Gimble session on %s/%s. Type 'gim exit' to leave.\n", runtime.GOOS, runtime.GOARCH)
		fmt.Println("Tip: update API keys anytime with: gimble keys")
	}

	shellName := filepath.Base(shell)
	switch shellName {
	case "bash":
		env = append(env, fmt.Sprintf("PS1=(%s) \\u@\\h:\\w\\$ ", promptPrefix))
	case "zsh":
		env = append(env, fmt.Sprintf("PROMPT=(%s) %%n@%%m:%%~%%# ", promptPrefix))
	}

	logPath, err := prepareSessionLogPath()
	if err != nil {
		return err
	}
	fmt.Printf("Session logging enabled: %s\n", logPath)
	env = append(env, "GIMBLE_SESSION_LOG_PATH="+logPath)

	cmd, err := newLoggedShellCommand(shell, logPath)
	if err != nil {
		return err
	}
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		if _, ok := err.(*exec.ExitError); ok {
			return nil
		}
		return fmt.Errorf("failed to start Gimble session: %w", err)
	}

	return nil
}

func prepareSessionLogPath() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("failed to resolve user config dir for session logs: %w", err)
	}
	dir := filepath.Join(base, "gimble", "session-logs")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("failed to create session log dir: %w", err)
	}
	name := "session-" + time.Now().Format("20060102-150405") + ".log"
	path := filepath.Join(dir, name)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return "", fmt.Errorf("failed to create session log file: %w", err)
	}
	_ = f.Close()
	latest := filepath.Join(dir, "session-latest.log")
	_ = os.Remove(latest)
	_ = os.Symlink(name, latest)
	return path, nil
}

func findSessionLogSanitizerScript() (string, error) {
	if explicit := strings.TrimSpace(os.Getenv("GIMBLE_LOG_SANITIZER")); explicit != "" {
		if _, err := os.Stat(explicit); err == nil {
			return explicit, nil
		}
		return "", fmt.Errorf("GIMBLE_LOG_SANITIZER points to a missing file: %s", explicit)
	}

	exePath, _ := os.Executable()
	exeDir := filepath.Dir(exePath)
	candidates := []string{
		filepath.Join("python", "session_logging", "sanitizer.py"),
		filepath.Join("python", "session_log_sanitizer.py"),
		filepath.Join(exeDir, "..", "share", "gimble", "python", "session_logging", "sanitizer.py"),
		filepath.Join(exeDir, "..", "share", "gimble", "python", "session_log_sanitizer.py"),
		filepath.Join(exeDir, "python", "session_logging", "sanitizer.py"),
		filepath.Join(exeDir, "python", "session_log_sanitizer.py"),
		filepath.Join(exeDir, "..", "python", "session_logging", "sanitizer.py"),
		filepath.Join(exeDir, "..", "python", "session_log_sanitizer.py"),
	}
	for _, candidate := range candidates {
		if _, err := os.Stat(candidate); err == nil {
			return candidate, nil
		}
	}
	return "", fmt.Errorf("could not locate session log sanitizer script")
}

func findPythonForLogFilter() (string, error) {
	for _, candidate := range []string{"python3", "python"} {
		if p, err := exec.LookPath(candidate); err == nil {
			return p, nil
		}
	}
	return "", fmt.Errorf("python not found for log sanitization")
}

func newLoggedShellCommand(shell string, cleanLogPath string) (*exec.Cmd, error) {
	scriptBin, err := exec.LookPath("script")
	if err != nil {
		return nil, fmt.Errorf("terminal logger 'script' not found; install util-linux (Linux) or BSD script (macOS)")
	}
	pyBin, err := findPythonForLogFilter()
	if err != nil {
		return nil, err
	}
	sanitizerScript, err := findSessionLogSanitizerScript()
	if err != nil {
		return nil, err
	}

	rawLogPath := strings.TrimSuffix(cleanLogPath, ".log") + ".raw.log"
	rawFile, err := os.OpenFile(rawLogPath, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return nil, fmt.Errorf("failed to create raw session log file: %w", err)
	}
	_ = rawFile.Close()

	scriptFlag := "-f"
	if runtime.GOOS == "darwin" {
		scriptFlag = "-F"
	}

	wrapper := `set -eu
RAW="$1"
CLEAN="$2"
SHELL_BIN="$3"
SCRIPT_BIN="$4"
PY_BIN="$5"
SCRIPT_FLAG="$6"
SANITIZER="$7"

"$PY_BIN" -u "$SANITIZER" "$RAW" "$CLEAN" &
SAN_PID=$!

if [ "$(uname -s)" = "Darwin" ]; then
  "$SCRIPT_BIN" -q "$SCRIPT_FLAG" "$RAW" "$SHELL_BIN" -i
else
  "$SCRIPT_BIN" -q "$SCRIPT_FLAG" "$RAW" -c "$SHELL_BIN -i"
fi
STATUS=$?
sleep 0.3
kill "$SAN_PID" >/dev/null 2>&1 || true
wait "$SAN_PID" 2>/dev/null || true
exit "$STATUS"
`

	cmd := exec.Command("sh", "-c", wrapper, "sh", rawLogPath, cleanLogPath, shell, scriptBin, pyBin, scriptFlag, sanitizerScript)
	return cmd, nil
}

func createSessionShimDir() (cleanup func(), shimDir string, err error) {
	exe, err := os.Executable()
	if err != nil {
		return func() {}, "", err
	}

	dir, err := os.MkdirTemp("", "gimble-shim-*")
	if err != nil {
		return func() {}, "", err
	}

	gimScript := fmt.Sprintf("#!/bin/sh\nif [ \"$1\" = \"exit\" ]; then\n  exec %q __session_cmd exit\nfi\nexec %q __session_cmd \"$@\"\n", exe, exe)
	gimPath := filepath.Join(dir, "gim")
	if err := os.WriteFile(gimPath, []byte(gimScript), 0o755); err != nil {
		_ = os.RemoveAll(dir)
		return func() {}, "", err
	}

	blockScript := "#!/bin/sh\necho \"Already inside a Gimble session. Use 'gim exit' to leave.\" 1>&2\nexit 1\n"
	for _, name := range []string{"gimble", "Gimble"} {
		path := filepath.Join(dir, name)
		if err := os.WriteFile(path, []byte(blockScript), 0o755); err != nil {
			_ = os.RemoveAll(dir)
			return func() {}, "", err
		}
	}

	cleanupFn := func() {
		_ = os.RemoveAll(dir)
	}
	return cleanupFn, dir, nil
}

func printHelp() {
	fmt.Print(helpText())
}

func helpText() string {
	return `Usage:
  gimble                     Start Gimble shell session
  gimble session             Start Gimble shell session
  gimble --version           Print version
  gimble setup               Run first-time setup wizard
  gimble keys                Update OpenAI/Groq API keys
  gimble profile <command>   Manage Gimble profiles

Inside a Gimble session, use:
  gim chat                   Start Gimble Cloud session + log uploader
  gim disconnect             Stop Gimble cloud uploader, stay in current Gimble session
  gim exit                   Exit the active Gimble session

Profile Commands:
  gimble profile init --name <name> --email <email> --github <github> [--provider github|gitlab] [--profile <name>]
  gimble profile set --profile <name> [--name <name>] [--email <email>] [--github <github>] [--provider github|gitlab]
  gimble profile list
  gimble profile show [profile]
  gimble profile use <profile>
  gimble profile delete <profile>
`
}

func profileHelpText() string {
	return `Usage:
  gimble profile init --name <name> --email <email> --github <github> [--provider github|gitlab] [--profile <name>]
  gimble profile set --profile <name> [--name <name>] [--email <email>] [--github <github>] [--provider github|gitlab]
  gimble profile list
  gimble profile show [profile]
  gimble profile use <profile>
  gimble profile delete <profile>
`
}
