package agent

import (
	"os"
	"os/exec"
	"path/filepath"
)

// buildClaudeLive decides the tier-1 driver for claude (ADR-0005):
// the Agent SDK shim when available (real permission round-trips),
// falling back to the CLI stdio protocol. The resolver runs once at
// registry construction; the result is captured in the closure.
func buildClaudeLive(cliPath string) func(ref, cwd string) []string {
	shim, sdkPkg := detectSDKShim()
	if shim == "" {
		return claudeLiveCLI(cliPath)
	}
	return func(ref, cwd string) []string {
		argv := []string{"node", shim}
		if ref != "" {
			argv = append(argv, []string{"--", "resume=" + ref}...) // future-proofed; shim reads env today
		}
		// env is set by ensureLive via cmd.Env (see below); the shim
		// reads AGENTDECK_SDK_RESUME / AGENTDECK_SDK_PERMISSION_MODE.
		_ = sdkPkg
		return argv
	}
}

// detectSDKShim finds node + the vendored shim + the SDK package.
// Layout: <repo>/agent-sdk-shim/{shim.mjs,node_modules/@anthropic-ai/...}.
// Resolution order: AGENTDECK_SDK_SHIM (explicit), exe dir, cwd.
func detectSDKShim() (shim, sdkPkg string) {
	if p := os.Getenv("AGENTDECK_SDK_SHIM"); p != "" {
		if fileExists(p) {
			return p, ""
		}
	}
	if _, err := exec.LookPath("node"); err != nil {
		return "", ""
	}
	candidates := []string{}
	if exe, err := os.Executable(); err == nil {
		candidates = append(candidates, filepath.Join(filepath.Dir(exe), "agent-sdk-shim"))
	}
	if wd, err := os.Getwd(); err == nil {
		candidates = append(candidates, filepath.Join(wd, "agent-sdk-shim"))
	}
	for _, dir := range candidates {
		s := filepath.Join(dir, "shim.mjs")
		if fileExists(s) && fileExists(filepath.Join(dir, "node_modules", "@anthropic-ai", "claude-agent-sdk")) {
			return s, dir
		}
	}
	return "", ""
}

func fileExists(p string) bool {
	st, err := os.Stat(p)
	return err == nil && !st.IsDir()
}

// claudeLiveCLI is the pre-ADR-0005 fallback: the CLI's own
// stream-json bidirectional mode (no real permission round-trips).
func claudeLiveCLI(p string) func(ref, cwd string) []string {
	return func(ref, cwd string) []string {
		argv := []string{p, "-p", "--input-format", "stream-json",
			"--output-format", "stream-json", "--verbose",
			"--add-dir", homeDir()}
		if ref != "" {
			argv = append(argv, "--resume", ref)
		}
		return argv
	}
}
