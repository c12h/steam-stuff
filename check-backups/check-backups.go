package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	// "github.com/c12h/errs" //???TO-DO: better name coming one day ...
	"github.com/c12h/steam-stuff/steamfiles"
	"github.com/docopt/docopt-go"
)

type AppNum = steamfiles.AppNum

/*=================================== CLI ====================================*/

const VERSION = "0.4"

const USAGEf = `Usage:
  %s [options] [<Steam-library-folder> ...]
  %s (-h | --help  |  --version)

Check for missing and outdated Steam backups. If no Steam library folders are
specified as arguments, use those for the current user’s Steam installation.

Options:
  -b <backups-dir>  Scan this directory to search for backups instead of Steam’s default
  -r                Report backups with no appmanifest_<app#>.acf in <lib-dir>
  -s                Skip apps installed in user’s home Steam Library Folder
  -v                Output progress reports
`

//???TO-DO:
// steam-backups tidy ...???
// steam-backups check [-s] [-g] [-r] [-v] [-a=<steam-lib-dir> ...] [-b=<backups-dir> ...]
//
// Options:
//	-g, --group-warnings      Group warnings by kind of problem
//	-r, --report-uninstalled  Report any backups that are not installed
//	-s, --skip-initial-lib    Skip apps installed in the initial library directory
//

func main() {
	progName := filepath.Base(os.Args[0])
	usageText := fmt.Sprintf(USAGEf,
		progName, progName)
	parsedArgs, err :=
		docopt.ParseArgs(usageText, os.Args[1:], VERSION)
	DieIf2(err, "BUG", "docopt failed: %s", err)

	reportBackupsNotInLib := optSpecified("-r", parsedArgs)
	verbose := optSpecified("-v", parsedArgs)
	skipHomeSLF := optSpecified("-s", parsedArgs)

	backupsDir := filepath.Clean(getArg("-b", parsedArgs))
	steamLibDirs := getSteamLibDirs("<Steam-library-folder>", parsedArgs, &backupsDir)
	p, err := filepath.EvalSymlinks(backupsDir)
	DieIf(err, "cannot follow symlinks in %q: %s", backupsDir, err)
	if p != backupsDir {
		WriteMessage("", "backups directory %q symlinks to %q",
			backupsDir, p)
		backupsDir = p
	}

	scanSteamLibraryDirs(steamLibDirs, skipHomeSLF, verbose)

	check(backupsDir, reportBackupsNotInLib, verbose)
}

func optSpecified(key string, parsedArgs docopt.Opts) bool {
	val, err := parsedArgs.Bool(key)
	if err != nil {
		Die2("BUG", "no key %q in docopt result %+#v", key, parsedArgs)
	}
	return val
}

func getArg(key string, parsedArgs docopt.Opts) string {
	argsItem, haveItem := parsedArgs[key]
	if !haveItem {
		Die2("BUG", "no key %q in docopt result %+#v", key, parsedArgs)
	}
	if argsItem == nil {
		return ""
	}
	string, haveString := argsItem.(string)
	if !haveString {
		Die2("BUG", "weird value %#v for %q in docopt result", argsItem, key)
	}
	return string
}

func getSteamLibDirs(key string, parsedArgs docopt.Opts, backupsDir *string) []string {
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

	if *backupsDir == "" || *backupsDir == "." {
		configBackupsDir, err := steamfiles.DirectoryExists(SteamDir, "Backups")
		DieIf(err, "cannot find default backups directory: %s", err)
		*backupsDir = configBackupsDir
	}

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

//
/*============ Scanning Steam Library Folders for installed apps =============*/
//

var manifestInfoForAppNum steamfiles.InstalledAppForAppNum

func scanSteamLibraryDirs(dirList []string, skipHomeSLF, verbose bool) {
	manifestInfoForAppNum = make(steamfiles.InstalledAppForAppNum)

	nMappedApps := 0
	if skipHomeSLF {
		dirList = dirList[1:]
	}
	for _, dirPath := range dirList {
		err := steamfiles.ScanSteamLibDir(
			dirPath, manifestInfoForAppNum, reportOldManifest)
		DieIf(err, "")
		if verbose {
			nAdded := len(manifestInfoForAppNum) - nMappedApps
			reportCount(nAdded, "valid appmanifest_$N.acf file", dirPath)
		}
		nMappedApps = len(manifestInfoForAppNum)
	}
	if verbose && len(dirList) > 1 {
		reportCount(nMappedApps, "valid appmanifest_$N.acf file",
			fmt.Sprintf("in %d directories", len(dirList)))
	}
}

/*======================= Scanning a backup directory ========================*/

func check(steamBackupsDir string, reportUninstalled, verbose bool) {
	backupInfoForAppNum := make(steamfiles.AppBackupForAppNum)
	err := steamfiles.ScanBackupsDir(
		steamBackupsDir, backupInfoForAppNum, handleDupeBackup)
	DieIf(err, "")
	if verbose {
		reportCount(len(backupInfoForAppNum), "Steam backup", steamBackupsDir)
	}

	for mAppNum, mInfo := range manifestInfoForAppNum {
		bInfo, ok := backupInfoForAppNum[mAppNum]
		if !ok {
			recordProblem(noBackup, mInfo.AppName, mInfo.AppNumber)
		} else {
			// ???TO-DO: compare mInfo.Name to bInfo.Name

			if mInfo.ModTime.After(bInfo.ModTime) {
				newer, err := steamfiles.AppNewerThan(
					mInfo.LibraryFolders[0], mInfo.InstallDir,
					bInfo.ModTime)
				WarnIf(err, "")
				if newer {
					recordProblem(oldBackup, mInfo.AppName, mAppNum)
				}
			}
		}
	}

	if reportUninstalled {
		for bAppNum, bInfo := range backupInfoForAppNum {
			if _, ok := manifestInfoForAppNum[bAppNum]; !ok {
				recordProblem(notInstalled, bInfo.BackupName, bAppNum)
			}
		}
	}

	reportProblems(verbose)
}

/*---------------- Callback for reporting duplicate manifests ----------------*/

// reportOldManifest is called by ScanSteamLibDir() when it finds a second or
// later manifest for an app.
//
func reportOldManifest(prev, curr *steamfiles.InstalledApp, usingCurr bool) {
	t := new(strings.Builder)
	switch len(prev.LibraryFolders) {
	case 1:
		fmt.Fprintf(t, "a second manifest")
	default:
		fmt.Fprintf(t, "manifest #%d", len(prev.LibraryFolders)+1)
	}
	fmt.Fprintf(t, " for app #%d (%q) with ", curr.AppNumber, curr.AppName)

	isDifferent := true
	if prev.AppName == curr.AppName {
		if prev.InstallDir == curr.InstallDir {
			isDifferent = false
			fmt.Fprintf(t, "the same details")
		} else {
			fmt.Fprintf(t, `"installdir" now %q (was %q)`,
				curr.InstallDir, prev.InstallDir)
		}
	} else {
		if prev.InstallDir == curr.InstallDir {
			fmt.Fprintf(t, "new name (was %q)", prev.AppName)
		} else {
			fmt.Fprintf(t, `new name (was %q), "installdir"`, prev.AppName)
		}
	}
	fmt.Fprintf(t, "\n   in %q\n", curr.LibraryFolders[0]+string(filepath.Separator))
	for i, path := range prev.LibraryFolders {
		fmt.Fprintf(t, "%5s: %q\n", fmt.Sprintf("#%d", i+1), path)
	}

	verb := "Found"
	if isDifferent {
		if usingCurr {
			verb += "and using"
		} else {
			verb += "and ignoring"
		}
	}
	fmt.Fprintf(os.Stderr, " %s %s", verb, t.String())
}

/*---------- Callback for reporting and choosing duplicate backups -----------*/

func handleDupeBackup(appNum AppNum, prev, curr *steamfiles.AppBackup) bool {
	if len(prev.AppNumbers) > 1 && len(curr.AppNumbers) == 1 {
		reportBackupChoice(appNum,
			"single-app backup", curr,
			fmt.Sprintf("%d-app backup", len(prev.AppNumbers)), prev)
		return true
	} else if len(prev.AppNumbers) == 1 && len(curr.AppNumbers) > 1 {
		reportBackupChoice(appNum,
			"single-app backup", prev,
			fmt.Sprintf("%d-app backup", len(curr.AppNumbers)), curr)
		return false
	}

	// Discard the one with the earlier ModTime; Keep the latest.
	ret, d, k := true, prev, curr
	if d.ModTime.After(k.ModTime) {
		ret, d, k = false, curr, prev
	}
	reportBackupChoice(appNum,
		d.ModTime.Format("older backup (2006-01-02t15:04:05)"), curr,
		k.ModTime.Format("newer backup (2006-01-02t15:04:05)"), prev)
	return ret
}
func reportBackupChoice(appNum AppNum,
	dText string, dInfo *steamfiles.AppBackup, // The one to be discarded
	kText string, kInfo *steamfiles.AppBackup, // The one to be kept
) {

	appName, suffix := kInfo.BackupName, "?"
	if manifestInfo, haveManifest := manifestInfoForAppNum[appNum]; haveManifest {
		appName, suffix = manifestInfo.AppName, ""
	}
	fmt.Fprintf(os.Stderr,
		(" Found multiple backups for app %d (%q%s):\n" +
			"\tUsing %s %q,\n" +
			"\tIgnoring %s %q\n"),
		appNum, appName, suffix,
		dText, dInfo.BackupPath,
		kText, kInfo.BackupPath)
}

/*=========================== Reporting ‘problems’ ===========================*/

type problemKind byte
type problemInfo struct {
	kind      problemKind
	appName   string
	appNumber AppNum
}

const (
	noBackup     = problemKind('N')
	oldBackup    = problemKind('O')
	notInstalled = problemKind('U')
)

var formatForProblem = map[problemKind]string{
	noBackup:     "  no backup here for %q (%d)\n",
	oldBackup:    "  backup for %q (%d) may be out of date\n",
	notInstalled: "  %q (%d) is not installed there\n", // "there"???
}
var problems []problemInfo

func recordProblem(kind problemKind, appName string, appNumber AppNum) {
	problems = append(problems,
		problemInfo{
			kind:      kind,
			appName:   appName,
			appNumber: appNumber})
}
func reportProblems(verbose bool) {
	if len(problems) == 0 {
		if verbose {
			fmt.Printf(" No problems found\n")
		}
		return
	}
	reportCount(len(problems), "problem", "")

	// (???) For "problems sorted by app name" mode:
	sort.Slice(problems,
		func(i, j int) bool {
			return problems[i].appName < problems[j].appName
		})

	for _, p := range problems {
		fmt.Printf(formatForProblem[p.kind], p.appName, p.appNumber)
	}

	// ??? For "problems by category", things would be different.
}

/*============================ Utility Functions =============================*/

func reportCount(n int, noun, where string) {
	if strings.HasPrefix(where, "in ") {
		where = " " + where
	} else if where != "" {
		where = fmt.Sprintf(" in %q", where)
	}
	if n == 1 {
		fmt.Printf(" Found one %s%s\n", noun, where)
	} else {
		fmt.Printf(" Found %d %ss%s\n", n, noun, where)
	}
}
