package parser

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"unicode"

	"github.com/mithrandie/csvq/lib/cmd"
	"github.com/mithrandie/csvq/lib/value"

	"github.com/mithrandie/ternary"
)

const (
	EOF = -(iota + 1)
	Uncategorized
)

const (
	TokenFrom   = IDENTIFIER
	TokenTo     = SUBSTITUTION_OP
	KeywordFrom = SELECT
	KeywordTo   = JSON_OBJECT
)

const (
	VariableSign            = '@'
	EnvironmentVariableSign = '%'
	ExternalCommandSign     = '$'
	RuntimeInformationSign  = '#'

	SubstitutionOperator = ":="

	BeginExpression = '{'
	EndExpression   = '}'
)

var comparisonOperators = []string{
	">",
	"<",
	">=",
	"<=",
	"<>",
	"!=",
	"==",
}

var stringOperators = []string{
	"||",
}

var aggregateFunctions = []string{
	"MIN",
	"MAX",
	"SUM",
	"AVG",
	"MEDIAN",
}

var listFunctions = []string{
	"LISTAGG",
	"JSON_AGG",
}

var analyticFunctions = []string{
	"ROW_NUMBER",
	"RANK",
	"DENSE_RANK",
	"CUME_DIST",
	"PERCENT_RANK",
	"NTILE",
}

var functionsNth = []string{
	"FIRST_VALUE",
	"LAST_VALUE",
	"NTH_VALUE",
}

var functionsWithIgnoreNulls = []string{
	"LAG",
	"LEAD",
}

func TokenLiteral(token int) string {
	if TokenFrom <= token && token <= TokenTo {
		return yyToknames[token-TokenFrom+3]
	}
	return string(token)
}

type Scanner struct {
	src     []rune
	srcPos  int
	literal *bytes.Buffer
	err     error

	line       int
	char       int
	sourceFile string
}

func (s *Scanner) Init(src string, sourceFile string) *Scanner {
	s.src = []rune(src)
	s.srcPos = 0
	s.literal = new(bytes.Buffer)
	s.err = nil
	s.line = 1
	s.char = 0
	s.sourceFile = sourceFile
	return s
}

func (s *Scanner) peek() rune {
	if len(s.src) <= s.srcPos {
		return EOF
	}

	return s.src[s.srcPos]
}

func (s *Scanner) next() rune {
	ch := s.peek()
	if ch == EOF {
		return ch
	}

	s.srcPos++
	s.char++

	ch = s.checkNewLine(ch)

	return ch
}

func (s *Scanner) checkNewLine(ch rune) rune {
	if ch != '\r' && ch != '\n' {
		return ch
	}

	if ch == '\r' && s.peek() == '\n' {
		s.srcPos++
	}

	s.line++
	s.char = 0
	return s.src[s.srcPos-1]
}

func (s *Scanner) Scan() (Token, error) {
	for unicode.IsSpace(s.peek()) {
		s.next()
	}

	ch := s.next()
	token := ch
	literal := string(ch)
	quoted := false
	line := s.line
	char := s.char

	switch {
	case s.isDecimal(ch):
		token = s.scanNumber(ch)
		literal = s.literal.String()
	case s.isIdentRune(ch):
		s.scanIdentifier(ch)

		literal = s.literal.String()
		if _, e := ternary.ConvertFromString(literal); e == nil {
			token = TERNARY
		} else if t, e := s.searchKeyword(literal); e == nil {
			token = rune(t)
		} else if s.isAggregateFunctions(literal) {
			token = AGGREGATE_FUNCTION
		} else if s.isListaggFunctions(literal) {
			token = LIST_FUNCTION
		} else if s.isAnalyticFunctions(literal) {
			token = ANALYTIC_FUNCTION
		} else if s.isFunctionsNth(literal) {
			token = FUNCTION_NTH
		} else if s.isFunctionsWithIgnoreNulls(literal) {
			token = FUNCTION_WITH_INS
		} else {
			token = IDENTIFIER
		}
	case s.isOperatorRune(ch):
		s.scanOperator(ch)

		literal = s.literal.String()
		if s.isComparisonOperators(literal) {
			token = COMPARISON_OP
		} else if s.isStringOperators(literal) {
			token = STRING_OP
		} else if literal == SubstitutionOperator {
			token = SUBSTITUTION_OP
		} else if 1 < len(literal) {
			token = Uncategorized
		}
	case ch == VariableSign:
		switch s.peek() {
		case EnvironmentVariableSign:
			s.next()
			token = ENVIRONMENT_VARIABLE
		case RuntimeInformationSign:
			s.next()
			token = RUNTIME_INFORMATION
		case VariableSign:
			s.next()
			token = FLAG
		default:
			token = VARIABLE
		}

		if token == ENVIRONMENT_VARIABLE && s.peek() == '`' {
			s.scanString(s.next())
			literal = cmd.UnescapeIdentifier(s.literal.String())
			quoted = true
		} else {
			if s.isIdentRune(s.peek()) {
				s.scanIdentifier(s.next())
				literal = s.literal.String()
			} else {
				literal = ""
			}
		}

		if len(literal) < 1 {
			s.err = errors.New("invalid variable symbol")
		}
	case ch == ExternalCommandSign:
		s.scanExternalCommand()
		literal = s.literal.String()
		token = EXTERNAL_COMMAND
	case s.isCommentRune(ch):
		s.scanComment()
		return s.Scan()
	case s.isLineCommentRune(ch):
		s.scanLineComment()
		return s.Scan()
	default:
		switch ch {
		case EOF:
			break
		case '"', '\'':
			s.scanString(ch)
			literal = cmd.UnescapeString(s.literal.String())
			if _, e := value.StrToTime(literal); e == nil {
				token = DATETIME
			} else {
				token = STRING
			}
		case '`':
			s.scanString(ch)
			literal = cmd.UnescapeIdentifier(s.literal.String())
			token = IDENTIFIER
			quoted = true
		}
	}

	return Token{Token: int(token), Literal: literal, Quoted: quoted, Line: line, Char: char, SourceFile: s.sourceFile}, s.err
}

func (s *Scanner) scanString(quote rune) {
	s.literal.Reset()

	for {
		ch := s.next()

		if ch == EOF {
			s.err = errors.New("literal not terminated")
			break
		}

		if ch == quote {
			break
		}

		if ch == '\\' {
			switch s.peek() {
			case '\\', quote:
				ch = s.next()
			}
		}
		s.literal.WriteRune(ch)
	}
}

func (s *Scanner) scanIdentifier(head rune) {
	s.literal.Reset()

	s.literal.WriteRune(head)
	for s.isIdentRune(s.peek()) {
		s.literal.WriteRune(s.next())
	}
}

func (s *Scanner) isIdentRune(ch rune) bool {
	return ch == '_' || unicode.IsLetter(ch) || unicode.IsDigit(ch)
}

func (s *Scanner) isDecimal(ch rune) bool {
	return '0' <= ch && ch <= '9'
}

func (s *Scanner) scanNumber(head rune) rune {
	s.literal.Reset()

	s.literal.WriteRune(head)
	for s.isDecimal(s.peek()) {
		s.literal.WriteRune(s.next())
	}

	if s.peek() == '.' {
		s.literal.WriteRune(s.next())
		for s.isDecimal(s.peek()) {
			s.literal.WriteRune(s.next())
		}
		return FLOAT
	}

	return INTEGER
}

func (s *Scanner) scanOperator(head rune) {
	s.literal.Reset()

	s.literal.WriteRune(head)
	for s.isOperatorRune(s.peek()) {
		s.literal.WriteRune(s.next())
	}
}

func (s *Scanner) isOperatorRune(ch rune) bool {
	switch ch {
	case '=', '>', '<', '!', '|', ':':
		return true
	}
	return false
}

func (s *Scanner) searchKeyword(str string) (int, error) {
	for i := KeywordFrom; i <= KeywordTo; i++ {
		if strings.EqualFold(TokenLiteral(i), str) {
			return i, nil
		}
	}
	return IDENTIFIER, errors.New(fmt.Sprintf("%q is not a keyword", str))
}

func (s *Scanner) isAggregateFunctions(str string) bool {
	for _, v := range aggregateFunctions {
		if strings.EqualFold(v, str) {
			return true
		}
	}
	return false
}

func (s *Scanner) isListaggFunctions(str string) bool {
	for _, v := range listFunctions {
		if strings.EqualFold(v, str) {
			return true
		}
	}
	return false
}

func (s *Scanner) isAnalyticFunctions(str string) bool {
	for _, v := range analyticFunctions {
		if strings.EqualFold(v, str) {
			return true
		}
	}
	return false
}

func (s *Scanner) isFunctionsNth(str string) bool {
	for _, v := range functionsNth {
		if strings.EqualFold(v, str) {
			return true
		}
	}
	return false
}

func (s *Scanner) isFunctionsWithIgnoreNulls(str string) bool {
	for _, v := range functionsWithIgnoreNulls {
		if strings.EqualFold(v, str) {
			return true
		}
	}
	return false
}

func (s *Scanner) isComparisonOperators(str string) bool {
	for _, v := range comparisonOperators {
		if v == str {
			return true
		}
	}
	return false
}

func (s *Scanner) isStringOperators(str string) bool {
	for _, v := range stringOperators {
		if v == str {
			return true
		}
	}
	return false
}

func (s *Scanner) isCommentRune(ch rune) bool {
	if ch == '/' && s.peek() == '*' {
		s.next()
		return true
	}
	return false
}

func (s *Scanner) scanComment() {
	for {
		ch := s.next()
		if ch == EOF {
			break
		} else if ch == '*' {
			if s.peek() == '/' {
				s.next()
				break
			}
		}
	}
}

func (s *Scanner) isLineCommentRune(ch rune) bool {
	if ch == '-' && s.peek() == '-' {
		s.next()
		return true
	}
	return false
}

func (s *Scanner) scanLineComment() {
	for {
		ch := s.peek()
		if ch == '\r' || ch == '\n' || ch == EOF {
			break
		}
		s.next()
	}
}

func (s *Scanner) scanExternalCommand() {
	s.literal.Reset()

	for {
		ch := s.peek()
		if ch == ';' || ch == EOF {
			break
		}

		s.literal.WriteRune(s.next())

		if ch == '"' || ch == '\'' || ch == '`' {
			s.scanExternalCommandQuotedString(ch)
			continue
		}

		if ch == ExternalCommandSign && s.peek() == BeginExpression {
			s.literal.WriteRune(s.next())
			s.scanExternalCommandCSVQExpression()
		}
	}
}

func (s *Scanner) scanExternalCommandQuotedString(quote rune) {
	for {
		ch := s.peek()

		if ch == EOF {
			break
		}

		s.literal.WriteRune(s.next())

		if ch == quote {
			break
		}

		if ch == '\\' {
			switch s.peek() {
			case '\\', quote:
				s.literal.WriteRune(s.next())
			}
		}
	}
}

func (s *Scanner) scanExternalCommandCSVQExpression() {
	for {
		ch := s.peek()

		if ch == EOF {
			break
		}

		s.literal.WriteRune(s.next())

		if ch == EndExpression {
			break
		}

		if ch == '\\' {
			switch s.peek() {
			case '\\', BeginExpression, EndExpression:
				s.literal.WriteRune(s.next())
			}
		}
	}
}
