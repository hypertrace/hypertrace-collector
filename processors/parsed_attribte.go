package processors

import (
	"context"

	"go.opentelemetry.io/collector/consumer/pdata"
)

type contextKey struct{}

func FromContext(ctx context.Context) (context.Context, *ParsedTracesData) {
	sAttr, ok := ctx.Value(contextKey{}).(*ParsedTracesData)
	if ok {
		return ctx, sAttr
	}
	ptd := &ParsedTracesData{
		spanAttributeMap: map[pdata.SpanID]*ParsedSpanData{},
	}
	ctx = context.WithValue(ctx, contextKey{}, ptd)
	return ctx, ptd
}

type ParsedTracesData struct {
	spanAttributeMap map[pdata.SpanID]*ParsedSpanData
}

func (p *ParsedTracesData) GetParsedSpanData(spanID pdata.SpanID) *ParsedSpanData {
	pSpanData, ok := p.spanAttributeMap[spanID]
	if !ok {
		pSpanData = &ParsedSpanData{
			parsedAttributes: map[string]*ParsedAttribute{},
		}
		p.spanAttributeMap[spanID] = pSpanData
	}
	return pSpanData
}

type ParsedSpanData struct {
	parsedAttributes map[string]*ParsedAttribute
}

// TODO shall it accept span and the original value as well?
func (p *ParsedSpanData) GetAttribute(key string) *ParsedAttribute {
	pAttr, ok := p.parsedAttributes[key]
	if !ok {
		pAttr = &ParsedAttribute{
			Flattered: map[string]string{},
			Redacted: map[string]string{},
		}
		p.parsedAttributes[key] = pAttr
	}
	return pAttr
}

func (p *ParsedSpanData) PutParsedAttribute(key string, attr *ParsedAttribute) {
	p.parsedAttributes[key] = attr
}

type ComplexDataType int

const (
	Cookie ComplexDataType = iota
	JSON
)

// ParsedAttribute encapsulates
type ParsedAttribute struct {
	// flattered JSON, cookie of not redacted fields
	Flattered map[string]string
	// redacted flattered data
	Redacted map[string]string
}
