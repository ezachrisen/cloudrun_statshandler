package cloudrun_statshandler

import (
	"context"

	log "github.com/sirupsen/logrus"

	"net/http"

	httpprop "contrib.go.opencensus.io/exporter/stackdriver/propagation"
	"go.opencensus.io/trace/propagation"
	"google.golang.org/grpc/metadata"
	"google.golang.org/grpc/stats"
)

// NewTracer returns a gRPC stats handler that checks
// for httpHeader in the metadata of the grpc call,
// parses it, and passes it as binary trace to grpc.
// This is a workaround until https://github.com/cloudendpoints/esp/issues/416 is resolved.
func NewTracer(h stats.Handler) stats.Handler {
	return &traceHandler{h}
}

const (
	httpHeader = "X-Cloud-Trace-Context"
	binHeader  = "grpc-trace-bin"
)

// traceHandler wrapper for ESP
type traceHandler struct {
	h stats.Handler
}

func (th *traceHandler) TagRPC(ctx context.Context, ti *stats.RPCTagInfo) context.Context {
	md, ok := metadata.FromIncomingContext(ctx)

	cloudTrace := md.Get(httpHeader)
	log.WithContext(ctx).Println("Clod=ud trace", cloudTrace)

	if !ok || len(md.Get(httpHeader)) == 0 || len(md.Get(binHeader)) > 0 {
		return th.h.TagRPC(ctx, ti)
	}
	frmt := &httpprop.HTTPFormat{}
	httpReq, _ := http.NewRequest("GET", "/", nil)
	httpReq.Header.Add(httpHeader, md.Get(httpHeader)[0])
	sp, ok := frmt.SpanContextFromRequest(httpReq)
	if !ok {
		log.WithContext(ctx).Println("Returning2")

		return th.h.TagRPC(ctx, ti)
	}
	bin := propagation.Binary(sp)
	md = md.Copy()
	log.WithContext(ctx).Println("Setting binheader")
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
