package steamfiles

import (
	"fmt"
	"os"
	"syscall"
)

var notFoundErr = fmt.Errorf("no such entry")
var notADirErr = syscall.ENOTDIR

// findSteamHome is the system-dependent function that FindSteamHome is a
// wrapper for.
//
// It returns the pathname of the directory where Steam is installed for the
// current user (or returns an error).
//
func findSteamHome() (string, error) {
	// First, find the user’s home directory.
	path, err := os.UserHomeDir()
	if err != nil {
		return "", cannotFind("user home directory(!?)", err)
	}

	// Second, find their .steam directory.
	path, err = DirectoryExists(path, ".steam")
	if err != nil {
		return "", cannotFind("user’s .steam directory", nil)
	}

	// Third, look for "$HOME/.steam/steam" or else "$HOME/.steam/root",
	// and use the first found.  If neither exist, use "$HOME/.steam".
	homeSLF, err :=
		DirectoryExists(path, "steam")
	if homeSLF == "" {
		homeSLF, err =
			DirectoryExists(path, "root")
	}
	if homeSLF == "" {
		homeSLF = path
	}

	// Fourth, check that <homeSLF>/steamapps exists.
	// If it doesn’t, we don’t have any Steam Library Folder, which is bad.
	if _, err := DirectoryExists(homeSLF, "steamapps"); err != nil {
		return "", fmt.Errorf(`%q has no "steamapps" subdirectory (?!)`, homeSLF)
	}

	return homeSLF, nil
}
