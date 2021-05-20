package provider

import (
	"context"
	"fmt"
	"net/http"
	"reflect"
	"strings"
	"time"

	"github.com/devopsext/events/common"
	"github.com/devopsext/utils"
	"github.com/opentracing/opentracing-go"
	opentracingLog "github.com/opentracing/opentracing-go/log"
	"github.com/uber/jaeger-client-go"
	jaegerConfig "github.com/uber/jaeger-client-go/config"
	"github.com/uber/jaeger-lib/metrics/prometheus"
)

type JaegerOptions struct {
	ServiceName         string
	AgentHost           string
	AgentPort           int
	Endpoint            string
	User                string
	Password            string
	BufferFlushInterval int
	QueueSize           int
	Tags                string
}

type JaegerSpanContext struct {
	context opentracing.SpanContext
}

type JaegerSpan struct {
	span        opentracing.Span
	spanContext *JaegerSpanContext
	context     context.Context
	jaeger      *Jaeger
}

type Jaeger struct {
	options      JaegerOptions
	callerOffset int
	tracer       opentracing.Tracer
}

type JaegerLogger struct {
}

func (js JaegerSpan) GetContext() common.TracerSpanContext {
	if js.span == nil {
		return nil
	}

	if js.spanContext != nil {
		return js.spanContext
	}

	js.spanContext = &JaegerSpanContext{
		context: js.span.Context(),
	}
	return js.spanContext
}

func (js JaegerSpan) SetCarrier(object interface{}) common.TracerSpan {

	if js.span == nil {
		return nil
	}

	if reflect.TypeOf(object) != reflect.TypeOf(http.Header{}) {
		//log.Error(errors.New("Other than http.Header is not supported yet"))
		return js
	}

	var h http.Header = object.(http.Header)
	js.jaeger.tracer.Inject(js.span.Context(), opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(h))
	/*if err != nil {
		log.Error(err)
	}*/
	return js
}

func (js JaegerSpan) SetTag(key string, value interface{}) common.TracerSpan {

	if js.span == nil {
		return nil
	}

	js.span.SetTag(key, value)
	return js
}

func (js JaegerSpan) LogFields(fields map[string]interface{}) common.TracerSpan {

	if js.span == nil {
		return nil
	}

	if len(fields) <= 0 {
		return js
	}

	var logFields []opentracingLog.Field

	for k, v := range fields {

		if v != nil {

			var logField opentracingLog.Field
			switch v.(type) {
			case bool:
				logField = opentracingLog.Bool(k, v.(bool))
			case int:
				logField = opentracingLog.Int(k, v.(int))
			case int64:
				logField = opentracingLog.Int64(k, v.(int64))
			case string:
				logField = opentracingLog.String(k, v.(string))
			case float32:
				logField = opentracingLog.Float32(k, v.(float32))
			case float64:
				logField = opentracingLog.Float64(k, v.(float64))
			case error:
				logField = opentracingLog.Error(v.(error))
			}

			logFields = append(logFields, logField)
		}
	}

	if len(logFields) > 0 {
		js.span.LogFields(logFields...)
	}
	return js
}

func (js JaegerSpan) Error(err error) common.TracerSpan {

	if js.span == nil {
		return nil
	}

	_, file, line := common.GetCallerInfo(js.jaeger.callerOffset + 3)

	js.SetTag("error", true)
	js.LogFields(map[string]interface{}{
		"error.message": err.Error(),
		"error.line":    fmt.Sprintf("%s:%d", file, line),
	})
	return js
}

func (js JaegerSpan) Finish() {
	if js.span == nil {
		return
	}
	js.span.Finish()
}

func (j *Jaeger) startSpanFromContext(ctx context.Context, offset int, opts ...opentracing.StartSpanOption) (opentracing.Span, context.Context) {

	operation, file, line := common.GetCallerInfo(offset)

	span, context := opentracing.StartSpanFromContextWithTracer(ctx, j.tracer, operation, opts...)
	if span != nil {
		span.SetTag("caller.line", fmt.Sprintf("%s:%d", file, line))
	}
	return span, context
}

func (j *Jaeger) startChildOfSpan(ctx context.Context, spanContext opentracing.SpanContext) (opentracing.Span, context.Context) {

	var span opentracing.Span
	var context context.Context
	if spanContext != nil {
		span, context = j.startSpanFromContext(ctx, j.callerOffset+5, opentracing.ChildOf(spanContext))
	} else {
		span, context = j.startSpanFromContext(ctx, j.callerOffset+5)
	}
	return span, context
}

func (j *Jaeger) startFollowsFromSpan(ctx context.Context, spanContext opentracing.SpanContext) (opentracing.Span, context.Context) {

	var span opentracing.Span
	var context context.Context
	if spanContext != nil {
		span, context = j.startSpanFromContext(ctx, j.callerOffset+5, opentracing.FollowsFrom(spanContext))
	} else {
		span, context = j.startSpanFromContext(ctx, j.callerOffset+5)
	}
	return span, context
}

func (j *Jaeger) StartSpan() common.TracerSpan {

	s, ctx := j.startSpanFromContext(context.Background(), j.callerOffset+4)
	return JaegerSpan{
		span:    s,
		context: ctx,
		jaeger:  j,
	}
}

func (j *Jaeger) getOpentracingSpanContext(object interface{}) opentracing.SpanContext {

	h, ok := object.(http.Header)
	if ok {
		spanContext, err := j.tracer.Extract(opentracing.HTTPHeaders, opentracing.HTTPHeadersCarrier(h))
		if err != nil {
			//log.Error(err)
			return nil
		}
		return spanContext
	}

	sc, ok := object.(*JaegerSpanContext)
	if ok {
		return sc.context
	}
	return nil
}

func (j *Jaeger) StartChildSpan(object interface{}) common.TracerSpan {

	spanContext := j.getOpentracingSpanContext(object)
	if spanContext == nil {
		return j.StartSpan()
	}

	s, ctx := j.startChildOfSpan(context.Background(), spanContext)
	return JaegerSpan{
		span:    s,
		context: ctx,
		jaeger:  j,
	}
}

func (j *Jaeger) StartFollowSpan(object interface{}) common.TracerSpan {

	spanContext := j.getOpentracingSpanContext(object)
	if spanContext == nil {
		return j.StartSpan()
	}

	s, ctx := j.startFollowsFromSpan(context.Background(), spanContext)
	return JaegerSpan{
		span:    s,
		context: ctx,
		jaeger:  j,
	}
}

func (j *Jaeger) SetCallerOffset(offset int) {
	j.callerOffset = offset
}

func (jl *JaegerLogger) Error(msg string) {
	//	log.Error(msg)
}

func (jl *JaegerLogger) Infof(msg string, args ...interface{}) {

	if utils.IsEmpty(msg) {
		return
	}

	/*	msg = strings.TrimSpace(msg)
		if args != nil {
			log.Info(msg, args...)
		} else {
			log.Info(msg)
		}*/
}

func parseTags(sTags string) []opentracing.Tag {

	env := utils.GetEnvironment()
	pairs := strings.Split(sTags, ",")
	tags := make([]opentracing.Tag, 0)
	for _, p := range pairs {
		kv := strings.SplitN(p, "=", 2)
		k, v := strings.TrimSpace(kv[0]), strings.TrimSpace(kv[1])

		if strings.HasPrefix(v, "${") && strings.HasSuffix(v, "}") {
			ed := strings.SplitN(v[2:len(v)-1], ":", 2)
			e, d := ed[0], ed[1]
			v = env.Get(e, "").(string)
			if v == "" && d != "" {
				v = d
			}
		}

		tag := opentracing.Tag{Key: k, Value: v}
		tags = append(tags, tag)
	}
	return tags
}

func newJaegerTracer(options JaegerOptions) opentracing.Tracer {

	disabled := utils.IsEmpty(options.AgentHost) && utils.IsEmpty(options.Endpoint)
	tags := parseTags(options.Tags)

	cfg := &jaegerConfig.Configuration{

		ServiceName: options.ServiceName,
		Disabled:    disabled,
		Tags:        tags,

		// Use constant sampling to sample every trace
		Sampler: &jaegerConfig.SamplerConfig{
			Type:  jaeger.SamplerTypeConst,
			Param: 1,
		},

		// Enable LogSpan to log every span via configured Logger
		Reporter: &jaegerConfig.ReporterConfig{
			LogSpans:            true,
			User:                options.User,
			Password:            options.Password,
			LocalAgentHostPort:  fmt.Sprintf("%s:%d", options.AgentHost, options.AgentPort),
			CollectorEndpoint:   options.Endpoint,
			BufferFlushInterval: time.Duration(options.BufferFlushInterval) * time.Second,
			QueueSize:           options.QueueSize,
		},
	}

	metricsFactory := prometheus.New()
	tracer, _, err := cfg.NewTracer(jaegerConfig.Metrics(metricsFactory), jaegerConfig.Logger(&JaegerLogger{}))
	if err != nil {
		//log.Error(err)
		return opentracing.NoopTracer{}
	}
	opentracing.SetGlobalTracer(tracer) //?
	return tracer
}

func NewJaeger(options JaegerOptions) *Jaeger {

	return &Jaeger{
		options:      options,
		callerOffset: 0,
		tracer:       newJaegerTracer(options),
	}
}