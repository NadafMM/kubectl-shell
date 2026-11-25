package kube

import "strings"

// ViewMode is the active pane inside kube view.
type ViewMode string

const (
	ViewModeList    ViewMode = "list"
	ViewModeText    ViewMode = "text"
	ViewModeNS      ViewMode = "ns"
	ViewModeHelp    ViewMode = "help"
	ViewModeConfirm ViewMode = "confirm"
)

// ViewEffect is a side effect the view loop applies after a key.
type ViewEffect struct {
	Type  string // none, quit, reload, describe, logs, arm-delete, delete
	Index int
}

// ViewState holds keyboard-driven UI state for kube view.
type ViewState struct {
	Board        Board
	All          bool
	NS           string
	Filter       string
	Filtering    bool
	Cursor       int
	NSCursor     int
	Mode         ViewMode
	Scroll       int
	TextLength   int
	Command      string
	Commanding   bool
	Message      string
	PendingIndex *int
	Yes          bool
}

// NewViewState builds the starting state for a board.
func NewViewState(board Board, all bool, ns string, yes bool) ViewState {
	return ViewState{
		Board: board,
		All:   all,
		NS:    ns,
		Yes:   yes,
		Mode:  ViewModeList,
	}
}

func listMode(state ViewState, key string, rows []Row) (ViewState, ViewEffect) {
	cursor := state.Cursor
	if cursor > len(rows)-1 {
		if len(rows) == 0 {
			cursor = 0
		} else {
			cursor = len(rows) - 1
		}
	}
	base := state
	base.Cursor = cursor
	none := ViewEffect{Type: "none"}
	switch key {
	case "j", "down":
		if len(rows) > 0 {
			base.Cursor = min(len(rows)-1, cursor+1)
		}
		return base, none
	case "k", "up":
		base.Cursor = max(0, cursor-1)
		return base, none
	case "g":
		base.Cursor = 0
		return base, none
	case "G":
		base.Cursor = max(0, len(rows)-1)
		return base, none
	case "/":
		base.Filtering = true
		return base, none
	case ":":
		base.Commanding = true
		return base, none
	case "?":
		base.Mode = ViewModeHelp
		return base, none
	case "a":
		base.All = !state.All
		base.Cursor = 0
		return base, ViewEffect{Type: "reload"}
	case "r":
		return base, ViewEffect{Type: "reload"}
	case "n":
		base.Mode = ViewModeNS
		return base, none
	}
	if len(key) == 1 && key >= "1" && key <= "5" {
		boards := []Board{BoardPod, BoardDeployment, BoardService, BoardNode, BoardNamespace}
		base.Board = boards[key[0]-'1']
		base.Cursor = 0
		return base, none
	}
	var row *Row
	if cursor >= 0 && cursor < len(rows) {
		row = &rows[cursor]
	}
	if key == "\r" {
		if row == nil {
			return base, none
		}
		if row.Kind == "namespace" {
			base.NS = row.Name
			base.All = false
			base.Board = BoardPod
			base.Cursor = 0
			return base, ViewEffect{Type: "reload"}
		}
		return base, ViewEffect{Type: "describe", Index: cursor}
	}
	if key == "l" {
		if row == nil || row.Kind != "pod" {
			base.Message = "logs are for pods"
			return base, none
		}
		return base, ViewEffect{Type: "logs", Index: cursor}
	}
	if key == "d" {
		if row == nil || row.Kind == "node" || row.Kind == "namespace" {
			base.Message = "pick a pod, deployment, or service"
			return base, none
		}
		if state.Yes {
			return base, ViewEffect{Type: "delete", Index: cursor}
		}
		pending := cursor
		base.PendingIndex = &pending
		base.Mode = ViewModeConfirm
		return base, ViewEffect{Type: "arm-delete", Index: cursor}
	}
	return base, none
}

// Reduce applies one key to the view state.
func Reduce(state ViewState, key string, rows []Row, namespaces []string) (ViewState, ViewEffect) {
	none := ViewEffect{Type: "none"}
	if state.Commanding {
		if key == "\r" || key == "\n" {
			word := strings.ToLower(strings.TrimSpace(state.Command))
			next := state
			next.Commanding = false
			next.Command = ""
			next.Mode = ViewModeList
			if word == "q" || word == "quit" {
				return next, ViewEffect{Type: "quit"}
			}
			if word == "all" {
				next.All = true
				return next, ViewEffect{Type: "reload"}
			}
			if word != "" {
				boards := map[string]Board{
					"pods": BoardPod, "pod": BoardPod, "po": BoardPod,
					"deploy": BoardDeployment, "deployment": BoardDeployment, "deployments": BoardDeployment,
					"svc": BoardService, "service": BoardService, "services": BoardService,
					"nodes": BoardNode, "node": BoardNode, "no": BoardNode,
					"ns": BoardNamespace, "namespace": BoardNamespace, "namespaces": BoardNamespace,
				}
				if board, ok := boards[word]; ok {
					next.Board = board
				} else {
					next.Board = BoardPod
				}
				next.Cursor = 0
				return next, none
			}
			return next, none
		}
		if key == "esc" {
			state.Commanding = false
			state.Command = ""
			return state, none
		}
		if key == "\x7f" || key == "del" {
			if state.Command != "" {
				state.Command = state.Command[:len(state.Command)-1]
			}
			return state, none
		}
		if len(key) == 1 && key[0] >= ' ' {
			state.Command += key
			return state, none
		}
		return state, none
	}

	if state.Filtering {
		if key == "\r" || key == "\n" || key == "esc" {
			state.Filtering = false
			return state, none
		}
		if key == "\x7f" || key == "del" {
			if state.Filter != "" {
				state.Filter = state.Filter[:len(state.Filter)-1]
			}
			state.Cursor = 0
			return state, none
		}
		if len(key) == 1 && key[0] >= ' ' {
			state.Filter += key
			state.Cursor = 0
			return state, none
		}
		return state, none
	}

	if state.Mode == ViewModeConfirm {
		if (key == "y" || key == "Y") && state.PendingIndex != nil {
			index := *state.PendingIndex
			state.Mode = ViewModeList
			state.PendingIndex = nil
			return state, ViewEffect{Type: "delete", Index: index}
		}
		state.Mode = ViewModeList
		state.PendingIndex = nil
		state.Message = "not deleted"
		return state, none
	}

	if key == "q" || key == "\x03" {
		if state.Mode != ViewModeList {
			state.Mode = ViewModeList
			state.Message = ""
			return state, none
		}
		return state, ViewEffect{Type: "quit"}
	}
	if key == "esc" {
		state.Mode = ViewModeList
		state.Message = ""
		return state, none
	}

	if state.Mode == ViewModeText {
		last := max(0, state.TextLength-1)
		switch key {
		case "j", "down":
			state.Scroll = min(last, state.Scroll+1)
		case "k", "up":
			state.Scroll = max(0, state.Scroll-1)
		case "pgdn", " ":
			state.Scroll = min(last, state.Scroll+10)
		case "pgup":
			state.Scroll = max(0, state.Scroll-10)
		case "g":
			state.Scroll = 0
		}
		return state, none
	}

	if state.Mode == ViewModeNS {
		nsCursor := state.NSCursor
		if nsCursor > len(namespaces)-1 {
			if len(namespaces) == 0 {
				nsCursor = 0
			} else {
				nsCursor = len(namespaces) - 1
			}
		}
		state.NSCursor = nsCursor
		switch key {
		case "j", "down":
			if len(namespaces) > 0 {
				state.NSCursor = min(len(namespaces)-1, nsCursor+1)
			}
			return state, none
		case "k", "up":
			state.NSCursor = max(0, nsCursor-1)
			return state, none
		case "\r":
			if state.NSCursor < 0 || state.NSCursor >= len(namespaces) {
				state.Mode = ViewModeList
				return state, none
			}
			state.NS = namespaces[state.NSCursor]
			state.All = false
			state.Board = BoardPod
			state.Cursor = 0
			state.Mode = ViewModeList
			return state, ViewEffect{Type: "reload"}
		}
		return state, none
	}

	if state.Mode == ViewModeHelp {
		return state, none
	}
	return listMode(state, key, rows)
}
