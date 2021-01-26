package main

/// ??? WHAT HAPPENS if multiple games backed up together ???
///		AppInfo.Number int32  →  AppInfo.Numbers []int32

import (
	"fmt"
	"math"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"time"

	"github.com/c12h/steam-stuff/sVDF"
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

// An AppInfo represents the relevant details of an installed app (taken from its
// appmanifest_<N>.acf file) or a Steam backup (taken from its sku.sis file).
//
// For installed apps, DirName is the "installdir" and the apps files are in
// …/steamapps/common/$DirName; for backups, DirName is the basename of the directory
// containing the backups.
//
type AppInfo struct {
	Number  int32     // The app identifier, a positive integer
	Name    string    // The name of the app
	ModTime time.Time // When the VDF file was last modified
	DirName string    // The name of a related directory
}

type AppInfoForAppNum map[int32]*AppInfo

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

func check(steamAppsLibDir, steamBackupsDir string,
	reportUninstalled, verbose, debugging bool,
) {
	manifestInfoForAppNum, err := scanAppsLibDir(steamAppsLibDir)
	DieIf(err, "")
	if verbose {
		reportCount(len(manifestInfoForAppNum), "valid appmanifest_$N.acf file")
	}
	if debugging {
		dumpMap("Info from manifests:", manifestInfoForAppNum)
	}

	backupInfoForAppNum, err := scanBackupsDir(steamBackupsDir)
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
				newer, err := appNewerThan(
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

//
/*==================== Scanning appmanifest_<N>.acf files ====================*/
//

var reManifestFile = regexp.MustCompile(`^appmanifest_(\d+)\.acf$`)

func scanAppsLibDir(path string) (AppInfoForAppNum, error) {
	dh, err := os.Open(path)
	if err != nil {
		return nil, cannot(err, "open", path)
	}

	allNames, err := dh.Readdirnames(-1)
	dh.Close()
	if err != nil {
		return nil, cannot(err, "read directory", path)
	}

	manifestsMap := make(AppInfoForAppNum, len(allNames))

	for _, n := range allNames {
		if match := reManifestFile.FindStringSubmatch(n); match != nil {
			appInfo, err :=
				parseManifest(filepath.Join(path, n))
			if err != nil {
				return nil, err
			}
			manifestsMap[appInfo.Number] = appInfo
			//
			if strconv.Itoa(int(appInfo.Number)) != match[1] {
				Warn("file %q has appid=%d?!", n, appInfo.Number)
			}
		}
	}
	return manifestsMap, nil

}

func parseManifest(mfPath string) (*AppInfo, error) {
	mfInfo, err := sVDF.FromFile(mfPath)
	if err != nil {
		return nil, err
	}
	if mfInfo.TopName != "AppState" {
		return nil, badFile(mfPath, mfInfo.TopName,
			`content has name %q, not "AppState"`, mfInfo.TopName)
	}

	idText, err := mfInfo.Lookup("appid")
	if err != nil {
		return nil, cannot(err, "get app ID from", mfPath)
	}
	appNum, err := parseAppId(idText, mfPath)
	if err != nil {
		return nil, err
	}

	appName, err := mfInfo.Lookup("name")
	if err != nil {
		return nil, cannot(err, "get app name from", mfPath)
	}

	installDir, err := mfInfo.Lookup("installdir")
	if err != nil {
		return nil, cannot(err, `get "installdir" from`, mfPath)
	}

	ret := &AppInfo{
		Name:    appName,
		Number:  appNum,
		ModTime: mfInfo.ModTime,
		DirName: installDir}
	return ret, nil
}

//
/*================= Scanning sku.sis files in Steam backups ==================*/
//

func scanBackupsDir(backupsDirPath string) (AppInfoForAppNum, error) {
	dh, err := os.Open(backupsDirPath)
	if err != nil {
		return nil, cannot(err, "open", backupsDirPath)
	}

	allNames, err := dh.Readdirnames(-1)
	dh.Close()
	if err != nil {
		return nil, cannot(err, "read directory", backupsDirPath)
	}

	backupsMap := make(AppInfoForAppNum, len(allNames))

	for _, n := range allNames {
		path := filepath.Join(backupsDirPath, n)
		nodeInfo, err := os.Lstat(path)
		if err != nil {
			return nil, cannot(err, "examine", path)
		}
		if !nodeInfo.IsDir() {
			continue
		}
		skuPath := filepath.Join(path, "sku.sis")
		nodeInfo, err = os.Lstat(skuPath)
		if err != nil {
			if ! os.IsNotExist(err) {
				return nil, cannot(err, "look for sku.sis in", skuPath)
			}
			skuPath = filepath.Join(path, "Disk_1", "sku.sis")
			nodeInfo, err = os.Lstat(skuPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
				return nil, cannot(err, "examine", skuPath)
			} 
		}

		skuInfo, err := sVDF.FromFile(skuPath)
		if err != nil {
			return nil, err
		}

		if skuInfo.TopName != "sku" && skuInfo.TopName != "SKU" {
			return nil, badFile(skuPath, skuInfo.TopName,
				`content has name %q, not "sku" or "SKU"`,
				skuInfo.TopName)
		}

		/// ??? WHAT HAPPENS if multiple games backed up together ???
		appName, err := skuInfo.Lookup("name")
		if err != nil {
			return nil, cannot(err, "get app name from", skuPath)
		}
		key := "apps"
		appNumText, err := skuInfo.Lookup(key, "0")
		if _, nameNotFound := err.(*sVDF.UnknownNameError); nameNotFound {
			key = "Apps"
			appNumText, err = skuInfo.Lookup(key, "0")
		}
		if err != nil {
			return nil, cannot(err, "get app number from", skuPath)
		}
		appNum, err := parseAppId(appNumText, skuPath)
		if err != nil {
			return nil, err
		}
		backupsMap[appNum] = &AppInfo{
			Name:    appName,
			Number:  appNum,
			ModTime: skuInfo.ModTime,
			DirName: n}
	}

	return backupsMap, nil
}

/*============================= Helper functions =============================*/

func parseAppId(text, path string) (int32, error) {
	appNum, err := strconv.Atoi(text)
	if err != nil {
		return 0, badFile(path, "", "has appid %q, need integer", text)
	}
	if appNum > math.MaxInt32 {
		Die2("BUG", "appid %d from file %q is too big for int32!", appNum, path)
	} else if appNum <= 0 {
		return 0, badFile(path, "", "has appid %d!?", appNum)
	}
	return int32(appNum), nil
}

func reportCount(n int, noun string) {
	if n == 1 {
		fmt.Printf(" Found one %s\n", noun)
	} else {
		fmt.Printf(" Found %d %ss\n", n, noun)
	}
}

func dumpMap(what string, m AppInfoForAppNum) {
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

//
/*================================== Errors ==================================*/
//

type Cannot struct {
	Verb, Noun string
	BaseErr    error
}

func cannot(baseErr error, verb, noun string) error {
	return &Cannot{
		Verb:    verb,
		Noun:    noun,
		BaseErr: baseErr}
}
func (e *Cannot) Error() string {
	ret := fmt.Sprintf("cannot %s %q", e.Verb, e.Noun)
	baseErr := e.BaseErr
	if baseErr != nil {
		if pe, isPathErr := baseErr.(*os.PathError); isPathErr {
			baseErr = pe.Unwrap()
		}
		ret += fmt.Sprintf(": %s", baseErr)
	}
	return ret
}
func (e *Cannot) Unwrap() error {
	return e.BaseErr
}

//

type BadFile struct {
	Path    string
	Problem string
	BadName string
}

func badFile(filepath, badName, format string, args ...interface{}) error {
	return &BadFile{
		Path:    filepath,
		Problem: fmt.Sprintf(format, args...),
		BadName: badName}
}
func (e *BadFile) Error() string {
	return fmt.Sprintf("%s in file %q", e.Problem, e.Path)
}
