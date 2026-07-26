package main

import (
	"bytes"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestHealth(t *testing.T) {
	rec := httptest.NewRecorder()
	handleHealth(rec, httptest.NewRequest(http.MethodGet, "/health", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
}

func TestExamplesAllVerify(t *testing.T) {
	// Every bundled example must open and verify without a server error.
	for _, ex := range examples {
		data, err := examplesFS.ReadFile(ex.File)
		if err != nil {
			t.Fatalf("embed missing %s: %v", ex.File, err)
		}
		report, err := verifyBytes(data, ex.Id)
		if err != nil {
			t.Fatalf("example %s failed to open: %v", ex.Id, err)
		}
		if len(report.Results) == 0 {
			t.Errorf("example %s produced no validator results", ex.Id)
		}
	}
}

func TestVerifyMultipartUpload(t *testing.T) {
	data, err := examplesFS.ReadFile("examples/acme-asc606.vwp")
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	fw, _ := mw.CreateFormFile("package", "acme-asc606.vwp")
	fw.Write(data)
	mw.Close()

	req := httptest.NewRequest(http.MethodPost, "/verify", &buf)
	req.Header.Set("Content-Type", mw.FormDataContentType())
	rec := httptest.NewRecorder()
	handleVerify(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d body = %s", rec.Code, rec.Body.String())
	}
	var report map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &report); err != nil {
		t.Fatalf("bad json: %v", err)
	}
	if report["valid"] != true {
		t.Errorf("expected valid=true, got %v", report["valid"])
	}
}

func TestVerifyRejectsGarbage(t *testing.T) {
	req := httptest.NewRequest(http.MethodPost, "/verify", bytes.NewReader([]byte("not a zip")))
	rec := httptest.NewRecorder()
	handleVerify(rec, req)
	if rec.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422 for garbage, got %d", rec.Code)
	}
}

func TestCORSPreflight(t *testing.T) {
	h := withCORS(http.NewServeMux())
	req := httptest.NewRequest(http.MethodOptions, "/verify", nil)
	req.Header.Set("Origin", "https://vwp.ac")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("preflight status = %d", rec.Code)
	}
	if got := rec.Header().Get("Access-Control-Allow-Origin"); got != "https://vwp.ac" {
		t.Errorf("allow-origin = %q", got)
	}
}
