package pattest

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
)

var ErrRuntime error = errors.New("runtime error")

type UntrustedPattern string

type UntrustedText string

type UntrustedInput struct {
	Pattern UntrustedPattern `json:"pattern"`
	Text    UntrustedText    `json:"text"`
}

func (i UntrustedInput) ToJson() ([]byte, error) {
	bytes, err := json.Marshal(i)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal input to JSON: %w", err)
	}
	return bytes, nil
}

type PatternTestResult struct {
	IsMatch bool
	Error   error
}

type PatternTestResultDto struct {
	//nolint:tagliatelle
	IsMatch bool `json:"is_match"`

	// No error if empty
	Error string `json:"error"`
}

func PatternTestResultDtoFromJson(j []byte) (d PatternTestResultDto, e error) {
	e = json.Unmarshal(j, &d)
	return
}

func (d PatternTestResultDto) ToResult() PatternTestResult {
	var err error
	if d.Error != "" {
		err = fmt.Errorf("%w: %s", ErrRuntime, d.Error)
	}
	return PatternTestResult{
		IsMatch: d.IsMatch,
		Error:   err,
	}
}

type PatternTester func(context.Context, UntrustedInput) PatternTestResult
