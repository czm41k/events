package output

import (
	"net"
	"strings"
	"sync"

	"github.com/devopsext/events/common"
	"github.com/devopsext/events/render"
	sreCommon "github.com/devopsext/sre/common"
	"github.com/devopsext/utils"
)

type CollectorOutputOptions struct {
	Address string
	Message string
}

type CollectorOutput struct {
	wg         *sync.WaitGroup
	options    CollectorOutputOptions
	connection *net.UDPConn
	message    *render.TextTemplate
	tracer     sreCommon.Tracer
	logger     sreCommon.Logger
	requests   sreCommon.Counter
	errors     sreCommon.Counter
}

func (c *CollectorOutput) Name() string {
	return "Collector"
}

func (c *CollectorOutput) Send(event *common.Event) {

	c.wg.Add(1)
	go func() {
		defer c.wg.Done()

		if c.connection == nil || c.message == nil {
			c.logger.Debug("No connection or message")
			return
		}

		if event == nil {
			c.logger.Debug("Event is empty")
			return
		}

		span := c.tracer.StartFollowSpan(event.GetSpanContext())
		defer span.Finish()

		b, err := c.message.Execute(event)
		if err != nil {
			c.logger.SpanError(span, err)
			return
		}

		message := strings.TrimSpace(b.String())
		if utils.IsEmpty(message) {
			c.logger.SpanDebug(span, "Collector message is empty")
			return
		}

		c.requests.Inc(c.options.Address)
		c.logger.SpanDebug(span, "Collector message => %s", message)

		_, err = c.connection.Write(b.Bytes())
		if err != nil {
			c.errors.Inc(c.options.Address)
			c.logger.SpanError(span, err)
		}
	}()
}

func makeCollectorOutputConnection(address string, logger sreCommon.Logger) *net.UDPConn {

	if utils.IsEmpty(address) {
		logger.Debug("Collector address is not defined. Skipped.")
		return nil
	}

	localAddr, err := net.ResolveUDPAddr("udp", "127.0.0.1:0")
	if err != nil {
		logger.Error(err)
		return nil
	}

	serverAddr, err := net.ResolveUDPAddr("udp", address)
	if err != nil {
		logger.Error(err)
		return nil
	}

	connection, err := net.DialUDP("udp", localAddr, serverAddr)
	if err != nil {
		logger.Error(err)
		return nil
	}

	return connection
}

func NewCollectorOutput(wg *sync.WaitGroup, options CollectorOutputOptions,
	templateOptions render.TextTemplateOptions, observability *common.Observability) *CollectorOutput {

	logger := observability.Logs()
	connection := makeCollectorOutputConnection(options.Address, logger)
	if connection == nil {
		return nil
	}

	return &CollectorOutput{
		wg:         wg,
		options:    options,
		message:    render.NewTextTemplate("collector-message", options.Message, templateOptions, options, logger),
		connection: connection,
		tracer:     observability.Traces(),
		logger:     logger,
		requests:   observability.Metrics().Counter("requests", "Count of all collector requests", []string{"address"}, "collector", "output"),
		errors:     observability.Metrics().Counter("errors", "Count of all collector errors", []string{"address"}, "collector", "output"),
	}
}
