package rsidecar

import (
	"bytes"
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
	"unicode/utf8"
)

const (
	rsidecarWorkSubdir  = ".vpro_rsidecar_work"
	rsidecarHistoryName = "__VPRO_RSidecarHistory"
	rsidecarHistoryDDL  = `CREATE TABLE IF NOT EXISTS main."__VPRO_RSidecarHistory" (
		"ID" INTEGER PRIMARY KEY AUTOINCREMENT,
		"CreatedAt" TEXT NOT NULL,
		"Action" TEXT NOT NULL,
		"DetailsJSON" TEXT NOT NULL
	)`
)

// RuntimeOptions configures optional path discovery overrides for out-of-process R and Quarto execution.
type RuntimeOptions struct {
	RscriptPath string
	QuartoPath  string
	WorkRootDir string
	LookupEnv   func(string) (string, bool)
}

// RPackageStatus reports installation and version status for a single R package.
type RPackageStatus struct {
	Name      string `json:"name"`
	Installed bool   `json:"installed"`
	Version   string `json:"version"`
	Required  bool   `json:"required"`
}

// RRuntimeStatus reports local Rscript, Quarto CLI, and BEC_R/climr package readiness.
type RRuntimeStatus struct {
	Available          bool             `json:"available"`
	RscriptPath        string           `json:"rscriptPath"`
	RVersion           string           `json:"rVersion"`
	RVersionFull       string           `json:"rVersionFull"`
	Platform           string           `json:"platform"`
	QuartoAvailable    bool             `json:"quartoAvailable"`
	QuartoPath         string           `json:"quartoPath"`
	QuartoVersion      string           `json:"quartoVersion"`
	ClimrCachePath     string           `json:"climrCachePath"`
	ClimrRastersCached bool             `json:"climrRastersCached"`
	ReadyForClimate    bool             `json:"readyForClimate"`
	ReadyForClustering bool             `json:"readyForClustering"`
	ReadyForQuarto     bool             `json:"readyForQuarto"`
	MissingRequired    []string         `json:"missingRequired"`
	Packages           []RPackageStatus `json:"packages"`
	SetupInstructions  []string         `json:"setupInstructions"`
	DiagnosticMessage  string           `json:"diagnosticMessage,omitempty"`
}

func effectiveOptions(opts ...RuntimeOptions) RuntimeOptions {
	var out RuntimeOptions
	if len(opts) > 0 {
		out = opts[0]
	}
	if out.LookupEnv == nil {
		out.LookupEnv = os.LookupEnv
	}
	return out
}

func isExecutableRegularFile(p string) bool {
	if strings.TrimSpace(p) == "" {
		return false
	}
	info, err := os.Stat(p)
	if err != nil || !info.Mode().IsRegular() {
		return false
	}
	return true
}

func resolveRscriptBinary(opts RuntimeOptions) (string, bool, error) {
	if explicit := strings.TrimSpace(opts.RscriptPath); explicit != "" {
		if strings.EqualFold(explicit, "none") || strings.EqualFold(explicit, "disabled") {
			return "", true, fmt.Errorf("Rscript disabled via explicit configuration (%q)", explicit)
		}
		if !isExecutableRegularFile(explicit) {
			return "", true, fmt.Errorf("configured Rscript binary was not found at %q", explicit)
		}
		abs, err := filepath.Abs(explicit)
		if err != nil {
			return explicit, true, nil
		}
		return abs, true, nil
	}

	if envVal, ok := opts.LookupEnv("VPRO_RSCRIPT_PATH"); ok && strings.TrimSpace(envVal) != "" {
		trimmed := strings.TrimSpace(envVal)
		if strings.EqualFold(trimmed, "none") || strings.EqualFold(trimmed, "disabled") {
			return "", true, fmt.Errorf("Rscript disabled via VPRO_RSCRIPT_PATH=%q", trimmed)
		}
		if !isExecutableRegularFile(trimmed) {
			return "", true, fmt.Errorf("VPRO_RSCRIPT_PATH binary was not found at %q", trimmed)
		}
		abs, err := filepath.Abs(trimmed)
		if err != nil {
			return trimmed, true, nil
		}
		return abs, true, nil
	}

	if lp, err := exec.LookPath("Rscript"); err == nil && isExecutableRegularFile(lp) {
		return lp, false, nil
	}

	var candidates []string
	if runtime.GOOS == "windows" {
		for _, root := range []string{`C:\Program Files\R`, `C:\Users\Public\R`} {
			matches, _ := filepath.Glob(filepath.Join(root, "R-*", "bin", "x64", "Rscript.exe"))
			sort.Sort(sort.Reverse(sort.StringSlice(matches)))
			candidates = append(candidates, matches...)
			matchesGeneric, _ := filepath.Glob(filepath.Join(root, "R-*", "bin", "Rscript.exe"))
			sort.Sort(sort.Reverse(sort.StringSlice(matchesGeneric)))
			candidates = append(candidates, matchesGeneric...)
		}
	} else {
		candidates = append(candidates,
			"/usr/bin/Rscript",
			"/usr/local/bin/Rscript",
			"/opt/homebrew/bin/Rscript",
		)
		optMatches, _ := filepath.Glob("/opt/R/*/bin/Rscript")
		sort.Sort(sort.Reverse(sort.StringSlice(optMatches)))
		candidates = append(candidates, optMatches...)
	}

	for _, c := range candidates {
		if isExecutableRegularFile(c) {
			return c, false, nil
		}
	}
	return "", false, errors.New("Rscript was not found on PATH or in standard R installation directories")
}

func preferWindowsQuartoExe(p string) string {
	if runtime.GOOS == "windows" {
		ext := strings.ToLower(filepath.Ext(p))
		if ext == ".cmd" || ext == ".bat" {
			exe := strings.TrimSuffix(p, filepath.Ext(p)) + ".exe"
			if isExecutableRegularFile(exe) {
				return exe
			}
		}
	}
	return p
}

func resolveSystemQuartoBinary(lookupEnv func(string) (string, bool)) string {
	if lookupEnv != nil {
		if envVal, ok := lookupEnv("VPRO_QUARTO_PATH"); ok && strings.TrimSpace(envVal) != "" {
			trimmed := strings.TrimSpace(envVal)
			if !strings.EqualFold(trimmed, "none") && !strings.EqualFold(trimmed, "disabled") && isExecutableRegularFile(trimmed) {
				if abs, err := filepath.Abs(trimmed); err == nil {
					return preferWindowsQuartoExe(abs)
				}
				return preferWindowsQuartoExe(trimmed)
			}
		}
	}
	if lp, err := exec.LookPath("quarto"); err == nil && isExecutableRegularFile(lp) {
		return preferWindowsQuartoExe(lp)
	}
	var candidates []string
	if runtime.GOOS == "windows" {
		candidates = append(candidates, `C:\Program Files\Quarto\bin\quarto.exe`)
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			candidates = append(candidates,
				filepath.Join(home, "AppData", "Local", "Programs", "Quarto", "bin", "quarto.exe"),
				filepath.Join(home, "tools", "quarto", "bin", "quarto.exe"),
			)
		}
	} else {
		candidates = append(candidates,
			"/usr/local/bin/quarto",
			"/usr/bin/quarto",
			"/opt/quarto/bin/quarto",
		)
		if home, err := os.UserHomeDir(); err == nil && home != "" {
			candidates = append(candidates, filepath.Join(home, ".local", "bin", "quarto"))
		}
	}
	for _, c := range candidates {
		if isExecutableRegularFile(c) {
			return preferWindowsQuartoExe(c)
		}
	}
	return ""
}

func resolveQuartoBinary(opts RuntimeOptions) string {
	if explicit := strings.TrimSpace(opts.QuartoPath); explicit != "" {
		if strings.EqualFold(explicit, "none") || strings.EqualFold(explicit, "disabled") {
			return ""
		}
		if isExecutableRegularFile(explicit) {
			if abs, err := filepath.Abs(explicit); err == nil {
				return preferWindowsQuartoExe(abs)
			}
			return preferWindowsQuartoExe(explicit)
		}
		return ""
	}
	if opts.LookupEnv != nil {
		if envVal, ok := opts.LookupEnv("VPRO_QUARTO_PATH"); ok && strings.TrimSpace(envVal) != "" {
			trimmed := strings.TrimSpace(envVal)
			if strings.EqualFold(trimmed, "none") || strings.EqualFold(trimmed, "disabled") {
				return ""
			}
			if isExecutableRegularFile(trimmed) {
				if abs, err := filepath.Abs(trimmed); err == nil {
					return preferWindowsQuartoExe(abs)
				}
				return preferWindowsQuartoExe(trimmed)
			}
			return ""
		}
	}
	return resolveSystemQuartoBinary(opts.LookupEnv)
}

func hasPandocBinaryInDir(dir string) bool {
	if strings.TrimSpace(dir) == "" {
		return false
	}
	for _, name := range []string{"pandoc.exe", "pandoc"} {
		if isExecutableRegularFile(filepath.Join(dir, name)) {
			return true
		}
	}
	return false
}

func pandocOnPath(opts RuntimeOptions) bool {
	if opts.LookupEnv != nil {
		if pathVal, ok := opts.LookupEnv("PATH"); ok {
			for _, dir := range filepath.SplitList(pathVal) {
				if hasPandocBinaryInDir(dir) {
					return true
				}
			}
			return false
		}
	}
	lp, err := exec.LookPath("pandoc")
	return err == nil && isExecutableRegularFile(lp)
}

// ResolveBundledPandocDir locates Quarto's bundled pandoc directory (<quarto_bin>/tools or <quarto_root>/bin/tools)
// when RSTUDIO_PANDOC is unset and pandoc is not already on PATH, so rmarkdown::render() succeeds on Windows.
func ResolveBundledPandocDir(opts RuntimeOptions) string {
	cfg := effectiveOptions(opts)
	if val, ok := cfg.LookupEnv("RSTUDIO_PANDOC"); ok && strings.TrimSpace(val) != "" {
		return ""
	}
	if pandocOnPath(cfg) {
		return ""
	}

	var quartoCandidates []string
	if q := resolveQuartoBinary(cfg); q != "" {
		quartoCandidates = append(quartoCandidates, q)
	}
	if sysQ := resolveSystemQuartoBinary(cfg.LookupEnv); sysQ != "" && (len(quartoCandidates) == 0 || quartoCandidates[0] != sysQ) {
		quartoCandidates = append(quartoCandidates, sysQ)
	}

	var toolDirs []string
	for _, qBin := range quartoCandidates {
		resolved := qBin
		if sym, err := filepath.EvalSymlinks(qBin); err == nil && sym != "" {
			resolved = sym
		}
		binDir := filepath.Dir(resolved)
		toolDirs = append(toolDirs,
			filepath.Join(binDir, "tools"),
			filepath.Join(filepath.Dir(binDir), "bin", "tools"),
			filepath.Join(binDir, "tools", "x86_64"),
			filepath.Join(binDir, "tools", "amd64"),
			filepath.Join(binDir, "tools", "aarch64"),
			filepath.Join(binDir, "tools", "arm64"),
		)
	}
	if runtime.GOOS == "windows" {
		toolDirs = append(toolDirs,
			`C:\Program Files\Pandoc`,
			`C:\Program Files\RStudio\resources\app\bin\quarto\bin\tools`,
		)
	}
	for _, d := range toolDirs {
		if hasPandocBinaryInDir(d) {
			return d
		}
	}
	return ""
}

func defaultSetupInstructions(missingR bool, missingQuarto bool, missingPkgs []string) []string {
	var steps []string
	if missingR {
		steps = append(steps,
			"Install R (4.3 or newer) from https://cloud.r-project.org/ so Rscript is available on PATH or under C:\\Program Files\\R\\.",
			"Alternatively set VPRO_RSCRIPT_PATH to the full path of Rscript / Rscript.exe.",
		)
	}
	if len(missingPkgs) > 0 {
		quoted := make([]string, 0, len(missingPkgs))
		for _, p := range missingPkgs {
			quoted = append(quoted, fmt.Sprintf("%q", p))
		}
		steps = append(steps, fmt.Sprintf(
			"Install required R packages in R: install.packages(c(%s), repos = 'https://cloud.r-project.org')",
			strings.Join(quoted, ", "),
		))
	}
	if missingQuarto {
		steps = append(steps,
			"Optional for Quarto CLI rendering: install Quarto from https://quarto.org/docs/get-started/ (VPro falls back to R rmarkdown/knitr standalone HTML when Quarto CLI is absent).",
		)
	}
	return steps
}

// prepareIsolatedRunDir creates a clean per-invocation work directory under baseDir/.vpro_rsidecar_work
// (never touching system temporary directories) and returns runDir, spaceFreeTmpDir, plus cleanup closure.
func prepareIsolatedRunDir(baseDir string) (string, string, func(), error) {
	root := strings.TrimSpace(baseDir)
	if root == "" {
		cwd, err := os.Getwd()
		if err != nil {
			return "", "", nil, fmt.Errorf("resolve working directory for R sidecar: %w", err)
		}
		root = cwd
	}
	absRoot, err := filepath.Abs(root)
	if err != nil {
		return "", "", nil, fmt.Errorf("resolve absolute work root %q: %w", root, err)
	}
	parent := filepath.Join(absRoot, rsidecarWorkSubdir)
	if err := os.MkdirAll(parent, 0700); err != nil {
		return "", "", nil, fmt.Errorf("create R sidecar work parent %q: %w", parent, err)
	}
	var token [8]byte
	if _, err := rand.Read(token[:]); err != nil {
		return "", "", nil, fmt.Errorf("generate R sidecar run token: %w", err)
	}
	runToken := "run-" + hex.EncodeToString(token[:])
	runDir := filepath.Join(parent, runToken)
	tmpDir := filepath.Join(runDir, "tmp")
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		return "", "", nil, fmt.Errorf("create R sidecar run directory: %w", err)
	}

	// GNU R aborts at C startup ("Fatal error: 'R_TempDir' contains space") if TMPDIR contains whitespace.
	// Keep script/JSON/data files in runDir (even with spaces!) while ensuring R's session TMPDIR is space-free.
	altTmpParent := ""
	altTmpDir := ""
	if strings.ContainsAny(tmpDir, " \t") {
		cacheRoot, cErr := os.UserCacheDir()
		if cErr != nil || strings.ContainsAny(cacheRoot, " \t") {
			if cwd, wErr := os.Getwd(); wErr == nil && !strings.ContainsAny(cwd, " \t") {
				cacheRoot = cwd
			}
		}
		if cacheRoot != "" && !strings.ContainsAny(cacheRoot, " \t") {
			altTmpParent = filepath.Join(cacheRoot, "vpro-rsidecar-tmp")
			altTmpDir = filepath.Join(altTmpParent, runToken)
			if mkErr := os.MkdirAll(altTmpDir, 0700); mkErr == nil {
				tmpDir = altTmpDir
			} else {
				altTmpDir = ""
			}
		}
	}

	cleanup := func() {
		_ = os.RemoveAll(runDir)
		_ = os.Remove(parent)
		if altTmpDir != "" {
			_ = os.RemoveAll(altTmpDir)
		}
		if altTmpParent != "" {
			_ = os.Remove(altTmpParent)
		}
	}
	return runDir, tmpDir, cleanup, nil
}

func buildIsolatedChildEnv(tmpDir string) []string {
	base := os.Environ()
	filtered := make([]string, 0, len(base)+4)
	stripTmp := !strings.ContainsAny(tmpDir, " \t")
	for _, kv := range base {
		upper := strings.ToUpper(kv)
		if strings.HasPrefix(upper, "QUARTO_PRINT_STACK=") {
			continue
		}
		if stripTmp && (strings.HasPrefix(upper, "TMPDIR=") ||
			strings.HasPrefix(upper, "TEMP=") ||
			strings.HasPrefix(upper, "TMP=")) {
			continue
		}
		filtered = append(filtered, kv)
	}
	if stripTmp && tmpDir != "" {
		filtered = append(filtered,
			"TMPDIR="+tmpDir,
			"TEMP="+tmpDir,
			"TMP="+tmpDir,
		)
	}
	filtered = append(filtered, "QUARTO_PRINT_STACK=true")
	return filtered
}

func dirForOS(goOS, p string) string {
	if goOS == "windows" {
		idx := strings.LastIndexAny(p, `\/`)
		if idx <= 0 {
			return "."
		}
		if idx == 2 && p[1] == ':' {
			return p[:3]
		}
		return p[:idx]
	}
	return filepath.Dir(p)
}

// ConfigureChildREnv prepends filepath.Dir(rscriptBin) case-insensitively to PATH/Path,
// configures QUARTO_R safely per OS (omitting QUARTO_R on Windows where Quarto CLI's rBinaryPath
// drops .exe when QUARTO_R is set, relying instead on prepended Path + PATHEXT), and populates
// RSTUDIO_PANDOC when pandocDir is provided and RSTUDIO_PANDOC is unset.
func ConfigureChildREnv(baseEnv []string, goOS, rscriptBin, pandocDir string) []string {
	rBinDir := ""
	if strings.TrimSpace(rscriptBin) != "" {
		rBinDir = dirForOS(goOS, rscriptBin)
	}
	out := make([]string, 0, len(baseEnv)+4)
	pathUpdated := false
	hasRStudioPandoc := false
	hasPathExt := false
	sep := string(os.PathListSeparator)
	if goOS == "windows" {
		sep = ";"
	}

	for _, kv := range baseEnv {
		eq := strings.IndexByte(kv, '=')
		if eq <= 0 {
			out = append(out, kv)
			continue
		}
		keyUpper := strings.ToUpper(kv[:eq])
		val := kv[eq+1:]
		switch keyUpper {
		case "QUARTO_R":
			// Strip inherited QUARTO_R; non-Windows re-adds exact rscriptBin below,
			// while Windows uses prepended Path so Quarto's `where Rscript` resolves Rscript.exe.
			continue
		case "PATH":
			if rBinDir != "" && !pathUpdated {
				out = append(out, kv[:eq+1]+rBinDir+sep+val)
				pathUpdated = true
			} else {
				out = append(out, kv)
			}
		case "RSTUDIO_PANDOC":
			if strings.TrimSpace(val) != "" {
				hasRStudioPandoc = true
				out = append(out, kv)
			}
		case "PATHEXT":
			hasPathExt = true
			out = append(out, kv)
		default:
			out = append(out, kv)
		}
	}

	if rBinDir != "" && !pathUpdated {
		out = append(out, "PATH="+rBinDir)
	}
	if goOS == "windows" && !hasPathExt {
		out = append(out, "PATHEXT=.COM;.EXE;.BAT;.CMD")
	}
	if goOS != "windows" && strings.TrimSpace(rscriptBin) != "" {
		out = append(out, "QUARTO_R="+rscriptBin)
	}
	if !hasRStudioPandoc && strings.TrimSpace(pandocDir) != "" {
		out = append(out, "RSTUDIO_PANDOC="+pandocDir)
	}
	return out
}

func stageEmbeddedAsset(runDir, assetName string) (string, error) {
	raw, err := embeddedAssets.ReadFile("assets/" + assetName)
	if err != nil {
		return "", fmt.Errorf("read embedded R sidecar asset %q: %w", assetName, err)
	}
	dest := filepath.Join(runDir, assetName)
	if err := os.WriteFile(dest, raw, 0600); err != nil {
		return "", fmt.Errorf("stage R sidecar asset %q: %w", assetName, err)
	}
	return dest, nil
}

// CheckpointProjectWAL flushes pending SQLite WAL frames passively so out-of-process R/Quarto readers
// observe all committed transactions on <Project>.db.
func CheckpointProjectWAL(ctx context.Context, db *sql.DB) error {
	if ctx == nil {
		return errors.New("WAL checkpoint requires a context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}
	if db == nil {
		return errors.New("project database handle is nil")
	}
	rows, err := db.QueryContext(ctx, "PRAGMA wal_checkpoint(PASSIVE);")
	if err != nil {
		return fmt.Errorf("checkpoint project SQLite WAL: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var busy, logFrames, checkpointed int
		if scanErr := rows.Scan(&busy, &logFrames, &checkpointed); scanErr != nil {
			break
		}
	}
	return rows.Err()
}

// executeRScriptJSON stages an embedded .R script + optional asset files, passes strict argv
// (rscriptBin, "--vanilla", scriptPath, reqPath, respPath) with zero shell interpolation,
// enforces context cancellation, and strictly validates the UTF-8 JSON response.
func executeRScriptJSON(
	ctx context.Context,
	opts RuntimeOptions,
	workBaseDir string,
	scriptAsset string,
	extraAssets []string,
	requestPayload any,
	outResponse any,
) error {
	if ctx == nil {
		return errors.New("R sidecar execution requires a non-nil context")
	}
	if err := ctx.Err(); err != nil {
		return err
	}

	rscriptBin, _, err := resolveRscriptBinary(opts)
	if err != nil {
		return fmt.Errorf("R runtime unavailable: %w", err)
	}

	baseDir := strings.TrimSpace(opts.WorkRootDir)
	if baseDir == "" {
		baseDir = workBaseDir
	}
	runDir, tmpDir, cleanup, err := prepareIsolatedRunDir(baseDir)
	if err != nil {
		return err
	}
	defer cleanup()

	scriptPath, err := stageEmbeddedAsset(runDir, scriptAsset)
	if err != nil {
		return err
	}
	for _, extra := range extraAssets {
		if _, err := stageEmbeddedAsset(runDir, extra); err != nil {
			return err
		}
	}

	reqBytes, err := json.Marshal(requestPayload)
	if err != nil {
		return fmt.Errorf("marshal R sidecar request JSON: %w", err)
	}
	reqPath := filepath.Join(runDir, "request.json")
	respPath := filepath.Join(runDir, "response.json")
	if err := os.WriteFile(reqPath, reqBytes, 0600); err != nil {
		return fmt.Errorf("write R sidecar request file: %w", err)
	}

	cmd := exec.CommandContext(ctx, rscriptBin, "--vanilla", scriptPath, reqPath, respPath)
	cmd.Dir = runDir
	cmd.Env = ConfigureChildREnv(buildIsolatedChildEnv(tmpDir), runtime.GOOS, rscriptBin, ResolveBundledPandocDir(opts))
	cmd.WaitDelay = 2 * time.Second

	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	runErr := cmd.Run()
	if ctxErr := ctx.Err(); ctxErr != nil {
		return ctxErr
	}
	if runErr != nil {
		stderrMsg := strings.TrimSpace(stderrBuf.String())
		if stderrMsg == "" {
			stderrMsg = strings.TrimSpace(stdoutBuf.String())
		}
		if len(stderrMsg) > 600 {
			stderrMsg = stderrMsg[:600] + "..."
		}
		return fmt.Errorf("Rscript %s failed (%v): %s", scriptAsset, runErr, stderrMsg)
	}

	respBytes, err := os.ReadFile(respPath)
	if err != nil {
		return fmt.Errorf("Rscript %s did not produce a response JSON file: %w", scriptAsset, err)
	}
	if len(bytes.TrimSpace(respBytes)) == 0 {
		return fmt.Errorf("Rscript %s produced an empty response JSON file", scriptAsset)
	}
	if !utf8.Valid(respBytes) {
		return fmt.Errorf("Rscript %s produced invalid UTF-8 response bytes", scriptAsset)
	}

	var envelope struct {
		OK    bool   `json:"ok"`
		Error string `json:"error"`
	}
	if err := json.Unmarshal(respBytes, &envelope); err != nil {
		return fmt.Errorf("Rscript %s returned malformed JSON: %w", scriptAsset, err)
	}
	if !envelope.OK {
		msg := strings.TrimSpace(envelope.Error)
		if msg == "" {
			msg = "R sidecar reported ok=false"
		}
		return fmt.Errorf("Rscript %s error: %s", scriptAsset, msg)
	}

	if outResponse != nil {
		if err := json.Unmarshal(respBytes, outResponse); err != nil {
			return fmt.Errorf("decode Rscript %s response payload: %w", scriptAsset, err)
		}
	}
	return nil
}

type probeRuntimeResponse struct {
	OK                 bool             `json:"ok"`
	RVersion           string           `json:"rVersion"`
	RVersionFull       string           `json:"rVersionFull"`
	Platform           string           `json:"platform"`
	JsonliteMissing    bool             `json:"jsonliteMissing"`
	ClimrCachePath     string           `json:"climrCachePath"`
	ClimrRastersCached bool             `json:"climrRastersCached"`
	Packages           []RPackageStatus `json:"packages"`
}

// DetectRRuntime probes Rscript, Quarto CLI, and required BEC_R/climr packages without ever crashing
// when R or Quarto is absent on a user workstation.
func DetectRRuntime(ctx context.Context, opts ...RuntimeOptions) (*RRuntimeStatus, error) {
	if ctx == nil {
		return nil, errors.New("DetectRRuntime requires a non-nil context")
	}
	if err := ctx.Err(); err != nil {
		return nil, err
	}

	cfg := effectiveOptions(opts...)
	status := &RRuntimeStatus{
		MissingRequired:   []string{},
		Packages:          []RPackageStatus{},
		SetupInstructions: []string{},
	}

	// Probe Quarto CLI first (independent of Rscript)
	if quartoBin := resolveQuartoBinary(cfg); quartoBin != "" {
		qCmd := exec.CommandContext(ctx, quartoBin, "--version")
		qCmd.WaitDelay = 2 * time.Second
		var qOut bytes.Buffer
		qCmd.Stdout = &qOut
		qCmd.Stderr = &qOut
		if qErr := qCmd.Run(); qErr == nil {
			status.QuartoAvailable = true
			status.QuartoPath = quartoBin
			status.QuartoVersion = strings.TrimSpace(strings.Split(strings.TrimSpace(qOut.String()), "\n")[0])
		} else if ctxErr := ctx.Err(); ctxErr != nil {
			return nil, ctxErr
		}
	}

	rscriptBin, _, rErr := resolveRscriptBinary(cfg)
	if rErr != nil {
		status.Available = false
		status.DiagnosticMessage = rErr.Error()
		status.MissingRequired = []string{"Rscript", "vegan", "vegclust", "DBI", "RSQLite", "data.table", "jsonlite", "knitr", "rmarkdown"}
		status.SetupInstructions = defaultSetupInstructions(true, !status.QuartoAvailable, []string{
			"vegan", "vegclust", "climr", "DBI", "RSQLite", "duckdb", "data.table", "jsonlite", "knitr", "rmarkdown",
		})
		return status, nil
	}

	status.RscriptPath = rscriptBin
	var probe probeRuntimeResponse
	err := executeRScriptJSON(ctx, cfg, "", "probe_runtime.R", nil, map[string]any{"probe": true}, &probe)
	if err != nil {
		if errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
			return nil, err
		}
		status.Available = false
		status.DiagnosticMessage = err.Error()
		status.SetupInstructions = defaultSetupInstructions(true, !status.QuartoAvailable, nil)
		return status, nil
	}

	status.Available = true
	status.RVersion = probe.RVersion
	status.RVersionFull = probe.RVersionFull
	status.Platform = probe.Platform
	status.ClimrCachePath = probe.ClimrCachePath
	status.ClimrRastersCached = probe.ClimrRastersCached
	if probe.Packages != nil {
		status.Packages = probe.Packages
	}

	installedMap := make(map[string]bool, len(status.Packages))
	for _, p := range status.Packages {
		installedMap[p.Name] = p.Installed
		if p.Required && !p.Installed {
			status.MissingRequired = append(status.MissingRequired, p.Name)
		}
	}
	if probe.JsonliteMissing {
		status.MissingRequired = append(status.MissingRequired, "jsonlite")
	}

	status.ReadyForClimate = installedMap["jsonlite"] && installedMap["data.table"] && installedMap["DBI"] && installedMap["RSQLite"]
	status.ReadyForClustering = status.ReadyForClimate && installedMap["vegan"] && installedMap["vegclust"]
	status.ReadyForQuarto = status.ReadyForClimate && (status.QuartoAvailable || (installedMap["knitr"] && installedMap["rmarkdown"]))
	if !status.QuartoAvailable || len(status.MissingRequired) > 0 {
		status.SetupInstructions = defaultSetupInstructions(false, !status.QuartoAvailable, status.MissingRequired)
	}
	return status, nil
}
