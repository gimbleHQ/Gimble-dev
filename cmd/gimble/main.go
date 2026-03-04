package main

import (
	"embed"
	"flag"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gimble-dev/gimble/internal/platform"
	"github.com/gimble-dev/gimble/internal/profile"
)

var version = "dev"

//go:embed web/chat/index.html
var chatAssets embed.FS

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

	inSession := os.Getenv("GIMBLE_SESSION") == "1"

	if inSession && len(args) == 0 {
		return fmt.Errorf("already inside a Gimble session; use 'exit' to leave")
	}

	if len(args) == 0 {
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
			return fmt.Errorf("already inside a Gimble session; use 'exit' to leave")
		}
		return runSession()
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
		return runChat(args[1:])
	default:
		return fmt.Errorf("unknown session command %q", args[0])
	}
}

func runChat(args []string) error {
	fs := flag.NewFlagSet("chat", flag.ContinueOnError)
	fs.SetOutput(os.Stderr)
	port := fs.Int("port", 5555, "preferred port")
	if err := fs.Parse(args); err != nil {
		return err
	}

	if *port < 0 || *port > 65535 {
		return fmt.Errorf("invalid port: %d", *port)
	}

	ln, actualPort, err := listenWithFallback(*port)
	if err != nil {
		return err
	}
	defer ln.Close()

	indexBytes, err := fsReadFile(chatAssets, "web/chat/index.html")
	if err != nil {
		return fmt.Errorf("failed to load chat UI: %w", err)
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(indexBytes)
	})

	url := fmt.Sprintf("http://localhost:%d", actualPort)
	// Print an OSC 8 hyperlink where supported, with the plain URL as a fallback.
	// OSC 8 sequence: ESC ] 8 ; ; <url> ESC \\ <text> ESC ] 8 ; ; ESC \\
	fmt.Printf("Gimble chat UI: %s\n", makeHyperlink(url)+" ("+url+")")
	fmt.Println("Open this URL in your browser. Press Ctrl+C to stop.")

	server := &http.Server{Handler: mux}
	if err := server.Serve(ln); err != nil && err != http.ErrServerClosed {
		return fmt.Errorf("chat server error: %w", err)
	}

	return nil
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

func fsReadFile(fsys fs.FS, name string) ([]byte, error) {
	data, err := fs.ReadFile(fsys, name)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// makeHyperlink returns an OSC 8 hyperlink sequence for terminals that support it.
// The returned string will display the URL as clickable text when supported.
func makeHyperlink(url string) string {
	// ESC ] 8 ; ; <url> ESC \ <text> ESC ] 8 ; ; ESC \
	return "\x1b]8;;" + url + "\x1b\\" + url + "\x1b]8;;\x1b\\"
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
		fmt.Printf("%s %s\t%s\t@%s\n", prefix, name, p.Email, p.GitHub)
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
	fmt.Printf("github:  @%s\n", p.GitHub)
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

	if strings.TrimSpace(p.Name) == "" || strings.TrimSpace(p.Email) == "" || strings.TrimSpace(p.GitHub) == "" {
		return fmt.Errorf("profile %q must include name, email, and github (use --name, --email, --github)", normalizedName)
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
		Name:   strings.TrimSpace(*name),
		Email:  strings.TrimSpace(*email),
		GitHub: profile.NormalizeGitHub(*github),
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
		)
		promptPrefix = "gimble:" + activeName
		fmt.Printf("Entering Gimble session as %s (%s, @%s). Type 'exit' to leave.\n", p.Name, p.Email, p.GitHub)
	} else {
		fmt.Printf("Entering Gimble session on %s/%s. Type 'exit' to leave.\n", runtime.GOOS, runtime.GOARCH)
		fmt.Println("Tip: initialize a profile with: gimble profile init --name \"Your Name\" --email you@example.com --github yourhandle")
	}

	shellName := filepath.Base(shell)
	switch shellName {
	case "bash":
		env = append(env, fmt.Sprintf("PS1=(%s) \\u@\\h:\\w\\$ ", promptPrefix))
	case "zsh":
		env = append(env, fmt.Sprintf("PROMPT=(%s) %%n@%%m:%%~%%# ", promptPrefix))
	}

	cmd := exec.Command(shell, "-i")
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

func createSessionShimDir() (cleanup func(), shimDir string, err error) {
	exe, err := os.Executable()
	if err != nil {
		return func() {}, "", err
	}

	dir, err := os.MkdirTemp("", "gimble-shim-*")
	if err != nil {
		return func() {}, "", err
	}

	gimScript := fmt.Sprintf("#!/bin/sh\nexec %q __session_cmd \"$@\"\n", exe)
	gimPath := filepath.Join(dir, "gim")
	if err := os.WriteFile(gimPath, []byte(gimScript), 0o755); err != nil {
		_ = os.RemoveAll(dir)
		return func() {}, "", err
	}

	blockScript := "#!/bin/sh\necho \"Already inside a Gimble session. Use 'exit' to leave.\" 1>&2\nexit 1\n"
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
  gimble profile <command>   Manage Gimble profiles

Inside a Gimble session, use:
  gim chat                   Start ChatGPT-style local chat UI server

Profile Commands:
  gimble profile init --name <name> --email <email> --github <github> [--profile <name>]
  gimble profile set --profile <name> [--name <name>] [--email <email>] [--github <github>]
  gimble profile list
  gimble profile show [profile]
  gimble profile use <profile>
  gimble profile delete <profile>
`
}

func profileHelpText() string {
	return `Usage:
  gimble profile init --name <name> --email <email> --github <github> [--profile <name>]
  gimble profile set --profile <name> [--name <name>] [--email <email>] [--github <github>]
  gimble profile list
  gimble profile show [profile]
  gimble profile use <profile>
  gimble profile delete <profile>
`
}
