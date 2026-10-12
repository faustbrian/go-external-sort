//go:build js || plan9

package externalsort

// These platforms cannot uphold descriptor-backed containment. Refuse Open
// rather than silently falling back to pathname-based storage and deletion.
func openRootDirectory(string) (rootDirectory, error) {
	return nil, ErrUnsafeParent
}
