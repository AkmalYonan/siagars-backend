package config

import (
	"encoding/base64"
	"fmt"
	"net/netip"
	"net/url"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Port, MongoURI, MongoDatabase, PublicOrigin, Environment string
	SessionTTL                                               time.Duration
	SecureCookie                                             bool
	// AllowedOrigins lists every browser origin accepted for login and mutations (exact match). The first
	// PUBLIC_ORIGIN entry is the primary one and decides the Secure cookie flag.
	AllowedOrigins []string
	// BPJSHistoryKey is the 32-byte server secret that encrypts the original input of BPJS checks; nil
	// means only masked history is stored.
	BPJSHistoryKey []byte
	// TrustedProxies are the networks whose X-Forwarded-For header is believed; empty means never.
	TrustedProxies []netip.Prefix
	SATUSEHAT      SATUSEHATConfig
}

// SATUSEHATConfig is read-only MSI Sandbox access. The API server only validates it;
// cmd/manage sync-satusehat is the sole user.
type SATUSEHATConfig struct {
	Enabled      bool
	Environment  string
	ClientID     string
	ClientSecret string
	PageLimit    int
	HTTPTimeout  time.Duration
	// KabKota limits sync to these 4-digit MSI kab/kota codes; empty means nationwide.
	KabKota []string
}

// Jabodetabek lists the Kemendagri kab/kota codes of DKI Jakarta, Bogor, Depok, Tangerang and Bekasi.
var Jabodetabek = []string{"3101", "3171", "3172", "3173", "3174", "3175", "3201", "3271", "3276", "3603", "3671", "3674", "3216", "3275"}

// OriginAllowed reports whether a browser Origin header matches an accepted origin exactly.
func (c Config) OriginAllowed(origin string) bool {
	if origin == "" {
		return false
	}
	return origin == c.PublicOrigin || slices.Contains(c.AllowedOrigins, origin)
}

// parseOrigins splits a comma-separated PUBLIC_ORIGIN. Each entry must be a bare origin. Outside
// development a non-primary entry may use http only for a loopback host, so local frontends can reach it.
func parseOrigins(raw, environment string) ([]string, error) {
	var out []string
	for i, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		u, e := url.Parse(part)
		if e != nil || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") || u.Path != "" || u.RawQuery != "" || u.Fragment != "" || u.User != nil || strings.HasSuffix(part, "/") || strings.Contains(part, "*") {
			return nil, fmt.Errorf("PUBLIC_ORIGIN must be one or more comma-separated origins without a path")
		}
		if u.Scheme == "http" && environment != "development" && i > 0 && u.Hostname() != "localhost" && u.Hostname() != "127.0.0.1" {
			return nil, fmt.Errorf("HTTPS required outside development (http allowed only for localhost)")
		}
		if !slices.Contains(out, part) {
			out = append(out, part)
		}
	}
	return out, nil
}

func Load() (Config, error) {
	c := Config{Port: get("PORT", "8080"), MongoURI: os.Getenv("MONGODB_URI"), MongoDatabase: get("MONGODB_DATABASE", "siagars"), PublicOrigin: get("PUBLIC_ORIGIN", "http://localhost:3000"), Environment: get("APP_ENV", "development")}
	if m, e := url.Parse(c.MongoURI); c.MongoURI == "" || e != nil || (m.Scheme != "mongodb" && m.Scheme != "mongodb+srv") || m.Host == "" {
		return c, fmt.Errorf("MONGODB_URI must be a mongodb:// or mongodb+srv:// URI")
	}
	if !databaseName.MatchString(c.MongoDatabase) {
		return c, fmt.Errorf("MONGODB_DATABASE must be 1..63 letters, digits, _ or -")
	}
	if c.Environment != "development" && c.Environment != "test" && c.Environment != "production" {
		return c, fmt.Errorf("invalid APP_ENV")
	}
	origins, e := parseOrigins(c.PublicOrigin, c.Environment)
	if e != nil {
		return c, e
	}
	c.PublicOrigin, c.AllowedOrigins = origins[0], origins
	u, _ := url.Parse(c.PublicOrigin)
	c.SecureCookie = u.Scheme == "https"
	if !c.SecureCookie && c.Environment != "development" {
		return c, fmt.Errorf("HTTPS required outside development")
	}
	hours, e := strconv.Atoi(get("SESSION_TTL_HOURS", "8"))
	if e != nil || hours < 1 || hours > 168 {
		return c, fmt.Errorf("SESSION_TTL_HOURS must be 1..168")
	}
	c.SessionTTL = time.Duration(hours) * time.Hour
	port, e := strconv.Atoi(c.Port)
	if e != nil || port < 1 || port > 65535 {
		return c, fmt.Errorf("invalid PORT")
	}
	if c.TrustedProxies, e = parseTrustedProxies(os.Getenv("TRUSTED_PROXIES")); e != nil {
		return c, e
	}
	if get("DEMO_MODE", "true") != "true" {
		return c, fmt.Errorf("v0.1 requires DEMO_MODE=true")
	}
	if c.SATUSEHAT, e = LoadSATUSEHAT(); e != nil {
		return c, e
	}
	if raw := strings.TrimSpace(os.Getenv("BPJS_HISTORY_KEY")); raw != "" {
		key, e := base64.StdEncoding.DecodeString(raw)
		if e != nil || len(key) != 32 {
			return c, fmt.Errorf("BPJS_HISTORY_KEY must be 32 random bytes, base64-encoded")
		}
		c.BPJSHistoryKey = key
	}
	return c, nil
}

// parseTrustedProxies reads a comma-separated CIDR list; a bare IP means a single host.
func parseTrustedProxies(raw string) ([]netip.Prefix, error) {
	var out []netip.Prefix
	for _, item := range strings.Split(raw, ",") {
		item = strings.TrimSpace(item)
		if item == "" {
			continue
		}
		p, e := netip.ParsePrefix(item)
		if e != nil {
			a, e2 := netip.ParseAddr(item)
			if e2 != nil {
				return nil, fmt.Errorf("TRUSTED_PROXIES must be comma-separated CIDRs or IP addresses")
			}
			p = netip.PrefixFrom(a, a.BitLen())
		}
		out = append(out, p.Masked())
	}
	return out, nil
}

// LoadSATUSEHAT validates SATUSEHAT settings. Messages name variables, never their values.
func LoadSATUSEHAT() (SATUSEHATConfig, error) {
	c := SATUSEHATConfig{Environment: get("SATUSEHAT_ENV", "sandbox"), ClientID: os.Getenv("SATUSEHAT_CLIENT_ID"), ClientSecret: os.Getenv("SATUSEHAT_CLIENT_SECRET")}
	switch get("SATUSEHAT_ENABLED", "false") {
	case "true":
		c.Enabled = true
	case "false":
	default:
		return c, fmt.Errorf("SATUSEHAT_ENABLED must be true or false")
	}
	switch c.Environment {
	case "sandbox":
	case "production":
		return c, fmt.Errorf("SATUSEHAT_ENV=production is not allowed in this phase; only sandbox is supported")
	default:
		return c, fmt.Errorf("SATUSEHAT_ENV must be sandbox")
	}
	limit, e := strconv.Atoi(get("SATUSEHAT_PAGE_LIMIT", "200"))
	if e != nil || limit < 1 || limit > 2000 {
		return c, fmt.Errorf("SATUSEHAT_PAGE_LIMIT must be 1..2000")
	}
	c.PageLimit = limit
	seconds, e := strconv.Atoi(get("SATUSEHAT_HTTP_TIMEOUT_SECONDS", "10"))
	if e != nil || seconds < 1 || seconds > 60 {
		return c, fmt.Errorf("SATUSEHAT_HTTP_TIMEOUT_SECONDS must be 1..60")
	}
	c.HTTPTimeout = time.Duration(seconds) * time.Second
	switch scope := strings.TrimSpace(os.Getenv("SATUSEHAT_KABKOTA")); strings.ToLower(scope) {
	case "":
	case "jabodetabek":
		c.KabKota = append([]string{}, Jabodetabek...)
	default:
		seen := map[string]bool{}
		for _, code := range strings.Split(scope, ",") {
			code = strings.TrimSpace(code)
			if !kabKota.MatchString(code) {
				return c, fmt.Errorf("SATUSEHAT_KABKOTA must be jabodetabek or comma-separated 4-digit kab/kota codes")
			}
			if !seen[code] {
				seen[code] = true
				c.KabKota = append(c.KabKota, code)
			}
		}
	}
	if c.Enabled && (strings.TrimSpace(c.ClientID) == "" || strings.TrimSpace(c.ClientSecret) == "") {
		return c, fmt.Errorf("SATUSEHAT_CLIENT_ID and SATUSEHAT_CLIENT_SECRET are required when SATUSEHAT_ENABLED=true")
	}
	return c, nil
}

var (
	databaseName = regexp.MustCompile(`^[A-Za-z0-9_-]{1,63}$`)
	kabKota      = regexp.MustCompile(`^[0-9]{4}$`)
)

func get(k, f string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return f
}
