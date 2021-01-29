// Functions etc for scanning appmanifest_<AppNum>.acf files
// and inspecting installed files for apps.

package steamfiles

import (
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/c12h/errs"
	"github.com/c12h/steam-stuff/sVDF"
)

// An InstalledApp value holds details of a Steam app as found on a local file
// system, taken from its appmanifest_<N>.acf file.
//
// If ScanSteamLibDir is used on multiple Steam library folders with the same
// map, it may find the same app installed multiple times. In that case,
// ScanSteamLibDir uses the most recent appmanifest_<AppNum>.acf file it finds,
// and ensures that .LibraryFolders[0] is the directory holding that file.
//
type InstalledApp struct {
	AppNumber      AppNum     // The app’s identifier
	AppName        string    // The app’s name
	LibraryFolders []string  // Which Steam library folders the app was found in
	InstallDir     string    // Its files go in/under <LibraryDir>/common/<InstallDir>
	ModTime        time.Time // When the manifest file was last modified
}

// ScanSteamLibDir adds InstalledApp values to a map indexed by AppNum.
//
type InstalledAppForAppNum map[AppNum]*InstalledApp

// An OldManifestReporter is a callback for reporting multiple manifests for the
// same app.
//
// If ScanSteamLibDir() finds more than one manifest for an app (eg., if a
// caller scans multiple Steam), it uses the most recently modified file, and
// calls the OldManifestReporter, passing in the previous InstalledApp value,
// the value just extracted from the manifest and a flag saying whether it will
// use the latter.
//
// When it is called, prev.LibraryFolders will list the directory(ies) in which
// the manifest was found earlier, and curr.LibraryFolders will specify the
// directory in which a new manifest was just found.
//
// The callback probably should report whether the AppName or InstallDir fields
// of the two InstalledApp values are different.  Sample code:
//	sameDetails := prev.AppName == curr.AppName && prev.InstallDir == curr.InstallDir
//
type OldManifestReporter func(prev, curr *InstalledApp, usingCurr bool)

// ScanSteamLibDir finds and parses all the appmanifest_<app#>.acf files in a
// ‘Steam library directory’, and records them in a caller-supplied map.
//
// If handleDiff is nil, ScanSteamLibDir will handle duplicate manifests by
// silently using the last one found.
//
func ScanSteamLibDir(
	libPath string, theMap map[AppNum]*InstalledApp, handleDiff OldManifestReporter,
) error {
	if handleDiff == nil {
		handleDiff = ignoreDiff
	}
	dh, err := os.Open(libPath)
	if err != nil {
		return cannot("open", "Steam library folder", libPath, err)
	}

	allNames, err := dh.Readdirnames(-1)
	dh.Close()
	if err != nil {
		return cannot("read", "directory", libPath, err)
	}

	nFound := 0
	for _, n := range allNames {
		if match := reManifestFile.FindStringSubmatch(n); match != nil {
			currInfo, err :=
				parseManifest(filepath.Join(libPath, n))
			if err != nil {
				return err
			}
			appNum := currInfo.AppNumber
			if strconv.Itoa(int(appNum)) != match[1] {
				return fileError(n, "appid",
					"wrong appid %d for file name", appNum)
			}

			prev, havePrev := theMap[appNum]
			if havePrev {
				// Assume that this manifest file is newer ...
				newInfo, oldInfo, usingCurr :=
					currInfo, prev, true
				SLFlist := append(
					[]string{libPath}, prev.LibraryFolders...)
				if newInfo.ModTime.Before(oldInfo.ModTime) {
					// ... or perhaps not.
					newInfo, oldInfo, usingCurr =
						prev, currInfo, false
					SLFlist = append(
						prev.LibraryFolders, libPath)
				}
				currInfo.LibraryFolders = []string{libPath}
				handleDiff(prev, currInfo, usingCurr)
				//
				currInfo = newInfo
				currInfo.LibraryFolders = SLFlist

			} else {
				currInfo.LibraryFolders = []string{libPath}
			}
			theMap[appNum] = currInfo
			nFound += 1
			//
		}
	}
	if nFound == 0 {
		return errs.Cannot("see any appmanifest_<N>.acf files in", "",
			libPath, true, " — not a Steam library folder?", nil)
	}
	return nil
}

// ignoreDiff is the default OldManifestReporter.
//
func ignoreDiff(prev, curr *InstalledApp, usingCurr bool) {}

var reManifestFile = regexp.MustCompile(`^appmanifest_(\d+)\.acf$`)

// parseManifest carefully (ie., with lots of checking) extracts details from an
// appmanifest_<app#>.acf file.
//
func parseManifest(mfPath string) (*InstalledApp, error) {
	mfInfo, err := sVDF.FromFile(mfPath, "AppState")
	if err != nil {
		return nil, err
	}

	idText, err := mfInfo.Lookup("appid")
	if err != nil {
		return nil, cannot("get app ID from", "", mfPath, err)
	}
	appNum, err := parseAppNum(idText, mfPath)
	if err != nil {
		return nil, err
	}

	appName, err := mfInfo.Lookup("name")
	if err != nil {
		return nil, cannot("get app name from", "", mfPath, err)
	}

	installDir, err := mfInfo.Lookup("installdir")
	if err != nil {
		return nil, cannot(`get "installdir" from`, "", mfPath, err)
	}

	ret := &InstalledApp{
		AppNumber: appNum,
		AppName:   appName,
		// ret.LibraryFolders is set by the caller, ScanSteamLibDir.
		InstallDir: installDir,
		ModTime:    mfInfo.ModTime}
	return ret, nil
}

// AppNewerThaN(steamappsDir, appInstallDir, t) reports whether any of the files
// in an installed app are newer than some cutoff time t (fx, when a backup was
// written).
//
// The files for an installed app can be found in a directory tree rooted at
//   <steam-library-folder>/steamapps/common/<installdir>/
// where the <installdir> comes from the apps manifest file.
//
// This function can cope with incorrect casing of the appInstallDir value on
// case-sensitive file systems.  For example "X3: Albion Prelude", a DLC, has
//	"installdir"	"x3 terran conflict"
// in its manifest (appmanifest_201310.acf) instead of "X3 Terran Conflict".
// (This problem would be detected and fixed in a base game, but apparently not
// in DLCs.)  It turns out we can handle this on Linux with a few dozen lines of
// code and a millisecond or so, so we do that.
//
func AppNewerThan(steamLibDir, appInstallDir string, skuTime time.Time) (bool, error) {
	installsDir := filepath.Join(steamLibDir, "common")
	appDir := filepath.Join(installsDir, appInstallDir)
	_, err := os.Lstat(appDir)
	if err != nil && os.IsNotExist(err) {
		appDir, err = findIgnoringCase(installsDir, appInstallDir)
		if err != nil {
			return false, err
		}
	}
	return anyFileNewerThan(appDir, skuTime)
}

// anyFileNewerThan is a helper function for appNewerThan. It checks the files in
// a particular directory, then recurses to check subdirectories.
//
func anyFileNewerThan(dirPath string, t time.Time) (bool, error) {
	dh, err := os.Open(dirPath)
	if err != nil {
		return false, cannot("open", "directory", dirPath, err)
	}

	nodes, err := dh.Readdir(-1)
	dh.Close()
	if err != nil {
		return false, cannot("read", "directory", dirPath, err)
	}

	subdirs := make([]string, 0, len(nodes))
	for _, node := range nodes {
		if node.IsDir() {
			subdirs = append(subdirs, node.Name())
		} else if isRegFile(node) {
			if node.ModTime().After(t) {
				//D// fmt.Printf(" #D# file %q has mtime %s > %s\n",
				//D//	filepath.Join(dirPath, node.Name()),
				//D//	node.ModTime().Format("2006-01-02t15:04:05"),
				//D//	t.Format("2006-01-02t15:04:05"))
				return true, nil

			}
		}
	}

	for _, subdir := range subdirs {
		newer, err := anyFileNewerThan(filepath.Join(dirPath, subdir), t)
		if err != nil {
			return false, err
		}
		if newer {
			return true, nil
		}
	}

	return false, nil
}

// isRegFile() is an oft-written one-liner to report whether an os.FileInfo
// describes a regular file. If the FileInfo came from os.Stat(), a symlink to a
// regular file will also count. (os.File.Readdir() uses os.Lstat().)
//
func isRegFile(nodeInfo os.FileInfo) bool {
	return nodeInfo.Mode()&os.ModeType == 0
}

// On Linux, we cache the entry names in each "<SLF>/steamapps/common" directory
// we encounter in namesCacheForDir, which maps each "…/common" directory path
// to a second-level map from monocase(entryName) to entryName.
//
// On Windows and Mac, findIgnoringCase() never gets called (unless something is
// badly amiss with the files in …/steamapps/), so this variable never becomes
// non-nil.
//
var namesCacheForDir map[string]map[string]string

// findIgnoringCase(dirPath, wrongName) uses the name-case-correction-map for dirPath
// (which must be of the form "/…/steamapps/common/") to find the correct form of
// a wrongly-cased name, building the maps as necessary.
//
func findIgnoringCase(dirPath, wrongName string) (string, error) {
	var err error
	if namesCacheForDir == nil {
		namesCacheForDir = make(map[string]map[string]string)
	}

	mapForDir, haveScannedDir := namesCacheForDir[dirPath]
	if !haveScannedDir {
		mapForDir, err = scanDirIgnoringCase(dirPath)
		if err != nil {
			return "", err
		}
		namesCacheForDir[dirPath] = mapForDir
	}
	mappedName, haveMappedName := mapForDir[monocase(wrongName)]
	if !haveMappedName {
		return "", cannot("find app, even ignoring case", "",
			filepath.Join(dirPath, wrongName), os.ErrNotExist)
	}
	//D// fmt.Printf("#D# namesCacheForDir[%q][%q] = %q\n",
	//D//	dirPath, monocase(wrongName), mappedName)
	return filepath.Join(dirPath, mappedName), nil
}

// scanDirIgnoringCase is the recursive directory scanner for findIgnoringCase.
//
func scanDirIgnoringCase(dirPath string) (map[string]string, error) {
	ret := make(map[string]string)

	dh, err := os.Open(dirPath)
	if err != nil {
		return nil, cannot("open", "directory", dirPath, err)
	}

	names, err := dh.Readdirnames(-1)
	dh.Close()
	if err != nil {
		return nil, cannot("read", "directory", dirPath, err)
	}

	//B// startTime := time.Now()
	for _, n := range names {
		ret[monocase(n)] = n
	}
	//B// duration := time.Since(startTime)
	//B// fmt.Printf("#B# monocasing %d names in dir %q took %s\n",
	//B//	len(names), dirPath, duration)
	return ret, nil
}

// I considered making this faster by only upcasing ASCII a-z, since non-ASCII
// letters are not found (or at least very rare) in app install directory names.
// But a little benchmarking showed that using strings.ToUpper() took < 1ms with
// over 500 titles, which is plenty good enough.

// monocase(s) returns the uppercased form of s.
//
func monocase(s string) string {
	return strings.ToUpper(s)
}
