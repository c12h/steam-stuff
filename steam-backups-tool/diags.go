package main

import (
	"fmt"
	"os"
	"path/filepath"
)

var progName = filepath.Base(os.Args[0])

var nWarnings = 0

func Warn(format string, fmtArgs ...interface{}) {
	Warn2("", format, fmtArgs...)
}

func Warn2(tag, format string, fmtArgs ...interface{}) {
	nWarnings++
	WriteMessage(tag, format, fmtArgs...)
}

func WarnIf(skipIfNil interface{}, format string, fmtArgs ...interface{}) {
	WarnIf2(skipIfNil, "", format, fmtArgs...)
}

func WarnIf2(skipIfNil interface{}, tag, format string, fmtArgs ...interface{}) {
	if skipIfNil != nil {
		if format == "" {
			Warn2("", "%s", skipIfNil)
		} else {
			Warn2("", format, fmtArgs...)
		}
	}
}

func Die(format string, fmtArgs ...interface{}) {
	Die2("", format, fmtArgs...)
}

func Die2(tag, format string, fmtArgs ...interface{}) {
	if format != "" {
		WriteMessage(tag, format, fmtArgs...)
	}
	//
	dieStatus := 2
	if nWarnings > 0 {
		dieStatus |= 1
	}
	os.Exit(dieStatus)
}

func DieIf(skipIfNil interface{}, format string, fmtArgs ...interface{}) {
	if skipIfNil == nil {
		return
	} else if format == "" {
		Die2("", "%s", skipIfNil)
	} else {
		Die2("", format, fmtArgs...)
	}
}

func DieIf2(skipIfNil interface{}, tag, format string, fmtArgs ...interface{}) {
	if skipIfNil == nil {
		return
	} else if format == "" {
		Die2(tag, "%s", skipIfNil)
	} else {
		Die2(tag, format, fmtArgs...)
	}
}

func WriteMessage(tag, format string, args ...interface{}) {
	text := progName
	if tag != "" {
		text += " " + tag
	}
	text += fmt.Sprintf(": "+format, args...)
	if l := len(text); text[l-1] == '\n' {
		text = text[:l-1]
	}
	fmt.Fprintln(os.Stderr, text)
}
