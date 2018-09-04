package model

import (
	"reflect"
	"testing"
	"time"
)

var (
	genCodeTestMeta = &Meta{
		created:   time.Now().Unix(),
		modified:  time.Now().Unix(),
		expires:   time.Now().Unix(),
		deleted:   time.Now().Unix(),
		secret:    true,
		cronjewel: true,
	}
)

func TestGenCode(t *testing.T) {
	encoded, err := genCodeTestMeta.GenCodeMarshal(nil)
	if err != nil {
		t.Fatal(err)
	}

	new := &Meta{}
	_, err = new.GenCodeUnmarshal(encoded)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(genCodeTestMeta, new) {
		t.Errorf("objects are not equal, got: %v", new)
	}
}
