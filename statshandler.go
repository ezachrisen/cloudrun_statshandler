package cloudrun_statshandler

import (
	"context"

	"net/http"

	httpprop "contrib.go.opencensus.io/exporter/stackdriver/propagation"
	"go.opencensus.io/trace/propagation"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
)

// NewTracer returns a gRPC stats handler that checks
// for cloudTraceHeader in the metadata of the grpc call,
// parses it, and passes it as binary trace to grpc.
// This is a workaround until https://github.com/cloudendpoints/esp/issues/416 is resolved.
func NewTracer(h stats.Handler) stats.Handler {
	return &traceHandler{h}
}

const (
	cloudTraceHeader = "X-Cloud-Trace-Context"
	binHeader        = "grpc-trace-bin"
)

// traceHandler wrapper for ESP
type traceHandler struct {
	h stats.Handler
}

func (th *traceHandler) TagRPC(ctx context.Context, ti *stats.RPCTagInfo) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)
	if !ok || len(md.Get(cloudTraceHeader)) == 0 || len(md.Get(binHeader)) > 0 {
		return th.h.TagRPC(ctx, ti)
	}
	frmt := &httpprop.HTTPFormat{}
	httpReq, _ := http.NewRequest("GET", "/", nil)
	httpReq.Header.Add(cloudTraceHeader, md.Get(cloudTraceHeader)[0])
	sp, ok := frmt.SpanContextFromRequest(httpReq)
	if !ok {
		return th.h.TagRPC(ctx, ti)
	}
	bin := propagation.Binary(sp)
	md = md.Copy()
	md.Set(binHeader, string(bin))
	ctx = metadata.NewIncomingContext(ctx, md)

	return th.h.TagRPC(ctx, ti)
}

func (th *traceHandler) HandleRPC(ctx context.Context, s stats.RPCStats) {
	th.h.HandleRPC(ctx, s)
}

func (th *traceHandler) TagConn(ctx context.Context, cti *stats.ConnTagInfo) context.Context {
	return th.h.TagConn(ctx, cti)
}

func (th *traceHandler) HandleConn(ctx context.Context, cs stats.ConnStats) {
	th.h.HandleConn(ctx, cs)
}
