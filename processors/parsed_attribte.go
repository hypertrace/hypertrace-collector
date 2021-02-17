package processors

import (
	"context"

	"go.opentelemetry.io/collector/consumer/pdata"
)

type contextKey struct{}

// FromContext returns ParsedTracesData and updated context
func FromContext(ctx context.Context) (context.Context, *ParsedTracesData) {
	sAttr, ok := ctx.Value(contextKey{}).(*ParsedTracesData)
	if ok {
		return ctx, sAttr
	}
	ptd := &ParsedTracesData{
		spanAttributeMap: map[pdata.Span]*ParsedSpanData{},
	}
	ctx = context.WithValue(ctx, contextKey{}, ptd)
	return ctx, ptd
}

// ParsedTracesData encapsulates parsed data for multiple traces e.g. pdata.Traces.
type ParsedTracesData struct {
	spanAttributeMap map[pdata.Span]*ParsedSpanData
}

// GetParsedSpanData returns ParsedSpanData for a given span.
func (p *ParsedTracesData) GetParsedSpanData(span pdata.Span) *ParsedSpanData {
	pSpanData, ok := p.spanAttributeMap[span]
	if !ok {
		pSpanData = &ParsedSpanData{
			parsedAttributes: map[string]*ParsedAttribute{},
		}
		p.spanAttributeMap[span] = pSpanData
	}
	return pSpanData
}

// ParsedSpanData encapsulates span parsed attributes.
type ParsedSpanData struct {
	parsedAttributes map[string]*ParsedAttribute
}

// GetAttribute returns ParsedAttribute for a given attribute key.
func (p *ParsedSpanData) GetAttribute(key string) *ParsedAttribute {
	pAttr, ok := p.parsedAttributes[key]
	if !ok {
		pAttr = &ParsedAttribute{
			Flattened: map[string]string{},
			Redacted:  map[string]string{},
		}
		p.parsedAttributes[key] = pAttr
	}
	return pAttr
}

// PutParsedAttribute puts ParsedAttribute into ParsedSpanData.
func (p *ParsedSpanData) PutParsedAttribute(key string, attr *ParsedAttribute) {
	p.parsedAttributes[key] = attr
}

// ParsedAttribute encapsulates parsed a single attribute.
type ParsedAttribute struct {
	// flattered JSON, cookie of not redacted fields
	// e.g. JSON {"a": "b"} will contain "$.a": "b"
	Flattened map[string]string
	// redacted flattered data
	Redacted map[string]string
}
