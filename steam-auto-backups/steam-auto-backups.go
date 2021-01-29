package main

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"time"

	"github.com/c12h/errs"
	"github.com/c12h/steam-stuff/sVDF"
	"github.com/c12h/steam-stuff/steamfiles"
	"github.com/docopt/docopt-go"
)

type AppNum = steamfiles.AppNum

const (
	modeAutoUpdate         = 0
	modeUpdateOnLaunch     = 1
	modePriorityAutoUpdate = 2
)

var (
	modeName = [3]string{"auto-update", "update on launch", "priority-auto-update"}
	//
	settingsToRetainWhenDisabling = [3]bool{false, true, true}
	settingsToRetainWhenEnabling  = [3]bool{true, false, true}
)

type AppManifest = struct {
	AppNumber         AppNum    // The app’s identifier
	AppName           string    // The app’s name
	AutoUpdateSetting int       // modeAutoUpdate, modeUpdateOnLaunch, etc
	ModTime           time.Time // When the manifest file was last modified
}

/*=================================== CLI ====================================*/

const VERSION = "0.1"

const USAGE = `Usage:
  steam-auto-backups [-n|-y] [-s] [-a] [-q|-v|-l] [-d] [<steam-library-folder> ...]
  steam-auto-backups (-h | --help  |  --version)

Set Steam apps auto-backups mode in appmanifest_<AppNum>.acf files, keeping
their last-modified-times unchanged.
If no Steam library folders are specified as arguments, use the SLFs for the
current user’s Steam installation.

Options:
  -a, --all-apps    Change all apps, even apps set to High-Priority Auto Update
                    which are normally left unchanged
  -d, --dry-run     Do not actually change any apps, just report what would be changed
                    (overrides -s; -l affects how many apps are reported)
  -l, --loquacious  Report every app found
  -n, --off         Set apps to update only on launch (this is the default)
  -q, --quiet       Do not output anything except error messages
  -s, --skip-home   Skip apps installed in the user’s home Steam Library Folder
  -v, --verbose     Report each app that was changed
  -y, --on          Set apps to automatically update (the default is -n/--off)
`

//	("AppState") → "AutoUpdateBehavior"
//		"0" = normal auto-update
//		"1" = Update on launch
//		"2" = high-priority auto-update
const (
	modeSilent = iota
	modeTerse
	modeVerbose
	modeLoquacious
)

func main() {
	parsedArgs, err :=
		docopt.ParseArgs(USAGE, os.Args[1:], VERSION)
	DieIf2(err, "BUG", "docopt failed: %s", err)

	verbosity := modeTerse
	if optSpecified("-q", parsedArgs) {
		verbosity = modeSilent
	} else if optSpecified("-v", parsedArgs) {
		verbosity = modeVerbose
	} else if optSpecified("-l", parsedArgs) {
		verbosity = modeLoquacious
	}
	doDryRun := optSpecified("-d", parsedArgs)
	if doDryRun && verbosity < modeVerbose {
		verbosity = modeVerbose
	}
	if !doDryRun { //D//
		fmt.Printf("#D# forcing -d/--dry-run mode\n") //D//
		doDryRun = true                               //D//
	} //D//

	affectAllApps := optSpecified("-a", parsedArgs)
	skipHomeSLF := optSpecified("-s", parsedArgs)

	enableOption := optSpecified("-y", parsedArgs)
	disableOption := optSpecified("-n", parsedArgs)
	enableAutoBackup := enableOption
	if enableOption {
		if disableOption { // Both --on and --off ⇒ bug
			Die2("BUG", "docopt allowed both -n and -y")
		} // Have --on, no --off ⇒ enable auto-backups
	} else if disableOption { // No --on, have --off ⇒ disable auto-backups
		enableAutoBackup = false
	} else {
		enableAutoBackup = true // Neither --on, --off ⇒ default is enable
	}

	dirList := getSteamLibDirs("<Steam-library-folder>", parsedArgs)
	if skipHomeSLF {
		dirList = dirList[1:]
	}
	if verbosity >= modeVerbose {
		reportSettings(len(dirList), enableAutoBackup, doDryRun, affectAllApps)
	}
	for _, dirPath := range dirList {
		doAppManifests(dirPath, verbosity,
			enableAutoBackup, doDryRun, affectAllApps)
	}
}

func optSpecified(key string, parsedArgs docopt.Opts) bool {
	val, err := parsedArgs.Bool(key)
	if err != nil {
		Die2("BUG", "no key %q in docopt result %+#v", key, parsedArgs)
	}
	return val
}

func getSteamLibDirs(key string, parsedArgs docopt.Opts) []string {
	SLFargs := make([]string, 0, len(os.Args))
	argsItem, haveItem := parsedArgs[key]
	if !haveItem {
		Die2("BUG", "no key %q in docopt result %+#v", key, parsedArgs)
	}
	if list, ok := argsItem.([]string); ok {
		SLFargs = list
	} else {
		Die2("BUG", "docopt[%q] == %#v", key, argsItem)
	}

	SteamDir, SLDsConfig, err := steamfiles.FindSteamLibraryDirs(warnBadSLF)
	DieIf(err, "")

	if len(SLFargs) == 0 {
		return SLDsConfig
	}

	// We expect/allow users to specifiy “Steam Library Folders” (the ones
	// which contain subdirectories named "steamapps"), but we return the
	// pathnames of those "steamapps" directories (‘Steam Library
	// Directories’).
	steamLibDirs := make([]string, 0, len(SLFargs))
	for _, arg := range SLFargs {
		if filepath.Base(arg) != "steamapps" {
			subdir, err :=
				steamfiles.DirectoryExists(arg, "steamapps")
			if err != nil {
				Warn("cannot scan %q: %s", arg, err)
			} else {
				arg = subdir
			}
		}
		steamLibDirs = append(steamLibDirs, arg)
	}

	return steamLibDirs
}

func warnBadSLF(slfPath string, e error) {
	Warn("invalid Steam Library Folder %q: %s", slfPath, e)
}

func reportSettings(numDirs int, enableAutoBackup, doDryRun, affectAllApps bool) {
	action := "disabling"
	if doDryRun {
		if enableAutoBackup {
			action = "pretending to enable"
		} else {
			action = "pretending to disable"
		}
	} else if enableAutoBackup {
		action = "enabling"
	}
	howMany := ""
	if affectAllApps {
		howMany = " all"
	}
	dirs := "one directory"
	if numDirs != 1 {
		dirs = fmt.Sprintf("%d directories", numDirs)
	}
	WriteMessage("%s auto-backup mode for%s apps in %s", action, howMany, dirs)
}

/*========================= Processing app manifests =========================*/

func doAppManifests(dirPath string, verbosity int,
	enableAutoBackup, doDryRun, affectAllApps bool,
) {
	dh, err := os.Open(dirPath)
	if err != nil {
		warnCannot("open", "Steam library folder", dirPath, err)
		return
	}

	allNames, err := dh.Readdirnames(-1)
	dh.Close()
	if err != nil {
		warnCannot("read", "directory", dirPath, err)
		return
	}

	doneText, notDoneText, newSetting, settingsToRetain :=
		"Disabled", "Did not disable",
		modeUpdateOnLaunch, settingsToRetainWhenDisabling
	if enableAutoBackup {
		doneText = "Enabled"
		newSetting = modeAutoUpdate
		settingsToRetain = settingsToRetainWhenEnabling
	}
	if affectAllApps {
		settingsToRetain[modePriorityAutoUpdate] = false
	}
	if doDryRun {
		if enableAutoBackup {
			doneText = "Would enable"
		} else {
			doneText = "Would disable"
		}
	}

	nFound, nChanged := 0, 0
	for _, n := range allNames {
		if match := reManifestFile.FindStringSubmatch(n); match != nil {
			nFound += 1
			manifestPath := filepath.Join(dirPath, n)
			mInfo :=
				parseManifest(manifestPath, match[1])
			if mInfo == nil {
				continue
			}
			if settingsToRetain[mInfo.AutoUpdateSetting] {
				if verbosity >= modeLoquacious {
					fmt.Printf("%s auto-updates for %q (app %d)\n",
						verb, mInfo.AppName, mInfo.AppNumber)
					//???
				}
				continue
			}
			rewriteManifest(manifestPath, mInfo,
				newSetting, doDryRun, verbosity, verb)
			nChanged += 1
		}
	}
	if nFound == 0 {
		Warn("%q has no appmanifest_<N>.acf files! Not a Steam Library Dir?",
			dirPath)
	}
}

var reManifestFile = regexp.MustCompile(`^appmanifest_(\d+)\.acf$`)

// parseManifest extracts details from an appmanifest_<app#>.acf file, with lots of
// checking.
//
func parseManifest(mfPath, appNumFromFileName string) *AppManifest {
	mfInfo, err := sVDF.FromFile(mfPath, "AppState")
	if err != nil {
		warnCannot("use", "", mfPath, err)
		return nil
	}

	idText, err := mfInfo.Lookup("appid")
	if err != nil {
		warnCannot("get app ID from", "", mfPath, err)
		return nil
	}
	if idText != appNumFromFileName {
		Warn("%q is for appid %d! Oops!", manifestPath, appNum)
		return nil
	}
	idNum, err := strconv.Atoi(idText)
	if err != nil {
		Warn("%q has appid=%q, need integer", mfPath, idText)
		return nil
	}
	if idNum > math.MaxInt32 || idNum < 0 {
		Warn("%q has out-of-range appid %d", mfPath, idNum)
		return nil
	}

	appName, err := mfInfo.Lookup("name")
	if err != nil {
		warnCannot("get app name from", "", mfPath, err)
		return nil
	}

	autoUpdateText, err := mfInfo.Lookup("AutoUpdateBehavior")
	if err != nil {
		warnCannot(`get "AutoUpdateBehavior" from`, "", mfPath, err)
		return
	}
	autoUpdateValue, err := strconv.Atoi(autoUpdateText)
	if err != nil {
		Warn("%q has AutoUpdateBehavior=%q, need integer", mfPath, autoUpdateText)
		return nil
	}
	if autoUpdateValue < modeAutoUpdate || autoUpdateValue > modePriorityAutoUpdate {
		Warn("%q has bad AutoUpdateBehavior %d; need %d to %d inclusive",
			mfPath, autoUpdateValue, modeAutoUpdate, modePriorityAutoUpdate)
		return nil
	}

	return &InstalledApp{
		AppNumber:         appNum,
		AppName:           appName,
		AutoUpdateSetting: autoUpdateValue,
		ModTime:           mfInfo.ModTime}
}

/*============================ Utility Functions =============================*/

func warnCannot(verb, adjective, noun string, err error) {
	return errs.Cannot(verb, adjective, noun, true, "", err)
}
