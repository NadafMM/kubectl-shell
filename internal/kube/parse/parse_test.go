package parse

import (
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"testing"
)

func TestTokenizeKeepsQuotes(t *testing.T) {
	tokens := Tokenize(`kubectl get pods -l "app=web"`)
	var got []string
	for _, token := range tokens {
		got = append(got, token.Text)
	}
	want := []string{"kubectl", "get", "pods", "-l", `"app=web"`}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v", got)
		}
	}
}

func TestPositionalWordsDropsNamespace(t *testing.T) {
	parsed := PositionalWords([]string{"get", "-n", "app", "pods"})
	if len(parsed.Words) != 2 || parsed.Words[0] != "get" || parsed.Words[1] != "pods" {
		t.Fatalf("got %#v", parsed.Words)
	}
}

func TestScopeOfReadsNamespace(t *testing.T) {
	scope := ScopeOf([]string{"kubectl", "get", "pods", "-n", "app"}, kube.InitialCluster())
	if scope.NS != "app" || scope.Context != "minikube" || scope.All {
		t.Fatalf("%#v", scope)
	}
}

func TestFlagValueEquals(t *testing.T) {
	got, ok := FlagValue([]string{"scale", "deploy", "api", "--replicas=0"}, []string{"--replicas"})
	if !ok || got != "0" {
		t.Fatalf("%q %v", got, ok)
	}
}

func TestUnquote(t *testing.T) {
	if Unquote(`"app=web"`) != "app=web" || Unquote("app=web") != "app=web" || Unquote(`"app`) != `"app` {
		t.Fatal("unquote")
	}
}

func TestQuoteArgRoundTrip(t *testing.T) {
	if got, ok := QuoteArg("pods"); !ok || got != "pods" {
		t.Fatal(got, ok)
	}
	if got, ok := QuoteArg("my file.yaml"); !ok || got != `"my file.yaml"` {
		t.Fatal(got, ok)
	}
	if got, ok := QuoteArg(`say "hi"`); !ok || got != `'say "hi"'` {
		t.Fatal(got, ok)
	}
	if _, ok := QuoteArg(`a "b" 'c'`); ok {
		t.Fatal("both quotes should fail")
	}
	args := []string{"apply", "-f", "my file.yaml"}
	line := ""
	for i, arg := range args {
		quoted, ok := QuoteArg(arg)
		if !ok {
			t.Fatal(arg)
		}
		if i > 0 {
			line += " "
		}
		line += quoted
	}
	var got []string
	for _, token := range Tokenize(line) {
		got = append(got, Unquote(token.Text))
	}
	if len(got) != len(args) {
		t.Fatal(got)
	}
	for i := range args {
		if got[i] != args[i] {
			t.Fatal(got)
		}
	}
}
