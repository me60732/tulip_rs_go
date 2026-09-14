package bench

import (
	"os"
	"strings"
)

// LoadDotEnv parses KEY=VALUE from .env files, ignoring comments and blank lines.
// Does NOT override already-set environment variables (mirrors python-dotenv default).
func LoadDotEnv() {
	// Look for .env file upward from current directory (mirrors python-dotenv find_dotenv)
	dir, err := os.Getwd()
	if err != nil {
		return
	}

	for {
		for _, candidate := range []string{dir + "/.env", dir + "/bench/.env"} {
			if _, err := os.Stat(candidate); err == nil {
				loadDotEnvFile(candidate)
				return
			}
		}

		// Walk upward
		lastDir := dir
		dir = strings.TrimRight(dir, "/")
		if i := strings.LastIndex(dir, "/"); i >= 0 {
			dir = dir[:i]
		} else {
			break
		}
		if dir == lastDir {
			break
		}
	}

	// Check explicit override
	if override, ok := os.LookupEnv("DOTENV_PATH"); ok {
		loadDotEnvFile(override)
	}
}

func loadDotEnvFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}

	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		// Skip comments and blank lines
		if len(line) == 0 || line[0] == '#' {
			continue
		}

		// Find the first '='
		eqIdx := strings.Index(line, "=")
		if eqIdx <= 0 {
			continue
		}

		key := strings.TrimSpace(line[:eqIdx])
		val := strings.TrimSpace(line[eqIdx+1:])

		// Remove surrounding quotes if present
		if len(val) >= 2 && ((val[0] == '"' && val[len(val)-1] == '"') || (val[0] == '\'' && val[len(val)-1] == '\'')) {
			val = val[1 : len(val)-1]
		}

		// Only set if not already present in env
		if _, exists := os.LookupEnv(key); !exists {
			os.Setenv(key, val)
		}
	}
	return nil
}
