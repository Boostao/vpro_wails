package cloudsync

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/boostao/vpro-wails/internal/sqlutil"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	RoleModeContributor = "contributor"
	RoleModeCustodian   = "custodian"
)

var (
	ipv4Pattern      = regexp.MustCompile(`\b\d{1,3}(?:\.\d{1,3}){3}\b`)
	pgURLPattern     = regexp.MustCompile(`(?i)postgres(?:ql)?://[^\s"']+`)
	kvPasswordRegexp = regexp.MustCompile(`(?i)(password\s*=\s*)([^\s"']+|'[^']*'|"[^"]*")`)
)

// EnvLookup resolves an environment variable key.
type EnvLookup func(key string) (string, bool)

// ResolvedCredentials holds parsed connection settings strictly in memory.
// Struct tags intentionally exclude JSON/YAML serialization to prevent accidental secret leakage.
type ResolvedCredentials struct {
	Source          string `json:"-" yaml:"-"` // "env" | "becmaster.env" | "session" | "unconfigured"
	EnvFileExists   bool   `json:"-" yaml:"-"`
	Host            string `json:"-" yaml:"-"`
	Port            uint16 `json:"-" yaml:"-"`
	Database        string `json:"-" yaml:"-"`
	SSLMode         string `json:"-" yaml:"-"`
	UserURL         string `json:"-" yaml:"-"`
	User            string `json:"-" yaml:"-"`
	UserPassword    string `json:"-" yaml:"-"`
	AdminURL        string `json:"-" yaml:"-"`
	AdminUser       string `json:"-" yaml:"-"`
	AdminPassword   string `json:"-" yaml:"-"`
	ActiveRoleMode  string `json:"-" yaml:"-"`
	SessionOverride bool   `json:"-" yaml:"-"`
}

// SessionConfigRequest updates in-memory-only role selection and optional session credential overrides.
// Never persisted to config.yml or disk.
type SessionConfigRequest struct {
	RoleMode        string `json:"roleMode"`
	ClearOverrides  bool   `json:"clearOverrides,omitempty"`
	Host            string `json:"host,omitempty"`
	Port            int    `json:"port,omitempty"`
	Database        string `json:"database,omitempty"`
	SSLMode         string `json:"sslMode,omitempty"`
	User            string `json:"user,omitempty"`
	Password        string `json:"password,omitempty"`
	AdminUser       string `json:"adminUser,omitempty"`
	AdminPassword   string `json:"adminPassword,omitempty"`
}

func (r *SessionConfigRequest) UnmarshalJSON(data []byte) error {
	type plain SessionConfigRequest
	return sqlutil.DecodeStrictJSON(data, (*plain)(r))
}

// CloudConfigStatus is the secret-free status summary safe for UI and logs.
type CloudConfigStatus struct {
	Configured          bool   `json:"configured"`
	ConfigSource        string `json:"configSource"` // "env" | "becmaster.env" | "session" | "unconfigured"
	EnvFileDetected     bool   `json:"envFileDetected"`
	ActiveRoleMode      string `json:"activeRoleMode"` // "contributor" | "custodian"
	HasContributorCreds bool   `json:"hasContributorCreds"`
	HasCustodianCreds   bool   `json:"hasCustodianCreds"`
	SessionOverride     bool   `json:"sessionOverride"`
	Port                int    `json:"port"`
	Database            string `json:"database"`
	SSLMode             string `json:"sslMode"`
}

type sessionOverrides struct {
	active        bool
	host          string
	port          uint16
	database      string
	sslMode       string
	user          string
	password      string
	adminUser     string
	adminPassword string
}

// ParseDotEnvContent parses KEY=VALUE lines (with optional leading "export " and surrounding quotes).
func ParseDotEnvContent(content string) map[string]string {
	values := make(map[string]string)
	for _, rawLine := range strings.Split(content, "\n") {
		line := strings.TrimSpace(rawLine)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		line = strings.TrimSpace(strings.TrimPrefix(line, "export "))
		idx := strings.IndexByte(line, '=')
		if idx <= 0 {
			continue
		}
		key := strings.TrimSpace(line[:idx])
		val := strings.TrimSpace(line[idx+1:])
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		} else if hashIdx := strings.Index(val, " #"); hashIdx >= 0 {
			val = strings.TrimSpace(val[:hashIdx])
		}
		if key != "" {
			values[key] = val
		}
	}
	return values
}

// RedactSecrets scrubs passwords, hostnames, DSNs, URLs, and IPv4 addresses from diagnostic strings.
func RedactSecrets(msg string, creds *ResolvedCredentials, extraSecrets ...string) string {
	if msg == "" {
		return ""
	}
	out := pgURLPattern.ReplaceAllString(msg, "postgres://<redacted>")
	out = kvPasswordRegexp.ReplaceAllString(out, "${1}<redacted>")
	var replacements []string
	addSecret := func(val, placeholder string) {
		trimmed := strings.TrimSpace(val)
		if trimmed == "" || len(trimmed) < 2 {
			return
		}
		replacements = append(replacements, trimmed, placeholder)
		if encoded := url.QueryEscape(trimmed); encoded != trimmed {
			replacements = append(replacements, encoded, placeholder)
		}
	}
	if creds != nil {
		addSecret(creds.UserPassword, "<redacted>")
		addSecret(creds.AdminPassword, "<redacted>")
		addSecret(creds.UserURL, "postgres://<redacted>")
		addSecret(creds.AdminURL, "postgres://<redacted>")
		addSecret(creds.Host, "<pg-host>")
		for _, rawU := range []string{creds.UserURL, creds.AdminURL} {
			if rawU == "" {
				continue
			}
			if parsed, err := url.Parse(rawU); err == nil {
				if h := parsed.Hostname(); h != "" {
					addSecret(h, "<pg-host>")
				}
				if parsed.User != nil {
					if pw, ok := parsed.User.Password(); ok {
						addSecret(pw, "<redacted>")
					}
				}
			}
			if pCfg, err := pgx.ParseConfig(rawU); err == nil {
				addSecret(pCfg.Host, "<pg-host>")
				addSecret(pCfg.Password, "<redacted>")
			}
		}
	}
	for _, s := range extraSecrets {
		addSecret(s, "<redacted>")
	}
	if len(replacements) > 0 {
		out = strings.NewReplacer(replacements...).Replace(out)
	}
	out = ipv4Pattern.ReplaceAllString(out, "<redacted-ip>")
	return out
}

func wrapRedactedError(prefix string, err error, creds *ResolvedCredentials) error {
	if err == nil {
		return nil
	}
	sanitized := RedactSecrets(err.Error(), creds)
	if prefix == "" {
		return errors.New(sanitized)
	}
	return fmt.Errorf("%s: %s", prefix, sanitized)
}

// Manager manages offline-safe PostgreSQL credentials, role mode selection, and connection pools.
type Manager struct {
	mu         sync.RWMutex
	envLookup  EnvLookup
	configDir  string
	roleMode   string
	overrides  sessionOverrides
	userPool   *pgxpool.Pool
	userKey    string
	adminPool  *pgxpool.Pool
	adminKey   string
}

func NewManager(configDir string, envLookup EnvLookup) *Manager {
	if envLookup == nil {
		envLookup = os.LookupEnv
	}
	return &Manager{
		envLookup: envLookup,
		configDir: configDir,
		roleMode:  RoleModeContributor,
	}
}

func (m *Manager) Close() {
	if m == nil {
		return
	}
	m.mu.Lock()
	uPool := m.userPool
	aPool := m.adminPool
	m.userPool = nil
	m.adminPool = nil
	m.userKey = ""
	m.adminKey = ""
	m.mu.Unlock()
	if uPool != nil {
		uPool.Close()
	}
	if aPool != nil {
		aPool.Close()
	}
}

func (m *Manager) resolveEnvFilePath() string {
	dir := strings.TrimSpace(m.configDir)
	if dir == "" {
		if d, err := sqlutil.UserConfigDir(); err == nil {
			dir = d
		}
	}
	if dir == "" {
		return ""
	}
	return filepath.Join(dir, "becmaster.env")
}

// ResolveCredentials loads credentials from process env vars first, falling back to becmaster.env,
// and overlays any in-memory session overrides. Zero network calls are performed.
func (m *Manager) ResolveCredentials() (*ResolvedCredentials, error) {
	if m == nil {
		return &ResolvedCredentials{Source: "unconfigured", ActiveRoleMode: RoleModeContributor}, nil
	}
	m.mu.RLock()
	roleMode := m.roleMode
	ov := m.overrides
	envLookup := m.envLookup
	m.mu.RUnlock()

	if roleMode == "" {
		roleMode = RoleModeContributor
	}

	envPath := m.resolveEnvFilePath()
	fileVals := map[string]string{}
	envFileExists := false
	if envPath != "" {
		if raw, err := os.ReadFile(envPath); err == nil {
			envFileExists = true
			fileVals = ParseDotEnvContent(string(raw))
		}
	}

	fromEnv := false
	fromFile := false
	get := func(key string) string {
		if envLookup != nil {
			if v, ok := envLookup(key); ok && strings.TrimSpace(v) != "" {
				fromEnv = true
				return strings.TrimSpace(v)
			}
		}
		if v, ok := fileVals[key]; ok && strings.TrimSpace(v) != "" {
			fromFile = true
			return strings.TrimSpace(v)
		}
		return ""
	}

	host := get("VPRO_PG_HOST")
	portStr := get("VPRO_PG_PORT")
	dbName := get("VPRO_PG_DBNAME")
	sslMode := get("VPRO_PG_SSLMODE")
	userURL := get("VPRO_PG_URL")
	user := get("VPRO_PG_USER")
	userPass := get("VPRO_PG_PASSWORD")
	adminURL := get("VPRO_PG_ADMIN_URL")
	adminUser := get("VPRO_PG_ADMIN_USER")
	adminPass := get("VPRO_PG_ADMIN_PASSWORD")

	port := uint16(5432)
	if portStr != "" {
		parsed, err := strconv.ParseUint(portStr, 10, 16)
		if err != nil || parsed == 0 {
			return nil, errors.New("invalid VPRO_PG_PORT value (must be 1..65535)")
		}
		port = uint16(parsed)
	}
	if dbName == "" {
		dbName = "becmaster"
	}
	if sslMode == "" {
		sslMode = "require"
	}

	// Extract non-secret metadata from URLs if explicit host/port/db omitted
	applyURLDefaults := func(rawURL string, targetUser, targetPass *string) {
		if rawURL == "" {
			return
		}
		if cfg, err := pgx.ParseConfig(rawURL); err == nil {
			if host == "" {
				host = cfg.Host
			}
			if portStr == "" && cfg.Port != 0 {
				port = cfg.Port
			}
			if cfg.Database != "" && get("VPRO_PG_DBNAME") == "" {
				dbName = cfg.Database
			}
			if *targetUser == "" && cfg.User != "" {
				*targetUser = cfg.User
			}
			if *targetPass == "" && cfg.Password != "" {
				*targetPass = cfg.Password
			}
		}
	}
	applyURLDefaults(userURL, &user, &userPass)
	applyURLDefaults(adminURL, &adminUser, &adminPass)

	source := "unconfigured"
	if fromEnv {
		source = "env"
	} else if fromFile {
		source = "becmaster.env"
	}

	if ov.active {
		if ov.host != "" {
			host = ov.host
			userURL = ""
			adminURL = ""
		}
		if ov.port != 0 {
			port = ov.port
			userURL = ""
			adminURL = ""
		}
		if ov.database != "" {
			dbName = ov.database
			userURL = ""
			adminURL = ""
		}
		if ov.sslMode != "" {
			sslMode = ov.sslMode
			userURL = ""
			adminURL = ""
		}
		if ov.user != "" {
			user = ov.user
			userURL = ""
		}
		if ov.password != "" {
			userPass = ov.password
			userURL = ""
		}
		if ov.adminUser != "" {
			adminUser = ov.adminUser
			adminURL = ""
		}
		if ov.adminPassword != "" {
			adminPass = ov.adminPassword
			adminURL = ""
		}
		source = "session"
	}

	return &ResolvedCredentials{
		Source:          source,
		EnvFileExists:   envFileExists,
		Host:            host,
		Port:            port,
		Database:        dbName,
		SSLMode:         sslMode,
		UserURL:         userURL,
		User:            user,
		UserPassword:    userPass,
		AdminURL:        adminURL,
		AdminUser:       adminUser,
		AdminPassword:   adminPass,
		ActiveRoleMode:  roleMode,
		SessionOverride: ov.active,
	}, nil
}

// GetConfigStatus returns a 100% offline, secret-free configuration summary.
func (m *Manager) GetConfigStatus() (CloudConfigStatus, error) {
	creds, err := m.ResolveCredentials()
	if err != nil {
		return CloudConfigStatus{}, err
	}
	hasContributor := creds.UserURL != "" || (creds.Host != "" && creds.User != "")
	hasCustodian := creds.AdminURL != "" || (creds.Host != "" && creds.AdminUser != "")
	configured := hasContributor || hasCustodian
	src := creds.Source
	if !configured {
		src = "unconfigured"
	}
	return CloudConfigStatus{
		Configured:          configured,
		ConfigSource:        src,
		EnvFileDetected:     creds.EnvFileExists,
		ActiveRoleMode:      creds.ActiveRoleMode,
		HasContributorCreds: hasContributor,
		HasCustodianCreds:   hasCustodian,
		SessionOverride:     creds.SessionOverride,
		Port:                int(creds.Port),
		Database:            creds.Database,
		SSLMode:             creds.SSLMode,
	}, nil
}

// ConfigureSession updates in-memory role selection and optional session credential overrides.
func (m *Manager) ConfigureSession(req SessionConfigRequest) (CloudConfigStatus, error) {
	if m == nil {
		return CloudConfigStatus{}, errors.New("cloud sync manager is unavailable")
	}
	mode := strings.TrimSpace(req.RoleMode)
	if mode != "" && mode != RoleModeContributor && mode != RoleModeCustodian {
		return CloudConfigStatus{}, fmt.Errorf("invalid roleMode %q (expected %q or %q)", mode, RoleModeContributor, RoleModeCustodian)
	}
	if req.Port < 0 || req.Port > 65535 {
		return CloudConfigStatus{}, errors.New("port must be between 1 and 65535")
	}
	ssl := strings.TrimSpace(req.SSLMode)
	if ssl != "" {
		switch ssl {
		case "disable", "allow", "prefer", "require", "verify-ca", "verify-full":
		default:
			return CloudConfigStatus{}, fmt.Errorf("invalid sslMode %q", ssl)
		}
	}

	m.mu.Lock()
	if mode != "" {
		m.roleMode = mode
	}
	if req.ClearOverrides {
		m.overrides = sessionOverrides{}
	}
	hasField := strings.TrimSpace(req.Host) != "" ||
		req.Port > 0 ||
		strings.TrimSpace(req.Database) != "" ||
		ssl != "" ||
		strings.TrimSpace(req.User) != "" ||
		req.Password != "" ||
		strings.TrimSpace(req.AdminUser) != "" ||
		req.AdminPassword != ""
	if hasField {
		m.overrides.active = true
		if h := strings.TrimSpace(req.Host); h != "" {
			m.overrides.host = h
		}
		if req.Port > 0 {
			m.overrides.port = uint16(req.Port)
		}
		if d := strings.TrimSpace(req.Database); d != "" {
			m.overrides.database = d
		}
		if ssl != "" {
			m.overrides.sslMode = ssl
		}
		if u := strings.TrimSpace(req.User); u != "" {
			m.overrides.user = u
		}
		if req.Password != "" {
			m.overrides.password = req.Password
		}
		if au := strings.TrimSpace(req.AdminUser); au != "" {
			m.overrides.adminUser = au
		}
		if req.AdminPassword != "" {
			m.overrides.adminPassword = req.AdminPassword
		}
	}
	// Reset cached pools whenever session credentials change
	uPool, aPool := m.userPool, m.adminPool
	if req.ClearOverrides || hasField {
		m.userPool = nil
		m.adminPool = nil
		m.userKey = ""
		m.adminKey = ""
	} else {
		uPool, aPool = nil, nil
	}
	m.mu.Unlock()

	if uPool != nil {
		uPool.Close()
	}
	if aPool != nil {
		aPool.Close()
	}
	return m.GetConfigStatus()
}

func buildConnURLStringForRole(creds *ResolvedCredentials, roleMode string) (string, error) {
	if creds == nil {
		return "", errors.New("PostgreSQL credentials are not configured")
	}
	isCustodian := roleMode == RoleModeCustodian
	rawURL := creds.UserURL
	user := creds.User
	pass := creds.UserPassword
	missingMsg := "PostgreSQL contributor credentials are not configured (set VPRO_PG_HOST/VPRO_PG_USER/VPRO_PG_PASSWORD or ~/.config/vpro/becmaster.env)"
	if isCustodian {
		rawURL = creds.AdminURL
		user = creds.AdminUser
		pass = creds.AdminPassword
		missingMsg = "PostgreSQL custodian admin credentials are not configured (set VPRO_PG_ADMIN_USER/VPRO_PG_ADMIN_PASSWORD or ~/.config/vpro/becmaster.env)"
	}
	if rawURL != "" {
		return rawURL, nil
	}
	if creds.Host == "" || user == "" {
		return "", errors.New(missingMsg)
	}
	ssl := creds.SSLMode
	if ssl == "" {
		ssl = "require"
	}
	port := creds.Port
	if port == 0 {
		port = 5432
	}
	db := creds.Database
	if db == "" {
		db = "becmaster"
	}
	q := url.Values{}
	q.Set("sslmode", ssl)
	u := &url.URL{
		Scheme:   "postgres",
		User:     url.UserPassword(user, pass),
		Host:     net.JoinHostPort(creds.Host, strconv.Itoa(int(port))),
		Path:     "/" + db,
		RawQuery: q.Encode(),
	}
	return u.String(), nil
}

func buildConnConfigForRole(creds *ResolvedCredentials, roleMode string) (*pgx.ConnConfig, error) {
	connStr, err := buildConnURLStringForRole(creds, roleMode)
	if err != nil {
		return nil, err
	}
	cfg, err := pgx.ParseConfig(connStr)
	if err != nil {
		return nil, wrapRedactedError("parse PostgreSQL connection config", err, creds)
	}
	return cfg, nil
}
