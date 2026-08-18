package server

import (
	"bytes"
	"encoding/json"
	"errors"
)

func decodeRequest(payload json.RawMessage, message *request) error {
	if err := rejectDuplicateFields(payload); err != nil {
		return protocolError("invalid_request", "request is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(message); err != nil {
		return protocolError("invalid_request", "request is invalid")
	}
	return nil
}

func requestID(payload json.RawMessage) string {
	var envelope struct {
		ID string `json:"id"`
	}
	if json.Unmarshal(payload, &envelope) != nil || !validProtocolText(envelope.ID, 64, true) {
		return ""
	}
	return envelope.ID
}

func validateRequest(message request) error {
	if !validProtocolText(message.ID, 64, true) || !validProtocolText(message.Type, 32, false) {
		return protocolError("invalid_request", "request id or type is invalid")
	}
	return nil
}

func validProtocolText(value string, limit int, colon bool) bool {
	if value == "" || len(value) > limit {
		return false
	}
	for _, character := range []byte(value) {
		if character >= 'a' && character <= 'z' || character >= 'A' && character <= 'Z' || character >= '0' && character <= '9' || character == '-' || character == '_' || character == '.' || colon && character == ':' {
			continue
		}
		return false
	}
	return true
}

func decodePayload(payload json.RawMessage, target any) error {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) == 0 || bytes.Equal(trimmed, []byte("null")) {
		return protocolError("invalid_request", "payload is required")
	}
	if err := rejectDuplicateFields(payload); err != nil {
		return protocolError("invalid_request", "payload is invalid")
	}
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return protocolError("invalid_request", "payload is invalid")
	}
	return nil
}

func rejectDuplicateFields(payload []byte) error {
	decoder := json.NewDecoder(bytes.NewReader(payload))
	return walkJSON(decoder, 0)
}

func walkJSON(decoder *json.Decoder, depth int) error {
	if depth > 32 {
		return errors.New("JSON is too deeply nested")
	}
	token, err := decoder.Token()
	if err != nil {
		return err
	}
	delimiter, ok := token.(json.Delim)
	if !ok {
		return nil
	}
	switch delimiter {
	case '{':
		fields := map[string]struct{}{}
		for decoder.More() {
			token, err := decoder.Token()
			if err != nil {
				return err
			}
			field, ok := token.(string)
			if !ok {
				return errors.New("object field is not a string")
			}
			if _, exists := fields[field]; exists {
				return errors.New("duplicate object field")
			}
			fields[field] = struct{}{}
			if err := walkJSON(decoder, depth+1); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	case '[':
		for decoder.More() {
			if err := walkJSON(decoder, depth+1); err != nil {
				return err
			}
		}
		_, err = decoder.Token()
		return err
	default:
		return errors.New("invalid JSON delimiter")
	}
}

func requireNoPayload(payload json.RawMessage) error {
	trimmed := bytes.TrimSpace(payload)
	if len(trimmed) != 0 && !bytes.Equal(trimmed, []byte("null")) {
		return protocolError("invalid_request", "message does not accept a payload")
	}
	return nil
}
