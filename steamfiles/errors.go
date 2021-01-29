package steamfiles

import (
	"fmt"

	"github.com/c12h/errs" //???TO-DO: better name coming one day ...
)

/*------------- cannot is a convenience wrapper for errs.Cannot --------------*/

func cannot(verb, adjective, noun string, baseError error) error {
	return errs.Cannot(verb, adjective, noun, true, "", baseError)
}

/*-------------------------------- FileError ---------------------------------*/

// A FileError (or at least a pointer to one) represents a defect relating to a
// local file.
//
type FileError struct {
	Path    string // The file that we have a problem with
	Problem string // Our complaint
	BadName string // included in error data but not in .Error() strings
}

// (Pointers to) FileError objects satisfy the error interface.
//
func (e *FileError) Error() string {
	return fmt.Sprintf("%s in file %q", e.Problem, e.Path)
}

// fileError is a convenience function to return a (pointer to a) FileError.
//
func fileError(filepath, badName, format string, args ...interface{}) error {
	return &FileError{
		Path:    filepath,
		Problem: fmt.Sprintf(format, args...),
		BadName: badName}
}

/*------------------------------ NotFoundError -------------------------------*/

// A NotFoundError represents a missing file or directory or part of a Steam
// instance.
//
type NotFoundError struct{
	What string
	BaseErr error
}

func (e *NotFoundError) Error() string {
	text := fmt.Sprintf("cannot find %", e.What)
	if e.BaseErr != nil {
		text = fmt.Sprintf("%s: %s", text, e.BaseErr)
	}
	return text
}
func (e *NotFoundError) Unwrap() error {
	return e.BaseErr
}

func cannotFind(what string, err error) *NotFoundError {
	return &NotFoundError{
		What: what,
		BaseErr: err}
}
