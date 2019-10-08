package updater

import (
	"fmt"
	"regexp"
	"strings"
)

var (
	fileVersionRegex = regexp.MustCompile(`_v([0-9]+-[0-9]+-[0-9]+b?|0)`)
	rawVersionRegex  = regexp.MustCompile(`^([0-9]+\.[0-9]+\.[0-9]+b?\*?|0)$`)
)

// GetIdentifierAndVersion splits the given file path into its identifier and version.
func GetIdentifierAndVersion(versionedPath string) (identifier, version string, ok bool) {
	// extract version
	rawVersion := fileVersionRegex.FindString(versionedPath)
	if rawVersion == "" {
		return "", "", false
	}

	// replace - with . and trim _
	version = strings.Replace(strings.TrimLeft(rawVersion, "_v"), "-", ".", -1)

	// put together without version
	i := strings.Index(versionedPath, rawVersion)
	if i < 0 {
		// extracted version not in string (impossible)
		return "", "", false
	}
	return versionedPath[:i] + versionedPath[i+len(rawVersion):], version, true
}

// GetVersionedPath combines the identifier and version and returns it as a file path.
func GetVersionedPath(identifier, version string) (versionedPath string) {
	// split in half
	splittedFilePath := strings.SplitN(identifier, ".", 2)
	// replace . with -
	transformedVersion := strings.Replace(version, ".", "-", -1)

	// put together
	if len(splittedFilePath) == 1 {
		return fmt.Sprintf("%s_v%s", splittedFilePath[0], transformedVersion)
	}
	return fmt.Sprintf("%s_v%s.%s", splittedFilePath[0], transformedVersion, splittedFilePath[1])
}
