package kube

import "testing"

func TestRedactLineLiteral(t *testing.T) {
	got := RedactLine("create secret generic t --from-literal=password=secret")
	want := "create secret generic t --from-literal=password=***"
	if got != want {
		t.Fatal(got)
	}
}
