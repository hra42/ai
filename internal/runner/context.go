package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
)

const (
	listingLimit = 50
)

// sensitivePatterns matches filenames that should never appear in the working
// directory listing. Curated from the high-signal filename rules used by
// gitleaks (config/gitleaks.toml) and the awslabs/git-secrets defaults; the
// ecosystem has no shared library for this — the well-known scanners are
// content scanners, not filename redactors.
var sensitivePatterns = []string{
	// dotenv & app secrets
	".env", ".env.*", "*.env", "secrets.yaml", "secrets.yml", "secrets.json",
	"credentials", "credentials.*", "*.credentials",
	// PEM-style keys / certs
	"*.pem", "*.key", "*.p12", "*.pfx", "*.pkcs12", "*.asc", "*.gpg", "*.kdbx",
	"*.jks", "*.keystore", "*.cer", "*.crt", "*.csr",
	// SSH
	"id_rsa*", "id_ed25519*", "id_ecdsa*", "id_dsa*", "known_hosts", "authorized_keys",
	// shell history & creds files
	".netrc", ".pgpass", ".npmrc", ".pypirc", ".dockercfg", ".docker",
	".bash_history", ".zsh_history", ".python_history",
	// cloud provider creds dirs/files
	".aws", ".gcp", ".azure", ".kube",
	"gcloud-service-key.json", "service-account*.json",
	// app-specific
	"*.token", "*token*", "*.secret", "*secret*",
	"jwt.txt", "*.jwt",
	// node / js
	".yarnrc", ".yarnrc.yml", ".npm-token", "firebase-adminsdk*.json",
	// php
	".htpasswd", "auth.json",
	// terraform
	".terraformrc", "*.tfstate", "*.tfstate.backup",
	// vpn / mobile
	"*.ovpn", "*.mobileprovision",
	"wg*.conf", "wireguard", "wg-*.conf",
}

// gatherWorkContext collects best-effort cwd and git context. Errors are
// swallowed (returns empty string) — context is a nice-to-have, not required.
// When redact is true, filenames matching sensitivePatterns are replaced with
// "[redacted]" before being sent.
func gatherWorkContext(ctx context.Context, redact bool) string {
	cwd, err := os.Getwd()
	if err != nil {
		return ""
	}

	var b strings.Builder
	fmt.Fprintf(&b, "--- working directory ---\n%s\n", cwd)

	if listing := listDir(ctx, cwd, redact); listing != "" {
		b.WriteString(listing)
	}

	if git := gitStatus(ctx, cwd); git != "" {
		b.WriteString("\n--- git ---\n")
		b.WriteString(git)
	}

	return strings.TrimRight(b.String(), "\n")
}

// listDir returns a formatted top-level listing of cwd. Inside a git repo it
// uses `git ls-files` so .gitignore is honored; outside, it falls back to a
// raw os.ReadDir. Sensitive filenames are redacted in either path.
func listDir(ctx context.Context, cwd string, redact bool) string {
	entries, fromGit := gitListing(ctx, cwd)
	if entries == nil {
		entries = readDirListing(cwd)
		fromGit = false
	}
	if len(entries) == 0 {
		return ""
	}
	sort.Slice(entries, func(i, j int) bool { return entries[i].name < entries[j].name })

	var b strings.Builder
	total := len(entries)
	shown := total
	if shown > listingLimit {
		shown = listingLimit
	}
	if fromGit {
		fmt.Fprintf(&b, "%d entries (gitignore respected):\n", total)
	} else {
		fmt.Fprintf(&b, "%d entries:\n", total)
	}
	for i := 0; i < shown; i++ {
		e := entries[i]
		name := e.name
		if redact && isSensitive(name) {
			name = "[redacted]"
		}
		switch {
		case e.isDir:
			fmt.Fprintf(&b, "  %s/\n", name)
		case e.isExec:
			fmt.Fprintf(&b, "  %s*\n", name)
		case e.isLink:
			fmt.Fprintf(&b, "  %s@\n", name)
		default:
			fmt.Fprintf(&b, "  %s\n", name)
		}
	}
	if total > shown {
		fmt.Fprintf(&b, "  … and %d more\n", total-shown)
	}
	return b.String()
}

type dirEntry struct {
	name   string
	isDir  bool
	isExec bool
	isLink bool
}

func readDirListing(cwd string) []dirEntry {
	es, err := os.ReadDir(cwd)
	if err != nil {
		return nil
	}
	out := make([]dirEntry, 0, len(es))
	for _, e := range es {
		info, err := e.Info()
		if err != nil {
			continue
		}
		out = append(out, dirEntry{
			name:   e.Name(),
			isDir:  e.IsDir(),
			isExec: !e.IsDir() && info.Mode()&0o111 != 0,
			isLink: info.Mode()&os.ModeSymlink != 0,
		})
	}
	return out
}

// gitListing returns the top-level entries that git considers visible
// (tracked + non-ignored untracked), or nil when not in a repo. Top-level
// only — nested files are aggregated under their top-level directory.
func gitListing(ctx context.Context, cwd string) ([]dirEntry, bool) {
	if _, err := exec.LookPath("git"); err != nil {
		return nil, false
	}
	check := exec.CommandContext(ctx, "git", "rev-parse", "--show-toplevel")
	check.Dir = cwd
	if err := check.Run(); err != nil {
		return nil, false
	}
	cmd := exec.CommandContext(ctx, "git", "ls-files", "-z", "--cached", "--others", "--exclude-standard")
	cmd.Dir = cwd
	out, err := cmd.Output()
	if err != nil {
		return nil, false
	}
	seen := map[string]bool{}
	var entries []dirEntry
	for _, p := range strings.Split(strings.TrimRight(string(out), "\x00"), "\x00") {
		if p == "" {
			continue
		}
		// Top-level segment only.
		if i := strings.IndexByte(p, '/'); i >= 0 {
			name := p[:i]
			if seen[name] {
				continue
			}
			seen[name] = true
			entries = append(entries, dirEntry{name: name, isDir: true})
			continue
		}
		if seen[p] {
			continue
		}
		seen[p] = true
		// Stat to learn about exec bit / symlinks.
		info, err := os.Lstat(filepath.Join(cwd, p))
		if err != nil {
			entries = append(entries, dirEntry{name: p})
			continue
		}
		entries = append(entries, dirEntry{
			name:   p,
			isExec: info.Mode()&0o111 != 0,
			isLink: info.Mode()&os.ModeSymlink != 0,
		})
	}
	return entries, true
}

// gitStatus returns "branch: X, status: clean" or "branch: X, status: M=2 ?=1"
// without leaking any actual file paths.
func gitStatus(ctx context.Context, cwd string) string {
	if _, err := exec.LookPath("git"); err != nil {
		return ""
	}
	branchCmd := exec.CommandContext(ctx, "git", "rev-parse", "--abbrev-ref", "HEAD")
	branchCmd.Dir = cwd
	branch, err := branchCmd.Output()
	if err != nil {
		return ""
	}
	statusCmd := exec.CommandContext(ctx, "git", "status", "--porcelain=v1")
	statusCmd.Dir = cwd
	status, err := statusCmd.Output()
	if err != nil {
		return ""
	}
	var modified, added, deleted, untracked int
	for _, line := range strings.Split(strings.TrimRight(string(status), "\n"), "\n") {
		if len(line) < 2 {
			continue
		}
		code := line[:2]
		switch {
		case code == "??":
			untracked++
		case strings.ContainsAny(code, "M"):
			modified++
		case strings.ContainsAny(code, "A"):
			added++
		case strings.ContainsAny(code, "D"):
			deleted++
		}
	}
	br := strings.TrimSpace(string(branch))
	if modified+added+deleted+untracked == 0 {
		return fmt.Sprintf("branch: %s, status: clean\n", br)
	}
	return fmt.Sprintf("branch: %s, status: M=%d A=%d D=%d ?=%d\n", br, modified, added, deleted, untracked)
}

func isSensitive(name string) bool {
	lower := strings.ToLower(name)
	for _, pat := range sensitivePatterns {
		if ok, _ := filepath.Match(strings.ToLower(pat), lower); ok {
			return true
		}
	}
	return false
}
