// Package steamfiles deals with locally-installed Steam apps and their backups.
//
// It allows callers to find and examine installed Steam apps and backups of
// Steam apps in local storage.
//
//
// Steam Library Folders
//
// Steam can use multiple "Steam Library Folders".  (Many users prefer to keep
// their installed Steam games on separate partions, allowing them to reinstall
// or replace their OS without having to reinstall their games.  This also allows
// multiple OS instances on different partitions to share one set of games.)
//
// Each Steam Library Folder has a subdirectory named "steamapps", which holds
// (among other things) the manifests and files for the apps installed there.
// The initial SLF has contains lots of other files and directories; other SLFs
// normally contain only "steamapps".
//
// Steam keeps its list of SLFs in a text file at
//	<initial-SLF>/steamapps/libraryfolders.vdf
// which is  in the ‘simple Valve Data Format’ (all double-quoted
// strings) that this package’s sibling sVDF can parse.  FindSteamLibraryFolders()
// finds the initial SLF and parses this file to find any other SLFs.
//
//
// Installed Apps
//
// Each app installed in a Steam Library Folder has a text file at the path
//	<SLF>/steamapps/appmanifest_<AppNum>.acf
// where <AppNum> is the (decimal form of) Steams numeric identifier for that
// app (called "AppNum" here, called "appid" by Valve).  These manifests are in
// the 'simple Valve Data Format'.  They specify (among other things) each app’s
// proper name and "installdir", where
//	<SLF>/steamapps/common/<installdir>
// is the (root of the) directory tree where all the app’s files live.
//
//
// Steam Backups
//
// Steam backups are stored as a directory whose name reflects the apps in that
// backup.  (You can backup multiple apps together; a backup directory holding
// one app has the same name as that app.)  A backup generally has a disk
// structure like this:
//	<backups-directory>
//		The Age of Decadence
//			Disk_1
//				230072_depotcache_1.csd
//				230072_depotcache_1.csm
//				sku.sis
//			Disk_2
//				230072_depotcache_2.csd
//				230072_depotcache_2.csm
//				sku.sis
// where each "sku.sis" file is a text file in the ‘simple Valve Data Format’.
//
// If a backup has no Disk_2, Disk_3, ... subdirectories, it can also look like:
//	<backups-directory>
//		The Age of Decadence
//			230072_depotcache_1.csd
//			230072_depotcache_1.csm
//			sku.sis
// with the files stored in the backup directory itself, not in a subdirectory.
//
package steamfiles // import "github.com/c12h/steam-stuff/steamfiles"
