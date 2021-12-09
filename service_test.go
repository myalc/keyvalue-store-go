package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/google/uuid"
)

// TODO: having a persistant file or not effects tests !!!!!
// Consider case above !!!
// TODO: benchmark tests

// use the same server, parrallel executions may be possible
var s = NewService()

func TestUnsupportedMediaType(t *testing.T) {
	req, _err := http.NewRequest("GET", "/api/v1/my/keys", nil)
	if _err != nil {
		t.Fatal(_err)
	}
	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(s.Handle)
	handler.ServeHTTP(recorder, req)

	status := recorder.Code
	if status != http.StatusUnsupportedMediaType {
		t.Errorf("---> TEST: Got %v, expected %v", status, http.StatusUnsupportedMediaType)
	}
	log.Printf("---> TEST: status: %v", status)
}

func TestResponseHasReuestId(t *testing.T) {
	req, _err := http.NewRequest("GET", "/api/v1/my/keys", nil)
	req.Header.Add("content-type", "application/json")
	if _err != nil {
		t.Fatal(_err)
	}
	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(s.Handle)
	handler.ServeHTTP(recorder, req)

	status := recorder.Code
	if status != http.StatusNotFound && status != http.StatusOK {
		t.Errorf("---> TEST: Got %v, expected %v or %v", status, http.StatusNotFound, http.StatusOK)
	}

	rid, err := uuid.Parse(recorder.Header().Get("x-request-id"))
	if err != nil {
		t.Errorf("---> TEST: Cannot get a valid resonse x-request-id in http header. Got %v", rid)
	}
	log.Printf("---> TEST: status: %v response x-request-id: %v", status, recorder.Header().Get("x-request-id"))
}

func TestGetAll_notFoundOrOk(t *testing.T) {
	req, _err := http.NewRequest("GET", "/api/v1/my/keys", nil)
	req.Header.Add("content-type", "application/json")
	if _err != nil {
		t.Fatal(_err)
	}
	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(s.Handle)
	handler.ServeHTTP(recorder, req)

	status := recorder.Code
	if status != http.StatusNotFound && status != http.StatusOK {
		t.Errorf("---> TEST: Got %v, expected %v or %v", status, http.StatusNotFound, http.StatusOK)
	}
	log.Printf("---> TEST: status: %v", status)
}

func TestDeleteAll(t *testing.T) {
	TestCreate(t)
	req, _err := http.NewRequest("DELETE", "/api/v1/my/keys", nil)
	req.Header.Add("content-type", "application/json")
	if _err != nil {
		t.Fatal(_err)
	}
	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(s.Handle)
	handler.ServeHTTP(recorder, req)

	status := recorder.Code
	if status != http.StatusNoContent {
		t.Errorf("---> TEST: Got %v, expected %v", status, http.StatusNoContent)
	}
	log.Printf("---> TEST: status: %v", status)
}

func TestCreateErr(t *testing.T) {
	req, _err := http.NewRequest("POST", "/api/v1/my/keys", bytes.NewBuffer([]byte(``)))
	req.Header.Add("content-type", "application/json")
	if _err != nil {
		t.Fatal(_err)
	}
	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(s.Handle)
	handler.ServeHTTP(recorder, req)

	status := recorder.Code
	if status != http.StatusBadRequest {
		t.Errorf("---> TEST: Got %v, expected %v", status, http.StatusBadRequest)
	}
	log.Printf("---> TEST: status: %v", status)
}

func TestCreate(t *testing.T) {
	req, _err := http.NewRequest("POST", "/api/v1/my/keys", bytes.NewBuffer([]byte(`{"key1": "value1"}`)))
	req.Header.Add("content-type", "application/json")
	if _err != nil {
		t.Fatal(_err)
	}
	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(s.Handle)
	handler.ServeHTTP(recorder, req)

	status := recorder.Code
	if status != http.StatusCreated {
		t.Errorf("---> TEST: Got %v, expected %v", status, http.StatusCreated)
	}
	log.Printf("---> TEST: status: %v", status)

	// get created one
	req2, _err2 := http.NewRequest("GET", "/api/v1/my/keys/key1", nil)
	req2.Header.Add("content-type", "application/json")
	if _err2 != nil {
		t.Fatal(_err2)
	}
	recorder2 := httptest.NewRecorder()
	handler2 := http.HandlerFunc(s.Handle)
	handler2.ServeHTTP(recorder2, req2)

	status2 := recorder2.Code
	if status2 != http.StatusOK {
		t.Errorf("---> TEST: Got %v, expected %v", status2, http.StatusOK)
	}
	log.Printf("---> TEST: status2: %v", status2)

	result := make(map[string]string)
	var _err1 = json.NewDecoder(recorder2.Body).Decode(&result)
	if _err1 != nil {
		t.Errorf("---> TEST: Cannot decode response: %v", recorder2.Body.String())
	}
	if result["key1"] != "value1" {
		t.Errorf("---> TEST: Response payload is wrong: %v", result)
	}
}
