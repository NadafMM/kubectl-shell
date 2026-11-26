package parse

import "testing"

func TestToEngineReplInsertsKubectl(t *testing.T) {
	got := ToEngine("get pods", 0, "repl")
	if got.Line != "kubectl get pods" || got.Cursor != 0 || got.Delta != 8 || got.Skip {
		t.Fatalf("%#v", got)
	}
	end := ToEngine("get pods", 8, "repl")
	if end.Line != "kubectl get pods" || end.Cursor != 16 || end.Delta != 8 || end.Skip {
		t.Fatalf("%#v", end)
	}
}

func TestToEngineShellRewritesKube(t *testing.T) {
	if !ToEngine("kube get pods", 2, "shell").Skip {
		t.Fatal("cursor inside kube should skip")
	}
	line := "kube get pods"
	got := ToEngine(line, len(line), "shell")
	if got.Line != "kubectl get pods" || got.Cursor != len("kubectl get pods") || got.Delta != 3 || got.Skip {
		t.Fatalf("%#v", got)
	}
}

func TestToEngineKeepsView(t *testing.T) {
	got := ToEngine("view", 4, "repl")
	if got.Line != "view" || got.Cursor != 4 || got.Delta != 0 || got.Skip {
		t.Fatalf("%#v", got)
	}
}

func TestSpliceTokenPads(t *testing.T) {
	line, cursor := SpliceToken("kubectl get ", 12, 12, "pods", true)
	if line != "kubectl get pods " || cursor != 17 {
		t.Fatalf("%q %d", line, cursor)
	}
}
