package app

import (
	"fmt"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/F1ameX/RepoPulse/project/backend/internal/auth/adapter/out/password"
)

type Config struct {
	GRPCAddr          string
	ShutdownTimeout   time.Duration
	DatabaseURL       string
	GuestSessionTTL   time.Duration
	AccountSessionTTL time.Duration
	HasherConfig      password.Params
}

const (
	defaultAuthGRPcAddr      = "127.0.0.1:9090"
	defaultShutDownTimeout   = "10s"
	defaultGuestSessionTTL   = "24h"
	defaultAccountSessionTTL = "168h"
)

// Argon2id defaults: OWASP minimum (19 MiB, t=2, p=1), RFC 9106 salt and key sizes.
const (
	defaultMemSize = "19456"
	defaultTime    = "2"
	defaultThreads = "1"
	defaultKeyLen  = "32"
	defaultSaltLen = "16"
)

// LoadConfig reads Auth settings from the process environment.
// Explicitly empty values are invalid.
func LoadConfig() (Config, error) {
	return loadConfig(os.LookupEnv)
}

func loadConfig(lookup func(string) (string, bool)) (Config, error) {
	value := func(key, fallback string) string {
		if v, ok := lookup(key); ok {
			return strings.TrimSpace(v)
		}
		return fallback
	}

	mandatoryValue := func(key string) (string, bool) {
		if v, ok := lookup(key); ok {
			return strings.TrimSpace(v), true
		}
		return "", false
	}

	cfg := Config{GRPCAddr: value("AUTH_GRPC_ADDR", defaultAuthGRPcAddr)}
	_, port, err := net.SplitHostPort(cfg.GRPCAddr)
	if err != nil {
		return Config{}, fmt.Errorf("AUTH_GRPC_ADDR must use host:port: %w", err)
	}
	portNumber, err := strconv.Atoi(port)
	if err != nil || portNumber < 1 || portNumber > 65535 {
		return Config{}, fmt.Errorf("AUTH_GRPC_ADDR port must be between 1 and 65535")
	}

	timeout, err := time.ParseDuration(value("SHUTDOWN_TIMEOUT", defaultShutDownTimeout))
	if err != nil || timeout <= 0 {
		return Config{}, fmt.Errorf("SHUTDOWN_TIMEOUT must be a positive duration (for example, 5s)")
	}
	cfg.ShutdownTimeout = timeout

	databaseURL, ok := mandatoryValue("AUTH_DATABASE_URL")
	if !ok || databaseURL == "" {
		return Config{}, fmt.Errorf("AUTH_DATABASE_URL must be presented")
	}
	cfg.DatabaseURL = databaseURL

	guestSessionTTL, err := time.ParseDuration(value("AUTH_GUEST_SESSION_TTL", defaultGuestSessionTTL))
	if err != nil || guestSessionTTL <= 0 {
		return Config{}, fmt.Errorf("AUTH_GUEST_SESSION_TTL must be a positive duration (for example, 24h)")
	}
	cfg.GuestSessionTTL = guestSessionTTL

	accountSessionTTL, err := time.ParseDuration(value("AUTH_ACCOUNT_SESSION_TTL", defaultAccountSessionTTL))
	if err != nil || accountSessionTTL <= 0 {
		return Config{}, fmt.Errorf("AUTH_ACCOUNT_SESSION_TTL must be a positive duration (for example, 168h)")
	}
	cfg.AccountSessionTTL = accountSessionTTL

	memory, err := uintInRange(value("AUTH_ARGON2_MEMORY_KIB", defaultMemSize), "AUTH_ARGON2_MEMORY_KIB",
		password.MinMemoryKiB, password.MaxMemoryKiB)
	if err != nil {
		return Config{}, err
	}
	hashTime, err := uintInRange(value("AUTH_ARGON2_TIME", defaultTime), "AUTH_ARGON2_TIME",
		password.MinTime, password.MaxTime)
	if err != nil {
		return Config{}, err
	}
	threads, err := uintInRange(value("AUTH_ARGON2_THREADS", defaultThreads), "AUTH_ARGON2_THREADS",
		password.MinThreads, password.MaxThreads)
	if err != nil {
		return Config{}, err
	}
	keyLen, err := uintInRange(value("AUTH_ARGON2_KEY_LEN", defaultKeyLen), "AUTH_ARGON2_KEY_LEN",
		password.MinKeyLen, password.MaxKeyLen)
	if err != nil {
		return Config{}, err
	}
	saltLen, err := uintInRange(value("AUTH_ARGON2_SALT_LEN", defaultSaltLen), "AUTH_ARGON2_SALT_LEN",
		password.MinSaltLen, password.MaxSaltLen)
	if err != nil {
		return Config{}, err
	}
	hasherCfg := password.Params{
		HashMem:     memory,
		HashTime:    hashTime,
		HashThreads: threads,
		KeyLen:      int(keyLen),
		SaltLen:     int(saltLen),
	}
	if err := hasherCfg.Validate(); err != nil {
		return Config{}, err
	}

	cfg.HasherConfig = hasherCfg
	return cfg, nil
}

// uintInRange parses an unsigned integer setting and checks its inclusive range.
func uintInRange(raw, key string, min, max uint64) (uint64, error) {
	n, err := strconv.ParseUint(raw, 10, 64)
	if err != nil || n < min || n > max {
		return 0, fmt.Errorf("%s must be an integer between %d and %d", key, min, max)
	}
	return n, nil
}
