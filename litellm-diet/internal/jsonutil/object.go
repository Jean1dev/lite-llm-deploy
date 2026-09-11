package jsonutil

import (
	"bytes"
	"encoding/json"
	"fmt"
)

func RemoveFields(body []byte, fields ...string) ([]byte, error) {
	obj, err := decodeObject(body)
	if err != nil {
		return nil, err
	}
	for _, f := range fields {
		delete(obj, f)
	}
	return json.Marshal(obj)
}

func ReplaceStringField(body []byte, field, value string) ([]byte, error) {
	obj, err := decodeObject(body)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("serialize %s: %w", field, err)
	}
	obj[field] = raw
	return json.Marshal(obj)
}

func SetBoolField(body []byte, field string, value bool) ([]byte, error) {
	obj, err := decodeObject(body)
	if err != nil {
		return nil, err
	}
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("serialize %s: %w", field, err)
	}
	obj[field] = raw
	return json.Marshal(obj)
}

func ExtractBool(body []byte, field string) (bool, bool) {
	obj, err := decodeObject(body)
	if err != nil {
		return false, false
	}
	raw, ok := obj[field]
	if !ok {
		return false, false
	}
	var v bool
	if err := json.Unmarshal(raw, &v); err != nil {
		return false, false
	}
	return v, true
}

func ExtractObject(body []byte, field string) (map[string]json.RawMessage, bool) {
	obj, err := decodeObject(body)
	if err != nil {
		return nil, false
	}
	raw, ok := obj[field]
	if !ok {
		return nil, false
	}
	var inner map[string]json.RawMessage
	if err := json.Unmarshal(raw, &inner); err != nil {
		return nil, false
	}
	return inner, true
}

func decodeObject(body []byte) (map[string]json.RawMessage, error) {
	dec := json.NewDecoder(bytes.NewReader(body))
	var obj map[string]json.RawMessage
	if err := dec.Decode(&obj); err != nil {
		return nil, fmt.Errorf("decode object: %w", err)
	}
	if obj == nil {
		obj = map[string]json.RawMessage{}
	}
	return obj, nil
}
