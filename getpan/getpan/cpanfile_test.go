package getpan

import (
	"testing"
	"github.com/stretchr/testify/assert"
	"github.com/ian-kent/gopan/getpan/getpan/token"
)

func TestParseCpanfile(t *testing.T) {
	testParser(t)
	//testBasicCpanfile(t)
}

func testParser(t *testing.T) {
	scanner := NewScanner([]byte("requires 'Some::Module', '0.01';"))
	statements := scanner.Parse()

	assert.Equal(t, len(statements), 1, "has 1 statement")
	assert.Equal(t, statements[0].Offset, 0, "offset is 0")
	assert.Equal(t, statements[0].Literal, "requires'Some::Module','0.01';", "literal is correct")

	scanner = NewScanner([]byte("requires 'Some::Module';\n#this is a comment\ntest_requires \"Another::Module\", \"== 1.74.22\""))
	statements = scanner.Parse()

	assert.Equal(t, len(statements), 3, "has 3 statements")
	assert.Equal(t, statements[0].Offset, 0, "offset is 0")
	assert.Equal(t, statements[0].Literal, "requires'Some::Module';", "literal is correct")
	assert.Equal(t, statements[1].Offset, 23, "offset is 23")
	assert.Equal(t, statements[1].Literal, "this is a comment", "literal is correct")
	assert.Equal(t, statements[2].Offset, 25, "offset is 25") // FIXME this seems wrong - comment length is ignored?
	assert.Equal(t, statements[2].Literal, "test_requires\"Another::Module\",\"== 1.74.22\"", "literal is correct")
}

func testScanner(t *testing.T) {
	scanner := NewScanner([]byte("requires 'Some::Module', '0.01';"))
	assertScan(t, scanner, 0, token.IDENT, "requires")
	assertScan(t, scanner, 9, token.STRING, "'Some::Module'")
	assertScan(t, scanner, 23, token.COMMA, "")
	assertScan(t, scanner, 25, token.STRING, "'0.01'")
	assertScan(t, scanner, 31, token.SEMICOLON, "")
	assertScan(t, scanner, 32, token.EOF, "")

	scanner = NewScanner([]byte("requires 'Some::Module', '>= 0.01';"))
	assertScan(t, scanner, 0, token.IDENT, "requires")
	assertScan(t, scanner, 9, token.STRING, "'Some::Module'")
	assertScan(t, scanner, 23, token.COMMA, "")
	assertScan(t, scanner, 25, token.STRING, "'>= 0.01'")
	assertScan(t, scanner, 34, token.SEMICOLON, "")
	assertScan(t, scanner, 35, token.EOF, "")

	scanner = NewScanner([]byte("requires 'Some::Module','>= 0.01';"))
	assertScan(t, scanner, 0, token.IDENT, "requires")
	assertScan(t, scanner, 9, token.STRING, "'Some::Module'")
	assertScan(t, scanner, 23, token.COMMA, "")
	assertScan(t, scanner, 24, token.STRING, "'>= 0.01'")
	assertScan(t, scanner, 33, token.SEMICOLON, "")
	assertScan(t, scanner, 34, token.EOF, "")

	scanner = NewScanner([]byte("requires'Some::Module','>= 0.01';"))
	assertScan(t, scanner, 0, token.IDENT, "requires")
	assertScan(t, scanner, 8, token.STRING, "'Some::Module'")
	assertScan(t, scanner, 22, token.COMMA, "")
	assertScan(t, scanner, 23, token.STRING, "'>= 0.01'")
	assertScan(t, scanner, 32, token.SEMICOLON, "")
	assertScan(t, scanner, 33, token.EOF, "")

	scanner = NewScanner([]byte("requires	'Some::Module'	,	'>= 0.01'	;	"))
	assertScan(t, scanner, 0, token.IDENT, "requires")
	assertScan(t, scanner, 9, token.STRING, "'Some::Module'")
	assertScan(t, scanner, 24, token.COMMA, "")
	assertScan(t, scanner, 26, token.STRING, "'>= 0.01'")
	assertScan(t, scanner, 36, token.SEMICOLON, "")
	assertScan(t, scanner, 38, token.EOF, "")

	scanner = NewScanner([]byte("requires 'Some::Module';"))
	assertScan(t, scanner, 0, token.IDENT, "requires")
	assertScan(t, scanner, 9, token.STRING, "'Some::Module'")
	assertScan(t, scanner, 23, token.SEMICOLON, "")
	assertScan(t, scanner, 24, token.EOF, "")

	scanner = NewScanner([]byte("test_requires 'Some::Module';"))
	assertScan(t, scanner, 0, token.IDENT, "test_requires")
	assertScan(t, scanner, 14, token.STRING, "'Some::Module'")
	assertScan(t, scanner, 28, token.SEMICOLON, "")
	assertScan(t, scanner, 29, token.EOF, "")

	scanner = NewScanner([]byte("requires 'Some::Module'"))
	assertScan(t, scanner, 0, token.IDENT, "requires")
	assertScan(t, scanner, 9, token.STRING, "'Some::Module'")
	assertScan(t, scanner, 23, token.EOF, "")

	scanner = NewScanner([]byte("requires 'Some::Module';\ntest_requires \"Another::Module\", \"== 1.74.22\""))
	assertScan(t, scanner, 0, token.IDENT, "requires")
	assertScan(t, scanner, 9, token.STRING, "'Some::Module'")
	assertScan(t, scanner, 23, token.SEMICOLON, "")
	assertScan(t, scanner, 25, token.IDENT, "test_requires")
	assertScan(t, scanner, 39, token.STRING, "\"Another::Module\"")
	assertScan(t, scanner, 56, token.COMMA, "")
	assertScan(t, scanner, 58, token.STRING, "\"== 1.74.22\"")
	assertScan(t, scanner, 70, token.EOF, "")

	scanner = NewScanner([]byte("requires 'Some::Module';\n#this is a comment\ntest_requires \"Another::Module\", \"== 1.74.22\""))
	assertScan(t, scanner, 0, token.IDENT, "requires")
	assertScan(t, scanner, 9, token.STRING, "'Some::Module'")
	assertScan(t, scanner, 23, token.SEMICOLON, "")
	assertScan(t, scanner, 25, token.COMMENT, "this is a comment")
	assertScan(t, scanner, 44, token.IDENT, "test_requires")
	assertScan(t, scanner, 58, token.STRING, "\"Another::Module\"")
	assertScan(t, scanner, 75, token.COMMA, "")
	assertScan(t, scanner, 77, token.STRING, "\"== 1.74.22\"")
	assertScan(t, scanner, 89, token.EOF, "")
}

func assertScan(t *testing.T, s *Scanner, pos int, tok token.Token, lit string) {
	p, tk, l := s.Scan()
	assert.Equal(t, p, pos, "pos = " + string(pos))
	assert.Equal(t, tk, tok, "tok = " + string(tok))
	assert.Equal(t, l, lit, "lit = '" + string(lit) + "'")
}

func testBasicCpanfile(t *testing.T) {
	cpanfile := "requires 'Some::Module', '0.01';"
	cpanf, err := ParseCpanfile(cpanfile)
	assert.Nil(t, err, "Error is nil")
	assert.NotNil(t, cpanf, "Dependency list is not nil")
	assert.Equal(t, len(cpanf.DependencyList.Dependencies), 1, "Dependency list has 1 item")
	assert.Equal(t, len(cpanf.DependencyList.Dependencies[0].Name), "Some::Module", "Dependency name is 'Some::Module'")
	assert.Equal(t, len(cpanf.DependencyList.Dependencies[0].Version), "0.01", "Dependency version is '0.01'")
}
