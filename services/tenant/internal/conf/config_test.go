package conf_test

import (
	"os"
	"regexp"
	"strings"
	"testing"

	"tenant/internal/conf"
	"tenant/internal/pkg/envconf"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
)

const configPath = "../../configs/config.yaml"

// load reads configs/config.yaml the way main does, every placeholder set to a dummy value but
// for the variables in env.
func load(t *testing.T, env map[string]string) (*conf.Bootstrap, error) {
	t.Helper()
	raw, err := os.ReadFile(configPath)
	if err != nil {
		t.Fatal(err)
	}
	for _, match := range regexp.MustCompile(`\$\{([^}]+)\}`).FindAllStringSubmatch(string(raw), -1) {
		t.Setenv(match[1], "placeholder")
	}
	for name, value := range env {
		t.Setenv(name, value)
	}
	source := config.New(config.WithSource(file.NewSource(configPath)), config.WithResolver(envconf.Resolver(&conf.Bootstrap{})))
	defer source.Close()
	if err := source.Load(); err != nil {
		return nil, err
	}
	var bootstrap conf.Bootstrap
	if err := source.Scan(&bootstrap); err != nil {
		return nil, err
	}
	return &bootstrap, nil
}

func TestRedisTLSFromEnvironment(t *testing.T) {
	for value, want := range map[string]bool{"true": true, "false": false} {
		t.Run(value, func(t *testing.T) {
			bootstrap, err := load(t, map[string]string{"KRATOS_REDIS_TLS": value})
			if err != nil {
				t.Fatal(err)
			}
			if got := bootstrap.GetData().GetRedis().GetTls(); got != want {
				t.Fatalf("data.redis.tls = %v with KRATOS_REDIS_TLS=%s, want %v", got, value, want)
			}
		})
	}
}

func TestRedisTLSRejectsOtherValues(t *testing.T) {
	_, err := load(t, map[string]string{"KRATOS_REDIS_TLS": "yes"})
	if err == nil || !strings.Contains(err.Error(), "data.redis.tls") {
		t.Fatalf("KRATOS_REDIS_TLS=yes: got error %v, want one naming data.redis.tls", err)
	}
}

func TestStringSettingsStayStrings(t *testing.T) {
	bootstrap, err := load(t, map[string]string{"KRATOS_REDIS_TLS": "false", "KRATOS_REDIS_PASSWORD": "true"})
	if err != nil {
		t.Fatal(err)
	}
	if got := bootstrap.GetData().GetRedis().GetPassword(); got != "true" {
		t.Fatalf("data.redis.password = %q, want the string %q", got, "true")
	}
}
