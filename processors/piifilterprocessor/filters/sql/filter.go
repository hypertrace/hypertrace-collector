package sql

import (
	"strings"
	"unicode"

	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/regexmatcher"
	"go.opentelemetry.io/collector/consumer/pdata"
)

type sqlFilter struct {
	rm         *regexmatcher.Matcher
	targetKeys map[string]struct{}
}

var _ filters.Filter = (*sqlFilter)(nil)

func toLookupMap(keys []string) map[string]struct{} {
	keysMap := map[string]struct{}{}
	for _, key := range keys {
		keysMap[key] = struct{}{}
	}
	return keysMap
}

func NewFilter(rm *regexmatcher.Matcher, targetKeys []string) filters.Filter {
	return &sqlFilter{
		rm:         rm,
		targetKeys: toLookupMap(targetKeys),
	}
}

func (f *sqlFilter) Name() string {
	return "sql"
}

func (f *sqlFilter) RedactAttribute(key string, value pdata.AttributeValue) (bool, error) {
	if _, ok := f.targetKeys[key]; !ok {
		return false, nil
	}

	if len(value.StringVal()) == 0 {
		return false, nil
	}

	is := newCaseChangingStream(antlr.NewInputStream(value.StringVal()), true)
	lexer := NewMySqlLexer(is)

	isRedacted := false
	var str strings.Builder
	for token := lexer.NextToken(); token.GetTokenType() != antlr.TokenEOF; {
		if token.GetTokenType() == MySqlLexerSTRING_LITERAL {
			text := token.GetText()
			openQuote := ""
			closeQuote := ""
			lenText := len(text)
			if len(text) > 0 && (text[0] == '"' || text[0] == '\'') {
				openQuote = string(text[0])
				text = text[1:]
				lenText--
			}
			if lenText > 0 && (text[lenText-1] == '"' || text[lenText-1] == '\'') {
				closeQuote = string(text[lenText-1])
				text = text[:lenText-1]
			}
			_, redacted := f.rm.RedactString(text)
			token.SetText(openQuote + redacted + closeQuote)
			isRedacted = true
		}
		str.WriteString(token.GetText())
		token = lexer.NextToken()
	}

	if isRedacted {
		value.SetStringVal(str.String())
	}

	return isRedacted, nil
}

type CaseChangingStream struct {
	antlr.CharStream
	upper bool
}

// NewCaseChangingStream returns a new CaseChangingStream that forces
// all tokens read from the underlying stream to be either upper case
// or lower case based on the upper argument.
func newCaseChangingStream(in antlr.CharStream, upper bool) *CaseChangingStream {
	return &CaseChangingStream{
		in, upper,
	}
}

// LA gets the value of the symbol at offset from the current position
// from the underlying CharStream and converts it to either upper case
// or lower case.
func (is *CaseChangingStream) LA(offset int) int {
	in := is.CharStream.LA(offset)
	if in < 0 {
		// Such as antlr.TokenEOF which is -1
		return in
	}
	if is.upper {
		return int(unicode.ToUpper(rune(in)))
	}
	return int(unicode.ToLower(rune(in)))
}
