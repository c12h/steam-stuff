// Package steamfiles deals with locally-installed Steam apps and their backups.
//
// It allows callers to find and examine installed Steam apps and backups of
// Steam apps in local storage.
//
// Steam can have more than one "Steam library folder".  Each such folder is
// rooted at a directory named "steamapps" (or perhaps "SteamApps"?).  That
// directory contains a text file named appmanifest_<N>.acf for each app
// installed there, where <N> is the app’s numeric (decimal) ID.  These files
// are in the Valve Data Format (specifically in what I call the "simple VDF" in
// which all keys and final values are double-quoted strings).
//
// Steam backups are stored as a directory whose name reflects the apps in that
// backup.  (You can backup multiple apps together.)  A backup is divided into
// one or more subdirectories named "Disk_1", "Disk_2", etc. ...???... sku.sis
//
package steamfiles // import "github.com/c12h/steam-stuff/steamfiles"

import (
	"fmt"
	"math"
	"strconv"
)

// Steam identifies apps by a positive integer.
//
// As of January 2021, the largest app ID in use is 2,028,850, so int32 is wide
// enough.
//
type AppNum int32

// parseAppNum gets an AppNum from a string, with lots of error checking.
//
func parseAppNum(text, path string) (AppNum, error) {
	appNum, err := strconv.Atoi(text)
	if err != nil {
		return 0, fileError(path, "", "has app ID %q, need integer", text)
	}
	if appNum > math.MaxInt32 {
		panic(fmt.Sprintf("app ID %d from file %q is too big for int32!",
			appNum, path))
	} else if appNum <= 0 {
		return 0, fileError(path, "", "has app ID %d!?", appNum)
	}
	return AppNum(appNum), nil
}
