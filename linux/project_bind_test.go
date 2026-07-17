package main

import (
	"reflect"
	"testing"
)

func TestParseServiceSelectionAllowsNone(t *testing.T) {
	project := Project{Services: []Service{{Name: "api"}, {Name: "web"}}}
	indexes, err := parseServiceSelection(project, "none")
	if err != nil {
		t.Fatal(err)
	}
	if len(indexes) != 0 {
		t.Fatalf("indexes=%v, want none", indexes)
	}

	indexes, err = parseServiceSelection(project, "2,api,2")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(indexes, []int{1, 0}) {
		t.Fatalf("indexes=%v", indexes)
	}
}
