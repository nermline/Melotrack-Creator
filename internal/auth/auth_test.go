package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/gin-gonic/gin"
)

func init() { gin.SetMode(gin.TestMode) }

func ctxWithMethod(method string) *gin.Context {
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	c.Request = httptest.NewRequest(method, "/api/projects", nil)
	return c
}

func TestAuthorizer_RolesAndMethods(t *testing.T) {
	authz := authorizer()
	cases := []struct {
		role, method string
		want         bool
	}{
		{"admin", http.MethodGet, true},
		{"admin", http.MethodPost, true},
		{"admin", http.MethodPut, true},
		{"admin", http.MethodDelete, true},
		{"editor", http.MethodGet, true},
		{"editor", http.MethodPost, true},
		{"editor", http.MethodDelete, true},
		{"operator", http.MethodGet, true},
		{"operator", http.MethodPost, false},
		{"operator", http.MethodPut, false},
		{"operator", http.MethodDelete, false},
		{"viewer", http.MethodGet, false}, // невідома роль
		{"", http.MethodGet, false},
	}
	for _, tc := range cases {
		data := map[string]interface{}{"role": tc.role}
		if got := authz(ctxWithMethod(tc.method), data); got != tc.want {
			t.Errorf("role=%q method=%s: got %v, want %v", tc.role, tc.method, got, tc.want)
		}
	}
}

func TestAuthorizer_BadClaims(t *testing.T) {
	authz := authorizer()
	if authz(ctxWithMethod(http.MethodGet), "not-a-map") {
		t.Error("expected false for non-map claims")
	}
	if authz(ctxWithMethod(http.MethodGet), map[string]interface{}{}) {
		t.Error("expected false for claims without role")
	}
	if authz(ctxWithMethod(http.MethodGet), map[string]interface{}{"role": 123}) {
		t.Error("expected false for non-string role")
	}
}
