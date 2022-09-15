// Copyright The OpenTelemetry Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package jaegerreceiver // import "github.com/open-telemetry/opentelemetry-collector-contrib/receiver/jaegerreceiver"

import (
	"context"
	"errors"
	"fmt"
	"html"
	"io/ioutil"
	"mime"
	"net/http"
	"sync"
	"time"

	apacheThrift "github.com/apache/thrift/lib/go/thrift"
	"github.com/gorilla/mux"
	"github.com/jaegertracing/jaeger/cmd/agent/app/configmanager"
	jSamplingConfig "github.com/jaegertracing/jaeger/cmd/agent/app/configmanager/grpc"
	"github.com/jaegertracing/jaeger/cmd/agent/app/httpserver"
	"github.com/jaegertracing/jaeger/cmd/agent/app/processors"
	"github.com/jaegertracing/jaeger/cmd/agent/app/servers"
	"github.com/jaegertracing/jaeger/cmd/agent/app/servers/thriftudp"
	"github.com/jaegertracing/jaeger/cmd/collector/app/handler"
	collectorSampling "github.com/jaegertracing/jaeger/cmd/collector/app/sampling"
	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/pkg/metrics"
	staticStrategyStore "github.com/jaegertracing/jaeger/plugin/sampling/strategystore/static"
	"github.com/jaegertracing/jaeger/proto-gen/api_v2"
	"github.com/jaegertracing/jaeger/thrift-gen/agent"
	"github.com/jaegertracing/jaeger/thrift-gen/baggage"
	"github.com/jaegertracing/jaeger/thrift-gen/jaeger"
	"github.com/jaegertracing/jaeger/thrift-gen/sampling"
	"github.com/jaegertracing/jaeger/thrift-gen/zipkincore"
	"go.opentelemetry.io/collector/component"
	"go.opentelemetry.io/collector/config"
	"go.opentelemetry.io/collector/config/configgrpc"
	"go.opentelemetry.io/collector/config/confighttp"
	"go.opentelemetry.io/collector/consumer"
	"go.opentelemetry.io/collector/obsreport"
	"go.uber.org/multierr"
	"go.uber.org/zap"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"

	jaegertranslator "github.com/open-telemetry/opentelemetry-collector-contrib/pkg/translator/jaeger"
)

// configuration defines the behavior and the ports that
// the Jaeger receiver will use.
type configuration struct {
	CollectorHTTPSettings       confighttp.HTTPServerSettings
	CollectorGRPCServerSettings configgrpc.GRPCServerSettings

	AgentCompactThrift                       ProtocolUDP
	AgentBinaryThrift                        ProtocolUDP
	AgentHTTPEndpoint                        string
	RemoteSamplingClientSettings             configgrpc.GRPCClientSettings
	RemoteSamplingStrategyFile               string
	RemoteSamplingStrategyFileReloadInterval time.Duration
}

// Receiver type is used to receive spans that were originally intended to be sent to Jaeger.
// This receiver is basically a Jaeger collector.
type jReceiver struct {
	nextConsumer consumer.Traces
	id           config.ComponentID

	config *configuration

	grpc            *grpc.Server
	collectorServer *http.Server

	agentSamplingManager *jSamplingConfig.SamplingManager
	agentProcessors      []processors.Processor
	agentServer          *http.Server

	goroutines sync.WaitGroup

	settings component.ReceiverCreateSettings

	grpcObsrecv *obsreport.Receiver
	httpObsrecv *obsreport.Receiver
}

const (
	agentTransportBinary   = "udp_thrift_binary"
	agentTransportCompact  = "udp_thrift_compact"
	collectorHTTPTransport = "collector_http"
	grpcTransport          = "grpc"

	thriftFormat   = "thrift"
	protobufFormat = "protobuf"
)

var (
	acceptedThriftFormats = map[string]struct{}{
		"application/x-thrift":                 {},
		"application/vnd.apache.thrift.binary": {},
	}
)

// newJaegerReceiver creates a TracesReceiver that receives traffic as a Jaeger collector, and
// also as a Jaeger agent.
func newJaegerReceiver(
	id config.ComponentID,
	config *configuration,
	nextConsumer consumer.Traces,
	set component.ReceiverCreateSettings,
) *jReceiver {
	return &jReceiver{
		config:       config,
		nextConsumer: nextConsumer,
		id:           id,
		settings:     set,
		grpcObsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             id,
			Transport:              grpcTransport,
			ReceiverCreateSettings: set,
		}),
		httpObsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
			ReceiverID:             id,
			Transport:              collectorHTTPTransport,
			ReceiverCreateSettings: set,
		}),
	}
}

func (jr *jReceiver) Start(_ context.Context, host component.Host) error {
	if err := jr.startAgent(host); err != nil {
		return err
	}

	return jr.startCollector(host)
}

func (jr *jReceiver) Shutdown(ctx context.Context) error {
	var errs error

	if jr.agentServer != nil {
		if aerr := jr.agentServer.Shutdown(ctx); aerr != nil {
			errs = multierr.Append(errs, aerr)
		}
	}
	for _, processor := range jr.agentProcessors {
		processor.Stop()
	}

	if jr.collectorServer != nil {
		if cerr := jr.collectorServer.Shutdown(ctx); cerr != nil {
			errs = multierr.Append(errs, cerr)
		}
	}
	if jr.grpc != nil {
		jr.grpc.GracefulStop()
	}

	jr.goroutines.Wait()
	return errs
}

func consumeTraces(ctx context.Context, batch *jaeger.Batch, consumer consumer.Traces) (int, error) {
	if batch == nil {
		return 0, nil
	}
	td, err := jaegertranslator.ThriftToTraces(batch)
	if err != nil {
		return 0, err
	}
	return len(batch.Spans), consumer.ConsumeTraces(ctx, td)
}

var _ agent.Agent = (*agentHandler)(nil)
var _ api_v2.CollectorServiceServer = (*jReceiver)(nil)
var _ configmanager.ClientConfigManager = (*jReceiver)(nil)

type agentHandler struct {
	nextConsumer consumer.Traces
	obsrecv      *obsreport.Receiver
}

// EmitZipkinBatch is unsupported agent's
func (h *agentHandler) EmitZipkinBatch(context.Context, []*zipkincore.Span) (err error) {
	panic("unsupported receiver")
}

// EmitBatch implements thrift-gen/agent/Agent and it forwards
// Jaeger spans received by the Jaeger agent processor.
func (h *agentHandler) EmitBatch(ctx context.Context, batch *jaeger.Batch) error {
	ctx = h.obsrecv.StartTracesOp(ctx)
	numSpans, err := consumeTraces(ctx, batch, h.nextConsumer)
	h.obsrecv.EndTracesOp(ctx, thriftFormat, numSpans, err)
	return err
}

func (jr *jReceiver) GetSamplingStrategy(ctx context.Context, serviceName string) (*sampling.SamplingStrategyResponse, error) {
	return jr.agentSamplingManager.GetSamplingStrategy(ctx, serviceName)
}

func (jr *jReceiver) GetBaggageRestrictions(ctx context.Context, serviceName string) ([]*baggage.BaggageRestriction, error) {
	br, err := jr.agentSamplingManager.GetBaggageRestrictions(ctx, serviceName)
	if err != nil {
		// Baggage restrictions are not yet implemented - refer to - https://github.com/jaegertracing/jaeger/issues/373
		// As of today, GetBaggageRestrictions() always returns an error.
		// However, we `return nil, nil` here in order to serve a valid `200 OK` response.
		return nil, nil
	}
	return br, nil
}

func (jr *jReceiver) PostSpans(ctx context.Context, r *api_v2.PostSpansRequest) (*api_v2.PostSpansResponse, error) {
	ctx = jr.grpcObsrecv.StartTracesOp(ctx)

	batch := r.GetBatch()
	td, err := jaegertranslator.ProtoToTraces([]*model.Batch{&batch})
	if err != nil {
		jr.grpcObsrecv.EndTracesOp(ctx, protobufFormat, len(batch.Spans), err)
		return nil, err
	}

	err = jr.nextConsumer.ConsumeTraces(ctx, td)
	jr.grpcObsrecv.EndTracesOp(ctx, protobufFormat, len(batch.Spans), err)
	if err != nil {
		return nil, err
	}

	return &api_v2.PostSpansResponse{}, nil
}

func (jr *jReceiver) startAgent(host component.Host) error {
	if jr.config == nil {
		return nil
	}

	if jr.config.AgentBinaryThrift.Endpoint != "" {
		h := &agentHandler{
			nextConsumer: jr.nextConsumer,
			obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
				ReceiverID:             jr.id,
				Transport:              agentTransportBinary,
				ReceiverCreateSettings: jr.settings,
			}),
		}
		processor, err := jr.buildProcessor(jr.config.AgentBinaryThrift.Endpoint, jr.config.AgentBinaryThrift.ServerConfigUDP, apacheThrift.NewTBinaryProtocolFactoryConf(nil), h)
		if err != nil {
			return err
		}
		jr.agentProcessors = append(jr.agentProcessors, processor)
	}

	if jr.config.AgentCompactThrift.Endpoint != "" {
		h := &agentHandler{
			nextConsumer: jr.nextConsumer,
			obsrecv: obsreport.NewReceiver(obsreport.ReceiverSettings{
				ReceiverID:             jr.id,
				Transport:              agentTransportCompact,
				ReceiverCreateSettings: jr.settings,
			}),
		}
		processor, err := jr.buildProcessor(jr.config.AgentCompactThrift.Endpoint, jr.config.AgentCompactThrift.ServerConfigUDP, apacheThrift.NewTCompactProtocolFactoryConf(nil), h)
		if err != nil {
			return err
		}
		jr.agentProcessors = append(jr.agentProcessors, processor)
	}

	jr.goroutines.Add(len(jr.agentProcessors))
	for _, processor := range jr.agentProcessors {
		go func(p processors.Processor) {
			defer jr.goroutines.Done()
			p.Serve()
		}(processor)
	}

	// Start upstream grpc client before serving sampling endpoints over HTTP
	if jr.config.RemoteSamplingClientSettings.Endpoint != "" {
		grpcOpts, err := jr.config.RemoteSamplingClientSettings.ToDialOptions(host, jr.settings.TelemetrySettings)
		if err != nil {
			jr.settings.Logger.Error("Error creating grpc dial options for remote sampling endpoint", zap.Error(err))
			return err
		}
		jr.config.RemoteSamplingClientSettings.SanitizedEndpoint()
		conn, err := grpc.Dial(jr.config.RemoteSamplingClientSettings.Endpoint, grpcOpts...)
		if err != nil {
			jr.settings.Logger.Error("Error creating grpc connection to jaeger remote sampling endpoint", zap.String("endpoint", jr.config.RemoteSamplingClientSettings.Endpoint))
			return err
		}

		jr.agentSamplingManager = jSamplingConfig.NewConfigManager(conn)
	}

	if jr.config.AgentHTTPEndpoint != "" {
		jr.agentServer = httpserver.NewHTTPServer(jr.config.AgentHTTPEndpoint, jr, metrics.NullFactory, jr.settings.Logger)

		jr.goroutines.Add(1)
		go func() {
			defer jr.goroutines.Done()
			if err := jr.agentServer.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) && err != nil {
				host.ReportFatalError(fmt.Errorf("jaeger agent server error: %w", err))
			}
		}()
	}

	return nil
}

func (jr *jReceiver) buildProcessor(address string, cfg ServerConfigUDP, factory apacheThrift.TProtocolFactory, a agent.Agent) (processors.Processor, error) {
	handler := agent.NewAgentProcessor(a)
	transport, err := thriftudp.NewTUDPServerTransport(address)
	if err != nil {
		return nil, err
	}
	if cfg.SocketBufferSize > 0 {
		if err = transport.SetSocketBufferSize(cfg.SocketBufferSize); err != nil {
			return nil, err
		}
	}
	server, err := servers.NewTBufferedServer(transport, cfg.QueueSize, cfg.MaxPacketSize, metrics.NullFactory)
	if err != nil {
		return nil, err
	}
	processor, err := processors.NewThriftProcessor(server, cfg.Workers, metrics.NullFactory, factory, handler, jr.settings.Logger)
	if err != nil {
		return nil, err
	}
	return processor, nil
}

func (jr *jReceiver) decodeThriftHTTPBody(r *http.Request) (*jaeger.Batch, *httpError) {
	bodyBytes, err := ioutil.ReadAll(r.Body)
	r.Body.Close()
	if err != nil {
		return nil, &httpError{
			handler.UnableToReadBodyErrFormat,
			http.StatusInternalServerError,
		}
	}

	contentType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil {
		return nil, &httpError{
			fmt.Sprintf("Cannot parse content type: %v", err),
			http.StatusBadRequest,
		}
	}
	if _, ok := acceptedThriftFormats[contentType]; !ok {
		return nil, &httpError{
			fmt.Sprintf("Unsupported content type: %v", contentType),
			http.StatusBadRequest,
		}
	}

	tdes := apacheThrift.NewTDeserializer()
	batch := &jaeger.Batch{}
	if err = tdes.Read(r.Context(), batch, bodyBytes); err != nil {
		return nil, &httpError{
			fmt.Sprintf(handler.UnableToReadBodyErrFormat, err),
			http.StatusBadRequest,
		}
	}
	return batch, nil
}

func toMetadata(header http.Header) metadata.MD {
	md := metadata.MD{}
	for k, v := range header {
		md.Append(k, v...)
	}
	return md
}

// HandleThriftHTTPBatch implements Jaeger HTTP Thrift handler.
func (jr *jReceiver) HandleThriftHTTPBatch(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()

	md := toMetadata(r.Header)
	ctx = metadata.NewIncomingContext(ctx, md)

	ctx = jr.httpObsrecv.StartTracesOp(ctx)

	batch, hErr := jr.decodeThriftHTTPBody(r)
	if hErr != nil {
		http.Error(w, html.EscapeString(hErr.msg), hErr.statusCode)
		jr.httpObsrecv.EndTracesOp(ctx, thriftFormat, 0, hErr)
		return
	}

	numSpans, err := consumeTraces(ctx, batch, jr.nextConsumer)
	if err != nil {
		http.Error(w, fmt.Sprintf("Cannot submit Jaeger batch: %v", err), http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusAccepted)
	}
	jr.httpObsrecv.EndTracesOp(ctx, thriftFormat, numSpans, err)
}

func (jr *jReceiver) startCollector(host component.Host) error {
	if jr.config == nil {
		return nil
	}

	if jr.config.CollectorHTTPSettings.Endpoint != "" {
		cln, err := jr.config.CollectorHTTPSettings.ToListener()
		if err != nil {
			return fmt.Errorf("failed to bind to Collector address %q: %w",
				jr.config.CollectorHTTPSettings.Endpoint, err)
		}

		nr := mux.NewRouter()
		nr.HandleFunc("/api/traces", jr.HandleThriftHTTPBatch).Methods(http.MethodPost)
		jr.collectorServer, err = jr.config.CollectorHTTPSettings.ToServer(host, jr.settings.TelemetrySettings, nr)
		if err != nil {
			return err
		}

		jr.goroutines.Add(1)
		go func() {
			defer jr.goroutines.Done()
			if errHTTP := jr.collectorServer.Serve(cln); !errors.Is(errHTTP, http.ErrServerClosed) && errHTTP != nil {
				host.ReportFatalError(errHTTP)
			}
		}()
	}

	if jr.config.CollectorGRPCServerSettings.NetAddr.Endpoint != "" {
		opts, err := jr.config.CollectorGRPCServerSettings.ToServerOption(host, jr.settings.TelemetrySettings)
		if err != nil {
			return fmt.Errorf("failed to build the options for the Jaeger gRPC Collector: %w", err)
		}

		jr.grpc = grpc.NewServer(opts...)
		ln, err := jr.config.CollectorGRPCServerSettings.ToListener()
		if err != nil {
			return fmt.Errorf("failed to bind to gRPC address %q: %w", jr.config.CollectorGRPCServerSettings.NetAddr, err)
		}

		api_v2.RegisterCollectorServiceServer(jr.grpc, jr)

		// init and register sampling strategy store
		ss, err := staticStrategyStore.NewStrategyStore(staticStrategyStore.Options{
			StrategiesFile: jr.config.RemoteSamplingStrategyFile,
			ReloadInterval: jr.config.RemoteSamplingStrategyFileReloadInterval,
		}, jr.settings.Logger)
		if err != nil {
			return fmt.Errorf("failed to create collector strategy store: %w", err)
		}
		api_v2.RegisterSamplingManagerServer(jr.grpc, collectorSampling.NewGRPCHandler(ss))

		jr.goroutines.Add(1)
		go func() {
			defer jr.goroutines.Done()
			if errGrpc := jr.grpc.Serve(ln); !errors.Is(errGrpc, grpc.ErrServerStopped) && errGrpc != nil {
				host.ReportFatalError(errGrpc)
			}
		}()
	}

	return nil
}
