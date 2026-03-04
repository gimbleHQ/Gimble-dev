package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/gimble-dev/gimble/internal/platform"
	"github.com/gimble-dev/gimble/internal/profile"
)

var version = "dev"

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

	if len(args) == 0 {
		return runSession()
	}

	switch args[0] {
	case "--version", "-version", "-v":
		fmt.Printf("gimble %s\n", version)
		return nil
	case "help", "--help", "-h":
		printHelp()
		return nil
	case "session":
		return runSession()
	case "profile":
		return runProfile(args[1:])
	default:
		return fmt.Errorf("unknown command %q\n\n%s", args[0], helpText())
	}
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
		env = append(env, fmt.Sprintf("PS1=(%s) ${PS1:-\\u@\\h:\\w\\$ }", promptPrefix))
	case "zsh":
		env = append(env, fmt.Sprintf("PROMPT=(%s) %%n@%%m:%%~%%# ", promptPrefix))
	}

	cmd := exec.Command(shell, "-i")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Env = env

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to start Gimble session: %w", err)
	}

	return nil
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
