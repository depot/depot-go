package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/pkg/errors"
)

type ErrorResponse struct {
	OK    bool   `json:"ok"`
	Error string `json:"error"`
}

func apiRequest[Response interface{}](method, url, token string, payload interface{}) (*Response, error) {
	var requestBody io.Reader

	if payload != nil {
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			return nil, err
		}
		requestBody = bytes.NewReader(jsonBytes)
	} else {
		requestBody = nil
	}

	client := &http.Client{}
	req, err := http.NewRequest(method, url, requestBody)
	if err != nil {
		return nil, err
	}

	req.Header.Add("Content-Type", "application/json")
	if token != "" {
		req.Header.Add("Authorization", fmt.Sprintf("Bearer %s", token))
	}

	req.Header.Add("User-Agent", Agent())
	req.Header.Add("Depot-User-Agent", Agent())

	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var errorResponse ErrorResponse
	err = json.Unmarshal(body, &errorResponse)
	if err == nil && !errorResponse.OK {
		return nil, fmt.Errorf("%s", errorResponse.Error)
	}

	var response Response
	err = json.Unmarshal(body, &response)
	if err != nil {
		return nil, errors.Wrap(err, string(body))
	}

	return &response, nil
}
