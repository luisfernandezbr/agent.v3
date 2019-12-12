package cmdbuild

import "path/filepath"

func fjoin(pathElem ...string) string {
	return filepath.Join(pathElem...)
}
