package getpan

import (
	"github.com/ian-kent/go-log/log"
	"io/ioutil"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
	"github.com/ian-kent/gopan/getpan/getpan/token"
	"fmt"
)

var re = regexp.MustCompile("^\\s*requires\\s+['\"]([^'\"]+)['\"](,\\s+['\"]([^'\"]+)['\"])?;\\s*(#.*)?")

type CPANFile struct {
	DependencyList
}

const bom = 0xFEFF // byte order mark, only permitted as very first character

// http://golang.org/src/pkg/go/scanner/scanner.go
type Scanner struct {
	// immutable state
	src []byte

	// scanning state
    ch         rune // current character
    offset     int  // character offset
    rdOffset   int  // reading offset (position after current character)
    lineOffset int  // current line offset
    ErrorCount int  // number of errors
}

type Token struct {
	Offset  int
	Token   token.Token
	Literal string
}

type Statement struct {
	Offset  int
	Tokens  []*Token
	Literal string
}

func (s *Scanner) next() {
	if s.rdOffset < len(s.src) {
		s.offset = s.rdOffset
		if s.ch == '\n' {
			s.lineOffset = s.offset
		}
		r, w := rune(s.src[s.rdOffset]), 1
		switch {
		case r == 0:
			s.error(s.offset, "illegal character NUL")
		case r >= 0x80:
			// not ASCII
			r, w = utf8.DecodeRune(s.src[s.rdOffset:])
			if r == utf8.RuneError && w == 1 {
				s.error(s.offset, "illegal UTF-8 encoding")
			} else if r == bom && s.offset > 0 {
				s.error(s.offset, "illegal byte order mark")
			}
		}
		s.rdOffset += w
		s.ch = r
	} else {
		s.offset = len(s.src)
		if s.ch == '\n' {
			s.lineOffset = s.offset
		}
		s.ch = -1 // eof
	}
}

func NewScanner(src []byte) *Scanner {
	s := &Scanner{
		src: src,
		ch: ' ',
		offset: 0,
		rdOffset: 0,
		lineOffset: 0,
		ErrorCount: 0,
	}

	s.next()
	if s.ch == bom {
		s.next() // ignore BOM
	}

	return s
}

func (s *Scanner) error(offset int, msg string) {
	log.Error("Parse error [%d]: %s", offset, msg)
	s.ErrorCount++
}

func (s *Scanner) skipWhitespace() {
	for s.ch == ' ' || s.ch == '\t' || s.ch == '\n' || s.ch == '\r' {
		s.next()
	}
}

func (s *Scanner) scanEscape(quote rune) bool {
	offs := s.offset

	var n int
	var base, max uint32
	switch s.ch {
	case 'a', 'b', 'f', 'n', 'r', 't', 'v', '\\', quote:
		s.next()
		return true
	case '0', '1', '2', '3', '4', '5', '6', '7':
		n, base, max = 3, 8, 255
	case 'x':
		s.next()
		n, base, max = 2, 16, 255
	case 'u':
		s.next()
		n, base, max = 4, 16, unicode.MaxRune
	case 'U':
		s.next()
		n, base, max = 8, 16, unicode.MaxRune
	default:
		msg := "unknown escape sequence"
		if s.ch < 0 {
			msg = "escape sequence not terminated"
		}
		s.error(offs, msg)
		return false
	}

	var x uint32
	for n > 0 {
		d := uint32(digitVal(s.ch))
		if d >= base {
			msg := fmt.Sprintf("illegal character %#U in escape sequence", s.ch)
			if s.ch < 0 {
				msg = "escape sequence not terminated"
			}
			s.error(s.offset, msg)
			return false
		}
		x = x*base + d
		s.next()
		n--
	}

	if x > max || 0xD800 <= x && x < 0xE000 {
		s.error(offs, "escape sequence is invalid Unicode code point")
		return false
	}

	return true
}

func (s *Scanner) scanString(term rune) string {
	// '"' opening already consumed
	offs := s.offset - 1

	for {
		ch := s.ch
		if ch == '\n' || ch < 0 {
			s.error(offs, "string literal not terminated")
			break
		}
		s.next()
		if ch == term {
			break
		}
		if ch == '\\' {
			s.scanEscape('"')
		}
	}

	return string(s.src[offs:s.offset])
}

func isLetter(ch rune) bool {
	return 'a' <= ch && ch <= 'z' || 'A' <= ch && ch <= 'Z' || ch == '_' || ch >= 0x80 && unicode.IsLetter(ch)
}

func isDigit(ch rune) bool {
	return '0' <= ch && ch <= '9' || ch >= 0x80 && unicode.IsDigit(ch)
}

func digitVal(ch rune) int {
	switch {
	case '0' <= ch && ch <= '9':
		return int(ch - '0')
	case 'a' <= ch && ch <= 'f':
		return int(ch - 'a' + 10)
	case 'A' <= ch && ch <= 'F':
		return int(ch - 'A' + 10)
	}
	return 16 // larger than any legal digit val
}

func (s *Scanner) scanMantissa(base int) {
	for digitVal(s.ch) < base {
		s.next()
	}
}

func (s *Scanner) scanNumber(seenDecimalPoint bool) (token.Token, string) {
 	// digitVal(s.ch) < 10
 	offs := s.offset
 	tok := token.INT
 
 	if seenDecimalPoint {
 		offs--
 		tok = token.FLOAT
 		s.scanMantissa(10)
 		goto exponent
 	}
 
 	if s.ch == '0' {
 		// int or float
 		offs := s.offset
 		s.next()
 		if s.ch == 'x' || s.ch == 'X' {
 			// hexadecimal int
 			s.next()
 			s.scanMantissa(16)
 			if s.offset-offs <= 2 {
 				// only scanned "0x" or "0X"
 				s.error(offs, "illegal hexadecimal number")
 			}
 		} else {
 			// octal int or float
 			seenDecimalDigit := false
 			s.scanMantissa(8)
 			if s.ch == '8' || s.ch == '9' {
 				// illegal octal int or float
 				seenDecimalDigit = true
 				s.scanMantissa(10)
 			}
 			if s.ch == '.' || s.ch == 'e' || s.ch == 'E' || s.ch == 'i' {
 				goto fraction
 			}
 			// octal int
 			if seenDecimalDigit {
 				s.error(offs, "illegal octal number")
 			}
 		}
 		goto exit
 	}
 
 	// decimal int or float
 	s.scanMantissa(10)
 
 fraction:
 	if s.ch == '.' {
 		tok = token.FLOAT
 		s.next()
 		s.scanMantissa(10)
 	}
 
 exponent:
 	if s.ch == 'e' || s.ch == 'E' {
 		tok = token.FLOAT
 		s.next()
 		if s.ch == '-' || s.ch == '+' {
 			s.next()
 		}
 		s.scanMantissa(10)
 	}
 
 	if s.ch == 'i' {
 		tok = token.IMAG
 		s.next()
 	}
 
 exit:
 	return tok, string(s.src[offs:s.offset])
}

func (s *Scanner) scanIdentifier() string {
 	offs := s.offset
 	for isLetter(s.ch) || isDigit(s.ch) {
 		s.next()
 	}
 	return string(s.src[offs:s.offset])
 }

func (s *Scanner) scanToEOL() string {
 	offs := s.offset
 	for s.ch != '\n' {
 		s.next()
 	}
 	return string(s.src[offs:s.offset])
 }

func (s *Scanner) Scan() (pos int, tok token.Token, lit string) {
	s.skipWhitespace()

	pos = s.offset

	switch ch := s.ch; {
	case isLetter(ch):
		lit = s.scanIdentifier()
		tok = token.IDENT
	case '0' <= ch && ch <= '9':
		tok, lit = s.scanNumber(false)
	default:
		s.next()
		switch ch {
		case -1:
			tok = token.EOF
		case '\n':
			tok = token.NEWLINE
			lit = "\n"
		case ';':
			tok = token.SEMICOLON
			lit = ";"
		case '"':
			tok = token.STRING
			lit = s.scanString('"')
		case '\'':
			tok = token.STRING
			lit = s.scanString('\'')
		case ',':
			tok = token.COMMA
			lit = ","
		case '#':
			tok = token.COMMENT
			lit = s.scanToEOL()
		}
	}

	return
}

func (s *Scanner) Parse() []*Statement {
	stmts := make([]*Statement, 0)

	stmt := &Statement{
		Offset: 0,
		Tokens: make([]*Token, 0),
		Literal: "",
	}
	for {
		p, t, l := s.Scan()

		if t == token.EOF {
			if len(stmt.Tokens) > 0 {
				stmts = append(stmts, stmt)
			}
			break
		}
		stmt.Tokens = append(stmt.Tokens, &Token{p, t, l})
		stmt.Literal += l
		if t == token.NEWLINE || t == token.SEMICOLON || t == token.COMMENT { // should this have implicit semicolon at EOL?
			stmts = append(stmts, stmt)
			stmt = &Statement{
				Offset: p,
				Tokens: make([]*Token, 0),
			}
			continue
		}
	}

	return stmts
}

func ParseCpanfile(cpanfile string) (*CPANFile, error) {
	s := NewScanner([]byte(cpanfile))

	for {
		pos, tok, lit := s.Scan()



		log.Info("Got token [%d]: %s (%s)", pos, tok, lit)
		if tok == token.EOF {
			break
		}
		switch tok {
		case token.IDENT:
			log.Info("Ident: %s", lit)
			break
		}
	}

	return nil, nil
}

func ParseCPANLine(line string) (*Dependency, error) {
	if len(line) == 0 {
		return nil, nil
	}

	matches := re.FindStringSubmatch(line)
	if len(matches) == 0 {
		log.Trace("Unable to parse line: %s", line)
		return nil, nil
	}

	module := matches[1]
	version := matches[3]
	comment := matches[4]

	dependency, err := DependencyFromString(module, version)

	if strings.HasPrefix(strings.Trim(comment, " "), "# REQS: ") {
		comment = strings.TrimPrefix(strings.Trim(comment, " "), "# REQS: ")
		log.Trace("Found additional dependencies: %s", comment)
		for _, req := range strings.Split(comment, ";") {
			req = strings.Trim(req, " ")
			bits := strings.Split(req, "-")
			new_dep, err := DependencyFromString(bits[0], bits[1])
			if err != nil {
				log.Error("Error parsing REQS dependency: %s", req)
				continue
			}
			log.Trace("Added dependency: %s", new_dep)
			dependency.Additional = append(dependency.Additional, new_dep)
		}
	}

	if err != nil {
		return nil, err
	}

	log.Info("%s (%s %s)", module, dependency.Modifier, dependency.Version)

	return dependency, err
}

func ParseCPANFile(file string) (*CPANFile, error) {
	bytes, err := ioutil.ReadFile(file)
	if err != nil {
		return nil, err
	}

	lines := strings.Split(string(bytes), "\n")

	return ParseCPANLines(lines)
}

func ParseCPANLines(lines []string) (*CPANFile, error) {
	cpanfile := &CPANFile{}

	for _, l := range lines {
		if len(l) == 0 {
			continue
		}

		log.Trace("Parsing line: %s", l)
		dep, err := ParseCPANLine(l)

		if err != nil {
			log.Error("=> Error parsing line: %s", err)
			continue
		}

		if dep != nil {
			log.Info("=> Found dependency: %s", dep)
			cpanfile.AddDependency(dep)
			continue
		}

		log.Trace("=> No error and no dependency found")
	}

	log.Info("Found %d dependencies in cpanfile", len(cpanfile.Dependencies))

	return cpanfile, nil
}
