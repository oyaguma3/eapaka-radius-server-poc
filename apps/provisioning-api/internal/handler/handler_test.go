package handler_test

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/gin-gonic/gin"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/audit"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/auth"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/handler"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/server"
	"github.com/oyaguma3/eapaka-radius-server-poc/apps/provisioning-api/internal/service"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/logging"
	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/masterdata"
	"github.com/redis/go-redis/v9"
)

const (
	testIMSI   = "001010000000001"
	testKi     = "465b5ce8b199b49faa5f0a2ee238a6bc"
	testOPc    = "cd63cb71954a9f4e48a5994e37a02baf"
	testIP     = "192.168.10.1"
	testSecret = "c2VjcmV0LWV4YW1wbGU"
)

func init() {
	gin.SetMode(gin.TestMode)
}

// env はテスト用の API と、その出力先を持つ。
type env struct {
	t       *testing.T
	engine  *gin.Engine
	mr      *miniredis.Miniredis
	appLog  *bytes.Buffer
	auditBu *bytes.Buffer
	cert    *x509.Certificate
}

// newCert は自己署名のクライアント証明書を生成する。
func newCert(t *testing.T) *x509.Certificate {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	tmpl := &x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "bff-01"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	der, err := x509.CreateCertificate(rand.Reader, tmpl, tmpl, &key.PublicKey, key)
	if err != nil {
		t.Fatal(err)
	}
	cert, err := x509.ParseCertificate(der)
	if err != nil {
		t.Fatal(err)
	}
	return cert
}

func newEnv(t *testing.T) *env {
	t.Helper()
	mr := miniredis.RunT(t)
	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	t.Cleanup(func() { _ = rdb.Close() })

	e := &env{t: t, mr: mr, appLog: &bytes.Buffer{}, auditBu: &bytes.Buffer{}, cert: newCert(t)}
	log := slog.New(slog.NewJSONHandler(e.appLog, &slog.HandlerOptions{Level: slog.LevelDebug}))
	svc := service.New(rdb, audit.NewLogger(e.auditBu))
	h := handler.New(svc, log, "0.1.0-test", "node-a", time.Date(2026, 10, 7, 0, 0, 0, 0, time.UTC))
	clients := auth.Clients{auth.Fingerprint(e.cert): "bff-01"}
	e.engine = server.NewEngine(h, clients, log, logging.NewMasker(true))
	return e
}

// do は要求を送り、応答を返す。body が string ならそのまま、それ以外は JSON にして送る。
func (e *env) do(method, path string, body any, headers ...string) *httptest.ResponseRecorder {
	e.t.Helper()
	var r *strings.Reader
	switch b := body.(type) {
	case nil:
		r = strings.NewReader("")
	case string:
		r = strings.NewReader(b)
	default:
		data, err := json.Marshal(b)
		if err != nil {
			e.t.Fatal(err)
		}
		r = strings.NewReader(string(data))
	}
	req := httptest.NewRequest(method, path, r)
	req.TLS = &tls.ConnectionState{PeerCertificates: []*x509.Certificate{e.cert}}
	if body != nil {
		ct := "application/json"
		if method == http.MethodPatch {
			ct = "application/merge-patch+json"
		}
		req.Header.Set("Content-Type", ct)
	}
	for i := 0; i+1 < len(headers); i += 2 {
		req.Header.Set(headers[i], headers[i+1])
	}
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	return w
}

// decode は応答の JSON を読む。
func decode(t *testing.T, w *httptest.ResponseRecorder) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(w.Body.Bytes(), &m); err != nil {
		t.Fatalf("response is not JSON: %q", w.Body.String())
	}
	return m
}

// expectProblem はエラー応答の status・cause・invalidParams の項目名を確かめる。
func expectProblem(t *testing.T, w *httptest.ResponseRecorder, status int, cause string, params ...string) {
	t.Helper()
	if w.Code != status {
		t.Fatalf("status = %d, want %d (body %s)", w.Code, status, w.Body.String())
	}
	if ct := w.Header().Get("Content-Type"); ct != "application/problem+json" {
		t.Errorf("Content-Type = %q", ct)
	}
	m := decode(t, w)
	if m["status"] != float64(status) || m["title"] != http.StatusText(status) {
		t.Errorf("problem = %v", m)
	}
	if cause != "" && m["cause"] != cause {
		t.Errorf("cause = %v, want %s", m["cause"], cause)
	}
	if _, ok := m["type"]; ok {
		t.Error("problem has type")
	}
	var got []string
	if ps, ok := m["invalidParams"].([]any); ok {
		for _, p := range ps {
			got = append(got, p.(map[string]any)["param"].(string))
		}
	}
	if strings.Join(got, ",") != strings.Join(params, ",") {
		t.Errorf("invalidParams = %v, want %v", got, params)
	}
}

func TestStatus(t *testing.T) {
	e := newEnv(t)
	e.mr.HSet(masterdata.SubscriberKey(testIMSI), "ki", "K")
	e.mr.HSet(masterdata.ClientKey(testIP), "secret", "s")

	w := e.do(http.MethodGet, "/admin/v1/status", nil)
	if w.Code != http.StatusOK {
		t.Fatalf("status = %d", w.Code)
	}
	m := decode(t, w)
	want := map[string]any{
		"version": "0.1.0-test", "nodeName": "node-a", "startedAt": "2026-10-07T00:00:00Z",
		"subscriberCount": float64(1), "clientCount": float64(1), "policyCount": float64(0),
	}
	for k, v := range want {
		if m[k] != v {
			t.Errorf("%s = %v, want %v", k, m[k], v)
		}
	}

	e.mr.SetError("forced error")
	expectProblem(t, e.do(http.MethodGet, "/admin/v1/status", nil), http.StatusInternalServerError, "SYSTEM_FAILURE")
}

func TestSubscriberLifecycle(t *testing.T) {
	e := newEnv(t)

	// 登録
	w := e.do(http.MethodPost, "/admin/v1/subscribers", map[string]string{"imsi": testIMSI, "ki": strings.ToUpper(testKi), "opc": testOPc},
		"X-Operator-Id", "alice@example.com", "X-Trace-ID", "trace-123")
	if w.Code != http.StatusCreated {
		t.Fatalf("create status = %d, body = %s", w.Code, w.Body.String())
	}
	if loc := w.Header().Get("Location"); loc != "/admin/v1/subscribers/"+testIMSI {
		t.Errorf("Location = %q", loc)
	}
	if w.Header().Get("X-Trace-ID") != "trace-123" {
		t.Errorf("X-Trace-ID = %q", w.Header().Get("X-Trace-ID"))
	}
	m := decode(t, w)
	if m["imsi"] != testIMSI || m["amf"] != "8000" || m["sqn"] != "000000000000" || m["createdAt"] == nil {
		t.Errorf("created = %v", m)
	}
	if _, ok := m["ki"]; ok {
		t.Error("response has ki")
	}
	expectProblem(t, e.do(http.MethodPost, "/admin/v1/subscribers", map[string]string{"imsi": testIMSI, "ki": testKi, "opc": testOPc}),
		http.StatusConflict, "SUBSCRIBER_ALREADY_EXISTS")

	// 取得（16進は小文字で返す）
	e.mr.HSet(masterdata.SubscriberKey(testIMSI), "sqn", "FF9BB4D0B607")
	m = decode(t, e.do(http.MethodGet, "/admin/v1/subscribers/"+testIMSI, nil))
	if m["sqn"] != "ff9bb4d0b607" {
		t.Errorf("sqn = %v", m["sqn"])
	}
	for _, k := range []string{"ki", "opc"} {
		if _, ok := m[k]; ok {
			t.Errorf("response has %s", k)
		}
	}

	// Ki / OPc の取得
	m = decode(t, e.do(http.MethodGet, "/admin/v1/subscribers/"+testIMSI+"/keys", nil))
	if m["ki"] != testKi || m["opc"] != testOPc {
		t.Errorf("keys = %v", m)
	}

	// 変更（SQN は変えない）
	w = e.do(http.MethodPatch, "/admin/v1/subscribers/"+testIMSI, `{"amf":"b9b9"}`)
	if w.Code != http.StatusOK {
		t.Fatalf("patch status = %d, body = %s", w.Code, w.Body.String())
	}
	if m = decode(t, w); m["amf"] != "b9b9" || m["sqn"] != "ff9bb4d0b607" {
		t.Errorf("patched = %v", m)
	}

	// 一覧
	m = decode(t, e.do(http.MethodGet, "/admin/v1/subscribers?prefix=00101&limit=10", nil))
	if items := m["items"].([]any); len(items) != 1 || m["total"] != float64(1) {
		t.Errorf("list = %v", m)
	}
	if _, ok := m["nextCursor"]; ok {
		t.Error("list has nextCursor on the last page")
	}

	// 削除
	if w = e.do(http.MethodDelete, "/admin/v1/subscribers/"+testIMSI, nil); w.Code != http.StatusNoContent || w.Body.Len() != 0 {
		t.Errorf("delete status = %d, body = %q", w.Code, w.Body.String())
	}
	expectProblem(t, e.do(http.MethodGet, "/admin/v1/subscribers/"+testIMSI, nil), http.StatusNotFound, "USER_NOT_FOUND")
	expectProblem(t, e.do(http.MethodDelete, "/admin/v1/subscribers/"+testIMSI, nil), http.StatusNotFound, "USER_NOT_FOUND")

	// 監査ログ: create / read / update / delete の4件。操作者と管理クライアントを記録する
	var ops []string
	for line := range strings.Lines(e.auditBu.String()) {
		var a map[string]any
		if err := json.Unmarshal([]byte(line), &a); err != nil {
			t.Fatal(err)
		}
		ops = append(ops, a["operation"].(string))
		if a["mgmt_client"] != "bff-01" {
			t.Errorf("mgmt_client = %v", a["mgmt_client"])
		}
		if a["operation"] == "create" && (a["admin_user"] != "alice@example.com" || a["trace_id"] != "trace-123") {
			t.Errorf("audit = %v", a)
		}
	}
	if got := strings.Join(ops, ","); got != "create,read,update,delete" {
		t.Errorf("audit operations = %s", got)
	}

	// アプリケーションログに秘密の値を出さず、パスの IMSI はマスクする
	logs := strings.ToLower(e.appLog.String())
	if strings.Contains(logs, testKi) || strings.Contains(logs, testOPc) || strings.Contains(logs, testIMSI) {
		t.Errorf("app log leaks a secret or an IMSI: %s", e.appLog.String())
	}
	if !strings.Contains(logs, `"path":"/admin/v1/subscribers/001010********1/keys"`) || !strings.Contains(logs, `"mgmt_client":"bff-01"`) {
		t.Errorf("request completed log = %s", e.appLog.String())
	}
}

func TestSubscriberPaging(t *testing.T) {
	e := newEnv(t)
	for _, imsi := range []string{"001010000000001", "001010000000002", "001010000000003"} {
		e.mr.HSet(masterdata.SubscriberKey(imsi), "ki", "K", "opc", "O", "amf", "8000", "sqn", "000000000000")
	}
	m := decode(t, e.do(http.MethodGet, "/admin/v1/subscribers?limit=2", nil))
	if m["total"] != float64(3) || m["nextCursor"] != "001010000000002" || len(m["items"].([]any)) != 2 {
		t.Fatalf("page 1 = %v", m)
	}
	// createdAt を持たない加入者では省略する
	if _, ok := m["items"].([]any)[0].(map[string]any)["createdAt"]; ok {
		t.Error("createdAt should be omitted")
	}
	m = decode(t, e.do(http.MethodGet, "/admin/v1/subscribers?limit=2&cursor=001010000000002", nil))
	if len(m["items"].([]any)) != 1 || m["nextCursor"] != nil {
		t.Errorf("page 2 = %v", m)
	}
	expectProblem(t, e.do(http.MethodGet, "/admin/v1/subscribers?limit=0&prefix=abc", nil), http.StatusBadRequest, "INVALID_QUERY_PARAM", "prefix", "limit")
	expectProblem(t, e.do(http.MethodGet, "/admin/v1/policies?cursor=x", nil), http.StatusBadRequest, "INVALID_QUERY_PARAM", "cursor")
}

func TestSubscriberValidation(t *testing.T) {
	e := newEnv(t)
	expectProblem(t, e.do(http.MethodPost, "/admin/v1/subscribers", map[string]string{"imsi": "123"}),
		http.StatusBadRequest, "MANDATORY_IE_MISSING", "ki", "opc", "imsi")
	expectProblem(t, e.do(http.MethodPost, "/admin/v1/subscribers", map[string]string{"imsi": testIMSI, "ki": testKi, "opc": testOPc, "amf": "1"}),
		http.StatusBadRequest, "OPTIONAL_IE_INCORRECT", "amf")
	expectProblem(t, e.do(http.MethodGet, "/admin/v1/subscribers/0010100000000011", nil), http.StatusBadRequest, "MANDATORY_IE_INCORRECT", "imsi")
	expectProblem(t, e.do(http.MethodPatch, "/admin/v1/subscribers/"+testIMSI, `{}`), http.StatusBadRequest, "MANDATORY_IE_MISSING")
	expectProblem(t, e.do(http.MethodPatch, "/admin/v1/subscribers/"+testIMSI, `{"sqn":null}`), http.StatusBadRequest, "OPTIONAL_IE_INCORRECT", "sqn")
	expectProblem(t, e.do(http.MethodPatch, "/admin/v1/subscribers/"+testIMSI, `{"amf":"8000"}`), http.StatusNotFound, "USER_NOT_FOUND")
	expectProblem(t, e.do(http.MethodGet, "/admin/v1/subscribers/"+testIMSI+"/keys", nil), http.StatusNotFound, "USER_NOT_FOUND")
	if e.auditBu.Len() != 0 {
		t.Errorf("failed requests wrote audit logs: %s", e.auditBu.String())
	}
}

func TestBodyFormat(t *testing.T) {
	e := newEnv(t)
	tests := []struct {
		name   string
		method string
		path   string
		body   string
		ct     string
	}{
		{"not JSON", http.MethodPost, "/admin/v1/subscribers", `{"imsi":`, "application/json"},
		{"unknown field", http.MethodPost, "/admin/v1/subscribers", `{"imsi":"001010000000001","foo":1}`, "application/json"},
		{"wrong type", http.MethodPost, "/admin/v1/clients", `{"ip":1}`, "application/json"},
		{"trailing data", http.MethodPost, "/admin/v1/clients", `{} {}`, "application/json"},
		{"array", http.MethodPut, "/admin/v1/policies/" + testIMSI, `[]`, "application/json"},
		{"no content type", http.MethodPost, "/admin/v1/subscribers", `{}`, ""},
		{"text content type", http.MethodPost, "/admin/v1/subscribers", `{}`, "text/plain"},
		{"merge patch on POST", http.MethodPost, "/admin/v1/subscribers", `{}`, "application/merge-patch+json"},
		{"patch wrong type", http.MethodPatch, "/admin/v1/clients/" + testIP, `{"name":1}`, "application/merge-patch+json"},
		{"patch unknown field", http.MethodPatch, "/admin/v1/clients/" + testIP, `{"ip":"10.0.0.1"}`, "application/merge-patch+json"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(tt.method, tt.path, strings.NewReader(tt.body))
			if tt.ct != "" {
				req.Header.Set("Content-Type", tt.ct)
			}
			w := httptest.NewRecorder()
			e.engine.ServeHTTP(w, req)
			expectProblem(t, w, http.StatusBadRequest, "INVALID_MSG_FORMAT")
		})
	}

	// PATCH は application/json でも受け付ける。charset 付きでもよい
	e.mr.HSet(masterdata.ClientKey(testIP), "secret", "s", "name", "AP", "vendor", "")
	req := httptest.NewRequest(http.MethodPatch, "/admin/v1/clients/"+testIP, strings.NewReader(`{"vendor":"v"}`))
	req.Header.Set("Content-Type", "application/json; charset=utf-8")
	w := httptest.NewRecorder()
	e.engine.ServeHTTP(w, req)
	if w.Code != http.StatusOK {
		t.Errorf("patch with application/json status = %d, body = %s", w.Code, w.Body.String())
	}
}

func TestClientLifecycle(t *testing.T) {
	e := newEnv(t)

	w := e.do(http.MethodPost, "/admin/v1/clients", map[string]string{"ip": testIP, "secret": testSecret, "name": "AP-OFFICE-01", "vendor": "generic"})
	if w.Code != http.StatusCreated || w.Header().Get("Location") != "/admin/v1/clients/"+testIP {
		t.Fatalf("create status = %d, Location = %q, body = %s", w.Code, w.Header().Get("Location"), w.Body.String())
	}
	if strings.Contains(w.Body.String(), testSecret) {
		t.Error("response has the secret")
	}
	expectProblem(t, e.do(http.MethodPost, "/admin/v1/clients", map[string]string{"ip": testIP, "secret": "s", "name": "AP"}),
		http.StatusConflict, "CLIENT_ALREADY_EXISTS")
	e.do(http.MethodPost, "/admin/v1/clients", map[string]string{"ip": "10.0.0.1", "secret": "s", "name": "AP-2"})

	m := decode(t, e.do(http.MethodGet, "/admin/v1/clients", nil))
	items := m["items"].([]any)
	if len(items) != 2 || items[0].(map[string]any)["ip"] != "10.0.0.1" {
		t.Errorf("list = %v", m)
	}
	if strings.Contains(e.do(http.MethodGet, "/admin/v1/clients", nil).Body.String(), testSecret) {
		t.Error("list has the secret")
	}

	m = decode(t, e.do(http.MethodGet, "/admin/v1/clients/"+testIP, nil))
	if m["name"] != "AP-OFFICE-01" || m["vendor"] != "generic" || m["secret"] != nil {
		t.Errorf("get = %v", m)
	}
	m = decode(t, e.do(http.MethodGet, "/admin/v1/clients/"+testIP+"/secret", nil))
	if m["secret"] != testSecret {
		t.Errorf("secret = %v", m)
	}
	m = decode(t, e.do(http.MethodPatch, "/admin/v1/clients/"+testIP, `{"name":"AP-OFFICE-02"}`))
	if m["name"] != "AP-OFFICE-02" || m["vendor"] != "generic" {
		t.Errorf("patched = %v", m)
	}

	if w = e.do(http.MethodDelete, "/admin/v1/clients/"+testIP, nil); w.Code != http.StatusNoContent {
		t.Errorf("delete status = %d", w.Code)
	}
	expectProblem(t, e.do(http.MethodGet, "/admin/v1/clients/"+testIP, nil), http.StatusNotFound, "CLIENT_NOT_FOUND")
	expectProblem(t, e.do(http.MethodGet, "/admin/v1/clients/"+testIP+"/secret", nil), http.StatusNotFound, "CLIENT_NOT_FOUND")
	expectProblem(t, e.do(http.MethodPatch, "/admin/v1/clients/"+testIP, `{"name":"x"}`), http.StatusNotFound, "CLIENT_NOT_FOUND")
	expectProblem(t, e.do(http.MethodDelete, "/admin/v1/clients/"+testIP, nil), http.StatusNotFound, "CLIENT_NOT_FOUND")
	expectProblem(t, e.do(http.MethodGet, "/admin/v1/clients/192.168.010.1", nil), http.StatusBadRequest, "MANDATORY_IE_INCORRECT", "ip")
	expectProblem(t, e.do(http.MethodPost, "/admin/v1/clients", map[string]string{"ip": testIP}), http.StatusBadRequest, "MANDATORY_IE_MISSING", "secret", "name")

	if strings.Contains(e.appLog.String(), testSecret) || strings.Contains(e.auditBu.String(), testSecret) {
		t.Error("logs contain the secret")
	}
}

func TestPolicyLifecycle(t *testing.T) {
	e := newEnv(t)
	body := `{"default":"deny","rules":[{"nasId":"AP-OFFICE-01","allowedSsids":["CORP-WIFI"],"vlanId":"100","sessionTimeout":3600},{"nasId":"*","allowedSsids":["GUEST-WIFI"]}]}`

	w := e.do(http.MethodPut, "/admin/v1/policies/"+testIMSI, body)
	if w.Code != http.StatusCreated || w.Header().Get("Location") != "/admin/v1/policies/"+testIMSI {
		t.Fatalf("put status = %d, Location = %q, body = %s", w.Code, w.Header().Get("Location"), w.Body.String())
	}
	want := `{"imsi":"001010000000001","default":"deny","rules":[{"nasId":"AP-OFFICE-01","allowedSsids":["CORP-WIFI"],"vlanId":"100","sessionTimeout":3600},{"nasId":"*","allowedSsids":["GUEST-WIFI"]}]}`
	if got := w.Body.String(); got != want {
		t.Errorf("put body = %s\nwant %s", got, want)
	}

	w = e.do(http.MethodPut, "/admin/v1/policies/"+testIMSI, `{"default":"allow","rules":[]}`)
	if w.Code != http.StatusOK || w.Header().Get("Location") != "" {
		t.Errorf("replace status = %d, Location = %q", w.Code, w.Header().Get("Location"))
	}
	if got := w.Body.String(); got != `{"imsi":"001010000000001","default":"allow","rules":[]}` {
		t.Errorf("replace body = %s", got)
	}

	m := decode(t, e.do(http.MethodGet, "/admin/v1/policies/"+testIMSI, nil))
	if m["default"] != "allow" {
		t.Errorf("get = %v", m)
	}
	m = decode(t, e.do(http.MethodGet, "/admin/v1/policies", nil))
	if m["total"] != float64(1) || len(m["items"].([]any)) != 1 {
		t.Errorf("list = %v", m)
	}

	if w = e.do(http.MethodDelete, "/admin/v1/policies/"+testIMSI, nil); w.Code != http.StatusNoContent {
		t.Errorf("delete status = %d", w.Code)
	}
	expectProblem(t, e.do(http.MethodGet, "/admin/v1/policies/"+testIMSI, nil), http.StatusNotFound, "POLICY_NOT_FOUND")
	expectProblem(t, e.do(http.MethodDelete, "/admin/v1/policies/"+testIMSI, nil), http.StatusNotFound, "POLICY_NOT_FOUND")
	expectProblem(t, e.do(http.MethodPut, "/admin/v1/policies/"+testIMSI, `{"default":"deny","rules":[{"nasId":"*","allowedSsids":[]}]}`),
		http.StatusBadRequest, "MANDATORY_IE_INCORRECT", "rules[0].allowedSsids")
	expectProblem(t, e.do(http.MethodPut, "/admin/v1/policies/"+testIMSI, `{"default":"deny","rules":[{"nasId":"*","allowedSsids":["A"],"foo":1}]}`),
		http.StatusBadRequest, "INVALID_MSG_FORMAT")
}

func TestOperatorAndTraceHeaders(t *testing.T) {
	e := newEnv(t)
	expectProblem(t, e.do(http.MethodGet, "/admin/v1/clients", nil, "X-Operator-Id", "bad operator"),
		http.StatusBadRequest, "OPTIONAL_IE_INCORRECT", "X-Operator-Id")

	// トレースIDがなければ採番する。長すぎる値は使わない
	for _, h := range []string{"", strings.Repeat("a", 65)} {
		w := e.do(http.MethodGet, "/admin/v1/clients", nil, "X-Trace-ID", h)
		if got := w.Header().Get("X-Trace-ID"); len(got) != 32 {
			t.Errorf("X-Trace-ID = %q", got)
		}
	}
}

func TestNotFoundAndMethodNotAllowed(t *testing.T) {
	e := newEnv(t)
	expectProblem(t, e.do(http.MethodGet, "/admin/v1/unknown", nil), http.StatusNotFound, "")
	expectProblem(t, e.do(http.MethodPost, "/admin/v1/status", nil), http.StatusMethodNotAllowed, "")
}

func TestInternalError(t *testing.T) {
	e := newEnv(t)
	e.mr.SetError("forced error")
	for _, path := range []string{"/admin/v1/subscribers", "/admin/v1/clients", "/admin/v1/policies", "/admin/v1/subscribers/" + testIMSI} {
		w := e.do(http.MethodGet, path, nil)
		expectProblem(t, w, http.StatusInternalServerError, "SYSTEM_FAILURE")
		if d := decode(t, w)["detail"]; d != nil {
			t.Errorf("500 has detail %v", d)
		}
	}
	for _, tc := range []struct{ method, path, body string }{
		{http.MethodPost, "/admin/v1/subscribers", `{"imsi":"001010000000001","ki":"` + testKi + `","opc":"` + testOPc + `"}`},
		{http.MethodPatch, "/admin/v1/subscribers/" + testIMSI, `{"amf":"8000"}`},
		{http.MethodDelete, "/admin/v1/subscribers/" + testIMSI, ""},
		{http.MethodGet, "/admin/v1/subscribers/" + testIMSI + "/keys", ""},
		{http.MethodPost, "/admin/v1/clients", `{"ip":"10.0.0.1","secret":"s","name":"AP"}`},
		{http.MethodGet, "/admin/v1/clients/" + testIP, ""},
		{http.MethodPatch, "/admin/v1/clients/" + testIP, `{"name":"AP"}`},
		{http.MethodDelete, "/admin/v1/clients/" + testIP, ""},
		{http.MethodGet, "/admin/v1/clients/" + testIP + "/secret", ""},
		{http.MethodGet, "/admin/v1/policies/" + testIMSI, ""},
		{http.MethodPut, "/admin/v1/policies/" + testIMSI, `{"default":"deny","rules":[]}`},
		{http.MethodDelete, "/admin/v1/policies/" + testIMSI, ""},
	} {
		var body any
		if tc.body != "" {
			body = tc.body
		}
		expectProblem(t, e.do(tc.method, tc.path, body), http.StatusInternalServerError, "SYSTEM_FAILURE")
	}
	if !strings.Contains(e.appLog.String(), `"event_id":"PROV_REQUEST_ERR"`) || strings.Contains(e.appLog.String(), testIMSI) {
		t.Errorf("error log = %s", e.appLog.String())
	}
}
