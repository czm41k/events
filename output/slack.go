package output

import (
	"bytes"
	"errors"
	"io/ioutil"
	"mime/multipart"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	"github.com/prometheus/alertmanager/template"
	"github.com/prometheus/client_golang/prometheus"
)

var slackOutputCount = prometheus.NewCounterVec(prometheus.CounterOpts{
	Name: "events_slack_output_count",
	Help: "Count of all slack outputs",
}, []string{})

type SlackOutputOptions struct {
	MessageTemplate  string
	SelectorTemplate string
	URL              string
	Timeout          int
	AlertExpression  string
}

type SlackOutput struct {
	wg       *sync.WaitGroup
	client   *http.Client
	message  *render.TextTemplate
	selector *render.TextTemplate
	grafana  *render.Grafana
	options  SlackOutputOptions
	tracer   common.Tracer
	logger   common.Logger
}

func (s *SlackOutput) post(spanCtx common.TracerSpanContext, URL, contentType string, body bytes.Buffer, message string) error {

	span := s.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	s.logger.Debug("Post to Slack (%s) => %s", URL, message)
	reader := bytes.NewReader(body.Bytes())

	req, err := http.NewRequest("POST", URL, reader)
	if err != nil {
		span.Error(err)
		return err
	}

	req.Header.Set("Content-Type", contentType)

	resp, err := s.client.Do(req)
	if err != nil {
		span.Error(err)
		return err
	}

	defer resp.Body.Close()

	b, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		span.Error(err)
		return err
	}

	//slackOutputCount.WithLabelValues(t.getBotID(URL)).Inc()

	s.logger.Debug("Response from Slack => %s", string(b))

	return nil
}

func (s *SlackOutput) sendMessage(spanCtx common.TracerSpanContext, URL, message, title, content string) error {

	span := s.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	defer func() {
		if err := w.Close(); err != nil {
			s.logger.Warn("Failed to close writer")
		}
	}()

	if err := w.WriteField("initial_comment", message); err != nil {
		span.Error(err)
		return err
	}

	if err := w.WriteField("title", title); err != nil {
		span.Error(err)
		return err
	}

	if err := w.WriteField("content", content); err != nil {
		span.Error(err)
		return err
	}

	if err := w.Close(); err != nil {
		span.Error(err)
		return err
	}

	return s.post(span.GetContext(), URL, w.FormDataContentType(), body, message)
}

func (s *SlackOutput) sendErrorMessage(spanCtx common.TracerSpanContext, URL, message, title string, err error) error {

	return s.sendMessage(spanCtx, URL, message, title, err.Error())
}

func (s *SlackOutput) sendPhoto(spanCtx common.TracerSpanContext, URL, message, fileName, title string, photo []byte) error {

	span := s.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	defer func() {
		if err := w.Close(); err != nil {
			s.logger.Warn("Failed to close writer")
		}
	}()

	if err := w.WriteField("initial_comment", message); err != nil {
		span.Error(err)
		return err
	}

	if err := w.WriteField("title", title); err != nil {
		span.Error(err)
		return err
	}

	fw, err := w.CreateFormFile("file", fileName)
	if err != nil {
		span.Error(err)
		return err
	}

	if _, err := fw.Write(photo); err != nil {
		span.Error(err)
		return err
	}

	if err := w.Close(); err != nil {
		span.Error(err)
		return err
	}

	return s.post(span.GetContext(), URL, w.FormDataContentType(), body, message)
}

func (s *SlackOutput) sendAlertmanagerImage(spanCtx common.TracerSpanContext, URL, message string, alert template.Alert) error {

	span := s.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	u, err := url.Parse(alert.GeneratorURL)
	if err != nil {
		span.Error(err)
		return err
	}

	values := u.Query()
	for k, v := range values {
		alert.Labels[k] = strings.Join(v, " ")
	}

	query, ok := alert.Labels[s.options.AlertExpression]
	if !ok {
		err := errors.New("No alert expression")
		span.Error(err)
		return err
	}

	caption := alert.Labels["alertname"]
	unit := alert.Labels["unit"]

	var minutes *int

	if m, err := strconv.Atoi(alert.Labels["minutes"]); err == nil {
		minutes = &m
	}

	expr, err := metricsql.Parse(query)
	if err != nil {
		span.Error(err)
		return err
	}

	metric := query
	operator := ""
	var value *float64

	binExpr, ok := expr.(*metricsql.BinaryOpExpr)
	if binExpr != nil && ok {
		metric = string(binExpr.Left.AppendString(nil))
		operator = binExpr.Op

		if v, err := strconv.ParseFloat(string(binExpr.Right.AppendString(nil)), 64); err == nil {
			value = &v
		}
	}

	if s.grafana == nil {
		return s.sendMessage(span.GetContext(), URL, message, query, "No image")
	}

	photo, fileName, err := s.grafana.GenerateDashboard(span.GetContext(), caption, metric, operator, value, minutes, unit)
	if err != nil {
		s.logger.Error(err)
		s.sendErrorMessage(span.GetContext(), URL, message, query, err)
		return nil
	}

	return s.sendPhoto(span.GetContext(), URL, message, fileName, query, photo)
}

func (s *SlackOutput) Send(event *common.Event) {

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()

		if s.client == nil || s.message == nil {
			s.logger.Error(errors.New("No client or message"))
			return
		}

		if event == nil {
			s.logger.Error(errors.New("Event is empty"))
			return
		}

		span := s.tracer.StartFollowSpan(event.GetSpanContext())
		defer span.Finish()

		if event.Data == nil {
			err := errors.New("Event data is empty")
			s.logger.Error(err)
			span.Error(err)
			return
		}

		jsonObject, err := event.JsonObject()
		if err != nil {
			s.logger.Error(err)
			span.Error(err)
			return
		}

		URLs := s.options.URL
		if s.selector != nil {

			b, err := s.selector.Execute(jsonObject)
			if err != nil {
				s.logger.Error(err)
			} else {
				URLs = b.String()
			}
		}

		if common.IsEmpty(URLs) {
			err := errors.New("Slack URLs are not found")
			s.logger.Error(err)
			span.Error(err)
			return
		}

		b, err := s.message.Execute(jsonObject)
		if err != nil {
			s.logger.Error(err)
			span.Error(err)
			return
		}

		message := b.String()
		if common.IsEmpty(message) {
			s.logger.Debug("Slack message is empty")
			return
		}

		arr := strings.Split(URLs, "\n")

		for _, URL := range arr {

			URL = strings.TrimSpace(URL)
			if common.IsEmpty(URL) {
				continue
			}

			switch event.Type {
			case "K8sEvent":
				s.sendMessage(span.GetContext(), URL, message, "No title", "No image")
			case "AlertmanagerEvent":

				if err := s.sendAlertmanagerImage(span.GetContext(), URL, message, event.Data.(template.Alert)); err != nil {
					s.logger.Error(err)
					s.sendErrorMessage(span.GetContext(), URL, message, "No title", err)
				}
			}
		}
	}()
}

func NewSlackOutput(wg *sync.WaitGroup,
	options SlackOutputOptions,
	templateOptions render.TextTemplateOptions,
	grafanaOptions render.GrafanaOptions,
	logger common.Logger,
	tracer common.Tracer) *SlackOutput {

	if common.IsEmpty(options.URL) {
		logger.Debug("Slack URL is not defined. Skipped")
		return nil
	}

	return &SlackOutput{
		wg:       wg,
		client:   common.MakeHttpClient(options.Timeout),
		message:  render.NewTextTemplate("slack-message", options.MessageTemplate, templateOptions, options, logger),
		selector: render.NewTextTemplate("slack-selector", options.SelectorTemplate, templateOptions, options, logger),
		grafana:  render.NewGrafana(grafanaOptions, logger, tracer),
		options:  options,
		logger:   logger,
		tracer:   tracer,
	}
}

func init() {
	prometheus.Register(slackOutputCount)
}
