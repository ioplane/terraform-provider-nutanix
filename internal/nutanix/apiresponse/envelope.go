// Package apiresponse decodes the common envelope of bounded Nutanix API responses.
package apiresponse

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
)

var (
	// ErrInvalidEnvelope identifies a missing, malformed, or inconsistent API envelope.
	ErrInvalidEnvelope = errors.New("nutanix API response envelope is invalid")
)

type rawEnvelope struct {
	Data json.RawMessage `json:"data"`
}

// DecodeList decodes an entity collection. An explicit null data value is an empty list.
func DecodeList[T any](body []byte) ([]T, error) {
	raw, err := decode(body)
	if err != nil {
		return nil, err
	}
	if bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return []T{}, nil
	}
	var values []T
	if err := json.Unmarshal(raw, &values); err != nil || values == nil {
		return nil, ErrInvalidEnvelope
	}
	return values, nil
}

// DecodeEntity decodes a single non-null entity.
func DecodeEntity[T any](body []byte) (T, error) {
	var zero T
	raw, err := decode(body)
	if err != nil || bytes.Equal(bytes.TrimSpace(raw), []byte("null")) {
		return zero, ErrInvalidEnvelope
	}
	var value T
	if err := json.Unmarshal(raw, &value); err != nil {
		return zero, ErrInvalidEnvelope
	}
	return value, nil
}

func decode(body []byte) (json.RawMessage, error) {
	decoder := json.NewDecoder(bytes.NewReader(body))
	var envelope rawEnvelope
	if err := decoder.Decode(&envelope); err != nil || envelope.Data == nil {
		return nil, ErrInvalidEnvelope
	}
	var trailing any
	if err := decoder.Decode(&trailing); err != io.EOF {
		return nil, ErrInvalidEnvelope
	}
	return envelope.Data, nil
}
