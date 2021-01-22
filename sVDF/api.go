// Package sVDF records details of simple, string-only Valve Data Format files.
package sVDF

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"
)

/*======================== Types for Names and Values ========================*/

// A Value is a named datum from a VDF file.
//    Possible actual types:
//	- a vdf.NamesValuesList		(all formats)
//	- a string			(all formats)
//	- nil		???		(all formats???)
//	- an integer			(not in StringyText format)
//	- a uint64			(not in StringyText format)
//	- a float			(not in StringyText format)
//	- a color			(not in StringyText format)
//	- a pointer			(not in StringyText format)
type Value interface{}

// A NamesValuesList represents a set of [sub]keys and their values.
type NamesValuesList map[string]Value

// nvl.Names() returns the [sub]keys from a NamesValuesList, sorted into Unicodal order.
//
// ???TO-DO: sort case-independently, at least for ASCII chars?
func (nvl *NamesValuesList) Names() []string {
	ret := make([]string, 0, len(*nvl))
	for n, _ := range *nvl {
		ret = append(ret, n)
	}
	sort.Strings(ret)
	return ret
}

// nvl.Get(n) returns the value, if any, for a key in a NVL. //Useful???
func (nvl *NamesValuesList) Get(n string) (Value, bool) {
	val, ok := (*nvl)[n]
	return val, ok
}

/*==================== Types and Functions for VDF Files =====================*/

// A File represents a VDF file that has been parsed successfully.
//
// (Note that this package can only parse a subset of textual VDF files, and
// cannot parse any binary VDF files.)
//
type File struct {
	Path    string    // The (or at least a) absolute path of the file
	ModTime time.Time // When the file was last modified
	Size    int64     // The current size of the file in bytes
	//Format Format
	TopName  string
	TopValue Value
}

// FromFile() opens, reads and parses a ‘simple VDF’ file, returning a (pointer
// to a)sVDF.File or an error.
//
func FromFile(filespec string) (*File, error) {
	fh, err := os.Open(filespec)
	if err != nil {
		return nil, cannot(err, "open", filespec)
	}
	defer fh.Close()
	fileInfo, err := fh.Stat()
	if err != nil {
		return nil, cannot(err, "examine", filespec)
	}
	ret := &File{
		Path:    filespec,
		ModTime: fileInfo.ModTime(),
		Size:    fileInfo.Size()}
	err = parseSimpleVDF(fh, ret)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// Lookup() returns the string value, if any, from nested name-value lists in a
// parsed VDF file.
//
// (That is, it takes the name of an entry in the top-level NVL, then the name
// of an entry in that NVL, and so on.  Hence all the names except the last
// should correspond to nested NVLs, and the last should correspond to a string
// value.)
//
func (f *File) Lookup(names ...string) (string, error) {
	v := f.TopValue
	iLastName := len(names) - 1
	for i := 0; i < iLastName; i++ {
		switch vv := v.(type) {
		case string:
			return "", &IsStringError{
				FilePath: f.Path,
				NamePath: names[:i],
				String:   vv}
		case NamesValuesList:
			sublist, ok := vv[names[i]]
			if !ok {
				return "", &UnknownNameError{
					FilePath: f.Path,
					NamePath: names[:i]}
			}
			v = sublist
		default:
			panic(fmt.Sprintf("%s = %+#v",
				filePaths(f.Path, names[:i], true), v))
		}
	}
	switch vv := v.(type) {
	case string:
		return vv, nil
	case NamesValuesList:
		return "", &NotStringError{
			FilePath: f.Path,
			NamePath: names,
			NVL:      vv}
	default:
		panic(fmt.Sprintf("%s = %+#v", filePaths(f.Path, names, true), v))
	}
}

/*================================== Errors ==================================*/

var verboseErrorStrings = false

// ReportErrorsVerbosely() controls whether e.Error() reports the full path of
// the problematic VDF file or just its basename, for any (pointer to an)
// IsStringError, NotStringError or UnknowNameError.
//
// It also controls whether NotStringError.Error() lists the subkeys of the
// NamesValuesList found where a string was expected.
//
func ReportErrorsVerbosely(setting bool) {
	verboseErrorStrings = setting
}

func filePaths(filespec string, names []string, fullFilePath bool) string {
	if !fullFilePath {
		filespec = filepath.Base(filespec)
	}
	namesPath := ""
	for _, n := range names {
		namesPath += fmt.Sprintf("→%q", n)
	}
	return fmt.Sprintf("file %q %s", filespec, namesPath)
}

type IsStringError struct {
	FilePath string
	NamePath []string
	String   string
}
type NotStringError struct {
	FilePath string
	NamePath []string
	NVL      NamesValuesList
}
type UnknownNameError struct {
	FilePath string
	NamePath []string
}

func (e *IsStringError) Error() string {
	lastName, prevNames := splitNamePath(e.NamePath)
	return fmt.Sprintf("key %q has value %q, not a NVL, in %s",
		lastName, e.String, filePaths(e.FilePath, prevNames, verboseErrorStrings))
}
func (e *NotStringError) Error() string {
	lastName, prevNames := splitNamePath(e.NamePath)
	t := fmt.Sprintf("key %q is NVL, not string, in %s",
		lastName, filePaths(e.FilePath, prevNames, verboseErrorStrings))
	if verboseErrorStrings {
		p := "\n\t(subkeys are"
		for n, _ := range e.NVL {
			t += p + fmt.Sprintf(" %q", n)
			p = ","
		}
	}
	return t
}
func (e *UnknownNameError) Error() string {
	lastName, prevNames := splitNamePath(e.NamePath)
	return fmt.Sprintf("unknown name %q in %s",
		lastName, filePaths(e.FilePath, prevNames, verboseErrorStrings))
}
func splitNamePath(names []string) (string, []string) {
	last := len(names) - 1
	return names[last], names[:last]
}

type CannotError struct {
	Verb      string
	Noun      string
	QuoteNoun bool
	BaseErr   error
}

func cannot(baseErr error, verb, filespec string) error {
	return &CannotError{
		Verb:      verb,
		Noun:      filespec,
		QuoteNoun: true,
		BaseErr:   baseErr}
}
func (e *CannotError) Error() string {
	noun := e.Noun
	if e.QuoteNoun {
		noun = fmt.Sprintf("%q", noun)
	}
	return fmt.Sprintf("cannot %s %s: %s", e.Verb, noun, e.BaseErr)
}
func (e *CannotError) Unwrap() error {
	return e.BaseErr
}

//========================= Doodling for a more general API
//
// type File interface {
// 	Path() string
// 	ModTime() time.Time
// 	Size() int
// 	Format() Format
// 	TopName() string
// 	TopValue() NamesValuesList
// }
//
// type Format byte
//
// const (
// 	_ = Format(iota)
// 	StringyText
// 	// GeneralText
// 	// Binary
// )
