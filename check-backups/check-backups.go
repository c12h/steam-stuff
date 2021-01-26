package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	// "github.com/c12h/errs" //???TO-DO: better name coming one day ...
	"github.com/c12h/steam-stuff/steamfiles"
	"github.com/docopt/docopt-go"
)

const (
	defaultAppsLibDir = "/Store/X/LinuxGames/Steam/SteamApps/steamapps/"
	defaultBackupsDir = "/C/++Steam-Backups-for-Linux/"
	//
	appsInstallRelPath = "common"
	//
	// Maybe one day ...???
	steamLibsList = "~/.steam/debian-installation/steamapps/libraryfolders.vdf"
	// Hmmm ....
	//  steamRoot, err := filepath.EvalSymlinks("$HOME/.steam/root")
	//  steamHome := filepath.Join(steamRoot, "steamapps")
	//  libDirsList := []string{steamHome}
	//  libFoldersSpec :=
	//	ioutil.ReadFile(filepath.Join(steamHome, "libraryfolders.vdf"))
	//  libPaths := []string{
)

/*=================================== CLI ====================================*/

const VERSION = "0.1"

const USAGEf = `Usage:
  %s [options]
  %s (-h | --help  |  --version)

Check for missing and outdated Steam backups.

Options:
  -a=<lib-dir>      Specify a steam library directory containing appmanifest_#.acf files
                    [default: %s]
  -b=<backups-dir>  Specify the directory to search for backups
                    [default: %s]
  -d                Output lots of information
  -r                Report backups with no appmanifest_<app#>.acf in <lib-dir>
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
		progName, progName, defaultAppsLibDir, defaultBackupsDir)
	parsedArgs, err :=
		docopt.ParseArgs(usageText, os.Args[1:], VERSION)
	DieIf2(err, "BUG", "docopt failed: %s", err)

	steamAppsLibDir := filepath.Clean(getArg("-a", parsedArgs))
	steamBackupsDir := filepath.Clean(getArg("-b", parsedArgs))
	reportBackupsNotInLib := optSpecified("-r", parsedArgs)
	debugging := optSpecified("-d", parsedArgs)
	verbose := optSpecified("-v", parsedArgs)

	check(steamAppsLibDir, steamBackupsDir,
		reportBackupsNotInLib, verbose, debugging)
}

func optSpecified(key string, parsedArgs docopt.Opts) bool {
	val, err := parsedArgs.Bool(key)
	if err != nil {
		Die2("usage", "no key %q in docopt result %+#v", key, parsedArgs)
	}
	return val
}

func getArg(key string, parsedArgs docopt.Opts) string {
	val, err := parsedArgs.String(key)
	if err != nil {
		Die2("BUG", "no key %q in docopt result %+#v", key, parsedArgs)
	}
	return val
}

//
/*============================== Main function ===============================*/
//

func handleDupeBackup(prev, cur *steamfiles.AppInfo) bool {
	fmt.Fprintf(os.Stderr,
		" Found multiple backups for app %d (%q):\n"+
			"\tUsing    %q,\n\tIgnoring %q\n",
		cur.Number, cur.Name, cur.DirName, prev.DirName)
	return true // Use cur, discard prev
}

func check(steamAppsLibDir, steamBackupsDir string,
	reportUninstalled, verbose, debugging bool,
) {
	manifestInfoForAppNum := make(steamfiles.AppInfoForAppNum)
	err :=
		steamfiles.ScanSteamLibDir(steamAppsLibDir, manifestInfoForAppNum)
	DieIf(err, "")
	if verbose {
		reportCount(len(manifestInfoForAppNum), "valid appmanifest_$N.acf file")
	}
	if debugging { //D//?
		dumpMap("Info from manifests:", manifestInfoForAppNum)
	}

	backupInfoForAppNum := make(steamfiles.AppInfoForAppNum)
	err = steamfiles.ScanBackupsDir(
		steamBackupsDir, backupInfoForAppNum, handleDupeBackup)
	DieIf(err, "")
	if verbose {
		reportCount(len(backupInfoForAppNum), "Steam backup")
	}
	if debugging {
		dumpMap("Info from backups:", backupInfoForAppNum)
	}

	for mAppId, mInfo := range manifestInfoForAppNum {
		bInfo, ok := backupInfoForAppNum[mAppId]
		if !ok {
			recordProblem(noBackup, mInfo.Name, mAppId)
		} else {
			// ???TO-DO: compare mInfo.Name to bInfo.Name

			if mInfo.ModTime.After(bInfo.ModTime) {
				newer, err := steamfiles.AppNewerThan(
					steamAppsLibDir, mInfo.DirName, bInfo.ModTime)
				WarnIf(err, "")
				if newer {
					recordProblem(oldBackup, mInfo.Name, mAppId)
				}
			}
		}
	}

	if reportUninstalled {
		for bAppId, bInfo := range backupInfoForAppNum {
			if _, ok := manifestInfoForAppNum[bAppId]; !ok {
				recordProblem(notInstalled, bInfo.Name, bAppId)
			}
		}
	}

	reportProblems(verbose)
}

type problemKind byte
type problemInfo struct {
	kind      problemKind
	appName   string
	appNumber int32
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

func recordProblem(kind problemKind, appName string, appNumber int32) {
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
	reportCount(len(problems), "problem")

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

func reportCount(n int, noun string) {
	if n == 1 {
		fmt.Printf(" Found one %s\n", noun)
	} else {
		fmt.Printf(" Found %d %ss\n", n, noun)
	}
}

func dumpMap(what string, m steamfiles.AppInfoForAppNum) {
	fmt.Println(what)

	keys := make([]int, 0, len(m))
	for k, _ := range m {
		keys = append(keys, int(k))
	}
	fmt.Printf("  #D# sorting %d keys\n", len(keys))
	sort.Ints(keys)

	for _, k := range keys {
		kk := int32(k)
		v := m[kk]
		if v.Number != kk {
			Warn("key=%d but .Number=%d", k, v.Number)
		}
		fmt.Printf("%8d %q\n", v.Number, v.Name)
	}
}
