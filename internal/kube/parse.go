package kube

import "strings"

type Token struct {
	Text  string
	Start int
	End   int
}

func Tokenize(line string) []Token {
	var tokens []Token
	i := 0
	for i < len(line) {
		for i < len(line) && line[i] == ' ' {
			i++
		}
		if i >= len(line) {
			break
		}
		start := i
		var text strings.Builder
		var quote byte
		if line[i] == '"' || line[i] == '\'' {
			quote = line[i]
		}
		if quote != 0 {
			text.WriteByte(line[i])
			i++
			for i < len(line) && line[i] != quote {
				text.WriteByte(line[i])
				i++
			}
			if i < len(line) {
				text.WriteByte(line[i])
				i++
			}
		} else {
			for i < len(line) && line[i] != ' ' {
				text.WriteByte(line[i])
				i++
			}
		}
		tokens = append(tokens, Token{Text: text.String(), Start: start, End: i})
	}
	return tokens
}

type Located struct {
	Start  int
	End    int
	Index  int
	Tokens []Token
}

func Locate(line string, cursor int) Located {
	tokens := Tokenize(line)
	c := cursor
	if c < 0 {
		c = 0
	}
	if c > len(line) {
		c = len(line)
	}
	for i, token := range tokens {
		if c < token.Start {
			return Located{Start: c, End: c, Index: i, Tokens: tokens}
		}
		if c <= token.End {
			return Located{Start: token.Start, End: token.End, Index: i, Tokens: tokens}
		}
	}
	return Located{Start: c, End: c, Index: len(tokens), Tokens: tokens}
}

type Parsed struct {
	Words         []string
	AfterDashDash bool
	DashWords     []string
}

// PositionalWords drops flags and the values they consume.
func PositionalWords(tokens []string) Parsed {
	var words, dashWords []string
	after := false
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if after {
			dashWords = append(dashWords, token)
			continue
		}
		if token == "--" {
			after = true
			continue
		}
		var verb string
		if len(words) > 0 {
			verb = words[0]
		}
		if strings.HasPrefix(token, "-") && token != "-" {
			if !strings.Contains(token, "=") && TakesOf(token, verb) != "" {
				i++
			}
			continue
		}
		words = append(words, token)
	}
	return Parsed{Words: words, AfterDashDash: after, DashWords: dashWords}
}

type Scope struct {
	NS      string
	All     bool
	Context string
}

func ScopeOf(tokens []string, cluster Cluster) Scope {
	context := cluster.Current
	ns := NamespaceOf(cluster)
	all := false
	for i := 0; i < len(tokens); i++ {
		token := tokens[i]
		if token == "-A" || token == "--all-namespaces" {
			all = true
			continue
		}
		if token == "--context" && i+1 < len(tokens) {
			context = tokens[i+1]
			i++
			if found, ok := findContext(cluster, context); ok {
				ns = found.Namespace
			}
			continue
		}
		if strings.HasPrefix(token, "--context=") {
			context = strings.TrimPrefix(token, "--context=")
			if found, ok := findContext(cluster, context); ok {
				ns = found.Namespace
			}
			continue
		}
		if (token == "-n" || token == "--namespace") && i+1 < len(tokens) {
			ns = tokens[i+1]
			i++
			continue
		}
		if strings.HasPrefix(token, "--namespace=") {
			ns = strings.TrimPrefix(token, "--namespace=")
		}
	}
	return Scope{NS: ns, All: all, Context: context}
}

func findContext(cluster Cluster, name string) (Context, bool) {
	for _, item := range cluster.Contexts {
		if item.Name == name {
			return item, true
		}
	}
	return Context{}, false
}

func VerbOf(args []string) string {
	words := PositionalWords(args).Words
	if len(words) == 0 {
		return ""
	}
	return words[0]
}

func FlagsForVerb(verb string) []FlagDef {
	return FlagCatalog(verb)
}

// QuoteArg quotes an argument for a shell line. The second result is false when
// the argument contains both quote styles and cannot be passed.
func QuoteArg(text string) (string, bool) {
	hasSingle := strings.Contains(text, "'")
	hasDouble := strings.Contains(text, `"`)
	if !strings.Contains(text, " ") && !hasSingle && !hasDouble {
		return text, true
	}
	if hasSingle && hasDouble {
		return "", false
	}
	if hasDouble {
		return "'" + text + "'", true
	}
	return `"` + text + `"`, true
}

func Unquote(text string) string {
	if len(text) >= 2 && ((strings.HasPrefix(text, `"`) && strings.HasSuffix(text, `"`)) ||
		(strings.HasPrefix(text, "'") && strings.HasSuffix(text, "'"))) {
		return text[1 : len(text)-1]
	}
	return text
}

func FlagValue(tokens []string, names []string) (string, bool) {
	for i, token := range tokens {
		for _, name := range names {
			if token == name && i+1 < len(tokens) {
				return tokens[i+1], true
			}
			if strings.HasPrefix(token, name+"=") {
				return token[len(name)+1:], true
			}
		}
	}
	return "", false
}
