package replay

import "path/filepath"

func dataPath(parts ...string) string { return filepath.Join(parts...) }
