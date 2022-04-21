package output

import (
	"encoding/json"
	"errors"
	"fmt"
	toolsVendors "github.com/devopsext/tools/vendors"
	"net/url"
	"strconv"
	"strings"
	"sync"

	"github.com/VictoriaMetrics/metricsql"
	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"

	"github.com/prometheus/alertmanager/template"
)

type TelegramOutputOptions struct {
	Message             string
	URLSelector         string
	URL                 string
	Timeout             int
	AlertExpression     string
	DisableNotification bool
}

type TelegramOutput struct {
	wg       *sync.WaitGroup
	telegram *toolsVendors.Telegram
	message  *render.TextTemplate
	selector *render.TextTemplate
	grafana  *render.GrafanaRender
	options  TelegramOutputOptions
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	counter  sreCommon.Counter
}

// assume that url is => https://api.telegram.org/botID:botToken/sendMessage?chat_id=%s

func (t *TelegramOutput) getBotID(URL string) string {

	arr := strings.Split(URL, "/bot")
	if len(arr) > 1 {
		arr = strings.Split(arr[1], ":")
		if len(arr) > 0 {
			return arr[0]
		}
	}
	return ""
}

func (t *TelegramOutput) getBotToken(URL string) string {

	arr := strings.Split(URL, "/bot")
	if len(arr) > 1 {
		arr = strings.Split(arr[1], ":")
		if len(arr) > 1 {
			return arr[1]
		}
	}
	return ""
}

func (t *TelegramOutput) getChatID(URL string) string {

	u, err := url.Parse(URL)
	if err != nil {
		return ""
	}
	return u.Query().Get("chat_id")
}

func (t *TelegramOutput) getSendPhotoURL(URL string) string {
	return strings.Replace(URL, "sendMessage", "sendPhoto", -1)
}

func (t *TelegramOutput) sendMessage(spanCtx sreCommon.TracerSpanContext, URL, message string) ([]byte, error) {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	b, err := t.telegram.SendCustom(URL, message, "", "")
	if err != nil {
		t.logger.SpanError(span, err)
		return nil, err
	}

	t.logger.SpanDebug(span, "Response from Telegram => %s", string(b))
	t.counter.Inc(t.getChatID(URL))
	return b, err
}

func (t *TelegramOutput) sendErrorMessage(spanCtx sreCommon.TracerSpanContext, URL, message string, err error) error {

	_, e := t.sendMessage(spanCtx, URL, fmt.Sprintf("%s\n%s", message, err.Error()))
	return e
}

func (t *TelegramOutput) sendImage(spanCtx sreCommon.TracerSpanContext, URL, message, fileName string, image []byte) ([]byte, error) {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	return t.telegram.SendCustomFile(URL, message, fileName, "", image)
}

func (t *TelegramOutput) sendAlertmanagerImage(spanCtx sreCommon.TracerSpanContext, URL, message string, alert template.Alert) ([]byte, error) {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	u, err := url.Parse(alert.GeneratorURL)
	if err != nil {
		return nil, err
	}

	values := u.Query()
	for k, v := range values {
		alert.Labels[k] = strings.Join(v, " ")
	}

	query, ok := alert.Labels[t.options.AlertExpression]
	if !ok {
		err := errors.New("No alert expression")
		return nil, err
	}

	caption := alert.Labels["alertname"]
	unit := alert.Labels["unit"]

	var minutes *int

	if m, err := strconv.Atoi(alert.Labels["minutes"]); err == nil {
		minutes = &m
	}

	expr, err := metricsql.Parse(query)
	if err != nil {
		return nil, err
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

	messageQuery := fmt.Sprintf("%s\n<i>%s</i>", message, query)
	if t.grafana == nil {
		return t.sendMessage(span.GetContext(), URL, messageQuery)
	}

	image, fileName, err := t.grafana.GenerateDashboard(span.GetContext(), caption, metric, operator, value, minutes, unit)
	if err != nil {
		t.sendErrorMessage(span.GetContext(), URL, messageQuery, err)
		return nil, nil
	}

	return t.sendImage(span.GetContext(), t.getSendPhotoURL(URL), messageQuery, fileName, image)
}

func (t *TelegramOutput) sendGlobally(spanCtx sreCommon.TracerSpanContext, event *common.Event, bytes []byte) {

	span := t.tracer.StartChildSpan(spanCtx)
	defer span.Finish()

	var obj interface{}
	if err := json.Unmarshal(bytes, &obj); err != nil {
		t.logger.SpanError(span, err)
		return
	}

	via := event.Via
	if via == nil {
		via = make(map[string]interface{})
	}
	via["Telegram"] = obj

	e := common.Event{
		Time:    event.Time,
		Channel: event.Channel,
		Type:    event.Type,
		Data:    event.Data,
		Via:     via,
	}
	e.SetLogger(t.logger)
	e.SetSpanContext(span.GetContext())

	t.outputs.SendExclude(&e, []common.Output{t})
}

func (t *TelegramOutput) Send(event *common.Event) {

	t.wg.Add(1)
	go func() {
		defer t.wg.Done()

		if t.telegram == nil || t.message == nil {
			t.logger.Debug("No telegram client or message")
			return
		}

		if event == nil {
			t.logger.Debug("Event is empty")
			return
		}

		span := t.tracer.StartFollowSpan(event.GetSpanContext())
		defer span.Finish()

		if event.Data == nil {
			t.logger.SpanError(span, "Event data is empty")
			return
		}

		jsonObject, err := event.JsonObject()
		if err != nil {
			t.logger.SpanError(span, err)
			return
		}

		URLs := t.options.URL
		if t.selector != nil {
			b, err := t.selector.Execute(jsonObject)
			if err != nil {
				t.logger.SpanDebug(span, err)
			} else {
				URLs = b.String()
			}
		}

		if utils.IsEmpty(URLs) {
			t.logger.SpanError(span, "Telegram URLs are not found")
			return
		}

		b, err := t.message.Execute(jsonObject)
		if err != nil {
			t.logger.SpanError(span, err)
			return
		}

		message := b.String()
		if utils.IsEmpty(message) {
			t.logger.SpanDebug(span, "Telegram message is empty")
			return
		}

		t.logger.SpanDebug(span, "Telegram message => %s", message)

		arr := strings.Split(URLs, "\n")
		for _, URL := range arr {

			URL = strings.TrimSpace(URL)
			if utils.IsEmpty(URL) {
				continue
			}

			switch event.Type {
			case "AlertmanagerEvent":
				bytes, err := t.sendAlertmanagerImage(span.GetContext(), URL, message, event.Data.(template.Alert))
				if err != nil {
					t.sendErrorMessage(span.GetContext(), URL, message, err)
				} else {
					t.sendGlobally(span.GetContext(), event, bytes)
				}
			default:
				bytes, err := t.sendMessage(span.GetContext(), URL, message)
				if err == nil {
					t.sendGlobally(span.GetContext(), event, bytes)
				}
			}
		}
	}()
}

func NewTelegramOutput(wg *sync.WaitGroup,
	options TelegramOutputOptions,
	templateOptions render.TextTemplateOptions,
	grafanaRenderOptions render.GrafanaRenderOptions,
	observability *common.Observability,
	outputs *common.Outputs) *TelegramOutput {

	logger := observability.Logs()
	if utils.IsEmpty(options.URL) {
		logger.Debug("Telegram URL is not defined. Skipped")
		return nil
	}

	return &TelegramOutput{
		wg: wg,
		telegram: toolsVendors.NewTelegram(toolsVendors.TelegramOptions{
			URL:                 options.URL,
			Timeout:             options.Timeout,
			DisableNotification: options.DisableNotification,
		}),
		message:  render.NewTextTemplate("telegram-message", options.Message, templateOptions, options, logger),
		selector: render.NewTextTemplate("telegram-selector", options.URLSelector, templateOptions, options, logger),
		grafana:  render.NewGrafanaRender(grafanaRenderOptions, observability),
		options:  options,
		outputs:  outputs,
		logger:   logger,
		tracer:   observability.Traces(),
		counter:  observability.Metrics().Counter("requests", "Count of all telegram requests", []string{"chat_id"}, "telegram", "output"),
	}
}
