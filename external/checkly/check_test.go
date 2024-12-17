/*
Copyright 2022.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package external

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/checkly/checkly-go-sdk"
)

func TestChecklyCheck(t *testing.T) {
	data := Check{
		Name:            "foo",
		Namespace:       "bar",
		Frequency:       15,
		MaxResponseTime: 2000,
		Endpoint:        "https://foo.bar/baz",
		Muted:           true,
		Method:          "POST",
		Body:            `{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}`,
		BodyType:        "JSON",
		Assertions: []checkly.Assertion{
			{
				Source:     "JSONBody",
				Property:   "$.result",
				Comparison: "Equals",
				Target:     "false",
			},
			{
				Source:     "JSONBody",
				Comparison: "NotNull",
			},
		},
	}

	testData, err := checklyCheck(data)
	if err != nil {
		t.Fatalf("Expected no error, got %v", err)
	}

	if testData.Name != data.Name {
		t.Errorf("Expected %s, got %s", data.Name, testData.Name)
	}

	if testData.Frequency != data.Frequency {
		t.Errorf("Expected %d, got %d", data.Frequency, testData.Frequency)
	}

	if testData.MaxResponseTime != data.MaxResponseTime {
		t.Errorf("Expected %d, got %d", data.MaxResponseTime, testData.MaxResponseTime)
	}

	if len(testData.Request.Assertions) != len(data.Assertions) {
		t.Errorf("Expected %d assertions, got %d", len(data.Assertions), len(testData.Request.Assertions))
	}

	for i, assertion := range testData.Request.Assertions {
		if assertion.Source != data.Assertions[i].Source {
			t.Errorf("Expected Source %s, got %s", data.Assertions[i].Source, assertion.Source)
		}
		if assertion.Comparison != data.Assertions[i].Comparison {
			t.Errorf("Expected Comparison %s, got %s", data.Assertions[i].Comparison, assertion.Comparison)
		}
		if assertion.Target != data.Assertions[i].Target {
			t.Errorf("Expected Target %s, got %s", data.Assertions[i].Target, assertion.Target)
		}
	}

	if testData.Request.Method != data.Method {
		t.Errorf("Expected Method %s, got %s", data.Method, testData.Request.Method)
	}

	if testData.Request.BodyType != data.BodyType {
		t.Errorf("Expected BodyType %s, got %s", data.BodyType, testData.Request.BodyType)
	}

	expectedBody := `{"jsonrpc":"2.0","method":"eth_syncing","params":[],"id":1}`
	var expectedBodyFormatted map[string]interface{}
	json.Unmarshal([]byte(expectedBody), &expectedBodyFormatted)

	var actualBodyFormatted map[string]interface{}
	json.Unmarshal([]byte(testData.Request.Body), &actualBodyFormatted)

	if !equalJSON(expectedBodyFormatted, actualBodyFormatted) {
		t.Errorf("Expected Body %v, got %v", expectedBodyFormatted, actualBodyFormatted)
	}

	if testData.Muted != data.Muted {
		t.Errorf("Expected %t, got %t", data.Muted, testData.Muted)
	}

}

func TestChecklyCheckActions(t *testing.T) {

	expectedCheckID := "2"
	expectedGroupID := 1
	testData := Check{
		Name:            "foo",
		Namespace:       "bar",
		Frequency:       15,
		MaxResponseTime: 2000,
		Endpoint:        "https://foo.bar/baz",
		Method:          "PUT",
		Body:            `{"query":"query { status }"}`,
		BodyType:        "graphql",
		ID:              "",
		Assertions: []checkly.Assertion{
			{
				Source:     "StatusCode",
				Comparison: "Equals",
				Target:     "200",
			},
			{
				Source:     "JSONBody",
				Property:   "$.result",
				Comparison: "NotNull",
			},
		},
	}

	// Test errors
	testClientFail := checkly.NewClient(
		"http://localhost:5556",
		"foobarbaz",
		nil,
		nil,
	)
	// Create
	_, err := Create(testData, testClientFail)
	if err == nil {
		t.Error("Expected error, got none")
	}

	// Update
	err = Update(testData, testClientFail)
	if err == nil {
		t.Error("Expected error, got none")
	}

	// Delete
	err = Delete(expectedCheckID, testClientFail)
	if err == nil {
		t.Error("Expected error, got none")
	}

	// Test happy path
	testClient := checkly.NewClient(
		"http://localhost:5555",
		"foobarbaz",
		nil,
		nil,
	)
	testClient.SetAccountId("1234567890")

	go func() {
		http.HandleFunc("/v1/checks", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]string{"id": expectedCheckID}
			jsonResp, _ := json.Marshal(resp)
			w.Write(jsonResp)
			return
		})
		http.HandleFunc("/v1/checks/2", func(w http.ResponseWriter, r *http.Request) {
			r.ParseForm()
			switch r.Method {
			case "PUT":
				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "application/json")
				resp := map[string]string{"id": expectedCheckID}
				jsonResp, _ := json.Marshal(resp)
				w.Write(jsonResp)
			case "DELETE":
				w.WriteHeader(http.StatusNoContent)
			}
			return
		})
		http.HandleFunc("/v1/check-groups", func(w http.ResponseWriter, _ *http.Request) {
			w.WriteHeader(http.StatusCreated)
			w.Header().Set("Content-Type", "application/json")
			resp := map[string]interface{}{"id": expectedGroupID}
			jsonResp, _ := json.Marshal(resp)
			w.Write(jsonResp)
			return
		})
		http.HandleFunc("/v1/check-groups/1", func(w http.ResponseWriter, r *http.Request) {
			r.ParseForm()
			switch r.Method {
			case "PUT":
				w.WriteHeader(http.StatusOK)
				w.Header().Set("Content-Type", "application/json")
				resp := map[string]interface{}{"id": expectedGroupID}
				jsonResp, _ := json.Marshal(resp)
				w.Write(jsonResp)
			case "DELETE":
				w.WriteHeader(http.StatusNoContent)
			}
			return
		})
		http.ListenAndServe(":5555", nil)
	}()

	testID, err := Create(testData, testClient)
	if err != nil {
		t.Errorf("Expected no error, got %e", err)
	}

	if testID != expectedCheckID {
		t.Errorf("Expected %s, got %s", expectedCheckID, testID)
	}

	testData.ID = expectedCheckID
	err = Update(testData, testClient)
	if err != nil {
		t.Errorf("Expected no error, got %e", err)
	}

	err = Delete(expectedCheckID, testClient)
	if err != nil {
		t.Errorf("Expected no error, got %e", err)
	}

	return
}

func equalJSON(expected, actual map[string]interface{}) bool {
	expectedBytes, _ := json.Marshal(expected)
	actualBytes, _ := json.Marshal(actual)
	return string(expectedBytes) == string(actualBytes)
}
