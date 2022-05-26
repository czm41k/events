package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"github.com/buger/jsonparser"
	"github.com/devopsext/events/common"
	sreCommon "github.com/devopsext/sre/common"
	"reflect"
	"strings"
	"time"
)

type VCenterProcessor struct {
	outputs  *common.Outputs
	tracer   sreCommon.Tracer
	logger   sreCommon.Logger
	requests sreCommon.Counter
	errors   sreCommon.Counter
}

type VCenterResponse struct {
	Message string
}

func VCenterProcessorType() string {
	return "vcenter"
}

func (p *VCenterProcessor) EventType() string {
	return common.AsEventType(VCenterProcessorType())
}

func (p *VCenterProcessor) prepareStatus(status string) string {
	return strings.Title(strings.ToLower(status))
}

func (p *VCenterProcessor) send(span sreCommon.TracerSpan, channel string, data string) {

	for _, alert := range data {

		e := &common.Event{
			Channel: channel,
			Type:    p.EventType(),
			Data:    alert,
		}
		//e.SetTime(alert..UTC())
		if span != nil {
			e.SetSpanContext(span.GetContext())
			e.SetLogger(p.logger)
		}
		p.outputs.Send(e)
	}
}

var ErrorEventNotImpemented = errors.New("vcenter event not implemented yet")

type vcenterEvent struct {
	Subject            string
	CreatedTime        string
	FullUsername       string
	Message            string
	VmName             string
	OrigClusterName    string
	OrigDatacenterName string
	OrigLocation       string
	OrigESXiHostName   string
	OrigDatastoreName  string
	DestClusterName    string
	DestDatacenterName string
	DestLocation       string
	DestESXiHostName   string
	DestDatastoreName  string
}

func (vce *vcenterEvent) ParseDrsVmMigratedEvent(jsonByte []byte) error {
	//	TODO
	//var err error
	//if err != nil && err != jsonparser.KeyPathNotFoundError {
	//	return err
	//}
	vce.CreatedTime, _ = jsonparser.GetString(jsonByte, "data", "CreatedTime")
	vce.Message, _ = jsonparser.GetString(jsonByte, "data", "FullFormattedMessage")
	vce.VmName, _ = jsonparser.GetString(jsonByte, "data", "Vm", "Name")
	vce.OrigClusterName, _ = jsonparser.GetString(jsonByte, "data", "SourceDatacenter", "Name")
	vce.OrigDatacenterName, _ = jsonparser.GetString(jsonByte, "data", "SourceDatacenter", "Datacenter", "Value")
	vce.OrigESXiHostName, _ = jsonparser.GetString(jsonByte, "data", "SourceHost", "Name")
	vce.OrigDatastoreName, _ = jsonparser.GetString(jsonByte, "data", "SourceDatastore", "Name")
	vce.DestLocation, _ = jsonparser.GetString(jsonByte, "data", "ComputeResource", "Name")
	vce.DestClusterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Name")
	vce.DestDatacenterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Datacenter", "Value")
	vce.DestESXiHostName, _ = jsonparser.GetString(jsonByte, "data", "DestESXiHostName", "Name")
	vce.DestDatastoreName, _ = jsonparser.GetString(jsonByte, "data", "Ds", "Name")

	return nil
}

func (vce *vcenterEvent) ParseVmPoweredOffEvent(jsonByte []byte) error {
	//	TODO
	//var err error
	//if err != nil && err != jsonparser.KeyPathNotFoundError {
	//	return err
	//}
	vce.CreatedTime, _ = jsonparser.GetString(jsonByte, "data", "CreatedTime")
	vce.FullUsername, _ = jsonparser.GetString(jsonByte, "data", "UserName")
	vce.Message, _ = jsonparser.GetString(jsonByte, "data", "FullFormattedMessage")
	vce.VmName, _ = jsonparser.GetString(jsonByte, "data", "Vm", "Name")
	vce.OrigClusterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Name")
	vce.OrigDatacenterName, _ = jsonparser.GetString(jsonByte, "data", "Datacenter", "Datacenter", "Value")
	vce.OrigLocation, _ = jsonparser.GetString(jsonByte, "data", "ComputeResource", "Name")
	vce.OrigESXiHostName, _ = jsonparser.GetString(jsonByte, "data", "Host", "Name")

	return nil
}

func (vce *vcenterEvent) parse(jsonString string) error {
	parse := reflect.ValueOf(vce).MethodByName("Parse" + vce.Subject)
	if !parse.IsValid() {
		err := fmt.Errorf("%w", ErrorEventNotImpemented)
		return fmt.Errorf("%s %w", vce.Subject, err)
	}

	values := parse.Call([]reflect.Value{reflect.ValueOf([]byte(jsonString))})
	v := values[0].Interface()
	if v == nil {
		return nil
	} else {
		return v.(error)
	}
}

func (p *VCenterProcessor) HandleEvent(e *common.Event) error {

	//	p.requests.Inc(e.Channel)
	if e == nil {
		p.logger.Debug("Event is not defined")
		return nil
	}

	jsonString := e.Data.(string)
	subject, err := jsonparser.GetString([]byte(jsonString), "subject")
	if err != nil {
		return err
	}
	vce := vcenterEvent{
		Subject: subject,
	}

	err = vce.parse(jsonString)
	b, err := json.Marshal(vce)
	if err != nil {
		p.logger.Debug(err)
		return err
	}

	curevent := &common.Event{
		Data:    string(b),
		Channel: e.Channel,
		Type:    "vcenterEvent",
	}
	curevent.SetLogger(p.logger)
	eventTime, _ := time.Parse(time.RFC3339Nano, vce.CreatedTime)

	curevent.SetTime(eventTime)
	p.outputs.Send(curevent)

	return nil
}
func NewVCenterProcessor(outputs *common.Outputs, observability *common.Observability) *VCenterProcessor {

	return &VCenterProcessor{
		outputs:  outputs,
		logger:   observability.Logs(),
		tracer:   observability.Traces(),
		requests: observability.Metrics().Counter("requests", "Count of all google processor requests", []string{"channel"}, "aws", "processor"),
		errors:   observability.Metrics().Counter("errors", "Count of all google processor errors", []string{"channel"}, "aws", "processor"),
	}
}
