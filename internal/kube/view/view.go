package view

import (
	"github.com/NadafMM/kubectl-shell/internal/kube"
	"os"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"
)

// Board is one resource pane in kube view.
type Board string

const (
	BoardPod        Board = "pod"
	BoardDeployment Board = "deployment"
	BoardService    Board = "service"
	BoardNode       Board = "node"
	BoardNamespace  Board = "namespace"
)

// Row is one line in the resource list.
type Row struct {
	Name   string
	NS     string
	Info   string
	Status string
	Age    string
	Kind   string
}

type boardLabel struct {
	ID    Board
	Label string
}

var boards = []boardLabel{
	{BoardPod, "pods"},
	{BoardDeployment, "deploy"},
	{BoardService, "svc"},
	{BoardNode, "nodes"},
	{BoardNamespace, "ns"},
}

var (
	toneBad  = regexp.MustCompile(`(?i)CrashLoop|Error|ImagePull|BackOff|Failed|Evicted|NotReady`)
	toneWarn = regexp.MustCompile(`(?i)Pending|Unknown`)
	toneOK   = regexp.MustCompile(`(?i)Running|Active|Ready|Bound|Succeeded`)
)

// BoardFrom maps a CLI word to a view board.
func BoardFrom(word string) Board {
	token := strings.ToLower(strings.TrimSpace(word))
	if token == "" {
		token = "pods"
	}
	switch token {
	case "po", "pod", "pods":
		return BoardPod
	case "deploy", "deployment", "deployments":
		return BoardDeployment
	case "svc", "service", "services":
		return BoardService
	case "no", "node", "nodes":
		return BoardNode
	case "ns", "namespace", "namespaces":
		return BoardNamespace
	default:
		return BoardPod
	}
}

// DeleteEngineLine builds a kubectl delete line for a selected row.
func DeleteEngineLine(row Row) string {
	parts := []string{"kubectl", "delete", row.Kind, row.Name}
	if row.NS != "" {
		parts = append(parts, "-n", row.NS)
	}
	return strings.Join(parts, " ")
}

func toneOf(status string) string {
	if toneBad.MatchString(status) {
		return "bad"
	}
	if toneWarn.MatchString(status) {
		return "warn"
	}
	if toneOK.MatchString(status) {
		return "ok"
	}
	return "plain"
}

func objectRow(item kube.KObject) Row {
	info := ""
	if item.Kind == "service" {
		info = item.SvcType
	}
	status := item.Status
	if status == "" {
		status = item.Kind
	}
	return Row{
		Name:   item.Name,
		NS:     item.NS,
		Info:   info,
		Status: status,
		Age:    item.Age,
		Kind:   item.Kind,
	}
}

func byProblem(a, b Row) bool {
	rank := func(row Row) int {
		switch toneOf(row.Status) {
		case "bad":
			return 0
		case "warn":
			return 1
		default:
			return 2
		}
	}
	ra, rb := rank(a), rank(b)
	if ra != rb {
		return ra < rb
	}
	if a.NS != b.NS {
		return a.NS < b.NS
	}
	return a.Name < b.Name
}

// RowsOf builds the visible rows for a board.
func RowsOf(cluster kube.Cluster, board Board, ns string, all bool, filter string) []Row {
	needle := strings.ToLower(strings.TrimSpace(filter))
	keep := func(row Row) bool {
		if needle == "" {
			return true
		}
		hay := strings.ToLower(row.Name + " " + row.NS + " " + row.Status)
		return strings.Contains(hay, needle)
	}
	var rows []Row
	switch board {
	case BoardNode:
		for _, node := range cluster.Nodes {
			info := node.Roles
			if info == "" {
				info = node.Version
			}
			row := Row{Name: node.Name, Status: node.Status, Age: node.Age, Kind: "node", Info: info}
			if keep(row) {
				rows = append(rows, row)
			}
		}
	case BoardNamespace:
		for _, item := range cluster.Namespaces {
			row := Row{Name: item.Name, Status: item.Status, Age: item.Age, Kind: "namespace"}
			if keep(row) {
				rows = append(rows, row)
			}
		}
	default:
		for _, item := range cluster.Objects {
			if item.Kind != string(board) {
				continue
			}
			if !all && item.NS != ns {
				continue
			}
			row := objectRow(item)
			if keep(row) {
				rows = append(rows, row)
			}
		}
	}
	sort.SliceStable(rows, func(i, j int) bool { return byProblem(rows[i], rows[j]) })
	return rows
}

// TableText is the non-TTY kube view output.
func TableText(cluster kube.Cluster, board Board, all bool) string {
	ns := kube.NamespaceOf(cluster)
	rows := RowsOf(cluster, board, ns, all, "")
	var lines []string
	if all {
		lines = append(lines, "NAMESPACE  NAME  STATUS")
	} else {
		lines = append(lines, "NAME  STATUS")
	}
	for _, row := range rows {
		prefix := ""
		if all && row.NS != "" {
			prefix = row.NS + "  "
		}
		lines = append(lines, prefix+row.Name+"  "+row.Status)
	}
	return strings.Join(lines, "\n")
}

func fit(text string, width int) string {
	clean := strings.Join(strings.Fields(text), " ")
	if width <= 0 {
		return ""
	}
	if utf8.RuneCountInString(clean) >= width {
		runes := []rune(clean)
		return string(runes[:width])
	}
	return clean + strings.Repeat(" ", width-utf8.RuneCountInString(clean))
}

func colorDisabled() bool {
	return os.Getenv("NO_COLOR") != ""
}

func paint(text, tone string) string {
	if colorDisabled() {
		return text
	}
	switch tone {
	case "selected":
		return "\x1b[7m" + text + "\x1b[0m"
	case "head":
		return "\x1b[1m" + text + "\x1b[0m"
	case "bad":
		return "\x1b[31m" + text + "\x1b[0m"
	case "warn":
		return "\x1b[33m" + text + "\x1b[0m"
	case "ok":
		return "\x1b[32m" + text + "\x1b[0m"
	default:
		return text
	}
}

func boardLabelOf(board Board) string {
	for _, item := range boards {
		if item.ID == board {
			return item.Label
		}
	}
	return string(board)
}
