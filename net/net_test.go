package net

import (
	"reflect"
	"testing"
)

func TestGetTxFrom(t *testing.T) {
	if !reflect.DeepEqual(GetTxFrom("743a90e62590728a56c6078af55a38e74d1533f2430ca59c27d50f57fc34b8f1"), "TNYmZq4oppcQrAA55xydbD7GPtrR49ULL6") {
		t.Fail()
	}
}
