package hypervisor

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/SkycoinProject/skycoin/src/util/logging"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMain(m *testing.M) {
	loggingLevel, ok := os.LookupEnv("TEST_LOGGING_LEVEL")
	if ok {
		lvl, err := logging.LevelFromString(loggingLevel)
		if err != nil {
			log.Fatal(err)
		}
		logging.SetLevel(lvl)
	} else {
		logging.Disable()
	}

	os.Exit(m.Run())
}

func TestNewNode(t *testing.T) {
	config := makeConfig()

	confDir, err := ioutil.TempDir(os.TempDir(), "SWHV")
	require.NoError(t, err)
	config.DBPath = filepath.Join(confDir, "users.db")

	defaultMockConfig := func() MockConfig {
		return MockConfig{
			Nodes:            5,
			MaxTpsPerNode:    10,
			MaxRoutesPerNode: 10,
			EnableAuth:       true,
		}
	}

	startNode := func(mock MockConfig) (string, *http.Client, func()) {
		node, err := NewNode(config)
		require.NoError(t, err)
		require.NoError(t, node.AddMockData(mock))

		srv := httptest.NewTLSServer(node)
		node.c.Cookies.Domain = srv.Listener.Addr().String()

		client := srv.Client()
		jar, err := cookiejar.New(&cookiejar.Options{})
		require.NoError(t, err)
		client.Jar = jar

		return srv.Listener.Addr().String(), client, func() {
			srv.Close()
			require.NoError(t, os.Remove(config.DBPath))
		}
	}

	type TestCase struct {
		ReqMethod  string
		ReqURI     string
		ReqBody    io.Reader
		ReqMod     func(req *http.Request)
		RespStatus int
		RespBody   func(t *testing.T, resp *http.Response)
	}

	testCases := func(t *testing.T, addr string, client *http.Client, cases []TestCase) {
		for i, tc := range cases {
			testTag := fmt.Sprintf("[%d] %s", i, tc.ReqURI)

			req, err := http.NewRequest(tc.ReqMethod, "https://"+addr+tc.ReqURI, tc.ReqBody)
			require.NoError(t, err, testTag)

			if tc.ReqMod != nil {
				tc.ReqMod(req)
			}

			resp, err := client.Do(req)
			require.NoError(t, err, testTag)

			assert.Equal(t, tc.RespStatus, resp.StatusCode, testTag)
			if tc.RespBody != nil {
				tc.RespBody(t, resp)
			}
		}
	}

	t.Run("no_access_without_login", func(t *testing.T) {
		addr, client, stop := startNode(defaultMockConfig())
		defer stop()

		makeCase := func(method string, uri string, body io.Reader) TestCase {
			return TestCase{
				ReqMethod:  method,
				ReqURI:     uri,
				ReqBody:    body,
				RespStatus: http.StatusUnauthorized,
				RespBody: func(t *testing.T, r *http.Response) {
					body, err := decodeErrorBody(r.Body)
					assert.NoError(t, err)
					assert.Equal(t, ErrBadSession.Error(), body.Error)
				},
			}
		}

		testCases(t, addr, client, []TestCase{
			makeCase(http.MethodGet, "/api/user", nil),
			makeCase(http.MethodPost, "/api/change-password", strings.NewReader(`{"old_password":"old","new_password":"new"}`)),
			makeCase(http.MethodGet, "/api/nodes", nil),
		})
	})

	t.Run("only_admin_account_allowed", func(t *testing.T) {
		addr, client, stop := startNode(defaultMockConfig())
		defer stop()

		testCases(t, addr, client, []TestCase{
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/create-account",
				ReqBody:    strings.NewReader(`{"username":"invalid_user","password":"Secure1234"}`),
				RespStatus: http.StatusForbidden,
				RespBody: func(t *testing.T, r *http.Response) {
					body, err := decodeErrorBody(r.Body)
					assert.NoError(t, err)
					assert.Equal(t, ErrUserNotCreated.Error(), body.Error)
				},
			},
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/create-account",
				ReqBody:    strings.NewReader(`{"username":"admin","password":"Secure1234"}`),
				RespStatus: http.StatusOK,
				RespBody: func(t *testing.T, r *http.Response) {
					var ok bool
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&ok))
					assert.True(t, ok)
				},
			},
		})
	})

	t.Run("cannot_login_twice", func(t *testing.T) {
		addr, client, stop := startNode(defaultMockConfig())
		defer stop()

		testCases(t, addr, client, []TestCase{
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/create-account",
				ReqBody:    strings.NewReader(`{"username":"admin","password":"Secure1234"}`),
				RespStatus: http.StatusOK,
				RespBody: func(t *testing.T, r *http.Response) {
					var ok bool
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&ok))
					assert.True(t, ok)
				},
			},
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/login",
				ReqBody:    strings.NewReader(`{"username":"admin","password":"Secure1234"}`),
				RespStatus: http.StatusOK,
				RespBody: func(t *testing.T, r *http.Response) {
					var ok bool
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&ok))
					assert.True(t, ok)
				},
			},
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/login",
				ReqBody:    strings.NewReader(`{"username":"admin","password":"Secure1234"}`),
				RespStatus: http.StatusForbidden,
				RespBody: func(t *testing.T, r *http.Response) {
					body, err := decodeErrorBody(r.Body)
					assert.NoError(t, err)
					assert.Equal(t, ErrNotLoggedOut.Error(), body.Error)
				},
			},
		})
	})

	t.Run("access_after_login", func(t *testing.T) {
		addr, client, stop := startNode(defaultMockConfig())
		defer stop()

		testCases(t, addr, client, []TestCase{
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/create-account",
				ReqBody:    strings.NewReader(`{"username":"admin","password":"Secure1234"}`),
				RespStatus: http.StatusOK,
				RespBody: func(t *testing.T, r *http.Response) {
					var ok bool
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&ok))
					assert.True(t, ok)
				},
			},
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/login",
				ReqBody:    strings.NewReader(`{"username":"admin","password":"Secure1234"}`),
				RespStatus: http.StatusOK,
				RespBody: func(t *testing.T, r *http.Response) {
					var ok bool
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&ok))
					assert.True(t, ok)
				},
			},
			{
				ReqMethod:  http.MethodGet,
				ReqURI:     "/api/user",
				RespStatus: http.StatusOK,
			},
			{
				ReqMethod:  http.MethodGet,
				ReqURI:     "/api/nodes",
				RespStatus: http.StatusOK,
			},
		})
	})

	t.Run("no_access_after_logout", func(t *testing.T) {
		addr, client, stop := startNode(defaultMockConfig())
		defer stop()

		testCases(t, addr, client, []TestCase{
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/create-account",
				ReqBody:    strings.NewReader(`{"username":"admin","password":"Secure1234"}`),
				RespStatus: http.StatusOK,
				RespBody: func(t *testing.T, r *http.Response) {
					var ok bool
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&ok))
					assert.True(t, ok)
				},
			},
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/login",
				ReqBody:    strings.NewReader(`{"username":"admin","password":"Secure1234"}`),
				RespStatus: http.StatusOK,
				RespBody: func(t *testing.T, r *http.Response) {
					var ok bool
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&ok))
					assert.True(t, ok)
				},
			},
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/logout",
				RespStatus: http.StatusOK,
				RespBody: func(t *testing.T, r *http.Response) {
					var ok bool
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&ok))
					assert.True(t, ok)
				},
			},
			{
				ReqMethod:  http.MethodGet,
				ReqURI:     "/api/user",
				RespStatus: http.StatusUnauthorized,
				RespBody: func(t *testing.T, r *http.Response) {
					body, err := decodeErrorBody(r.Body)
					assert.NoError(t, err)
					assert.Equal(t, ErrBadSession.Error(), body.Error)
				},
			},
			{
				ReqMethod:  http.MethodGet,
				ReqURI:     "/api/nodes",
				RespStatus: http.StatusUnauthorized,
				RespBody: func(t *testing.T, r *http.Response) {
					body, err := decodeErrorBody(r.Body)
					assert.NoError(t, err)
					assert.Equal(t, ErrBadSession.Error(), body.Error)
				},
			},
		})
	})

	t.Run("change_password", func(t *testing.T) {
		// - Create account.
		// - Login.
		// - Change Password.
		// - Attempt action (should fail).
		// - Logout.
		// - Login with old password (should fail).
		// - Login with new password (should succeed).

		addr, client, stop := startNode(defaultMockConfig())
		defer stop()

		// To emulate an active session.
		var cookies []*http.Cookie

		testCases(t, addr, client, []TestCase{
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/create-account",
				ReqBody:    strings.NewReader(`{"username":"admin","password":"Secure1234"}`),
				RespStatus: http.StatusOK,
				RespBody: func(t *testing.T, r *http.Response) {
					var ok bool
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&ok))
					assert.True(t, ok)
				},
			},
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/login",
				ReqBody:    strings.NewReader(`{"username":"admin","password":"Secure1234"}`),
				RespStatus: http.StatusOK,
				RespBody: func(t *testing.T, r *http.Response) {
					cookies = r.Cookies()
					var ok bool
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&ok))
					assert.True(t, ok)
				},
			},
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/change-password",
				ReqBody:    strings.NewReader(`{"old_password":"Secure1234","new_password":"NewSecure1234"}`),
				RespStatus: http.StatusOK,
				RespBody: func(t *testing.T, r *http.Response) {
					var ok bool
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&ok))
					assert.True(t, ok)
				},
			},
			{
				ReqMethod: http.MethodGet,
				ReqURI:    "/api/nodes",
				ReqMod: func(req *http.Request) {
					for _, cookie := range cookies {
						req.AddCookie(cookie)
					}
				},
				RespStatus: http.StatusUnauthorized,
			},
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/logout",
				RespStatus: http.StatusOK,
				RespBody: func(t *testing.T, r *http.Response) {
					var ok bool
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&ok))
					assert.True(t, ok)
				},
			},
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/login",
				ReqBody:    strings.NewReader(`{"username":"admin","password":"Secure1234"}`),
				RespStatus: http.StatusUnauthorized,
				RespBody: func(t *testing.T, r *http.Response) {
					b, err := decodeErrorBody(r.Body)
					assert.NoError(t, err)
					require.Equal(t, ErrBadLogin.Error(), b.Error)
				},
			},
			{
				ReqMethod:  http.MethodPost,
				ReqURI:     "/api/login",
				ReqBody:    strings.NewReader(`{"username":"admin","password":"NewSecure1234"}`),
				RespStatus: http.StatusOK,
				RespBody: func(t *testing.T, r *http.Response) {
					var ok bool
					assert.NoError(t, json.NewDecoder(r.Body).Decode(&ok))
					assert.True(t, ok)
				},
			},
		})
	})
}

type ErrorBody struct {
	Error string `json:"error"`
}

func decodeErrorBody(rb io.Reader) (*ErrorBody, error) {
	b := new(ErrorBody)
	dec := json.NewDecoder(rb)
	dec.DisallowUnknownFields()
	return b, dec.Decode(b)
}
