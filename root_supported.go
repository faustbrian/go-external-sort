//go:build !js && !plan9

package externalsort

import "os"

// Anchor all temporary IO and cleanup to a rename-stable parent descriptor.
func openRootDirectory(path string) (rootDirectory, error) {
	return os.OpenRoot(path)
}
