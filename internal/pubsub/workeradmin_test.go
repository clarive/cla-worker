package pubsub

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// mockWorkerAdminController mimics the Perl WorkerAdmin controller behavior.
// The Perl controller returns responses in two patterns:
//   - Success:       {"token":"abc123"}
//   - Error (catch): {"error":1,"msg":"human-readable message"}
//   - Error (model): {"error":"human-readable message"}
//
// The "error" field can be either a number (1) or a string, depending on
// whether the error came from the dispatch catch block or from the model.
type mockWorkerAdminController struct {
	srv              *httptest.Server
	lastRegisterReq  map[string]string
	lastUnregisterReq map[string]string
	registerHandler  func(params map[string]string) (int, interface{})
	unregisterHandler func(params map[string]string) (int, interface{})
}

func newMockWorkerAdminController(t *testing.T) *mockWorkerAdminController {
	t.Helper()
	ctrl := &mockWorkerAdminController{}

	// Default: successful registration
	ctrl.registerHandler = func(params map[string]string) (int, interface{}) {
		return 200, map[string]interface{}{"token": "tok-" + params["id"]}
	}
	// Default: successful unregistration
	ctrl.unregisterHandler = func(params map[string]string) (int, interface{}) {
		return 200, map[string]interface{}{"deleted": 1}
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/workeradmin/register_api", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Query().Get("path")
		params := map[string]string{}
		for k, v := range r.URL.Query() {
			if len(v) > 0 {
				params[k] = v[0]
			}
		}

		w.Header().Set("Content-Type", "application/json; charset=utf-8")

		switch path {
		case "register":
			ctrl.lastRegisterReq = params
			code, resp := ctrl.registerHandler(params)
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(resp)
		case "unregister_by_token":
			ctrl.lastUnregisterReq = params
			code, resp := ctrl.unregisterHandler(params)
			w.WriteHeader(code)
			json.NewEncoder(w).Encode(resp)
		default:
			w.WriteHeader(200)
			json.NewEncoder(w).Encode(map[string]interface{}{
				"error": 1,
				"msg":   "unknown resource",
			})
		}
	})

	ctrl.srv = httptest.NewServer(mux)
	t.Cleanup(ctrl.srv.Close)
	return ctrl
}

// ---------------------------------------------------------------------------
// RegisterResult UnmarshalJSON tests
// ---------------------------------------------------------------------------

func TestRegisterResult_UnmarshalJSON_TokenOnly(t *testing.T) {
	body := `{"token":"abc123"}`
	var r RegisterResult
	require.NoError(t, json.Unmarshal([]byte(body), &r))
	assert.Equal(t, "abc123", r.Token)
	assert.Empty(t, r.Error)
}

func TestRegisterResult_UnmarshalJSON_ErrorAsString(t *testing.T) {
	body := `{"error":"invalid passkey"}`
	var r RegisterResult
	require.NoError(t, json.Unmarshal([]byte(body), &r))
	assert.Equal(t, "invalid passkey", r.Error)
	assert.Empty(t, r.Token)
}

func TestRegisterResult_UnmarshalJSON_ErrorAsNumberWithMsg(t *testing.T) {
	body := `{"error":1,"msg":"access denied"}`
	var r RegisterResult
	require.NoError(t, json.Unmarshal([]byte(body), &r))
	assert.Equal(t, "access denied", r.Error)
	assert.Empty(t, r.Token)
}

func TestRegisterResult_UnmarshalJSON_ErrorAsNumberNoMsg(t *testing.T) {
	body := `{"error":1}`
	var r RegisterResult
	require.NoError(t, json.Unmarshal([]byte(body), &r))
	assert.Equal(t, "1", r.Error)
}

func TestRegisterResult_UnmarshalJSON_ErrorAsBool(t *testing.T) {
	body := `{"error":true,"msg":"something broke"}`
	var r RegisterResult
	require.NoError(t, json.Unmarshal([]byte(body), &r))
	assert.Equal(t, "something broke", r.Error)
}

func TestRegisterResult_UnmarshalJSON_NoError(t *testing.T) {
	body := `{"token":"xyz","msg":""}`
	var r RegisterResult
	require.NoError(t, json.Unmarshal([]byte(body), &r))
	assert.Equal(t, "xyz", r.Token)
	assert.Empty(t, r.Error)
}

// ---------------------------------------------------------------------------
// Register: request parameter tests
// ---------------------------------------------------------------------------

func TestRegister_SendsAllParams(t *testing.T) {
	ctrl := newMockWorkerAdminController(t)
	c := NewClient(
		WithBaseURL(ctrl.srv.URL),
		WithID("joe@server1"),
		WithToken("existing-tok"),
		WithOrigin("joe@server1/123"),
		WithTags([]string{"linux", "docker"}),
		WithVersion("2.1.0"),
		WithServer("server1"),
		WithServerMID("mid-srv-1"),
		WithUser("joe"),
	)

	result, err := c.Register(context.Background(), "pk-abc")
	require.NoError(t, err)
	assert.Equal(t, "tok-joe@server1", result.Token)

	p := ctrl.lastRegisterReq
	assert.Equal(t, "register", p["path"])
	assert.Equal(t, "joe@server1", p["id"])
	assert.Equal(t, "existing-tok", p["token"])
	assert.Equal(t, "joe@server1/123", p["origin"])
	assert.Equal(t, "linux,docker", p["tags"])
	assert.Equal(t, "2.1.0", p["version"])
	assert.Equal(t, "pk-abc", p["passkey"])
	assert.Equal(t, "server1", p["server"])
	assert.Equal(t, "mid-srv-1", p["server_mid"])
	assert.Equal(t, "joe", p["user"])
}

func TestRegister_AlwaysSendsServerAndUser(t *testing.T) {
	// claude: server and user are always sent (defaults populated by cmd layer);
	// only server_mid is omitted when empty
	ctrl := newMockWorkerAdminController(t)
	c := NewClient(
		WithBaseURL(ctrl.srv.URL),
		WithID("w1"),
		WithOrigin("u@h/1"),
	)

	_, err := c.Register(context.Background(), "pk")
	require.NoError(t, err)

	_, hasServer := ctrl.lastRegisterReq["server"]
	_, hasServerMID := ctrl.lastRegisterReq["server_mid"]
	_, hasUser := ctrl.lastRegisterReq["user"]
	assert.True(t, hasServer, "server should always be sent")
	assert.False(t, hasServerMID, "server_mid should be omitted when empty")
	assert.True(t, hasUser, "user should always be sent")
}

// ---------------------------------------------------------------------------
// Register: Perl controller response patterns
// ---------------------------------------------------------------------------

func TestRegister_Success_TokenReturned(t *testing.T) {
	ctrl := newMockWorkerAdminController(t)
	ctrl.registerHandler = func(p map[string]string) (int, interface{}) {
		return 200, map[string]interface{}{"token": "new-token-xyz"}
	}
	c := NewClient(WithBaseURL(ctrl.srv.URL), WithID("w1"), WithOrigin("u@h/1"))

	result, err := c.Register(context.Background(), "pk")
	require.NoError(t, err)
	assert.Equal(t, "new-token-xyz", result.Token)
	assert.Empty(t, result.Error)
}

func TestRegister_ErrorNumericWithMsg(t *testing.T) {
	// Perl catch block: {"error":1,"msg":"something exploded"}
	ctrl := newMockWorkerAdminController(t)
	ctrl.registerHandler = func(p map[string]string) (int, interface{}) {
		return 200, map[string]interface{}{
			"error": 1,
			"msg":   "invalid passkey",
		}
	}
	c := NewClient(WithBaseURL(ctrl.srv.URL), WithID("w1"), WithOrigin("u@h/1"))

	result, err := c.Register(context.Background(), "bad-pk")
	require.NoError(t, err)
	assert.Equal(t, "invalid passkey", result.Error)
	assert.Empty(t, result.Token)
}

func TestRegister_ErrorStringFromModel(t *testing.T) {
	// Model returns: {"error":"missing --origin parameter..."}
	ctrl := newMockWorkerAdminController(t)
	ctrl.registerHandler = func(p map[string]string) (int, interface{}) {
		return 200, map[string]interface{}{
			"error": "missing --origin parameter, please set one to user@hostname",
		}
	}
	c := NewClient(WithBaseURL(ctrl.srv.URL), WithID("w1"), WithOrigin("u@h/1"))

	result, err := c.Register(context.Background(), "pk")
	require.NoError(t, err)
	assert.Contains(t, result.Error, "missing --origin")
}

func TestRegister_HTTP500(t *testing.T) {
	ctrl := newMockWorkerAdminController(t)
	ctrl.registerHandler = func(p map[string]string) (int, interface{}) {
		return 500, "internal server error"
	}
	c := NewClient(WithBaseURL(ctrl.srv.URL), WithID("w1"), WithOrigin("u@h/1"))

	_, err := c.Register(context.Background(), "pk")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestRegister_HTMLResponse(t *testing.T) {
	// Simulates the auth middleware returning a login page (HTML)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html")
		w.WriteHeader(200)
		w.Write([]byte("<html><body>Login required</body></html>"))
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithID("w1"), WithOrigin("u@h/1"))
	_, err := c.Register(context.Background(), "pk")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing register response")
}

func TestRegister_EmptyBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(200)
	}))
	defer srv.Close()

	c := NewClient(WithBaseURL(srv.URL), WithID("w1"), WithOrigin("u@h/1"))
	_, err := c.Register(context.Background(), "pk")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "parsing register response")
}

func TestRegister_NetworkError(t *testing.T) {
	c := NewClient(WithBaseURL("http://127.0.0.1:1"), WithID("w1"), WithOrigin("u@h/1"))
	_, err := c.Register(context.Background(), "pk")
	require.Error(t, err)
	assert.Contains(t, err.Error(), "register request")
}

// ---------------------------------------------------------------------------
// Unregister: Perl controller response patterns
// ---------------------------------------------------------------------------

func TestUnregister_Success(t *testing.T) {
	ctrl := newMockWorkerAdminController(t)
	c := NewClient(WithBaseURL(ctrl.srv.URL), WithID("w1"), WithToken("tok-1"))

	err := c.Unregister(context.Background())
	require.NoError(t, err)

	p := ctrl.lastUnregisterReq
	assert.Equal(t, "unregister_by_token", p["path"])
	assert.Equal(t, "w1", p["id"])
	assert.Equal(t, "tok-1", p["token"])
}

func TestUnregister_ErrorNumericWithMsg(t *testing.T) {
	ctrl := newMockWorkerAdminController(t)
	ctrl.unregisterHandler = func(p map[string]string) (int, interface{}) {
		return 200, map[string]interface{}{
			"error": 1,
			"msg":   "Could not find worker id `w1` for token `bad`",
		}
	}
	c := NewClient(WithBaseURL(ctrl.srv.URL), WithID("w1"), WithToken("bad"))

	err := c.Unregister(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "Could not find worker")
}

func TestUnregister_ErrorStringFromModel(t *testing.T) {
	ctrl := newMockWorkerAdminController(t)
	ctrl.unregisterHandler = func(p map[string]string) (int, interface{}) {
		return 200, map[string]interface{}{
			"error": "worker not found",
		}
	}
	c := NewClient(WithBaseURL(ctrl.srv.URL), WithID("w1"), WithToken("tok"))

	err := c.Unregister(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "worker not found")
}

func TestUnregister_HTTP500(t *testing.T) {
	ctrl := newMockWorkerAdminController(t)
	ctrl.unregisterHandler = func(p map[string]string) (int, interface{}) {
		return 500, "internal error"
	}
	c := NewClient(WithBaseURL(ctrl.srv.URL), WithID("w1"), WithToken("tok"))

	err := c.Unregister(context.Background())
	require.Error(t, err)
	assert.Contains(t, err.Error(), "500")
}

func TestUnregister_UnknownPath(t *testing.T) {
	// Simulates calling register_api with a bogus path
	ctrl := newMockWorkerAdminController(t)
	// Override to send unknown path
	c := NewClient(WithBaseURL(ctrl.srv.URL), WithID("w1"), WithToken("tok"))

	// We can't easily test unknown path through the client API since it
	// hardcodes the path. Instead test the controller mock directly.
	_ = c
	resp, err := http.Post(ctrl.srv.URL+"/workeradmin/register_api?path=bogus", "", nil)
	require.NoError(t, err)
	defer resp.Body.Close()

	var body map[string]interface{}
	json.NewDecoder(resp.Body).Decode(&body)
	assert.Equal(t, float64(1), body["error"])
	assert.Equal(t, "unknown resource", body["msg"])
}

// ---------------------------------------------------------------------------
// Register: re-registration with existing token
// ---------------------------------------------------------------------------

func TestRegister_ReregistrationSendsToken(t *testing.T) {
	ctrl := newMockWorkerAdminController(t)
	ctrl.registerHandler = func(p map[string]string) (int, interface{}) {
		if p["token"] == "" {
			return 200, map[string]interface{}{
				"error": "registering an existing worker id w1 requires sending its token with --token",
			}
		}
		return 200, map[string]interface{}{"token": p["token"]}
	}

	// First registration without token
	c1 := NewClient(WithBaseURL(ctrl.srv.URL), WithID("w1"), WithOrigin("u@h/1"))
	result, err := c1.Register(context.Background(), "pk")
	require.NoError(t, err)
	assert.Contains(t, result.Error, "requires sending its token")

	// Re-registration with token
	c2 := NewClient(WithBaseURL(ctrl.srv.URL), WithID("w1"), WithToken("my-tok"), WithOrigin("u@h/1"))
	result2, err := c2.Register(context.Background(), "pk")
	require.NoError(t, err)
	assert.Equal(t, "my-tok", result2.Token)
	assert.Empty(t, result2.Error)
}

// ---------------------------------------------------------------------------
// Register: server binding
// ---------------------------------------------------------------------------

func TestRegister_ServerBindingByName(t *testing.T) {
	ctrl := newMockWorkerAdminController(t)
	c := NewClient(
		WithBaseURL(ctrl.srv.URL),
		WithID("joe@myserver"),
		WithOrigin("joe@myserver/1"),
		WithServer("myserver"),
		WithUser("joe"),
	)

	_, err := c.Register(context.Background(), "pk")
	require.NoError(t, err)
	assert.Equal(t, "myserver", ctrl.lastRegisterReq["server"])
	assert.Equal(t, "joe", ctrl.lastRegisterReq["user"])
}

func TestRegister_ServerBindingByMID(t *testing.T) {
	ctrl := newMockWorkerAdminController(t)
	c := NewClient(
		WithBaseURL(ctrl.srv.URL),
		WithID("w1"),
		WithOrigin("u@h/1"),
		WithServerMID("mid-123"),
	)

	_, err := c.Register(context.Background(), "pk")
	require.NoError(t, err)
	assert.Equal(t, "mid-123", ctrl.lastRegisterReq["server_mid"])
}
