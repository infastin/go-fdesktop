package fdesktop

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

type byteScanner struct {
	fmt.ScanState
}

func (s byteScanner) ReadByte() (byte, error) {
	ch, n, err := s.ReadRune()
	if err == nil && n > 1 {
		err = fmt.Errorf("invalid rune %#U", ch)
	}
	return byte(ch), err
}

func (s byteScanner) UnreadByte() error {
	return s.UnreadRune()
}

type Locale struct {
	Lang     string
	Country  string
	Encoding string
	Modifier string
}

func (l *Locale) scan(s io.ByteScanner) (err error) {
	b := strings.Builder{}
	i := 0

	delims := [][]byte{
		{'_', '-'},
		{'.'},
		{'@'},
		{'\000'},
	}

	delimIndex := func(ch byte) int {
		for i := range delims {
			for _, d := range delims[i] {
				if d == ch {
					return i
				}
			}
		}

		return -1
	}

	rts := [][]*unicode.RangeTable{
		{unicode.Lower},
		{unicode.Letter, unicode.Dash},
		{unicode.Letter, unicode.Digit, unicode.Dash},
		{unicode.Letter},
	}

loop:
	for {
		var ch byte

		ch, err = s.ReadByte()
		if err != nil {
			break
		}

		switch k := delimIndex(ch); {
		case k >= i:
			if b.Len() == 0 {
				err = fmt.Errorf("expected something before '%c'", ch)
				break loop
			}

			switch i {
			case 0:
				l.Lang = b.String()
			case 1:
				l.Country = b.String()
			case 2:
				l.Encoding = b.String()
			case 3:
				l.Modifier = b.String()
			}

			i = k + 1
			b.Reset()

		default:
			if !unicode.IsOneOf(rts[i], rune(ch)) {
				err = fmt.Errorf("invalid rune %#U", ch)
				break loop
			}

			b.WriteByte(ch)
		}
	}

	if b.Len() != 0 {
		switch i {
		case 0:
			l.Lang = b.String()
		case 1:
			l.Country = b.String()
		case 2:
			l.Encoding = b.String()
		case 3:
			l.Modifier = b.String()
		}
	}

	if err == io.EOF {
		if i != 0 && b.Len() == 0 {
			err = fmt.Errorf("expected something after '%c'", delims[i-1])
		} else {
			err = nil
		}
	} else if err != nil {
		s.UnreadByte()
	}

	return
}

func (l *Locale) Scan(state fmt.ScanState, verb rune) error {
	state.SkipSpace()

	switch verb {
	case 'v', 'l':
	default:
		return errors.New("invalid verb")
	}

	return l.scan(byteScanner{state})
}

func (l Locale) String() string {
	var b strings.Builder

	if len(l.Lang) > 0 {
		b.WriteString(l.Lang)

		if len(l.Country) > 0 {
			b.WriteByte('_')
			b.WriteString(l.Country)
		}

		if len(l.Encoding) > 0 {
			b.WriteByte('.')
			b.WriteString(l.Encoding)
		}

		if len(l.Modifier) > 0 {
			b.WriteByte('@')
			b.WriteString(l.Modifier)
		}
	}

	return b.String()
}

type Value map[string]string

func (m Value) load(loc Locale, val string) bool {
	locS := loc.String()
	if _, ok := m[locS]; ok {
		return false
	}

	m[locS] = val
	return true
}

type KeyMap map[string]Value

func (m KeyMap) load(key string, loc Locale, val string) bool {
	var vm Value

	if v, ok := m[key]; ok {
		vm = v
	} else {
		vm = make(Value)
		m[key] = vm
	}

	return vm.load(loc, val)
}

func (m KeyMap) TryGetLocales(key string) ([]Locale, error) {
	if v, ok := m[key]; ok {
		locales := make([]Locale, len(v))

		i := 0
		for k := range v {
			fmt.Sscan(k, &locales[i])
			i++
		}

		return locales, nil
	}

	return nil, fmt.Errorf("key %s not found", key)
}

func (m KeyMap) GetLocales(key string) []Locale {
	v := m[key]
	locales := make([]Locale, len(v))

	i := 0
	for k := range v {
		fmt.Sscan(k, &locales[i])
		i++
	}

	return locales
}

func (m KeyMap) TryGetString(loc string, key string) (string, error) {
	if v, ok := m[key]; ok {
		if v, ok := v[loc]; ok {
			return v, nil
		}

		return "", fmt.Errorf("key %s[%s] not found", key, loc)
	}

	return "", fmt.Errorf("key %s[%s] not found", key, loc)
}

func (m KeyMap) GetString(loc string, key string) string {
	return m[key][loc]
}

func (m KeyMap) TryGetBoolean(loc string, key string) (bool, error) {
	str, err := m.TryGetString(loc, key)
	if err != nil {
		return false, err
	}

	switch str {
	case "true":
		return true, nil
	case "false":
		return false, nil
	default:
		return false, fmt.Errorf("value for key %s[%s] isn't boolean", key, loc)
	}
}

func (m KeyMap) GetBoolean(loc string, key string) bool {
	str := m.GetString(loc, key)

	switch str {
	case "true":
		return true
	case "false":
		return false
	default:
		panic(fmt.Sprintf("value for key %s[%s] isn't boolean", key, loc))
	}
}

func (m KeyMap) TryGetNumeric(loc string, key string) (float64, error) {
	str, err := m.TryGetString(loc, key)
	if err != nil {
		return 0, err
	}

	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		return 0, fmt.Errorf("value for key %s[%s] isn't numeric", key, loc)
	}

	return f, nil
}

func (m KeyMap) GetNumeric(loc string, key string) float64 {
	str := m.GetString(loc, key)

	f, err := strconv.ParseFloat(str, 64)
	if err != nil {
		panic(fmt.Sprintf("value for key %s[%s] isn't numeric", key, loc))
	}

	return f
}

type GroupMap map[string]KeyMap

func (m GroupMap) add(group string) bool {
	if _, ok := m[group]; ok {
		return false
	}

	m[group] = make(KeyMap)
	return true
}

func (m GroupMap) load(group, key string, loc Locale, val string) bool {
	if v, ok := m[group]; ok {
		return v.load(key, loc, val)
	}

	return false
}

type parser struct {
	groups  GroupMap
	current string
}

func newParser() *parser {
	return &parser{
		groups:  make(GroupMap),
		current: "",
	}
}

func (p *parser) parseLine(line []byte) error {
	line = bytes.TrimSpace(line)

	if len(line) == 0 || line[0] == '#' {
		return nil
	}

	// Parse group headers
	if line[0] == '[' {
		i := 1
		for i < len(line) {
			ch := line[i]
			if ch == ']' {
				break
			} else if unicode.IsGraphic(rune(ch)) {
				i++
			} else {
				return fmt.Errorf("invalid rune %#U", ch)
			}
		}

		if i == len(line) {
			return errors.New("expected ']' but was not found")
		}

		group := string(line[1:i])
		if ok := p.groups.add(group); !ok {
			return fmt.Errorf("group %q already exists", group)
		}

		p.current = group
		return nil
	}

	// Parse key
	i := 0
	for i < len(line) {
		ch := line[i]
		if unicode.In(rune(ch), unicode.Letter, unicode.Digit, unicode.Dash) {
			i++
		} else if ch == '[' || ch == '=' || unicode.IsSpace(rune(ch)) {
			break
		} else {
			return fmt.Errorf("invalid rune %#U", ch)
		}
	}

	if i == len(line) {
		return errors.New("expected '=' but was not found")
	}

	key := string(line[:i])
	line = line[i:]

	// Parse key locale
	loc := Locale{}
	if line[0] == '[' {
		j := bytes.IndexByte(line[1:], ']')
		if j == -1 {
			return errors.New("expected ']' but was not found")
		}

		if bytes.HasPrefix(line[1:], []byte("x-")) {
			loc.Lang = string([]byte(line[1 : 1+j]))
		} else {
			_, err := fmt.Fscanf(bytes.NewReader(line[1:1+j]), "%l", &loc)
			if err != nil {
				return err
			}
		}

		line = line[j+2:]
	}

	line = bytes.TrimSpace(line)
	if len(line) == 0 || line[0] != '=' {
		return errors.New("expected '=' but was not found")
	}

	line = bytes.TrimSpace(line[1:])

	i = 0
	for i < len(line) {
		ch, size := utf8.DecodeRune(line[i:])
		if unicode.IsControl(ch) && !unicode.IsSpace(ch) {
			return fmt.Errorf("invalid rune %#U", ch)
		}
		i += size
	}

	val := string(line)
	p.groups.load(p.current, key, loc, val)

	return nil
}

func (p *parser) parse(reader io.Reader) (err error) {
	scanner := bufio.NewReader(reader)
	i := 1

	for {
		var line []byte
		var isPrefix bool

		line, isPrefix, err = scanner.ReadLine()
		if err != nil {
			break
		}

		if isPrefix {
			err = fmt.Errorf("line %d: line is too big", i)
			break
		}

		err = p.parseLine(line)
		if err != nil {
			break
		}

		i++
	}

	if err != io.EOF {
		err = fmt.Errorf("line %d: %v", i, err)
	} else {
		err = nil
	}

	return
}

type Entry struct {
	AppId  string
	Groups GroupMap
	Path   string
}

func NewEntry(appId, path string) *Entry {
	return &Entry{
		AppId: appId,
		Path: path,
	}
}

func (e *Entry) Decode(reader io.Reader) error {
	p := newParser()

	err := p.parse(reader)
	if err != nil {
		return err
	}

	e.Groups = p.groups
	return nil
}

func (e *Entry) TryGroup(name string) (KeyMap, bool) {
	if m, ok := e.Groups[name]; ok {
		return m, true
	}

	return nil, false
}

func (e *Entry) Group(name string) KeyMap {
	return e.Groups[name]
}

func (e *Entry) TryName() (string, bool) {
	m, ok := e.TryGroup("Desktop Entry")
	if !ok {
		return "", false
	}

	v, err := m.TryGetString("", "Name")
	if err != nil {
		return "", false
	}

	return v, true
}

func (e *Entry) Name() string {
	return e.Group("Desktop Entry").GetString("", "Name")
}
