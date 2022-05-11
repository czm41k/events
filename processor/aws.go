package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"net/http"
	"strings"
	"time"

	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
)

type AWSProcessor struct {
	outputs *common.Outputs
	tracer  sreCommon.Tracer
	logger  sreCommon.Logger
	counter sreCommon.Counter
}

type AWSRequest struct {
	Version    string      `json:"version"`
	Source     string      `json:"source"`
	Account    string      `json:"account"`
	Time       time.Time   `json:"time"`
	Region     string      `json:"region"`
	DetailType string      `json:"detail-type"`
	Detail     interface{} `json:"detail"`
}

type AWSResponse struct {
	Message string
}

func AWSProcessorType() string {
	return "AWS"
}

func (p *AWSProcessor) EventType() string {
	return common.AsEventType(AWSProcessorType())
}

func (p *AWSProcessor) send(span sreCommon.TracerSpan, channel string, o interface{}, t *time.Time) {

	e := &common.Event{
		Channel: channel,
		Type:    p.EventType(),
		Data:    o,
	}
	if t != nil && (*t).UnixNano() > 0 {
		e.SetTime((*t).UTC())
	} else {
		e.SetTime(time.Now().UTC())
	}
	if span != nil {
		e.SetSpanContext(span.GetContext())
		e.SetLogger(p.logger)
	}
	p.outputs.Send(e)
	p.counter.Inc(e.Channel)
}

func (p *AWSProcessor) HandleEvent(e *common.Event) {

	if e == nil {
		p.logger.Debug("Event is not defined")
		return
	}

	p.outputs.Send(e)
	p.counter.Inc(e.Channel)
}

func (p *AWSProcessor) HandleHttpRequest(w http.ResponseWriter, r *http.Request) {

	span := p.tracer.StartChildSpan(r.Header)
	defer span.Finish()

	var body []byte
	if r.Body != nil {
		if data, err := ioutil.ReadAll(r.Body); err == nil {
			body = data
		}
	}

	if len(body) == 0 {
		err := errors.New("empty body")
		p.logger.SpanError(span, err)
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}

	p.logger.SpanDebug(span, "Body => %s", body)

	contentType := r.Header.Get("Content-Type")
	if contentType != "application/json" {
		p.logger.SpanError(span, "Content-Type=%s, expect application/json", contentType)
		http.Error(w, "invalid Content-Type, expect application/json", http.StatusUnsupportedMediaType)
		return
	}

	var request AWSRequest
	if err := json.Unmarshal(body, &request); err != nil {
		p.logger.SpanError(span, err)
		http.Error(w, "Error unmarshaling message", http.StatusInternalServerError)
		return
	}

	channel := strings.TrimLeft(r.URL.Path, "/")
	if request.Time.UnixMilli() > 0 {
		t := request.Time
		p.send(span, channel, request, &t)
	} else {
		p.send(span, channel, request, nil)
	}

	response := &AWSResponse{
		Message: "OK",
	}

	resp, err := json.Marshal(response)
	if err != nil {
		p.logger.SpanError(span, "Can't encode response: %v", err)
		http.Error(w, fmt.Sprintf("could not encode response: %v", err), http.StatusInternalServerError)
	}

	if _, err := w.Write(resp); err != nil {
		p.logger.SpanError(span, "Can't write response: %v", err)
		http.Error(w, fmt.Sprintf("could not write response: %v", err), http.StatusInternalServerError)
	}
}

func NewAWSProcessor(outputs *common.Outputs, observability *common.Observability) *AWSProcessor {

	return &AWSProcessor{
		outputs: outputs,
		logger:  observability.Logs(),
		tracer:  observability.Traces(),
		counter: observability.Metrics().Counter("requests", "Count of all google processor requests", []string{"channel"}, "aws", "processor"),
	}
}