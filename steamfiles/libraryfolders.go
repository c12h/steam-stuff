package steamfiles

import (
	"os"
	"path/filepath"
	"strconv"

	"github.com/c12h/steam-stuff/sVDF"
)

/*--------------------------- FindSteamLibraryDirs ---------------------------*/

// A BadSteamLibraryDirReporter is a callback that FindSteamLibraryDirs can use
// to report that a Steam Library Folder is not valid, a situation that is
// unlikely but not impossible.
//
type BadSteamLibraryDirReporter func(slfDir string, err error)

// FindSteamLibraryDirs returns (1) the user’s Steam intallation directory plus
// (2) a list of Steam’s Library directories, or (3) an error.  (Directories are
// returned as pathnames in the local OS’s syntax.)
//
// The list it returns contains the pathnames of the "steamapps" directory in
// each Steam Library Folder, not those of the Steam Library Folders themselves.
//
// Callers can supply a callback to report any invalid SLF; the default is to
// silently ignore
//
func FindSteamLibraryDirs(reportBadSLF BadSteamLibraryDirReporter,
) (string, []string, error) {
	SteamDir, err := FindSteamHome()
	if err != nil {
		return "", nil, err
	}

	libraryDirs := []string{filepath.Join(SteamDir, "steamapps")}
	libraryFoldersFilePath :=
		filepath.Join(libraryDirs[0], "libraryfolders.vdf")
	libraryFoldersInfo, err := sVDF.FromFile(libraryFoldersFilePath, "LibraryFolders")
	if err != nil {
		return SteamDir, nil, cannotFind("Steam library folders", err)
	}
	for i := 1; ; i += 1 {
		s := strconv.Itoa(i)
		if !libraryFoldersInfo.HaveString(s) {
			break
		}
		slf, err := libraryFoldersInfo.Lookup(s)
		if err != nil {
			panic(err.Error())
		}
		p, err := DirectoryExists(slf, "steamapps")
		if err != nil {
			if reportBadSLF != nil {
				reportBadSLF(slf, err)
			}
			continue
		}
		libraryDirs = append(libraryDirs, p)
	}

	return SteamDir, libraryDirs, nil
}

//
/*----------------------------- DirectoryExists ------------------------------*/
//

// DirectoryExists is used in this package, and is useful for callers of
// FindSteamLibraryDirs.
//

// DirectoryExists reports whether a directory exists.  The directory is
// specified as a base B (which probably should be an absolute path) plus zero
// or more child names C[i].  The function checks that B, B/C[0], B/C[0]/C[1],
// etc are directories.  (It follows any symbolic links.)  If so, it returns the
// pathname B/C[0]/C[1]/…/C[n] (in the target system’s syntax, of course).  If
// not, it returns an empty string and an error.
//
func DirectoryExists(base string, childNames ...string) (string, error) {
	p := base
	for i := -1; i < len(childNames); i++ {
		if i >= 0 {
			p = filepath.Join(p, childNames[i])
		}
		nodeinfo, err := os.Stat(p)
		if err != nil {
			if os.IsNotExist(err) {
				return "", cannot("find", "", p, notFoundErr)
			}
			return "", cannot("examine", "directory", p, err)
		}
		if !nodeinfo.IsDir() {
			return "", cannot("lookup in", "", p, notADirErr)
		}
	}
	return p, nil
}
