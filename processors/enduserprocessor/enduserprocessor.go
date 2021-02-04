package enduserprocessor

import (
	"context"
	"go.opentelemetry.io/collector/consumer/pdata"
	"go.opentelemetry.io/collector/processor/processorhelper"
)

type processor struct {
}

var _ processorhelper.TProcessor = (*processor)(nil)

func (p processor) ProcessTraces(ctx context.Context, traces pdata.Traces) (pdata.Traces, error) {
	panic("implement me")
}
