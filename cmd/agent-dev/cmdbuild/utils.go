package cmdbuild

import "path/filepath"

func fjoin(parts ...string) string {
	return filepath.Join(parts...)
}
