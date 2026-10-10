package dto

import (
	"encoding/json"
	"net/http"
	"testing"

	"github.com/oyaguma3/eapaka-radius-server-poc/pkg/model"
)

func TestOptional(t *testing.T) {
	var u SubscriberUpdate
	if err := json.Unmarshal([]byte(`{"ki":null,"amf":"b9b9"}`), &u); err != nil {
		t.Fatalf("Unmarshal() error = %v", err)
	}
	if !u.Ki.Set || !u.Ki.Null {
		t.Errorf("ki = %+v, want set and null", u.Ki)
	}
	if !u.AMF.Set || u.AMF.Null || u.AMF.Value != "b9b9" {
		t.Errorf("amf = %+v", u.AMF)
	}
	if u.OPc.Set || u.SQN.Set || u.IsEmpty() {
		t.Errorf("opc = %+v, sqn = %+v", u.OPc, u.SQN)
	}
	if err := json.Unmarshal([]byte(`{"sqn":1}`), &u); err == nil {
		t.Error("wrong type accepted")
	}

	var c ClientUpdate
	if !c.IsEmpty() {
		t.Error("empty ClientUpdate is not empty")
	}
	if err := json.Unmarshal([]byte(`{"vendor":""}`), &c); err != nil || c.IsEmpty() {
		t.Errorf("ClientUpdate = %+v, %v", c, err)
	}
}

func TestNewSubscriber(t *testing.T) {
	sub := &model.Subscriber{IMSI: "001010000000001", Ki: "AABB", OPc: "CCDD", AMF: "B9B9", SQN: "FF9BB4D0B607", CreatedAt: "2026-10-07T12:00:00+09:00"}
	got := NewSubscriber(sub)
	if got.AMF != "b9b9" || got.SQN != "ff9bb4d0b607" || got.CreatedAt != "2026-10-07T03:00:00Z" {
		t.Errorf("NewSubscriber() = %+v", got)
	}
	if keys := NewSubscriberKeys(sub); keys.Ki != "aabb" || keys.OPc != "ccdd" {
		t.Errorf("NewSubscriberKeys() = %+v", keys)
	}
	// 解釈できない日時は省略する
	sub.CreatedAt = "yesterday"
	if got := NewSubscriber(sub); got.CreatedAt != "" {
		t.Errorf("CreatedAt = %q", got.CreatedAt)
	}
}

func TestNewPolicy(t *testing.T) {
	p := &model.Policy{IMSI: "001010000000001", Default: "deny", Rules: []model.PolicyRule{
		{NasID: "*", AllowedSSIDs: nil, VlanID: "10", SessionTimeout: 60},
	}}
	data, err := json.Marshal(NewPolicy(p))
	if err != nil {
		t.Fatal(err)
	}
	want := `{"imsi":"001010000000001","default":"deny","rules":[{"nasId":"*","allowedSsids":[],"vlanId":"10","sessionTimeout":60}],"status":"active"}`
	if string(data) != want {
		t.Errorf("NewPolicy() = %s, want %s", data, want)
	}
	p.Status = model.PolicyStatusSuspended
	if got := NewPolicy(p); got.Status != "suspended" {
		t.Errorf("NewPolicy().Status = %q, want suspended", got.Status)
	}
	if got := NewClient(&model.RadiusClient{IP: "10.0.0.1", Secret: "s", Name: "AP"}); got.IP != "10.0.0.1" || got.Name != "AP" {
		t.Errorf("NewClient() = %+v", got)
	}
}

func TestNewProblem(t *testing.T) {
	p := NewProblem(http.StatusConflict, CauseClientAlreadyExists, "client already exists")
	data, _ := json.Marshal(p)
	if string(data) != `{"title":"Conflict","status":409,"detail":"client already exists","cause":"CLIENT_ALREADY_EXISTS"}` {
		t.Errorf("NewProblem() = %s", data)
	}
}
