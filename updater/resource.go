package updater

import (
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	semver "github.com/hashicorp/go-version"

	"github.com/safing/portbase/log"
)

var devVersion *semver.Version

func init() {
	var err error
	devVersion, err = semver.NewVersion("0")
	if err != nil {
		panic(err)
	}
}

// Resource represents a resource (via an identifier) and multiple file versions.
type Resource struct {
	sync.Mutex
	registry *ResourceRegistry
	notifier *notifier

	// Identifier is the unique identifier for that resource.
	// It forms a file path using a forward-slash as the
	// path separator.
	Identifier string

	// Versions holds all available resource versions.
	Versions []*ResourceVersion

	// ActiveVersion is the last version of the resource
	// that someone requested using GetFile().
	ActiveVersion *ResourceVersion

	// SelectedVersion is newest, selectable version of
	// that resource that is available. A version
	// is selectable if it's not blacklisted by the user.
	// Note that it's not guaranteed that the selected version
	// is available locally. In that case, GetFile will attempt
	// to download the latest version from the updates servers
	// specified in the resource registry.
	SelectedVersion *ResourceVersion
}

// ResourceVersion represents a single version of a resource.
type ResourceVersion struct {
	resource *Resource

	// VersionNumber is the string representation of the resource
	// version.
	VersionNumber string
	semVer        *semver.Version

	// Available indicates if this version is available locally.
	Available bool

	// CurrentRelease indicates that this is the current release that should be
	// selected, if possible.
	CurrentRelease bool

	// PreRelease indicates that this version is pre-release.
	PreRelease bool

	// Blacklisted may be set to true if this version should
	// be skipped and not used. This is useful if the version
	// is known to be broken.
	Blacklisted bool
}

func (rv *ResourceVersion) String() string {
	return rv.VersionNumber
}

// SemVer returns the semantiv version of the resource.
func (rv *ResourceVersion) SemVer() *semver.Version {
	return rv.semVer
}

// EqualsVersion normalizes the given version and checks equality with semver.
func (rv *ResourceVersion) EqualsVersion(version string) bool {
	cmpSemVer, err := semver.NewVersion(version)
	if err != nil {
		return false
	}

	return rv.semVer.Equal(cmpSemVer)
}

// isSelectable returns true if the version represented by rv is selectable.
// A version is selectable if it's not blacklisted and either already locally
// available or ready to be downloaded.
func (rv *ResourceVersion) isSelectable() bool {
	return !rv.Blacklisted && (rv.Available || rv.resource.registry.Online)
}

// isBetaVersionNumber checks if rv is marked as a beta version by checking
// the version string. It does not honor the BetaRelease field of rv!
func (rv *ResourceVersion) isBetaVersionNumber() bool { //nolint:unused
	// "b" suffix check if for backwards compatibility
	// new versions should use the pre-release suffix as
	// declared by https://semver.org
	// i.e. 1.2.3-beta
	switch rv.semVer.Prerelease() {
	case "b", "beta":
		return true
	default:
		return false
	}
}

// Len is the number of elements in the collection.
// It implements sort.Interface for ResourceVersion.
func (res *Resource) Len() int {
	return len(res.Versions)
}

// Less reports whether the element with index i should
// sort before the element with index j.
// It implements sort.Interface for ResourceVersions.
func (res *Resource) Less(i, j int) bool {
	return res.Versions[i].semVer.GreaterThan(res.Versions[j].semVer)
}

// Swap swaps the elements with indexes i and j.
// It implements sort.Interface for ResourceVersions.
func (res *Resource) Swap(i, j int) {
	res.Versions[i], res.Versions[j] = res.Versions[j], res.Versions[i]
}

// available returns whether any version of the resource is available.
func (res *Resource) available() bool {
	for _, rv := range res.Versions {
		if rv.Available {
			return true
		}
	}
	return false
}

// inUse returns true if the resource is currently in use.
func (res *Resource) inUse() bool {
	return res.ActiveVersion != nil
}

// AnyVersionAvailable returns true if any version of
// res is locally available.
func (res *Resource) AnyVersionAvailable() bool {
	res.Lock()
	defer res.Unlock()

	return res.available()
}

func (reg *ResourceRegistry) newResource(identifier string) *Resource {
	return &Resource{
		registry:   reg,
		Identifier: identifier,
		Versions:   make([]*ResourceVersion, 0, 1),
	}
}

// AddVersion adds a resource version to a resource.
func (res *Resource) AddVersion(version string, available, currentRelease, preRelease bool) error {
	res.Lock()
	defer res.Unlock()

	// reset current release flags
	if currentRelease {
		for _, rv := range res.Versions {
			rv.CurrentRelease = false
		}
	}

	var rv *ResourceVersion
	// check for existing version
	for _, possibleMatch := range res.Versions {
		if possibleMatch.VersionNumber == version {
			rv = possibleMatch
			break
		}
	}

	// create new version if none found
	if rv == nil {
		// parse to semver
		sv, err := semver.NewVersion(version)
		if err != nil {
			return err
		}

		rv = &ResourceVersion{
			resource:      res,
			VersionNumber: sv.String(), // Use normalized version.
			semVer:        sv,
		}
		res.Versions = append(res.Versions, rv)
	}

	// set flags
	if available {
		rv.Available = true
	}
	if currentRelease {
		rv.CurrentRelease = true
	}
	if preRelease || rv.semVer.Prerelease() != "" {
		rv.PreRelease = true
	}

	return nil
}

// GetFile returns the selected version as a *File.
func (res *Resource) GetFile() *File {
	res.Lock()
	defer res.Unlock()

	// check for notifier
	if res.notifier == nil {
		// create new notifier
		res.notifier = newNotifier()
	}

	// check if version is selected
	if res.SelectedVersion == nil {
		res.selectVersion()
	}

	// create file
	return &File{
		resource:      res,
		version:       res.SelectedVersion,
		notifier:      res.notifier,
		versionedPath: res.SelectedVersion.versionedPath(),
		storagePath:   res.SelectedVersion.storagePath(),
	}
}

//nolint:gocognit // function already kept as simple as possible
func (res *Resource) selectVersion() {
	sort.Sort(res)

	// export after we finish
	defer func() {
		log.Tracef("updater: selected version %s for resource %s", res.SelectedVersion, res.Identifier)

		if res.inUse() &&
			res.SelectedVersion != res.ActiveVersion && // new selected version does not match previously selected version
			res.notifier != nil {

			res.notifier.markAsUpgradeable()
			res.notifier = nil

			log.Debugf("updater: active version of %s is %s, update available", res.Identifier, res.ActiveVersion.VersionNumber)
		}
	}()

	if len(res.Versions) == 0 {
		// TODO: find better way to deal with an empty version slice (which should not happen)
		res.SelectedVersion = nil
		return
	}

	// Target selection

	// 1) Dev release if dev mode is active and ignore blacklisting
	if res.registry.DevMode {
		// Get last version, as this will be v0.0.0, if available.
		rv := res.Versions[len(res.Versions)-1]
		// Check if it's v0.0.0.
		if rv.semVer.Equal(devVersion) && rv.Available {
			res.SelectedVersion = rv
			return
		}
	}

	// 2) Find the current release. This may be also be a pre-release.
	for _, rv := range res.Versions {
		if rv.CurrentRelease {
			if rv.isSelectable() {
				res.SelectedVersion = rv
				return
			}
			// There can only be once current release,
			// so we can abort after finding one.
			break
		}
	}

	// 3) If UsePreReleases is set, find any newest version.
	if res.registry.UsePreReleases {
		for _, rv := range res.Versions {
			if rv.isSelectable() {
				res.SelectedVersion = rv
				return
			}
		}
	}

	// 4) Find the newest stable version.
	for _, rv := range res.Versions {
		if !rv.PreRelease && rv.isSelectable() {
			res.SelectedVersion = rv
			return
		}
	}

	// 5) Default to newest.
	res.SelectedVersion = res.Versions[0]
	log.Warningf("updater: falling back to version %s for %s because we failed to find a selectable one", res.SelectedVersion, res.Identifier)
}

// Blacklist blacklists the specified version and selects a new version.
func (res *Resource) Blacklist(version string) error {
	res.Lock()
	defer res.Unlock()

	// count available and valid versions
	valid := 0
	for _, rv := range res.Versions {
		if rv.semVer.Equal(devVersion) {
			continue // ignore dev versions
		}
		if !rv.Blacklisted {
			valid++
		}
	}
	if valid <= 1 {
		return errors.New("cannot blacklist last version") // last one, cannot blacklist!
	}

	// find version and blacklist
	for _, rv := range res.Versions {
		if rv.VersionNumber == version {
			// blacklist and update
			rv.Blacklisted = true
			res.selectVersion()
			return nil
		}
	}

	return errors.New("could not find version")
}

// Purge deletes old updates, retaining a certain amount, specified by
// the keep parameter. Purge will always keep at least 2 versions so
// specifying a smaller keep value will have no effect.
func (res *Resource) Purge(keepExtra int) { //nolint:gocognit
	res.Lock()
	defer res.Unlock()

	// If there is any blacklisted version within the resource, pause purging.
	// In this case we may need extra available versions beyond what would be
	// available after purging.
	for _, rv := range res.Versions {
		if rv.Blacklisted {
			log.Debugf(
				"%s: pausing purging of resource %s, as it contains blacklisted items",
				res.registry.Name,
				rv.resource.Identifier,
			)
			return
		}
	}

	// Safeguard the amount of extra version to keep.
	if keepExtra < 2 {
		keepExtra = 2
	}

	// Search for purge boundary.
	var purgeBoundary int
	var skippedActiveVersion bool
	var skippedSelectedVersion bool
	var skippedStableVersion bool
boundarySearch:
	for i, rv := range res.Versions {
		// Check if required versions are already skipped.
		switch {
		case !skippedActiveVersion && res.ActiveVersion != nil:
			// Skip versions until the active version, if it's set.
		case !skippedSelectedVersion && res.SelectedVersion != nil:
			// Skip versions until the selected version, if it's set.
		case !skippedStableVersion:
			// Skip versions until the stable version.
		default:
			// All required version skipped, set purge boundary.
			purgeBoundary = i + keepExtra
			break boundarySearch
		}

		// Check if current instance is a required version.
		if rv == res.ActiveVersion {
			skippedActiveVersion = true
		}
		if rv == res.SelectedVersion {
			skippedSelectedVersion = true
		}
		if !rv.PreRelease {
			skippedStableVersion = true
		}
	}

	// Check if there is anything to purge at all.
	if purgeBoundary <= keepExtra || purgeBoundary >= len(res.Versions) {
		return
	}

	// Purge everything beyond the purge boundary.
	for _, rv := range res.Versions[purgeBoundary:] {
		storagePath := rv.storagePath()
		// Remove resource file.
		err := os.Remove(storagePath)
		if err != nil {
			log.Warningf("%s: failed to purge resource %s v%s: %s", res.registry.Name, rv.resource.Identifier, rv.VersionNumber, err)
		} else {
			log.Tracef("%s: purged resource %s v%s", res.registry.Name, rv.resource.Identifier, rv.VersionNumber)
		}

		// Remove unpacked version of resource.
		ext := filepath.Ext(storagePath)
		if ext == "" {
			// Nothing to do if file does not have an extension.
			continue
		}
		unpackedPath := strings.TrimSuffix(storagePath, ext)

		// Remove if it exists, or an error occurs on access.
		_, err = os.Stat(unpackedPath)
		if err == nil || !os.IsNotExist(err) {
			err = os.Remove(unpackedPath)
			if err != nil {
				log.Warningf("%s: failed to purge unpacked resource %s v%s: %s", res.registry.Name, rv.resource.Identifier, rv.VersionNumber, err)
			} else {
				log.Tracef("%s: purged unpacked resource %s v%s", res.registry.Name, rv.resource.Identifier, rv.VersionNumber)
			}
		}
	}

	// remove entries of deleted files
	res.Versions = res.Versions[purgeBoundary:]
}

func (rv *ResourceVersion) versionedPath() string {
	return GetVersionedPath(rv.resource.Identifier, rv.VersionNumber)
}

func (rv *ResourceVersion) storagePath() string {
	return filepath.Join(rv.resource.registry.storageDir.Path, filepath.FromSlash(rv.versionedPath()))
}
