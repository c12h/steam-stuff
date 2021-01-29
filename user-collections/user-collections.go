package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/c12h/errs"
	"github.com/c12h/steam-stuff/sVDF"
	"github.com/c12h/steam-stuff/steamfiles"
	"github.com/docopt/docopt-go"
)

/*=================================== CLI ====================================*/

const VERSION = "0.1"

const USAGE = `Usage:
 steam-collections [-n|--count | -c|--csv | -t|--tsv | -T|--html-table] [<steam-home-dir>]
 steam-collections (-h | --help  |  -v | --version)

Output ...???
Reports ... Steam home directory
 ... contains subdirectories "steamapps" and "userdata"
 ... uses Steam home for current user ...

Options:
  -n, --count       Report only the number of apps in each collection
  -j, --json        Output JSON ({"username":[[appid, appname, collection...], ...], ...})
  -t, --tsv         Output Tab-Separated-Values text
  -c, --csv         Output Comma-Separated-Values text
  -T, --html-table  Output HTML defining a <table>
`

// Output modes:
const (
	modeJSON = iota
	modeCount
	modeTSV
	modeCSV
	modeHTML
)

func main() {
	parsedArgs, err :=
		docopt.ParseArgs(USAGE, os.Args[1:], VERSION)
	DieIf2(err, "BUG", "docopt failed: %s", err)

	mode := modeJSON
	if optSpecified("-n", parsedArgs) {
		mode = modeCount
	} else if optSpecified("-t", parsedArgs) {
		mode = modeTSV
	} else if optSpecified("-c", parsedArgs) {
		mode = modeCSV
	} else if optSpecified("-T", parsedArgs) {
		mode = modeHTML
	}

	SteamHomeDir, isFromArg := getArgMaybe("<steam-home-dir>", parsedArgs), true
	if SteamHomeDir == "" {
		SteamHomeDir, err := steamfiles.FindSteamHomeDir()
		DieIf(err, "")
		isFromArg = false
	}

	recordCollections(mode, SteamHomeDir, isFromArg)
}

func optSpecified(key string, parsedArgs docopt.Opts) bool {
	val, err := parsedArgs.Bool(key)
	if err != nil {
		Die2("BUG", "no key %q in docopt result %+#v", key, parsedArgs)
	}
	return val
}

func getArgMaybe(key string, parsedArgs docopt.Opts) string {
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

/*========================= Processing app manifests =========================*/

func recordCollections(mode int, SteamHomeDir string, isFromArg bool) *CollectionsInfo {
	userdataDir, err := steamfiles.DirectoryExists(SteamHomeDir, "userdata")
	if err != nil && isFromArg {
		Die("are you sure about %q?: %s", SteamHomeDir, err)
	}
	DieIf(err, "")

	dh, err := os.Open(userdataDir)
	DieIf(err, "cannot open directory %q: %s", userdataDir, err)

	names, err := dh.Readdirnames(-1)
	dh.Close()
	DieIf(err, "cannot read directory %q: %s", userdataDir, err)

	collections := CollectionsInfo{} //???

	nUsersFound := 0
	for _, name := range names {
		if reDigits.MatchString(name) {
			nUsersFound += 1
			userDir := filepath.Join(userdataDir, name)
			userName, err := processUserConfigFile(userDir, name)
			DieIf(err, "")
			recordUserCollection(userDir, userName, mode, &collections)
		}
	}
	if nUsersFound == 0 {
		Die("no user directories found in %q", userdataDir)
	}

	return collections
}

func processUserConfigFile(userDir, userNumberText string) (string, err) {
	userConfigDir, err := DirectoryExists(userDir, "config")
	if err != nil {
		return "", fmt.Errorf(`user %s has no "config" directory: %s`,
			userNumberText, err)
	}
	userConfigPath := filepath.Join(userConfigDir, "localconfig.vdf")
	userConfigInfo, err := sVDF.FromFile(userConfigPath, "UserLocalConfigStore")
	if err != nil {
		return "", cannot("use", "", userConfigPath, err)
	}
	userName, err := userConfigInfo.Lookup("friends", "PersonaName")
	if err != nil {
		return "", cannot("get user name from", "", userConfigPath, err)
	}
	return userName, nil
}

/*============================ Utility Functions =============================*/

func warnCannot(verb, adjective, noun string, err error) {
	return errs.Cannot(verb, adjective, noun, true, "", err)
}
