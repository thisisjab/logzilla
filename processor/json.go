package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/thisisjab/logzilla/entity"
)

// JsonLogProcessor is a simple JSON log processor. It parses JSON logs and extracts log level, message,
// and timestamp, and any other fields will be considered as metadata.
type JsonLogProcessor struct {
	logLevelFieldName     string
	logMessageFieldName   string
	logTimestampFieldName string
}

// NewJsonLogProcessor creates a new instance of JsonLogProcessor.
func NewJsonLogProcessor(logLevelFieldName, logMessageFieldName, logTimestampFieldName string) *JsonLogProcessor {
	return &JsonLogProcessor{
		logLevelFieldName:     logLevelFieldName,
		logMessageFieldName:   logMessageFieldName,
		logTimestampFieldName: logTimestampFieldName,
	}
}

// Process parses a JSON log record and extracts log level, message, and timestamp, and metadata.
func (p *JsonLogProcessor) Process(record entity.LogRecord) (entity.LogRecord, error) {
	data := make(map[string]any)

	err := json.Unmarshal(record.RawData, &data)
	if err != nil {
		return entity.LogRecord{}, err
	}

	// Parsing time
	val, ok := data[p.logTimestampFieldName]
	timestampValue, isString := val.(string)
	if !ok || !isString || timestampValue == "" {
		return entity.LogRecord{}, errors.New("timestamp field is missing or not a string")
	}

	timestamp, err := time.Parse(time.RFC3339, timestampValue)
	if err != nil {
		return entity.LogRecord{}, fmt.Errorf("cannot parse timestamp: %w", err)
	}
	delete(data, p.logTimestampFieldName)

	// Parsing level
	val, ok = data[p.logLevelFieldName]
	levelValue, isString := val.(string)
	if !ok || !isString {
		return entity.LogRecord{}, errors.New("level field is missing or not a string")
	}
	level := parseLevel(levelValue)
	delete(data, p.logLevelFieldName)

	// Getting message
	val = data[p.logMessageFieldName]
	messageValue, _ := val.(string)
	delete(data, p.logMessageFieldName)

	return entity.LogRecord{
		Level:     level,
		Message:   messageValue,
		Timestamp: timestamp,
		Metadata:  data,
	}, nil
}
