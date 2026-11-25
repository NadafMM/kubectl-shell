package kube

import "regexp"

type EngineView struct {
	Line   string
	Cursor int
	Delta  int
	Skip   bool
}

var leadWord = regexp.MustCompile(`^(\s*)(\S*)`)

func ToEngine(line string, cursor int, mode string) EngineView {
	c := cursor
	if c < 0 {
		c = 0
	}
	if c > len(line) {
		c = len(line)
	}
	match := leadWord.FindStringSubmatch(line)
	lead, first := "", ""
	if match != nil {
		lead, first = match[1], match[2]
	}
	firstEnd := len(lead) + len(first)

	if mode == "shell" && (first == "kube" || first == "kubectl-shell") {
		if c <= firstEnd {
			return EngineView{Line: line, Cursor: c, Delta: 0, Skip: true}
		}
		delta := len("kubectl") - len(first)
		return EngineView{
			Line:   lead + "kubectl" + line[firstEnd:],
			Cursor: c + delta,
			Delta:  delta,
			Skip:   false,
		}
	}

	if first == "kubectl" || first == "k" || first == "help" || shellMetaFirstWord(first) {
		return EngineView{Line: line, Cursor: c, Delta: 0, Skip: false}
	}

	if first == "" {
		engineLine := lead + "kubectl "
		return EngineView{
			Line:   engineLine,
			Cursor: len(engineLine),
			Delta:  len(engineLine) - len(lead),
			Skip:   false,
		}
	}

	delta := len("kubectl ")
	nextCursor := c
	if c > len(lead) {
		nextCursor = c + delta
	}
	return EngineView{
		Line:   lead + "kubectl " + line[len(lead):],
		Cursor: nextCursor,
		Delta:  delta,
		Skip:   false,
	}
}

var trailingPad = regexp.MustCompile(`[=/]$`)

func SpliceToken(line string, start, end int, value string, pad bool) (string, int) {
	after := ""
	if end <= len(line) {
		after = line[end:]
	}
	spaced := pad && !trailingPad.MatchString(value)
	if !spaced || (len(after) > 0 && after[0] == ' ') {
		next := line[:start] + value + after
		cursor := start + len(value)
		if spaced && len(after) > 0 && after[0] == ' ' {
			cursor++
		}
		return next, cursor
	}
	return line[:start] + value + " " + after, start + len(value) + 1
}
