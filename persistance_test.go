package main

import (
	"bytes"
	"encoding/json"
	"log"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestPersist(t *testing.T) {
	pers := NewPersistance(30)
	dict := map[string]string{"A": "1", "B": "2", "C": "3", "D": "4"}
	filename := pers.Persist(&dict)
	if filename == "" {
		t.Errorf("---> TEST: Cannot create file")
	}
	err := os.Remove(filename)
	if err != nil {
		t.Errorf("---> TEST: Cannot delete created file (%v) in test case. err:%v", filename, err)
	} else {
		log.Printf("---> TEST: file '%v' deleted after test case", filename)
	}
}

func TestRestore(t *testing.T) {
	pers := NewPersistance(30)
	dict := map[string]string{"A": "1", "B": "2", "C": "3", "D": "4"}
	filename := pers.Persist(&dict)
	if filename == "" {
		t.Errorf("---> TEST: Cannot create file")
	}

	dict, err := pers.RestoreFromPersistance()
	if err != nil {
		t.Errorf("---> TEST: Data recovered from tmp directory. dict.len:%v \r\n", len(dict))
	}
}

/* Service and Persistance tests */
func TestCreatePersists(t *testing.T) {
	s := NewService(1) // create a service with 3 seconds persistance interval
	req, _err := http.NewRequest("POST", "/api/v1/my/keys", bytes.NewBuffer([]byte(`{"key1": "value1"}`)))
	req.Header.Add("content-type", "application/json")
	if _err != nil {
		t.Fatal(_err)
	}
	recorder := httptest.NewRecorder()
	handler := http.HandlerFunc(s.Handle)
	handler.ServeHTTP(recorder, req)
	_ = recorder.Code

	// wait for 3 seconds
	time.Sleep(3 * time.Second)
	s2 := NewService(300) // create another service, restart app

	// get the previosly created and persisted key1
	req2, _err2 := http.NewRequest("GET", "/api/v1/my/keys/key1", nil)
	req2.Header.Add("content-type", "application/json")
	if _err2 != nil {
		t.Fatal(_err2)
	}
	recorder2 := httptest.NewRecorder()
	handler2 := http.HandlerFunc(s2.Handle)
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
