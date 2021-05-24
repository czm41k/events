package common

import (
	"encoding/json"
)

type Event struct {
	Time        string      `json:"time"`
	TimeNano    int64       `json:"timeNano"`
	Channel     string      `json:"channel"`
	Type        string      `json:"type"`
	Data        interface{} `json:"data"`
	spanContext TracerSpanContext
	logger      Logger
}

func (e *Event) JsonObject() (interface{}, error) {

	bytes, err := json.Marshal(e)
	if err != nil {
		if e.logger != nil {
			e.logger.Error(err)
		}
		return "", err
	}

	var object interface{}

	if err := json.Unmarshal(bytes, &object); err != nil {
		if e.logger != nil {
			e.logger.Error(err)
		}
		return "", err
	}

	return object, nil
}

func (e *Event) SetSpanContext(context TracerSpanContext) {
	e.spanContext = context
}

func (e *Event) GetSpanContext() TracerSpanContext {
	return e.spanContext
}

func (e *Event) SetLogger(logger Logger) {
	e.logger = logger
}
