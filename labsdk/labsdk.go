package labsdk

// This file is only importing our go dependencies, so they won't be deducted with `go mod tidy`
// The actual code, is being auto-generated via the `setup.py` file, and is not committed to the repo(gitignore'd)
import (
	_ "github.com/go-logr/logr"
	_ "github.com/go-python/gopy/gopyh"
)
