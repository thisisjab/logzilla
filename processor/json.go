package processor

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/thisisjab/logzilla/entity"
)

type JsonLogProcessorConfig struct {
	Name                  string `yaml:"-"`
	LogLevelFieldName     string `yaml:"level_field"`
	LogMessageFieldName   string `yaml:"message_field"`
	LogTimestampFieldName string `yaml:"timestamp_field"`
}

// JsonLogProcessor is a simple JSON log processor. It parses JSON logs and extracts log level, message,
// and timestamp, and any other fields will be considered as metadata.
type JsonLogProcessor struct {
	cfg JsonLogProcessorConfig
}

// NewJsonLogProcessor creates a new instance of JsonLogProcessor.
func NewJsonLogProcessor(cfg JsonLogProcessorConfig) (*JsonLogProcessor, error) {
	return &JsonLogProcessor{cfg: cfg}, nil
}

func (p *JsonLogProcessor) Name() string {
	return p.cfg.Name
}

// Process parses a JSON log record and extracts log level, message, and timestamp, and metadata.
func (p *JsonLogProcessor) Process(record entity.LogRecord) (entity.LogRecord, error) {
	data := make(map[string]any)

	err := json.Unmarshal(record.RawData, &data)
	if err != nil {
		return entity.LogRecord{}, err
	}

	// Parsing time
	val, ok := data[p.cfg.LogTimestampFieldName]
	timestampValue, isString := val.(string)
	if !ok || !isString || timestampValue == "" {
		return entity.LogRecord{}, errors.New("timestamp field is missing or not a string")
	}

	timestamp, err := time.Parse(time.RFC3339, timestampValue)
	if err != nil {
		return entity.LogRecord{}, fmt.Errorf("cannot parse timestamp: %w", err)
	}
	delete(data, p.cfg.LogTimestampFieldName)

	// Parsing level
	val, ok = data[p.cfg.LogLevelFieldName]
	levelValue, isString := val.(string)
	if !ok || !isString {
		return entity.LogRecord{}, errors.New("level field is missing or not a string")
	}
	level := parseLevel(levelValue)
	delete(data, p.cfg.LogLevelFieldName)

	// Getting message
	val = data[p.cfg.LogMessageFieldName]
	messageValue, _ := val.(string)
	delete(data, p.cfg.LogMessageFieldName)

	return entity.LogRecord{
		Level:     level,
		Message:   messageValue,
		Timestamp: timestamp,
		Metadata:  data,
	}, nil
}
