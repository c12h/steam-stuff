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
				NamePath: names[:i],
				String:   vv}
		case NamesValuesList:
			valForName, ok := vv[names[i]]
			if !ok {
				return "", &UnknownNameError{
					NamePath: names[:i]}
			}
			v = valForName
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
			NamePath: names,
			NVL:      vv}
	default:
		panic(fmt.Sprintf("%s = %+#v", filePaths(f.Path, names, true), v))
	}
}

/*================================== Errors ==================================*/

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
	NamePath []string
	String   string
}
type NotStringError struct {
	NamePath []string
	NVL      NamesValuesList
}
type UnknownNameError struct {
	NamePath []string
}

func (e *IsStringError) Error() string {
	return fmt.Sprintf("key %s has value %q, not a NVL",
		namesPath(e.NamePath), e.String)
}
func (e *NotStringError) Error() string {
	text := "{}"
	if len(e.NVL) > 0 {
		for k, v := range e.NVL {
			text = fmt.Sprintf("{%q %q", k, v)
			break
		}
		if len(e.NVL) > 1 {
			text += " ..."
		}
		text += "}"
	}
	return fmt.Sprintf("key %s has NVL %s, not a string",
		namesPath(e.NamePath), text)
}
func (e *UnknownNameError) Error() string {
	last := len(e.NamePath) - 1
	return fmt.Sprintf("unknown name %q at %s",
		e.NamePath[last], namesPath(e.NamePath[:last]))
}
func namesPath(names []string) string {
	text := ""
	for _, n := range names {
		text += fmt.Sprintf("→%q", n)
	}
	return text[1:]
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
