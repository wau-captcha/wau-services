package envconf

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/go-kratos/kratos/v3/config"
	"github.com/go-kratos/kratos/v3/config/file"
)

func TestResolveFillsPlaceholders(t *testing.T) {
	t.Setenv("WAU_TEST_ADDR", "127.0.0.1:8000")
	t.Setenv("WAU_TEST_HOST", "db")
	t.Setenv("WAU_TEST_PORT", "5432")
	values := map[string]any{
		"server": map[string]any{"addr": "${WAU_TEST_ADDR}", "timeout": "3s"},
		"source": "postgres://${WAU_TEST_HOST}:${ WAU_TEST_PORT }/wau",
		"routes": []any{map[string]any{"upstream": "${WAU_TEST_ADDR}"}, "${WAU_TEST_HOST}", 10},
	}

	if err := Resolve(values); err != nil {
		t.Fatalf("Resolve: %v", err)
	}

	want := map[string]any{
		"server": map[string]any{"addr": "127.0.0.1:8000", "timeout": "3s"},
		"source": "postgres://db:5432/wau",
		"routes": []any{map[string]any{"upstream": "127.0.0.1:8000"}, "db", 10},
	}
	if !reflect.DeepEqual(values, want) {
		t.Fatalf("got %v, want %v", values, want)
	}
}

func TestResolveReportsEveryMissingVariable(t *testing.T) {
	t.Setenv("WAU_TEST_BLANK", "  ")
	values := map[string]any{
		"a": "${WAU_TEST_UNSET_B}",
		"b": []any{"${WAU_TEST_BLANK}", "${WAU_TEST_UNSET_A}"},
		"c": "${WAU_TEST_UNSET_B}",
	}

	err := Resolve(values)
	if err == nil {
		t.Fatal("want an error for unset and blank variables")
	}
	if !strings.HasSuffix(err.Error(), ": WAU_TEST_BLANK, WAU_TEST_UNSET_A, WAU_TEST_UNSET_B") {
		t.Fatalf("error should list each variable once, sorted: %v", err)
	}
}

func TestResolveRejectsInlineDefaults(t *testing.T) {
	t.Setenv("WAU_TEST_SET", "value")
	err := Resolve(map[string]any{"a": "${WAU_TEST_SET:fallback}"})
	if err == nil || !strings.Contains(err.Error(), "${WAU_TEST_SET:fallback}") {
		t.Fatalf("want an error naming the defaulted placeholder, got %v", err)
	}
}

// The resolver is plugged into Kratos config the way every service's main.go does it.
func TestResolveAsKratosResolver(t *testing.T) {
	dir := t.TempDir()
	yaml := "server:\n  addr: ${WAU_TEST_ADDR}\n  timeout: 3s\nredis:\n  addrs: ${WAU_TEST_REDIS}\n"
	if err := os.WriteFile(filepath.Join(dir, "config.yaml"), []byte(yaml), 0o600); err != nil {
		t.Fatal(err)
	}
	load := func() (config.Config, error) {
		c := config.New(config.WithSource(file.NewSource(dir)), config.WithResolver(Resolve))
		t.Cleanup(func() { _ = c.Close() })
		return c, c.Load()
	}

	if _, err := load(); err == nil || !strings.Contains(err.Error(), "WAU_TEST_ADDR, WAU_TEST_REDIS") {
		t.Fatalf("want missing variables to stop loading, got %v", err)
	}

	t.Setenv("WAU_TEST_ADDR", "0.0.0.0:8000")
	t.Setenv("WAU_TEST_REDIS", "127.0.0.1:7001,127.0.0.1:7002")
	c, err := load()
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	var got struct {
		Server struct{ Addr, Timeout string }
		Redis  struct{ Addrs string }
	}
	if err := c.Scan(&got); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if got.Server.Addr != "0.0.0.0:8000" || got.Server.Timeout != "3s" || got.Redis.Addrs != "127.0.0.1:7001,127.0.0.1:7002" {
		t.Fatalf("unexpected config %+v", got)
	}
}

func TestList(t *testing.T) {
	tests := map[string]struct {
		in   string
		want []string
	}{
		"single":             {"127.0.0.1:7001", []string{"127.0.0.1:7001"}},
		"several":            {"127.0.0.1:7001,127.0.0.1:7002", []string{"127.0.0.1:7001", "127.0.0.1:7002"}},
		"spaces are trimmed": {" 127.0.0.0/8 , ::1/128 ", []string{"127.0.0.0/8", "::1/128"}},
		"blanks are dropped": {"a,, ,b,", []string{"a", "b"}},
		"empty":              {"", nil},
	}
	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if got := List(tt.in); !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("List(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
