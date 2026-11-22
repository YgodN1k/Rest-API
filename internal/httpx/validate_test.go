package httpx

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDecodeJSON(t *testing.T) {
	type payload struct {
		Name string `json:"name"`
		Age  int    `json:"age"`
	}

	cases := []struct {
		name    string
		body    string
		wantErr string
	}{
		{"ok", `{"name":"a","age":3}`, ""},
		{"empty", ``, "must not be empty"},
		{"malformed", `{"name":`, "malformed JSON"},
		{"syntax", `{name}`, "malformed JSON"},
		{"wrong type", `{"age":"x"}`, `field "age" has invalid type`},
		{"unknown field", `{"name":"a","admin":true}`, `unknown field "admin"`},
		{"two objects", `{"name":"a"}{"name":"b"}`, "single JSON object"},
		{"too large", `{"name":"` + strings.Repeat("a", MaxBodyBytes) + `"}`, "must not be larger"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/", strings.NewReader(tc.body))
			var p payload
			err := DecodeJSON(httptest.NewRecorder(), req, &p)
			if tc.wantErr == "" {
				require.NoError(t, err)
				require.Equal(t, payload{Name: "a", Age: 3}, p)
				return
			}
			require.ErrorContains(t, err, tc.wantErr)
		})
	}
}
