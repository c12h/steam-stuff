package main

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// appNewerThaN(steamappsDir, appInstallDir, t) reports whether any of the files
// in/under the install directory for an app in a particular steamLibDir are
// newer than some cutoff time t (fx, when a backup was written).
//
// This function can cope with incorrect casing of the appInstallDir value on
// case-sensitive file systems.
//
// (I have seen this, in a DLC (where it does not matter to Steam and so can get
// past their testing. "X3: Albion Prelude" (appmanifest_201310.acf) has
//	"installdir"	"x3 terran conflict"
// but should have "X3 Terran Conflict". It turns out we can handle this on
// Linux with a few extra lines of code and a millisecond or so, so we do that.)
//
func appNewerThan(steamLibDir, appInstallDir string, skuTime time.Time) (bool, error) {
	installsDir := filepath.Join(steamLibDir, appsInstallRelPath)
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
		return false, cannot(err, "open directory", dirPath)
	}

	nodes, err := dh.Readdir(-1)
	dh.Close()
	if err != nil {
		return false, cannot(err, "read directory", dirPath)
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

// On Linux, we cache the app install directory names in each
// …/steamapps/common/ directory we encounter as a map from directory name to a
// map from monocase(appInstallDir) to appInstallDir.
//
// On Windows and Mac, findIgnoringCase() never gets called (unless something is
// wrong with the files in …/steamapps/), so this
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
		return "", cannot(os.ErrNotExist,
			"find app, even ignoring case", filepath.Join(dirPath, wrongName))
	}
	//D// fmt.Printf("#D# namesCacheForDir[%q][%q] = %q\n",
	//D//	dirPath, monocase(wrongName), mappedName)
	return filepath.Join(dirPath, mappedName), nil
}

func scanDirIgnoringCase(dirPath string) (map[string]string, error) {
	ret := make(map[string]string)

	dh, err := os.Open(dirPath)
	if err != nil {
		return nil, cannot(err, "open directory", dirPath)
	}

	names, err := dh.Readdirnames(-1)
	dh.Close()
	if err != nil {
		return nil, cannot(err, "read directory", dirPath)
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
