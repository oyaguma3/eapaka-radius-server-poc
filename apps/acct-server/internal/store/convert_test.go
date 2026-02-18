package store

import (
	"testing"
)

type testStruct struct {
	Name   string `redis:"name"`
	Age    int    `redis:"age"`
	Score  int64  `redis:"score"`
	Level  uint8  `redis:"level"`
	Active bool   `redis:"active"`
	Skip   string `redis:"-"`
	NoTag  string
}

func TestStructToMap(t *testing.T) {
	s := testStruct{
		Name:   "alice",
		Age:    30,
		Score:  100,
		Level:  5,
		Active: true,
		Skip:   "should_skip",
		NoTag:  "no_tag",
	}
	m := StructToMap(s)

	if m["name"] != "alice" {
		t.Errorf("name: got %v, want alice", m["name"])
	}
	if m["age"] != 30 {
		t.Errorf("age: got %v, want 30", m["age"])
	}
	if m["score"] != int64(100) {
		t.Errorf("score: got %v, want 100", m["score"])
	}
	if m["level"] != uint8(5) {
		t.Errorf("level: got %v, want 5", m["level"])
	}
	if m["active"] != true {
		t.Errorf("active: got %v, want true", m["active"])
	}
}

func TestStructToMapSkipsTags(t *testing.T) {
	s := testStruct{Skip: "skip_val", NoTag: "notag_val"}
	m := StructToMap(s)

	if _, ok := m["Skip"]; ok {
		t.Error("redis:\"-\" tagged field should be skipped")
	}
	if _, ok := m["NoTag"]; ok {
		t.Error("field without redis tag should be skipped")
	}
}

func TestMapToStructAllTypes(t *testing.T) {
	m := map[string]string{
		"name":   "bob",
		"age":    "25",
		"score":  "999",
		"level":  "10",
		"active": "true",
	}
	var s testStruct
	if err := MapToStruct(m, &s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "bob" {
		t.Errorf("Name: got %v, want bob", s.Name)
	}
	if s.Age != 25 {
		t.Errorf("Age: got %v, want 25", s.Age)
	}
	if s.Score != 999 {
		t.Errorf("Score: got %v, want 999", s.Score)
	}
	if s.Level != 10 {
		t.Errorf("Level: got %v, want 10", s.Level)
	}
	if s.Active != true {
		t.Errorf("Active: got %v, want true", s.Active)
	}
}

func TestMapToStructMissingField(t *testing.T) {
	m := map[string]string{"name": "carol"}
	var s testStruct
	if err := MapToStruct(m, &s); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if s.Name != "carol" {
		t.Errorf("Name: got %v, want carol", s.Name)
	}
	if s.Age != 0 {
		t.Errorf("Age: got %v, want 0", s.Age)
	}
}

func TestMapToStructInvalidInt(t *testing.T) {
	m := map[string]string{"age": "not_a_number"}
	var s testStruct
	err := MapToStruct(m, &s)
	if err == nil {
		t.Fatal("expected error for invalid int value")
	}
}

func TestMapToStructInvalidBool(t *testing.T) {
	m := map[string]string{"active": "not_a_bool"}
	var s testStruct
	err := MapToStruct(m, &s)
	if err == nil {
		t.Fatal("expected error for invalid bool value")
	}
}
