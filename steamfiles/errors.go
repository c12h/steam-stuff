package steamfiles

import (
	"fmt"

	"github.com/c12h/errs" //???TO-DO: better name coming one day ...
)

// cannot is a convenience wrapper for errs.Cannot
//
func cannot(verb, adjective, noun string, baseError error) error {
	return errs.Cannot(verb, adjective, noun, true, "", baseError)
}

// A FileError represents a defect relating to a local file.   ...???
type FileError struct {
	Path    string
	Problem string
	BadName string
}

// (Pointers to) FileError objects satisfy the error interface.
func (e *FileError) Error() string {
	return fmt.Sprintf("%s in file %q", e.Problem, e.Path)
}

// fileError returns a (pointer to a) FileError.
func fileError(filepath, badName, format string, args ...interface{}) error {
	return &FileError{
		Path:    filepath,
		Problem: fmt.Sprintf(format, args...),
		BadName: badName}
}
