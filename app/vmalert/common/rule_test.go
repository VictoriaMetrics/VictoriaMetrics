package common

import "testing"

func TestRule_Validate(t *testing.T) {
	if err := (Rule{}).Validate(); err == nil {
		t.Errorf("exptected empty name error")
	}
	if err := (Rule{Name: "alert"}).Validate(); err == nil {
		t.Errorf("exptected empty expr error")
	}
	if err := (Rule{Name: "alert", Expr: "test{"}).Validate(); err == nil {
		t.Errorf("exptected invalid expr error")
	}
	if err := (Rule{Name: "alert", Expr: "test>0"}).Validate(); err != nil {
		t.Errorf("exptected valid rule got %s", err)
	}
}
