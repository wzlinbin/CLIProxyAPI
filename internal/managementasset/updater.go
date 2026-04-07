package managementasset

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/router-for-me/CLIProxyAPI/v6/internal/config"
	"github.com/router-for-me/CLIProxyAPI/v6/internal/util"
	sdkconfig "github.com/router-for-me/CLIProxyAPI/v6/sdk/config"
	log "github.com/sirupsen/logrus"
	"golang.org/x/sync/singleflight"
)

const (
	defaultManagementReleaseURL  = "https://api.github.com/repos/router-for-me/Cli-Proxy-API-Management-Center/releases/latest"
	defaultManagementFallbackURL = "https://cpamc.router-for.me/"
	managementAssetName          = "management.html"
	opsBillingAssetName          = "ops-billing.html"
	httpUserAgent                = "CLIProxyAPI-management-updater"
	managementSyncMinInterval    = 30 * time.Second
	updateCheckInterval          = 3 * time.Hour
	maxAssetDownloadSize         = 50 << 20 // 10 MB safety limit for management asset downloads
)

// ManagementFileName exposes the control panel asset filename.
const ManagementFileName = managementAssetName

// OpsBillingFileName exposes the standalone operations billing asset filename.
const OpsBillingFileName = opsBillingAssetName

var (
	lastUpdateCheckMu   sync.Mutex
	lastUpdateCheckTime time.Time
	currentConfigPtr    atomic.Pointer[config.Config]
	schedulerOnce       sync.Once
	schedulerConfigPath atomic.Value
	sfGroup             singleflight.Group
)

var executablePathFunc = os.Executable

// SetCurrentConfig stores the latest configuration snapshot for management asset decisions.
func SetCurrentConfig(cfg *config.Config) {
	if cfg == nil {
		currentConfigPtr.Store(nil)
		return
	}
	currentConfigPtr.Store(cfg)
}

// StartAutoUpdater launches a background goroutine that periodically ensures the management asset is up to date.
// It respects the disable-control-panel flag on every iteration and supports hot-reloaded configurations.
func StartAutoUpdater(ctx context.Context, configFilePath string) {
	log.Debug("management asset auto-updater disabled: management.html is served from local files only")
}

func runAutoUpdater(ctx context.Context) {
	if ctx == nil {
		ctx = context.Background()
	}

	ticker := time.NewTicker(updateCheckInterval)
	defer ticker.Stop()

	runOnce := func() {
		cfg := currentConfigPtr.Load()
		if cfg == nil {
			log.Debug("management asset auto-updater skipped: config not yet available")
			return
		}
		if cfg.RemoteManagement.DisableControlPanel {
			log.Debug("management asset auto-updater skipped: control panel disabled")
			return
		}
		if cfg.RemoteManagement.DisableAutoUpdatePanel {
			log.Debug("management asset auto-updater skipped: disable-auto-update-panel is enabled")
			return
		}

		configPath, _ := schedulerConfigPath.Load().(string)
		staticDir := StaticDir(configPath)
		EnsureLatestManagementHTML(ctx, staticDir, cfg.ProxyURL, cfg.RemoteManagement.PanelGitHubRepository)
	}

	runOnce()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			runOnce()
		}
	}
}

func newHTTPClient(proxyURL string) *http.Client {
	client := &http.Client{Timeout: 15 * time.Second}

	sdkCfg := &sdkconfig.SDKConfig{ProxyURL: strings.TrimSpace(proxyURL)}
	util.SetProxy(sdkCfg, client)

	return client
}

type releaseAsset struct {
	Name               string `json:"name"`
	BrowserDownloadURL string `json:"browser_download_url"`
	Digest             string `json:"digest"`
}

type releaseResponse struct {
	Assets []releaseAsset `json:"assets"`
}

// StaticDir resolves the directory that stores the management control panel asset.
func StaticDir(configFilePath string) string {
	if override := strings.TrimSpace(os.Getenv("MANAGEMENT_STATIC_PATH")); override != "" {
		cleaned := filepath.Clean(override)
		if strings.EqualFold(filepath.Base(cleaned), managementAssetName) {
			return filepath.Dir(cleaned)
		}
		return cleaned
	}

	return preferredStaticDir(configFilePath)
}

// FilePath resolves the absolute path to the management control panel asset.
func FilePath(configFilePath string) string {
	return AssetFilePath(configFilePath, ManagementFileName)
}

// AssetFilePath resolves the absolute path to a local management asset.
func AssetFilePath(configFilePath string, assetName string) string {
	if override := strings.TrimSpace(os.Getenv("MANAGEMENT_STATIC_PATH")); override != "" {
		cleaned := filepath.Clean(override)
		if strings.EqualFold(filepath.Base(cleaned), assetName) {
			return cleaned
		}
		return filepath.Join(cleaned, assetName)
	}

	dir := StaticDir(configFilePath)
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, assetName)
}

// ExistingFilePath returns the first existing management asset path from the known
// candidate directories. This allows packaged executables to keep working even when
// the current config path points elsewhere.
func ExistingFilePath(configFilePath string) string {
	return ExistingAssetFilePath(configFilePath, ManagementFileName)
}

// ExistingAssetFilePath returns the first existing path for the supplied asset.
func ExistingAssetFilePath(configFilePath string, assetName string) string {
	for _, candidate := range fileCandidates(configFilePath, assetName) {
		info, err := os.Stat(candidate)
		if err == nil && !info.IsDir() {
			return candidate
		}
	}
	return ""
}

// EnsureLatestManagementHTML checks the latest management.html asset and updates the local copy when needed.
// It coalesces concurrent sync attempts and returns whether the asset exists after the sync attempt.
func EnsureLatestManagementHTML(ctx context.Context, staticDir string, proxyURL string, panelRepository string) bool {
	staticDir = strings.TrimSpace(staticDir)
	if staticDir == "" {
		log.Debug("management asset local lookup skipped: empty static directory")
		return false
	}
	localPath := filepath.Join(staticDir, managementAssetName)
	if info, err := os.Stat(localPath); err == nil && !info.IsDir() {
		return true
	}
	return false
}

func ensureFallbackManagementHTML(ctx context.Context, client *http.Client, localPath string) bool {
	data, downloadedHash, err := downloadAsset(ctx, client, defaultManagementFallbackURL)
	if err != nil {
		log.WithError(err).Warn("failed to download fallback management control panel page")
		return false
	}

	log.Warnf("management asset downloaded from fallback URL without digest verification (hash=%s) — "+
		"enable verified GitHub updates by keeping disable-auto-update-panel set to false", downloadedHash)

	if err = atomicWriteFile(localPath, data); err != nil {
		log.WithError(err).Warn("failed to persist fallback management control panel page")
		return false
	}

	log.Infof("management asset updated from fallback page successfully (hash=%s)", downloadedHash)
	return true
}

func resolveReleaseURL(repo string) string {
	repo = strings.TrimSpace(repo)
	if repo == "" {
		return defaultManagementReleaseURL
	}

	parsed, err := url.Parse(repo)
	if err != nil || parsed.Host == "" {
		return defaultManagementReleaseURL
	}

	host := strings.ToLower(parsed.Host)
	parsed.Path = strings.TrimSuffix(parsed.Path, "/")

	if host == "api.github.com" {
		if !strings.HasSuffix(strings.ToLower(parsed.Path), "/releases/latest") {
			parsed.Path = parsed.Path + "/releases/latest"
		}
		return parsed.String()
	}

	if host == "github.com" {
		parts := strings.Split(strings.Trim(parsed.Path, "/"), "/")
		if len(parts) >= 2 && parts[0] != "" && parts[1] != "" {
			repoName := strings.TrimSuffix(parts[1], ".git")
			return fmt.Sprintf("https://api.github.com/repos/%s/%s/releases/latest", parts[0], repoName)
		}
	}

	return defaultManagementReleaseURL
}

func fetchLatestAsset(ctx context.Context, client *http.Client, releaseURL string) (*releaseAsset, string, error) {
	if strings.TrimSpace(releaseURL) == "" {
		releaseURL = defaultManagementReleaseURL
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, releaseURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create release request: %w", err)
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", httpUserAgent)
	gitURL := strings.ToLower(strings.TrimSpace(os.Getenv("GITSTORE_GIT_URL")))
	if tok := strings.TrimSpace(os.Getenv("GITSTORE_GIT_TOKEN")); tok != "" && strings.Contains(gitURL, "github.com") {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("execute release request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, "", fmt.Errorf("unexpected release status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	var release releaseResponse
	if err = json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return nil, "", fmt.Errorf("decode release response: %w", err)
	}

	for i := range release.Assets {
		asset := &release.Assets[i]
		if strings.EqualFold(asset.Name, managementAssetName) {
			remoteHash := parseDigest(asset.Digest)
			return asset, remoteHash, nil
		}
	}

	return nil, "", fmt.Errorf("management asset %s not found in latest release", managementAssetName)
}

func downloadAsset(ctx context.Context, client *http.Client, downloadURL string) ([]byte, string, error) {
	if strings.TrimSpace(downloadURL) == "" {
		return nil, "", fmt.Errorf("empty download url")
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, downloadURL, nil)
	if err != nil {
		return nil, "", fmt.Errorf("create download request: %w", err)
	}
	req.Header.Set("User-Agent", httpUserAgent)

	resp, err := client.Do(req)
	if err != nil {
		return nil, "", fmt.Errorf("execute download request: %w", err)
	}
	defer func() {
		_ = resp.Body.Close()
	}()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, "", fmt.Errorf("unexpected download status %d: %s", resp.StatusCode, strings.TrimSpace(string(body)))
	}

	data, err := io.ReadAll(io.LimitReader(resp.Body, maxAssetDownloadSize+1))
	if err != nil {
		return nil, "", fmt.Errorf("read download body: %w", err)
	}
	if int64(len(data)) > maxAssetDownloadSize {
		return nil, "", fmt.Errorf("download exceeds maximum allowed size of %d bytes", maxAssetDownloadSize)
	}

	sum := sha256.Sum256(data)
	return data, hex.EncodeToString(sum[:]), nil
}

func fileSHA256(path string) (string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer func() {
		_ = file.Close()
	}()

	h := sha256.New()
	if _, err = io.Copy(h, file); err != nil {
		return "", err
	}

	return hex.EncodeToString(h.Sum(nil)), nil
}

func atomicWriteFile(path string, data []byte) error {
	tmpFile, err := os.CreateTemp(filepath.Dir(path), "management-*.html")
	if err != nil {
		return err
	}

	tmpName := tmpFile.Name()
	defer func() {
		_ = tmpFile.Close()
		_ = os.Remove(tmpName)
	}()

	if _, err = tmpFile.Write(data); err != nil {
		return err
	}

	if err = tmpFile.Chmod(0o644); err != nil {
		return err
	}

	if err = tmpFile.Close(); err != nil {
		return err
	}

	if err = os.Rename(tmpName, path); err != nil {
		return err
	}

	return nil
}

func parseDigest(digest string) string {
	digest = strings.TrimSpace(digest)
	if digest == "" {
		return ""
	}

	if idx := strings.Index(digest, ":"); idx >= 0 {
		digest = digest[idx+1:]
	}

	return strings.ToLower(strings.TrimSpace(digest))
}

func fileCandidates(configFilePath string, assetName string) []string {
	if override := strings.TrimSpace(os.Getenv("MANAGEMENT_STATIC_PATH")); override != "" {
		cleaned := filepath.Clean(override)
		if strings.EqualFold(filepath.Base(cleaned), assetName) {
			return []string{cleaned}
		}
		return []string{filepath.Join(cleaned, assetName)}
	}

	candidates := make([]string, 0, 4)
	for _, baseDir := range candidateBaseDirs(configFilePath) {
		candidates = append(candidates, filepath.Join(baseDir, "static", assetName))
	}
	if writable := util.WritablePath(); writable != "" {
		candidates = append(candidates, filepath.Join(writable, "static", assetName))
	}

	seen := make(map[string]struct{}, len(candidates))
	unique := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		cleaned := filepath.Clean(candidate)
		if _, exists := seen[cleaned]; exists {
			continue
		}
		seen[cleaned] = struct{}{}
		unique = append(unique, cleaned)
	}
	return unique
}

func preferredStaticDir(configFilePath string) string {
	baseDirs := candidateBaseDirs(configFilePath)
	localStaticDirs := make([]string, 0, len(baseDirs))
	for _, baseDir := range baseDirs {
		localStaticDirs = append(localStaticDirs, filepath.Join(baseDir, "static"))
	}

	for _, dir := range localStaticDirs {
		if fileExists(filepath.Join(dir, ManagementFileName)) {
			return dir
		}
	}

	for _, dir := range localStaticDirs {
		if directoryExists(dir) {
			return dir
		}
	}

	if writable := util.WritablePath(); writable != "" {
		writableDir := filepath.Join(writable, "static")
		if fileExists(filepath.Join(writableDir, ManagementFileName)) {
			return writableDir
		}
		if directoryExists(writableDir) {
			return writableDir
		}
	}

	if len(localStaticDirs) > 0 {
		return localStaticDirs[0]
	}

	if writable := util.WritablePath(); writable != "" {
		return filepath.Join(writable, "static")
	}

	return ""
}

func candidateBaseDirs(configFilePath string) []string {
	candidates := make([]string, 0, 3)

	configFilePath = strings.TrimSpace(configFilePath)
	if configFilePath != "" {
		base := filepath.Dir(configFilePath)
		if fileInfo, err := os.Stat(configFilePath); err == nil && fileInfo.IsDir() {
			base = configFilePath
		}
		candidates = append(candidates, filepath.Clean(base))
	}

	if workingDir, err := os.Getwd(); err == nil && strings.TrimSpace(workingDir) != "" {
		candidates = append(candidates, filepath.Clean(workingDir))
	}

	if executableDir := resolveExecutableDir(); executableDir != "" {
		candidates = append(candidates, filepath.Clean(executableDir))
	}

	seen := make(map[string]struct{}, len(candidates))
	unique := make([]string, 0, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" {
			continue
		}
		if _, exists := seen[candidate]; exists {
			continue
		}
		seen[candidate] = struct{}{}
		unique = append(unique, candidate)
	}

	return unique
}

func resolveExecutableDir() string {
	executablePath, err := executablePathFunc()
	if err != nil || strings.TrimSpace(executablePath) == "" {
		return ""
	}

	if resolvedPath, errEval := filepath.EvalSymlinks(executablePath); errEval == nil && strings.TrimSpace(resolvedPath) != "" {
		executablePath = resolvedPath
	}

	return filepath.Dir(executablePath)
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func directoryExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}
