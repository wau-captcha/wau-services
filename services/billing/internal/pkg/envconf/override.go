package envconf

import (
	"os"
	"strings"
)

// Override replaces the values of the named top-level sections with environment variables that
// are set: KRATOS_<SECTION>_<KEY>, upper case, for <section>.<key>, e.g. KRATOS_RATE_LIMIT_NEW_PER_IP
// for rate_limit.new_per_ip. Unlike ${NAME} placeholders these sections keep the value config.yaml
// gives when the variable is not set: they hold tuning values with a sensible default, not the
// settings of an environment.
func Override(values map[string]any, sections ...string) {
	for _, section := range sections {
		entries, ok := values[section].(map[string]any)
		if !ok {
			continue
		}
		for key := range entries {
			name := "KRATOS_" + strings.ToUpper(section+"_"+key)
			if value, ok := os.LookupEnv(name); ok && strings.TrimSpace(value) != "" {
				entries[key] = strings.TrimSpace(value)
			}
		}
	}
}
