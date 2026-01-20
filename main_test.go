package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"os"
	"strings"
	"testing"
)

func TestOutputJSON(t *testing.T) {
	tests := []struct {
		name        string
		results     []LinkResult
		brokenCount int
		wantTotal   int
		wantBroken  int
		wantSuccess int
	}{
		{
			name: "all successful",
			results: []LinkResult{
				{URL: "https://example.com", Status: 200, IsBroken: false},
				{URL: "https://example.com/page", Status: 200, IsBroken: false},
			},
			brokenCount: 0,
			wantTotal:   2,
			wantBroken:  0,
			wantSuccess: 2,
		},
		{
			name: "mixed results",
			results: []LinkResult{
				{URL: "https://example.com", Status: 200, IsBroken: false},
				{URL: "https://example.com/404", Status: 404, IsBroken: true, SourceURL: "https://example.com"},
				{URL: "https://broken.com", Status: 0, Error: errors.New("connection refused"), IsBroken: true},
			},
			brokenCount: 2,
			wantTotal:   3,
			wantBroken:  2,
			wantSuccess: 1,
		},
		{
			name:        "empty results",
			results:     []LinkResult{},
			brokenCount: 0,
			wantTotal:   0,
			wantBroken:  0,
			wantSuccess: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Capture stdout
			oldStdout := os.Stdout
			r, w, _ := os.Pipe()
			os.Stdout = w

			outputJSON(tt.results, tt.brokenCount)

			w.Close()
			os.Stdout = oldStdout

			var buf bytes.Buffer
			io.Copy(&buf, r)

			// Parse JSON output
			var output JSONOutput
			if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
				t.Fatalf("Failed to parse JSON output: %v", err)
			}

			// Verify summary
			if output.Summary.Total != tt.wantTotal {
				t.Errorf("Total = %d, want %d", output.Summary.Total, tt.wantTotal)
			}
			if output.Summary.Broken != tt.wantBroken {
				t.Errorf("Broken = %d, want %d", output.Summary.Broken, tt.wantBroken)
			}
			if output.Summary.Success != tt.wantSuccess {
				t.Errorf("Success = %d, want %d", output.Summary.Success, tt.wantSuccess)
			}

			// Verify results count
			if len(output.Results) != len(tt.results) {
				t.Errorf("Results count = %d, want %d", len(output.Results), len(tt.results))
			}
		})
	}
}

func TestOutputJSON_ErrorHandling(t *testing.T) {
	results := []LinkResult{
		{
			URL:       "https://example.com/error",
			Status:    0,
			Error:     errors.New("network timeout"),
			IsBroken:  true,
			SourceURL: "https://example.com",
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputJSON(results, 1)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)

	var output JSONOutput
	if err := json.Unmarshal(buf.Bytes(), &output); err != nil {
		t.Fatalf("Failed to parse JSON: %v", err)
	}

	// Verify error is included in JSON
	if output.Results[0].Error == nil {
		t.Error("Expected error field to be present")
	}

	if *output.Results[0].Error != "network timeout" {
		t.Errorf("Error = %s, want 'network timeout'", *output.Results[0].Error)
	}

	if !output.Results[0].Broken {
		t.Error("Expected broken to be true")
	}
}

func TestOutputHuman_NormalMode(t *testing.T) {
	results := []LinkResult{
		{URL: "https://example.com", Status: 200, IsBroken: false},
		{URL: "https://example.com/404", Status: 404, IsBroken: true, SourceURL: "https://example.com"},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputHuman(results, 1, false)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify output contains key elements
	if !strings.Contains(output, "Results:") {
		t.Error("Expected 'Results:' header")
	}

	if !strings.Contains(output, "✓ [200] https://example.com") {
		t.Error("Expected successful link output")
	}

	if !strings.Contains(output, "✗ [404] https://example.com/404") {
		t.Error("Expected broken link output")
	}

	if !strings.Contains(output, "Summary:") {
		t.Error("Expected summary")
	}
}

func TestOutputHuman_QuietMode(t *testing.T) {
	results := []LinkResult{
		{URL: "https://example.com", Status: 200, IsBroken: false},
		{URL: "https://example.com/404", Status: 404, IsBroken: true, SourceURL: "https://example.com"},
		{URL: "https://error.com", Status: 0, Error: errors.New("timeout"), IsBroken: true},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputHuman(results, 2, true)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// In quiet mode, should NOT contain success messages
	if strings.Contains(output, "✓ [200]") {
		t.Error("Quiet mode should not show successful links")
	}

	if strings.Contains(output, "Results:") {
		t.Error("Quiet mode should not show 'Results:' header")
	}

	if strings.Contains(output, "Summary:") {
		t.Error("Quiet mode should not show summary")
	}

	// Should still show broken links
	if !strings.Contains(output, "✗ [404]") {
		t.Error("Quiet mode should show broken links")
	}

	if !strings.Contains(output, "✗ [error]") {
		t.Error("Quiet mode should show error links")
	}
}

func TestOutputHuman_WithError(t *testing.T) {
	results := []LinkResult{
		{
			URL:       "https://example.com/timeout",
			Status:    0,
			Error:     errors.New("connection timeout"),
			IsBroken:  true,
			SourceURL: "https://example.com",
		},
	}

	// Capture stdout
	oldStdout := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	outputHuman(results, 1, false)

	w.Close()
	os.Stdout = oldStdout

	var buf bytes.Buffer
	io.Copy(&buf, r)
	output := buf.String()

	// Verify error output format
	if !strings.Contains(output, "✗ [error]") {
		t.Error("Expected error marker")
	}

	if !strings.Contains(output, "connection timeout") {
		t.Error("Expected error message")
	}

	if !strings.Contains(output, "Source: https://example.com") {
		t.Error("Expected source URL")
	}
}

func TestJSONResult_Serialization(t *testing.T) {
	errMsg := "test error"
	result := JSONResult{
		URL:       "https://example.com",
		Status:    http.StatusNotFound,
		Error:     &errMsg,
		Broken:    true,
		SourceURL: "https://source.com",
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	var decoded JSONResult
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatalf("Failed to unmarshal JSON: %v", err)
	}

	if decoded.URL != result.URL {
		t.Errorf("URL = %s, want %s", decoded.URL, result.URL)
	}

	if decoded.Status != result.Status {
		t.Errorf("Status = %d, want %d", decoded.Status, result.Status)
	}

	if decoded.Error == nil || *decoded.Error != *result.Error {
		t.Errorf("Error mismatch")
	}

	if decoded.Broken != result.Broken {
		t.Errorf("Broken = %v, want %v", decoded.Broken, result.Broken)
	}
}

func TestJSONResult_NilError(t *testing.T) {
	result := JSONResult{
		URL:    "https://example.com",
		Status: http.StatusOK,
		Error:  nil,
		Broken: false,
	}

	data, err := json.Marshal(result)
	if err != nil {
		t.Fatalf("Failed to marshal JSON: %v", err)
	}

	// Verify error field is omitted when nil
	if strings.Contains(string(data), "error") {
		t.Error("Expected error field to be omitted when nil")
	}
}
