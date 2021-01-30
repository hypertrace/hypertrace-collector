package sql

import (
	"strings"
	"unicode"

	"github.com/antlr/antlr4/runtime/Go/antlr"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters"
	"github.com/hypertrace/collector/processors/piifilterprocessor/filters/sql/internal"
	"github.com/hypertrace/collector/processors/piifilterprocessor/redaction"
	"go.opentelemetry.io/collector/consumer/pdata"
)

type sqlFilter struct {
	redactor redaction.Redactor
}

var _ filters.Filter = (*sqlFilter)(nil)

func NewFilter(r redaction.Redactor) filters.Filter {
	return &sqlFilter{r}
}

func (f *sqlFilter) Name() string {
	return "sql"
}

func (f *sqlFilter) RedactAttribute(key string, value pdata.AttributeValue) (bool, error) {
	if len(value.StringVal()) == 0 {
		return false, nil
	}

	is := newCaseChangingStream(antlr.NewInputStream(value.StringVal()), true)
	lexer := internal.NewMySqlLexer(is)

	isRedacted := false
	var str strings.Builder
	for token := lexer.NextToken(); token.GetTokenType() != antlr.TokenEOF; {
		if token.GetTokenType() == internal.MySqlLexerSTRING_LITERAL {
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
			redacted := f.redactor(text)
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

type caseChangingStream struct {
	antlr.CharStream
	upper bool
}

// newCaseChangingStream returns a new CaseChangingStream that forces
// all tokens read from the underlying stream to be either upper case
// or lower case based on the upper argument.
func newCaseChangingStream(in antlr.CharStream, upper bool) *caseChangingStream {
	return &caseChangingStream{
		in, upper,
	}
}

// LA gets the value of the symbol at offset from the current position
// from the underlying CharStream and converts it to either upper case
// or lower case.
func (is *caseChangingStream) LA(offset int) int {
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
