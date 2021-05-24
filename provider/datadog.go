package provider

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"reflect"
	"strconv"

	"github.com/devopsext/events/common"
	"github.com/devopsext/utils"
	"github.com/sirupsen/logrus"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace"
	"gopkg.in/DataDog/dd-trace-go.v1/ddtrace/tracer"
)

type DataDogOptions struct {
	TracerHost  string
	TracerPort  int
	ServiceName string
	LoggerHost  string
	LoggerPort  int
}

type DataDogTracerSpanContext struct {
	context ddtrace.SpanContext
}

type DataDogTracerSpan struct {
	span        ddtrace.Span
	spanContext *DataDogTracerSpanContext
	context     context.Context
	datadog     *DataDog
}

type DataDogTracerLogger struct {
	logger common.Logger
}

type DataDogUDPLogger struct {
	connection *net.UDPConn
	stdout     *Stdout
	log        *logrus.Logger
}

type DataDog struct {
	options      DataDogOptions
	logger       common.Logger
	udpLogger    *DataDogUDPLogger
	callerOffset int
}

func (dds DataDogTracerSpan) GetContext() common.TracerSpanContext {
	if dds.span == nil {
		return nil
	}

	if dds.spanContext != nil {
		return dds.spanContext
	}

	dds.spanContext = &DataDogTracerSpanContext{
		context: dds.span.Context(),
	}
	return dds.spanContext
}

func (dds DataDogTracerSpan) SetCarrier(object interface{}) common.TracerSpan {

	if dds.span == nil {
		return nil
	}

	if reflect.TypeOf(object) != reflect.TypeOf(http.Header{}) {
		dds.datadog.logger.Error(errors.New("Other than http.Header is not supported yet"))
		return dds
	}

	var h http.Header = object.(http.Header)
	err := tracer.Inject(dds.span.Context(), tracer.HTTPHeadersCarrier(h))
	if err != nil {
		dds.datadog.logger.Error(err)
	}
	return dds
}

func (dds DataDogTracerSpan) SetTag(key string, value interface{}) common.TracerSpan {

	if dds.span == nil {
		return nil
	}
	dds.span.SetTag(key, value)
	return dds
}

func (dds DataDogTracerSpan) Error(err error) common.TracerSpan {

	if dds.span == nil {
		return nil
	}

	dds.SetTag("error", true)
	return dds
}

func (dds DataDogTracerSpan) Finish() {
	if dds.span == nil {
		return
	}
	dds.span.Finish()
}

func (ddtl *DataDogTracerLogger) Log(msg string) {
	ddtl.logger.Info(msg)
}

func (dd *DataDog) startSpanFromContext(ctx context.Context, offset int, opts ...tracer.StartSpanOption) (ddtrace.Span, context.Context) {

	operation, file, line := common.GetCallerInfo(offset)

	span, context := tracer.StartSpanFromContext(ctx, operation, opts...)
	if span != nil {
		span.SetTag("caller.line", fmt.Sprintf("%s:%d", file, line))
	}
	return span, context
}

func (dd *DataDog) startChildOfSpan(ctx context.Context, spanContext ddtrace.SpanContext) (ddtrace.Span, context.Context) {

	var span ddtrace.Span
	var context context.Context
	if spanContext != nil {
		span, context = dd.startSpanFromContext(ctx, dd.callerOffset+5, tracer.ChildOf(spanContext))
	} else {
		span, context = dd.startSpanFromContext(ctx, dd.callerOffset+5)
	}
	return span, context
}

func (dd *DataDog) StartSpan() common.TracerSpan {

	s, ctx := dd.startSpanFromContext(context.Background(), dd.callerOffset+4)
	return DataDogTracerSpan{
		span:    s,
		context: ctx,
		datadog: dd,
	}
}

func (dd *DataDog) getOpentracingSpanContext(object interface{}) ddtrace.SpanContext {

	h, ok := object.(http.Header)
	if ok {
		spanContext, err := tracer.Extract(tracer.HTTPHeadersCarrier(h))
		if err != nil {
			dd.logger.Error(err)
			return nil
		}
		return spanContext
	}

	ddsc, ok := object.(*DataDogTracerSpanContext)
	if ok {
		return ddsc.context
	}
	return nil
}

func (dd *DataDog) StartChildSpan(object interface{}) common.TracerSpan {

	spanContext := dd.getOpentracingSpanContext(object)
	if spanContext == nil {
		return dd.StartSpan()
	}

	s, ctx := dd.startChildOfSpan(context.Background(), spanContext)
	return DataDogTracerSpan{
		span:    s,
		context: ctx,
		datadog: dd,
	}
}

func (dd *DataDog) StartFollowSpan(object interface{}) common.TracerSpan {
	spanContext := dd.getOpentracingSpanContext(object)
	if spanContext == nil {
		return dd.StartSpan()
	}

	s, ctx := dd.startChildOfSpan(context.Background(), spanContext)
	return DataDogTracerSpan{
		span:    s,
		context: ctx,
		datadog: dd,
	}
}

func (dd *DataDog) SetCallerOffset(offset int) {
	dd.callerOffset = offset
}

func (dd *DataDog) Info(obj interface{}, args ...interface{}) {

	if dd.udpLogger == nil {
		return
	}

	if exists, fields, message := dd.udpLogger.exists(obj, args...); exists {
		dd.udpLogger.log.WithFields(fields).Infoln(message)
	}
}

func (dd *DataDog) Warn(obj interface{}, args ...interface{}) {

	if dd.udpLogger == nil {
		return
	}

	if exists, fields, message := dd.udpLogger.exists(obj, args...); exists {
		dd.udpLogger.log.WithFields(fields).Warnln(message)
	}
}

func (dd *DataDog) Error(obj interface{}, args ...interface{}) {

	if dd.udpLogger == nil {
		return
	}

	if exists, fields, message := dd.udpLogger.exists(obj, args...); exists {
		dd.udpLogger.log.WithFields(fields).Errorln(message)
	}
}

func (dd *DataDog) Debug(obj interface{}, args ...interface{}) {

	if dd.udpLogger == nil {
		return
	}

	if exists, fields, message := dd.udpLogger.exists(obj, args...); exists {
		dd.udpLogger.log.WithFields(fields).Debugln(message)
	}
}

func (dd *DataDog) Panic(obj interface{}, args ...interface{}) {

	if dd.udpLogger == nil {
		return
	}

	if exists, fields, message := dd.udpLogger.exists(obj, args...); exists {
		dd.udpLogger.log.WithFields(fields).Panicln(message)
	}
}

func (ddul *DataDogUDPLogger) exists(obj interface{}, args ...interface{}) (bool, logrus.Fields, string) {

	message := ""

	switch v := obj.(type) {
	case error:
		message = v.Error()
	case string:
		message = v
	default:
		message = "not implemented"
	}

	if len(args) > 0 {
		message = fmt.Sprintf(message, args...)
	}

	if utils.IsEmpty(message) {
		return false, nil, ""
	}

	function, file, line := common.GetCallerInfo(4)
	fields := logrus.Fields{
		"file": fmt.Sprintf("%s:%d", file, line),
		"func": function,
	}
	return true, fields, message
}

func newDataDogUDPLogger(options DataDogOptions, stdout *Stdout) *DataDogUDPLogger {

	if utils.IsEmpty(options.LoggerHost) {
		stdout.Debug("DataDog logger is disabled.")
		return nil
	}

	address := fmt.Sprintf("%s:%d", options.LoggerHost, options.LoggerPort)
	serverAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		stdout.Error(err)
		return nil
	}

	connection, err := net.DialUDP("udp", nil, serverAddr)
	if err != nil {
		stdout.Error(err)
		return nil
	}

	log := logrus.New()
	log.SetFormatter(&logrus.JSONFormatter{})
	log.SetOutput(connection)

	return &DataDogUDPLogger{
		connection: connection,
		stdout:     stdout,
		log:        log,
	}
}

func startDataDogTracer(options DataDogOptions, logger common.Logger, stdout *Stdout) {

	disabled := utils.IsEmpty(options.TracerHost)
	if disabled {
		stdout.Debug("DataDog tracer is disabled.")
		return
	}

	addr := net.JoinHostPort(
		options.TracerHost,
		strconv.Itoa(options.TracerPort),
	)
	tracer.Start(tracer.WithAgentAddr(addr),
		tracer.WithServiceName(options.ServiceName),
		tracer.WithLogger(&DataDogTracerLogger{logger: logger}))
}

func NewDataDog(options DataDogOptions, logger common.Logger, stdout *Stdout) *DataDog {

	startDataDogTracer(options, logger, stdout)

	return &DataDog{
		options:      options,
		callerOffset: 0,
		logger:       logger,
		udpLogger:    newDataDogUDPLogger(options, stdout),
	}
}
