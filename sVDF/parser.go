package sVDF

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"unicode/utf8"
)

const expectTabs = 0
const expectNewline = 1

func parseSimpleVDF(fr *os.File, fileInfo *File) error {
	// fileInfo has: .Path, .ModTime, .Size
	// fileInfo needs: .TopName (a string), .TopValue (a string or NamesValuesList)
	data, err := ioutil.ReadAll(fr)
	if err != nil {
		return cannot(err, "read", fileInfo.Path)
	}
	p := &parser{filespec: fileInfo.Path, buf: data}
	fileInfo.TopName, err = parseString(p, expectTabs)
	if err != nil {
		return err
	}
	fileInfo.TopValue, err = parseValue(p)
	if err != nil {
		return err
	}
	return nil
}

type parser struct {
	filespec    string
	buf         []byte
	pos         int
	nIndentTabs int
	nWarnings   int
}

// Parse a double-quoted string, which may be a name (=key) or a value.
//
// Expects p.pos to be the index of the first '"".
//
func parseString(p *parser, expectation int) (string, error) {
	pos := p.pos
	if p.buf[pos] != '"' {
		return "", parseError(p, `expected '"', got`)
	}
	b := make([]byte, 0, len(p.buf)-pos)
	for pos++; pos < len(p.buf) && p.buf[pos] != '"'; pos++ {
		ch := p.buf[pos]
		if ch == '\\' {
			pos++
			if pos >= len(p.buf) {
				p.pos = pos
				return "", parseError(p, `\ just before EOF`)
			}
			ch = p.buf[pos]
			// Taken from source-sdk-2013-master/sp/src/tier1/utlbuffer.cpp:
			switch ch {
			case 'a':
				ch = '\a'
			case 'b':
				ch = '\b'
			case 'f':
				ch = '\f'
			case 'n':
				ch = '\n'
			case 'r':
				ch = '\r'
			case 't':
				ch = '\t'
			case 'v':
				ch = '\v'
			case '"':
				ch = '"'
			case '?':
				ch = '?'
			case '\\':
				ch = '\\'
			case '\'':
				ch = '\''
			default:
				seq := p.buf[pos-1 : pos]
				p.pos = pos
				return "", parseError(p, `bad escape sequence %q`, seq)
			}
		}
		b = append(b, ch)
	}
	if pos >= len(p.buf) {
		//???
		return "", parseError(p, `Unterminated string`)
	}
	//D// fmt.Printf("#D# parseString got %q, skipping ...\n", b) //D//
	p.pos = pos
	err := skipWhitespace(p, expectation)
	if err != nil {
		return "", err
	}

	return string(b[:]), nil
}

// Parse a value, which may be a double-quoted string
// or a '{' followed by a name-value list and a '}'.
//
// Expects p.pos to be the index of the '"' or '{'.
//
func parseValue(p *parser) (Value, error) {
	pos := p.pos
	//D// fmt.Printf("#D# parseValue @ offset %d in %q\n", pos, p.filespec) //D//
	ch := p.buf[pos]
	// Getting a string value is easy here.
	if ch == '"' {
		return parseString(p, expectNewline)
	} else if ch != '{' {
		return nil, parseError(p, `Expected '"' or '{', got`)
	}

	// OK, so we got a '{' and now need to parse a name-value-list.
	//D// fmt.Printf("#D# parsing braced NVL ('{'@%d) ...\n", pos)
	p.nIndentTabs += 1
	p.pos = pos
	err := skipWhitespace(p, expectNewline)
	if err != nil {
		return nil, err
	}
	nvl := make(NamesValuesList, 0)
	for p.pos < len(p.buf) {
		switch p.buf[p.pos] {
		case '"':
			name, err := parseString(p, expectTabs)
			if err != nil {
				return nil, parseError(p,
					`Expected double-quoted name, got`)
			}
			value, err := parseValue(p)
			if err != nil {
				return nil, err
			}
			nvl[name] = value
		case '}':
			p.nIndentTabs -= 1
			err = skipWhitespace(p, expectNewline)
			if err != nil {
				return nil, err
			}
			return nvl, nil
		default:
			return nil, parseError(p, `expected '}', '"' or '{', got`)
		}
	}
	err = skipWhitespace(p, expectNewline)
	if err != nil {
		return nil, err
	}
	return nil, parseError(p, `unexpected EOF in NVL`)
}

// Skip over, but check, whitespace characters, starting at p.pos+1.
//
func skipWhitespace(p *parser, expectation int) error {
	atBOL := false
	what := "value"
	if expectation == expectTabs {
		what = "name not followed by tabs"
	}

	pos := p.pos + 1
	if pos >= len(p.buf) {
		return nil
	}
	ch := p.buf[pos]
	//D// fmt.Printf("  #D# skipWhitespace() found %q at %d\n", ch, pos)

	if ch == '\t' {
		if expectation != expectTabs {
			warnOddWS(p, pos, `expected newline after value`)
		} else {
			for ch == '\t' {
				pos += 1
				if pos >= len(p.buf) {
					warnOddWS(p, pos, `EOF after tab`)
					return nil
				}
				ch = p.buf[pos]
			}
			if ch != '"' {
				warnOddWS(p, pos, `expected '"' after name and tabs`)
			}
		}
	} else if ch == '\r' && pos+1 < len(p.buf) && p.buf[pos+1] == '\n' {
		pos += 2
		atBOL = true
		//D// fmt.Printf("  #D# skipping CR/LF ...\n")
	} else if ch == '\n' {
		pos += 1
		atBOL = true
		//D// fmt.Printf("  #D# skipping LF ...\n")
	} else {
		warnOddWS(p, pos, `expected newline after `+what)
	}
	if atBOL {
		nTabs := 0
		for ; pos < len(p.buf) && p.buf[pos] == '\t'; pos += 1 {
			nTabs += 1
		}
		//D// fmt.Printf("  #D# found new line with %d-tab indent, pos=%d\n",
		//D//	nTabs, pos)
		if pos >= len(p.buf) {
			if nTabs > 0 {
				warnOddWS(p, pos, `EOF after tab`)
			}
			return nil
		}
		ch = p.buf[pos]
		expNumTabs := p.nIndentTabs
		if ch == '}' {
			expNumTabs -= 1
		}
		if nTabs != expNumTabs {
			warnOddWS(p, pos, `expected %s, found %s`,
				plural(expNumTabs, "tab"),
				plural(nTabs, "tab"))
		}
	}
skippingExtra:
	for ; pos < len(p.buf); pos += 1 {
		ch = p.buf[pos]
		switch ch {
		case ' ', '\t', '\r', '\n':
			continue
		default:
			break skippingExtra
		}
	}
	p.pos = pos
	//D// fmt.Printf("  #D# skipped to pos=%d\n", pos)
	return nil
}

type ParseError struct {
	FilePath   string // Which file
	FileOffset int    // Position at which error was detected (zero-origin)
	LineNumber int    // Which line error is in (one-origin)
	RuneNumber int    // Which rune error is at in that line (one-origin)
	NextRune   rune   // The next rune after where error was detected
	Diagnostic string // A description of the problem
}

func (e *ParseError) Error() string {
	return fmt.Sprintf("%s:%d:%d: %s",
		e.FilePath, e.LineNumber, e.RuneNumber, e.Diagnostic)
}
func parseError(p *parser, format string, args ...interface{}) error {
	pos := p.pos
	lineNum := bytes.Count(p.buf[:pos], []byte{'\n'}) + 1
	lastBOL := bytes.LastIndex(p.buf[:pos], []byte{'\n'}) + 1
	runeNum := utf8.RuneCount(p.buf[lastBOL:pos]) + 1
	nextRune, _ := utf8.DecodeRune(p.buf[pos:])

	diagnostic := ""
	if len(args) == 0 && strings.HasSuffix(format, " got") {
		diagnostic = fmt.Sprintf(format+" %#v", nextRune)
	} else {
		diagnostic = fmt.Sprintf(format, args...)
	}
	return &ParseError{
		FilePath:   p.filespec,
		FileOffset: pos,
		NextRune:   nextRune,
		LineNumber: lineNum,
		RuneNumber: runeNum,
		Diagnostic: diagnostic}
}

func warnOddWS(p *parser, pos int, format string, args ...interface{}) {
	lineNum := bytes.Count(p.buf[:pos], []byte{'\n'}) + 1
	nextRune, _ := utf8.DecodeRune(p.buf[pos:])

	diagnostic := ""
	if len(args) == 0 {
		diagnostic = fmt.Sprintf(format+", got %#v", nextRune)
	} else {
		diagnostic = fmt.Sprintf(format, args...)
	}
	fmt.Fprintf(os.Stderr, " Odd whitespace in %q at offset %d (line %d): %s\n",
		p.filespec, pos, lineNum, diagnostic)
}
func plural(count int, noun string) string {
	if count == 1 {
		return "one " + noun
	}
	return fmt.Sprintf("%d %ss", count, noun)
}
