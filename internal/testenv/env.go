package testenv

import "path/filepath"

// Dirs contains isolated directories for vibeusage config/data/cache in tests.
type Dirs struct {
	Base   string
	Config string
	Data   string
	Cache  string
}

// VibeusageDirs returns conventional test directories rooted at base.
func VibeusageDirs(base string) Dirs {
	return Dirs{
		Base:   base,
		Config: filepath.Join(base, "config"),
		Data:   filepath.Join(base, "data"),
		Cache:  filepath.Join(base, "cache"),
	}
}

// ApplyVibeusage sets VIBEUSAGE_* env vars to isolated test directories.
func ApplyVibeusage(setenv func(string, string), base string) Dirs {
	dirs := VibeusageDirs(base)
	setenv("VIBEUSAGE_CONFIG_DIR", dirs.Config)
	setenv("VIBEUSAGE_DATA_DIR", dirs.Data)
	setenv("VIBEUSAGE_CACHE_DIR", dirs.Cache)
	return dirs
}

// ApplySameDir points config/data/cache to the same directory.
// Useful in tests that expect ConfigDir() to exactly match a temp dir path.
func ApplySameDir(setenv func(string, string), dir string) {
	setenv("VIBEUSAGE_CONFIG_DIR", dir)
	setenv("VIBEUSAGE_DATA_DIR", dir)
	setenv("VIBEUSAGE_CACHE_DIR", dir)
}
