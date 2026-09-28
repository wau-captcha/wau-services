package envconf

import (
	"os"
	"path/filepath"
	"sync"
)

// configsDir is the directory a service keeps its config.yaml in.
const configsDir = "configs"

// maxParents is how far up Dir looks: a service's cmd/<name> sits two levels below its root.
const maxParents = 4

var serviceDirOnce = sync.OnceValue(findServiceDir)

// ServiceDir is the directory a service keeps its configs and .env in: the working directory, or
// the nearest parent that holds a configs directory. `go run ./cmd/<name>` starts in the
// service root, `kratos run` starts in cmd/<name>, and both find the same files.
func ServiceDir() string { return serviceDirOnce() }

// ConfigDir is the config directory to load when no -conf flag is given.
func ConfigDir() string { return filepath.Join(ServiceDir(), configsDir) }

// DotEnvPath is the .env file beside the configs, which fills the ${NAME} placeholders.
func DotEnvPath() string { return filepath.Join(ServiceDir(), ".env") }

func findServiceDir() string {
	dir, err := os.Getwd()
	if err != nil {
		return "."
	}
	for range maxParents + 1 {
		if info, err := os.Stat(filepath.Join(dir, configsDir)); err == nil && info.IsDir() {
			return dir
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	return "."
}
