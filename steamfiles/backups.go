package steamfiles

import (
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/c12h/steam-stuff/sVDF"
)

// An AppBackup holds the relevant information about a Steam backup, taken from
// its sku.sis file.
//
// A Steam backup contains one or more apps (in my case, usually one). It
// consists of a directory ...???
//
type AppBackup struct {
	AppNumbers []AppNum  // Which apps are saved in this backup
	BackupName string    // The "name" field from the backup's sku.sis file
	BackupPath string    // The pathname of the backup directory
	ModTime    time.Time // When the sku.sis file was last modified
}

// ScanBackupsDir adds AppBackup values to a map indexed by AppNum.
//
// Since Steam backups can hold multiple apps, multiple map entries may point to
// the same AppBackup object.
//
type AppBackupForAppNum map[AppNum]*AppBackup

// A DupeBackupHandler is called if the same app is found in multiple backups,
// giving callers control over which backup is used for that app.
//
// Each time ScanBackupsDir finds an AppNum already in the map, it calls handleDupe.
// The return value tells ScanBackupsDir which backup to record and which to forget:
// true means use curr, false selects prev.
//
// For example, a caller can prefer backups containing just one app to those
// containing multiple apps with code like
//	if len(prev.AppNumbers) > 1 && len(curr.AppNumbers) == 1 {
//		return true
//	} else if len(prev.AppNumbers) == 1 && len(curr.AppNumbers) > 1 {
//		return false
//	}
//
type DupeBackupHandler func(appNum AppNum, prev, curr *AppBackup) bool

// ScanBackupsDir scans a directory for backups: that is, it looks for any
// immediate subdirectory D with a valid D/sku.sis or D/Disk_1/sku.sis file.
// It records any valid-seeming backups it finds in its map parameter.
//
// If it finds a backup containing an app which the map already has an AppBackup
// for, ScanBackupsDir calls handleDupe to let the caller perhaps log the
// details and say whether to use the previous or new AppBackup value. Passing
// nil for handleDupe causes ScanBackupsDir to always silently use the new
// value.
//
func ScanBackupsDir(
	backupsDirPath string,
	theMap map[AppNum]*AppBackup,
	handleDupe DupeBackupHandler,
) error {
	if handleDupe == nil {
		handleDupe = ignoreOlderDupe
	}
	dh, err := os.Open(backupsDirPath)
	if err != nil {
		return cannot("open", "", backupsDirPath, err)
	}

	allNames, err := dh.Readdirnames(-1)
	dh.Close()
	if err != nil {
		return cannot("read", "directory", backupsDirPath, err)
	}

	nFound := 0
	for _, n := range allNames {
		path := filepath.Join(backupsDirPath, n)
		nodeInfo, err := os.Lstat(path)
		if err != nil {
			return cannot("examine", "", path, err)
		}
		if !nodeInfo.IsDir() {
			continue
		}
		skuPath := filepath.Join(path, "sku.sis")
		nodeInfo, err = os.Lstat(skuPath)
		if err != nil && os.IsNotExist(err) {
			skuPath = filepath.Join(path, "Disk_1", "sku.sis")
			nodeInfo, err = os.Lstat(skuPath)
			if err != nil {
				if os.IsNotExist(err) {
					continue
				}
			}
		}
		if err != nil {
			return cannot("find sku.sis file for", "backup", path,
				os.ErrNotExist)
		}

		skuInfo, err := sVDF.FromFile(skuPath, "sku", "SKU")
		if err != nil {
			return err
		}

		backupName, err := skuInfo.Lookup("name")
		if err != nil {
			return cannot("get app name from", "", skuPath, err)
		}

		appNumbersList := make([]AppNum, 0, 1)
		appListKey := "apps"
		if !skuInfo.HaveString(appListKey, "0") {
			appListKey = "Apps"
			if !skuInfo.HaveString(appListKey, "0") {
				return cannot(`find apps (or Apps) in`, "", skuPath, nil)
			}
		}
		for i := 0; ; i += 1 {
			indexKey := strconv.Itoa(i)
			if !skuInfo.HaveString(appListKey, indexKey) {
				break
			}
			appNumText, err := skuInfo.Lookup(appListKey, indexKey)
			if err != nil {
				panic(err.Error())
			}
			appNum, err := parseAppNum(appNumText, skuPath)
			if err != nil {
				return err
			}
			appNumbersList = append(appNumbersList, appNum)
		}
		if len(appNumbersList) == 0 {
			return cannot("get any app numbers from", "", skuPath, nil)
		}

		nFound += 1
		newBackup := &AppBackup{
			AppNumbers: appNumbersList,
			BackupName: backupName,
			BackupPath: path,
			ModTime:    skuInfo.ModTime}

		for _, appNum := range appNumbersList {
			if prevBackup, havePrev := theMap[appNum]; havePrev {
				if !handleDupe(appNum, prevBackup, newBackup) {
					continue // Leave prevBackup in place
				}
			}
			theMap[appNum] = newBackup
		}
	}

	if nFound == 0 {
		return cannot("find any Steam backups in", "directory",
			backupsDirPath, nil)
	}
	return nil
}

// ignoreOlderDupe is a do-nothing default DupeBackupHandler.
func ignoreOlderDupe(appNum AppNum, prev, curr *AppBackup) bool { return true }
