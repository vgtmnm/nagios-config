/*
   Copyright 2017 Odd Eivind Ebbesen

   Licensed under the Apache License, Version 2.0 (the "License");
   you may not use this file except in compliance with the License.
   You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

   Unless required by applicable law or agreed to in writing, software
   distributed under the License is distributed on an "AS IS" BASIS,
   WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
   See the License for the specific language governing permissions and
   limitations under the License.
*/

package nagioscfg

/*
IO-related stuff for nagioscfg
Much of the stuff here is taken from Golangs encoding/json source and modified to the specific needs of this package.
See: https://golang.org/LICENSE
*/

import (
	"bufio"
	"bytes"
	"container/list"
	"errors"
	"fmt"
	log "github.com/Sirupsen/logrus"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"unicode"
)

// A ParseError is returned for parsing errors.
// The first line is 1.  The first column is 0.
type ParseError struct {
	Line   int   // Line where the error occurred
	Column int   // Column (rune index) where the error occurred
	Err    error // The actual error
}

// Error returns the error as a nicely formatted string
func (e *ParseError) Error() string {
	return fmt.Sprintf("line %d, column %d: %s", e.Line, e.Column, e.Err)
}

// These are the errors that can be returned in ParseError.Error
var (
	ErrNoValue = errors.New("only key given where key/value expected")
	ErrUnknown = errors.New("unknown parsing error")
)

type Reader struct {
	Comment   rune
	line      int
	inputline int // separate counter that should match the line number from input
	column    int
	field     bytes.Buffer
	r         *bufio.Reader
}

type FileReader struct {
	*Reader
	f *os.File
}

type MultiFileReader []*FileReader

func _debug(args ...interface{}) {
	fmt.Println(args)
}

func NewReader(rr io.Reader) *Reader {
	return &Reader{
		Comment: '#',
		r:       bufio.NewReader(rr),
	}
}

func NewFileReader(path string) *FileReader {
	file, err := os.Open(path)
	if err != nil {
		log.Errorf("%q %s", err, dbgStr(true))
		return nil
	}
	fr := &FileReader{}
	fr.Reader = NewReader(file)
	fr.f = file
	return fr
}

func NewMultiFileReader(paths ...string) MultiFileReader {
	mfr := make(MultiFileReader, 0, len(paths))
	for i := range paths {
		fr := NewFileReader(paths[i])
		if fr != nil {
			mfr = append(mfr, fr)
		}
	}
	return mfr
}

func (fr *FileReader) Close() error {
	return fr.f.Close()
}

func (fr *FileReader) AbsPath() (string, error) {
	return filepath.Abs(fr.f.Name())
}

func (fr *FileReader) String() string {
	fpath, err := fr.AbsPath()
	if err != nil {
		fpath = fr.f.Name()
	}
	return fmt.Sprintf("%s.FileReader: %q", PKGNAME, fpath)
}

func (mfr MultiFileReader) Close() error {
	errcnt := 0
	for i := range mfr {
		err := mfr[i].Close()
		if err != nil {
			log.Errorf("%q %s", err, dbgStr(true))
			errcnt++
		}
	}
	if errcnt > 0 {
		return fmt.Errorf("Error closing %d files %s", errcnt, dbgStr(true))
	}
	return nil
}

func (r *Reader) error(err error) error {
	return &ParseError{
		Line:   r.line,
		Column: r.column,
		Err:    err,
	}
}

// this is basically "dos2unix"
func (r *Reader) readRune() (rune, error) {
	r1, _, err := r.r.ReadRune()
	if r1 == '\r' {
		r1, _, err = r.r.ReadRune()
		if err == nil {
			if r1 != '\n' {
				r.r.UnreadRune()
				r1 = '\r'
			}
		}
	} else if r1 == '\n' {
		r.inputline++ // had to add this to find the non-breaking space bug from Nagios, 2017-07-24 18:49:16
	}
	r.column++
	return r1, err
}

// skip advances the reader until it reaches delim, ignoring everything it reads
func (r *Reader) skip(delim rune) error {
	for {
		r1, err := r.readRune()
		if err != nil {
			return err
		}
		if r1 == delim {
			return nil
		}
	}
}

func (r *Reader) parseFields() (haveField bool, delim rune, err error) {
	r.field.Reset() // clear buffer at each call

	r1, err := r.readRune()
	for err == nil && r1 != '\n' && unicode.IsSpace(r1) {
		r1, err = r.readRune()
	}
	if err == io.EOF && r.column != 0 {
		return true, 0, err
	}
	if err != nil {
		return false, 0, err
	}

	switch r1 {
	case '\n':
		//fallthrough
		return false, r1, nil
	case '\t':
		//fallthrough
		return false, r1, nil
	case ' ':
		//fallthrough
		return false, r1, nil
	case '{': // I don't get why this case is never triggered... - Yes, it's because it's at the beginning of a line...
		log.Debugf("Hit %q, line #%d col #%d %s", r1, r.line, r.column, dbgStr(false))
		return false, r1, nil
	case '}':
		if r.column > DEF_ALIGN {
			log.Debugf("Hit %q, line #%d col #%d %s", r1, r.line, r.column, dbgStr(false))
		}
		return true, r1, nil
	default:
		for {
			if !unicode.IsSpace(r1) {
				r.field.WriteRune(r1)
			}
			r1, err = r.readRune()
			if err != nil {
				log.Debug(err)
				break
			}
			if r1 == '{' {
				// this ugly little hack lets us consume {} that are part of some value. Fragile as fuck, will sure bite ass
				if r.column > DEF_ALIGN {
					log.Debugf("Hit %q at line #%d col #%d %s", r1, r.line, r.column, dbgStr(false))
					continue
				}
				break
			}
			if unicode.IsSpace(r1) {
				//log.Debugf("%s.Reader.parseFields(): Hit %q at line #%d col #%d", PKGNAME, r1, r.line, r.column)
				break
			}
			//if err != nil || r1 == '{' || r1 == '}' || unicode.IsSpace(r1) {
			//if err != nil || r1 == '{' || unicode.IsSpace(r1) {
			//	//log.Debugf("%s.Reader.parseFields(): Hit %q at line #%d col #%d", PKGNAME, r1, r.line, r.column)
			//	break
			//}
			//if r1 == '\n' {
			//	_debug("End of line, returning")
			//	return true, r1, nil
			//}
		}
	}

	if err != nil {
		if err == io.EOF {
			return true, 0, err
		}
		return false, 0, err
	}

	return true, r1, nil
}

func (r *Reader) parseLine() (fields []string, state IoState, err error) {
	r.line++
	r.column = -1

	r1, _, err := r.r.ReadRune()
	if err != nil {
		return nil, IO_OBJ_OUT, err
	}
	if r.Comment != 0 && r1 == r.Comment {
		return nil, IO_OBJ_OUT, r.skip('\n')
	}
	r.r.UnreadRune()

	for {
		haveField, delim, err := r.parseFields()
		if haveField {
			if fields == nil {
				fields = make([]string, 0, 6) // 6 is a random guess at what is suitable
			}
			fields = append(fields, r.field.String())
		}
		// 2017-01-30 21:07:19
		// we have some bugs with {} being part of command parameters
		if delim == '{' {
			return fields, IO_OBJ_BEGIN, err
		} else if delim == '}' {
			return fields, IO_OBJ_END, err
		} else if delim == '\n' {
			return fields, IO_OBJ_IN, err
		} else if err == io.EOF {
			return fields, IO_OBJ_OUT, err
		} else if err != nil {
			return nil, IO_OBJ_OUT, err
		}
	}
}

// Read reads from a Nagios config stream and returns the next config object.
// Should be called repeatedly. Returns err = io.EOF when done
func (r *Reader) Read(setUUID bool, fileID string) (*CfgObj, error) {
	var fields []string
	var state IoState
	var err error
	var co *CfgObj
	var prevState IoState = IO_OBJ_OUT

	for {
		fields, state, err = r.parseLine()
		if fields != nil {
			switch state {
			case IO_OBJ_BEGIN:
				if prevState != IO_OBJ_OUT {
					//log.Debugf("prevState: %d, skipping a round", prevState)
					// continue goes too far, need jump to label or something...
					continue
				}
				ct := CfgName(fields[1]).Type()
				if ct == T_INVALID {
					log.Debugf("Invalid type (f#1): %q, Err: %q %s", fields, err, dbgStr(false))
					return nil, r.error(ErrUnknown)
				}
				if setUUID {
					co = NewCfgObjWithUUID(ct)
					uuidorder = append(uuidorder, co.UUID) // keep track of original order of objects read
				} else {
					co = NewCfgObj(ct)
				}
				if fileID != "" {
					co.FileID = fileID
				}
				prevState = IO_OBJ_BEGIN
			case IO_OBJ_IN:
				//prevState = IO_OBJ_IN
				fl := len(fields)
				//_debug(fields)
				if fl < 2 || co == nil {
					//return nil, r.error(ErrNoValue)
					log.Debugf("Too few fields (#%d): %#v %s", fl, fields, dbgStr(false))
					continue
				}
				//log.Debugf("%q %q", fields[0], strings.Join(fields[1:fl], " "))
				co.Add(fields[0], strings.Join(fields[1:fl], " "))
			case IO_OBJ_END:
				//fmt.Printf("Obj size: %d\n", co.size()) // approx avg turned out to be ~362 bytes per declaration for our services.cfg file
				return co, nil
			default:
				return nil, r.error(ErrUnknown)
			}
		}
		if err != nil {
			return nil, err
		}
	}

	// should not get here
	return nil, r.error(ErrUnknown)
}

func (r *Reader) ReadChan(setUUID bool, fileID string) <-chan *CfgObj {
	objchan := make(chan *CfgObj, 2) // making the channel buffered seems to make the function slightly faster
	go func() {
		for {
			obj, err := r.Read(setUUID, fileID)
			if err == nil && obj != nil {
				objchan <- obj
			}
			if err != nil {
				if err != io.EOF {
					log.Errorf("%q %s", err, dbgStr(true))
					continue
				}
				break
			}
		}
		close(objchan)
	}()
	return objchan
}

func (mfr MultiFileReader) ReadChan(setUUID bool) <-chan *CfgObj {
	// Need to do some fan-out, fan-in stuff here
	var wg sync.WaitGroup
	out := make(chan *CfgObj)
	mfrlen := len(mfr)

	output := func(c <-chan *CfgObj) {
		defer wg.Done()
		for v := range c {
			out <- v
			// The variant below tends to fail now and then
			//select {
			//case out <- v:
			//default:
			//	log.Errorf("%s.MultiFileReader.ReadChan().output(): Unable to copy to new channel", PKGNAME)
			//	return
			//}
		}
	}

	fcs := make([]<-chan *CfgObj, mfrlen)
	for i := range mfr {
		fileID, err := mfr[i].AbsPath()
		if err != nil {
			log.Errorf("%q %s", err, dbgStr(true))
			fileID = mfr[i].f.Name()
		}
		fcs[i] = mfr[i].ReadChan(setUUID, fileID)
	}

	wg.Add(mfrlen)

	for _, c := range fcs {
		go output(c)
	}

	go func() {
		wg.Wait()
		close(out)
	}()

	return out
}

// ReadAllList does the same as ReadAll, but returns a list instead of a slice
func (r *Reader) ReadAllList(setUUID bool, fileID string) (*list.List, error) {
	l := list.New()
	for {
		obj, err := r.Read(setUUID, fileID)
		if err == nil && obj != nil {
			l.PushBack(obj)
		}
		if err != nil {
			if err != io.EOF {
				return l, err
			} else {
				break
			}
		}
	}
	return l, nil
}

func (r *Reader) ReadAllMap(fileID string) (CfgMap, error) {
	m := make(CfgMap)
	for {
		obj, err := r.Read(true, fileID) // we always want UUID when reading to map
		if err == nil && obj != nil {
			m[obj.UUID] = obj
		}
		if err != nil {
			if err != io.EOF {
				return m, err
			} else {
				break
			}
		}
	}

	return m, nil
}

func (mfr MultiFileReader) ReadAllMap() (CfgMap, error) {
	cm := make(CfgMap)
	errcnt := 0
	for i := range mfr {
		fileID, err := mfr[i].AbsPath()
		if err != nil {
			log.Errorf("%q %s", err, dbgStr(true))
			fileID = mfr[i].f.Name()
		}
		m, err := mfr[i].ReadAllMap(fileID)
		if err != nil {
			log.Errorf("%q %s", err, dbgStr(true))
			errcnt++
		}
		err = cm.Append(m)
		if err != nil {
			log.Errorf("%q %s", err, dbgStr(true))
		}
	}

	if errcnt > 0 {
		return nil, fmt.Errorf("Encountered %d errors, bailing out %s", errcnt, dbgStr(true))
	}
	return cm, nil
}

// PrintProps prints a CfgObj's properties in random order
func (co *CfgObj) PrintProps(w io.Writer, format string) {
	for k, v := range co.Props {
		fmt.Fprintf(w, format, k, v)
	}
}

// PrintPropsSorted prints a CfgObj's properties acording to sort order found here:
// https://assets.nagios.com/downloads/nagioscore/docs/nagioscore/3/en/objectdefinitions.html
func (co *CfgObj) PrintPropsSorted(w io.Writer, format string) {
	keypri := make(map[int]string)
	for k := range co.Props {
		keypri[CfgKeySortOrder[k][co.Type]] = k // should have error checking for non-existing keys/types
	}
	keys := make([]int, len(keypri))
	i := 0
	for k := range keypri {
		keys[i] = k
		i++
	}
	sort.Ints(keys)
	for _, k := range keys {
		fmt.Fprintf(w, format, keypri[k], co.Props[keypri[k]])
	}
}

// Print prints out a CfgObj in Nagios format
func (co *CfgObj) Print(w io.Writer, sorted bool) {
	prefix := strings.Repeat(" ", co.Indent)
	fstr := fmt.Sprintf("%s%s%d%s", prefix, "%-", co.Align, "s%s\n")
	co.generateComment() // this might fail, but don't care yet
	fmt.Fprintf(w, "%s\n", co.Comment)
	fmt.Fprintf(w, "define %s{\n", co.Type.String())
	if sorted {
		co.PrintPropsSorted(w, fstr)
	} else {
		co.PrintProps(w, fstr)
	}
	fmt.Fprintf(w, "%s}\n", prefix)
}

// Print writes a collection of CfgObj to a given stream
func (cos CfgObjs) Print(w io.Writer, sorted bool) {
	for i := range cos {
		cos[i].Print(w, sorted)
		fmt.Fprint(w, "\n")
	}
}

func (cm CfgMap) Print(w io.Writer, sorted bool) {
	if sorted {
		keys := cm.Keys()
		for i := range keys {
			cm[keys[i]].Print(w, sorted)
			fmt.Fprintf(w, "\n")
		}
	} else {
		for k := range cm {
			cm[k].Print(w, sorted)
			fmt.Fprintf(w, "\n")
		}
	}
}

func (cm CfgMap) PrintUUIDs(w io.Writer, u UUIDs, sorted bool) {
	for _, v := range u {
		obj, ok := cm.GetByUUID(v)
		if ok && obj != nil {
			obj.Print(w, sorted)
			fmt.Fprintf(w, "\n")
		}
	}
}

func (nc *NagiosCfg) Print(w io.Writer, sorted bool) {
	nc.Config.Print(w, sorted)
}

func (nc *NagiosCfg) PrintUUIDs(w io.Writer, u UUIDs, sorted bool) {
	nc.Config.PrintUUIDs(w, u, sorted)
}

func (nc *NagiosCfg) PrintMatches(w io.Writer, sorted bool) {
	if nc.matches == nil || len(nc.matches) == 0 {
		return
	}
	// I'd like original ordering here as well
	for i := range nc.matches {
		nc.Config[nc.matches[i]].Print(w, sorted)
		fmt.Fprintf(w, "\n")
	}
}

func (nc *NagiosCfg) DumpString() string {
	var buf bytes.Buffer
	w := bufio.NewWriter(&buf)
	nc.Print(w, true)
	w.Flush()
	return buf.String()
}

func (nc *NagiosCfg) SaveToOrigin(sorted bool) error {
	return nc.Config.WriteByFileID(sorted)
}

func (nc *NagiosCfg) WriteFile(filename string, sort bool) error {
	return nc.Config.WriteFile(filename, sort)
}

func (cm CfgMap) WriteFile(filename string, sort bool) error {
	fhnd, err := os.Create(filename)
	if err != nil {
		return err
	}
	defer fhnd.Close()
	w := bufio.NewWriter(fhnd)
	for k := range cm {
		cm[k].Print(w, sort)
	}
	w.Flush()
	return nil
}

func (cm CfgMap) WriteByFileID(sort bool) error {
	var wg sync.WaitGroup
	fmap := cm.SplitByFileID(sort) // sorted and ready
	schan := make(chan error)


	// debug dups
	//log.Debugf("fmap length: %d (in: %s)", len(fmap), oddebug.DebugInfoMedium(PROJECT_PREFIX))
	//for k := range fmap {
	//	dups := findDups(fmap[k])
	//	if dups != nil {
	//		log.Debugf("Dups in fmap[%s]: %q (in: %s)", k, fmap[k], oddebug.DebugInfoMedium(PROJECT_PREFIX))
	//	}
	//}

	for fname := range fmap {
		wg.Add(1)
		go func(filename string) {
			defer wg.Done()
			fhnd, err := os.Create(filename)
			if err != nil {
				schan <- err
				return
			}
			defer fhnd.Close()
			w := bufio.NewWriter(fhnd)
			for i := range fmap[filename] {
				cm[fmap[filename][i]].Print(w, sort)
				fmt.Fprintf(w, "\n") // add extra blank line between each object
			}
			w.Flush()
			schan <- nil
		}(fname)
	}

	go func() {
		wg.Wait()
		close(schan)
	}()

	var errcnt int
	for e := range schan {
		if e != nil {
			log.Error(e)
			errcnt++
		}
	}

	if errcnt > 0 {
		return fmt.Errorf("Error writing to %d files %s", errcnt, dbgStr(true))
	}

	return nil
}

