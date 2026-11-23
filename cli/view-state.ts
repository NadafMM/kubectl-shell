import type { Board, Row } from "./view";

export type ViewMode = "list" | "text" | "ns" | "help" | "confirm";

export type ViewEffect =
  | { type: "none" }
  | { type: "quit" }
  | { type: "reload" }
  | { type: "describe"; index: number }
  | { type: "logs"; index: number }
  | { type: "arm-delete"; index: number }
  | { type: "delete"; index: number };

export type ViewState = {
  board: Board;
  all: boolean;
  ns: string;
  filter: string;
  filtering: boolean;
  cursor: number;
  nsCursor: number;
  mode: ViewMode;
  scroll: number;
  textLength: number;
  command: string;
  commanding: boolean;
  message: string;
  pendingIndex: number | null;
  yes: boolean;
};

export function viewState(
  partial: Pick<ViewState, "board" | "all" | "ns" | "yes"> & Partial<ViewState>,
): ViewState {
  return {
    filter: "",
    filtering: false,
    cursor: 0,
    nsCursor: 0,
    mode: "list",
    scroll: 0,
    textLength: 0,
    command: "",
    commanding: false,
    message: "",
    pendingIndex: null,
    ...partial,
  };
}

function listMode(
  state: ViewState,
  key: string,
  rows: Row[],
): { state: ViewState; effect: ViewEffect } {
  const cursor = Math.min(state.cursor, Math.max(0, rows.length - 1));
  const base = { ...state, cursor };
  if (key === "j" || key === "down")
    return {
      state: { ...base, cursor: Math.min(rows.length - 1, cursor + 1) },
      effect: { type: "none" },
    };
  if (key === "k" || key === "up")
    return { state: { ...base, cursor: Math.max(0, cursor - 1) }, effect: { type: "none" } };
  if (key === "g") return { state: { ...base, cursor: 0 }, effect: { type: "none" } };
  if (key === "G")
    return { state: { ...base, cursor: Math.max(0, rows.length - 1) }, effect: { type: "none" } };
  if (key === "/") return { state: { ...base, filtering: true }, effect: { type: "none" } };
  if (key === ":") return { state: { ...base, commanding: true }, effect: { type: "none" } };
  if (key === "?") return { state: { ...base, mode: "help" }, effect: { type: "none" } };
  if (key === "a")
    return { state: { ...base, all: !state.all, cursor: 0 }, effect: { type: "reload" } };
  if (key === "r") return { state: base, effect: { type: "reload" } };
  if (key === "n") return { state: { ...base, mode: "ns" }, effect: { type: "none" } };
  if ("12345".includes(key)) {
    const boards: Board[] = ["pod", "deployment", "service", "node", "namespace"];
    return {
      state: { ...base, board: boards[Number(key) - 1]!, cursor: 0 },
      effect: { type: "none" },
    };
  }
  const row = rows[cursor];
  if (key === "\r") {
    if (!row) return { state: base, effect: { type: "none" } };
    if (row.kind === "namespace") {
      return {
        state: { ...base, ns: row.name, all: false, board: "pod", cursor: 0 },
        effect: { type: "reload" },
      };
    }
    return { state: base, effect: { type: "describe", index: cursor } };
  }
  if (key === "l") {
    if (!row || row.kind !== "pod")
      return { state: { ...base, message: "logs are for pods" }, effect: { type: "none" } };
    return { state: base, effect: { type: "logs", index: cursor } };
  }
  if (key === "d") {
    if (!row || row.kind === "node" || row.kind === "namespace") {
      return {
        state: { ...base, message: "pick a pod, deployment, or service" },
        effect: { type: "none" },
      };
    }
    if (state.yes) return { state: base, effect: { type: "delete", index: cursor } };
    return {
      state: { ...base, pendingIndex: cursor, mode: "confirm" },
      effect: { type: "arm-delete", index: cursor },
    };
  }
  return { state: base, effect: { type: "none" } };
}

/** One key in the resource screen. Effects are applied by the caller. */
export function reduce(
  state: ViewState,
  key: string,
  ctx: { rows: Row[]; namespaces: string[] },
): { state: ViewState; effect: ViewEffect } {
  if (state.commanding) {
    if (key === "\r" || key === "\n") {
      const word = state.command.trim().toLowerCase();
      const next = { ...state, commanding: false, command: "", mode: "list" as const };
      if (word === "q" || word === "quit") return { state: next, effect: { type: "quit" } };
      if (word === "all") return { state: { ...next, all: true }, effect: { type: "reload" } };
      if (word) {
        const boards: Record<string, Board> = {
          pods: "pod",
          pod: "pod",
          po: "pod",
          deploy: "deployment",
          deployment: "deployment",
          deployments: "deployment",
          svc: "service",
          service: "service",
          services: "service",
          nodes: "node",
          node: "node",
          no: "node",
          ns: "namespace",
          namespace: "namespace",
          namespaces: "namespace",
        };
        return {
          state: { ...next, board: boards[word] ?? "pod", cursor: 0 },
          effect: { type: "none" },
        };
      }
      return { state: next, effect: { type: "none" } };
    }
    if (key === "esc")
      return { state: { ...state, commanding: false, command: "" }, effect: { type: "none" } };
    if (key === "\x7f" || key === "del")
      return { state: { ...state, command: state.command.slice(0, -1) }, effect: { type: "none" } };
    if (key.length === 1 && key >= " ")
      return { state: { ...state, command: state.command + key }, effect: { type: "none" } };
    return { state, effect: { type: "none" } };
  }

  if (state.filtering) {
    if (key === "\r" || key === "\n" || key === "esc")
      return { state: { ...state, filtering: false }, effect: { type: "none" } };
    if (key === "\x7f" || key === "del")
      return {
        state: { ...state, filter: state.filter.slice(0, -1), cursor: 0 },
        effect: { type: "none" },
      };
    if (key.length === 1 && key >= " ")
      return {
        state: { ...state, filter: state.filter + key, cursor: 0 },
        effect: { type: "none" },
      };
    return { state, effect: { type: "none" } };
  }

  if (state.mode === "confirm") {
    if ((key === "y" || key === "Y") && state.pendingIndex !== null) {
      return {
        state: { ...state, mode: "list", pendingIndex: null },
        effect: { type: "delete", index: state.pendingIndex },
      };
    }
    return {
      state: { ...state, mode: "list", pendingIndex: null, message: "not deleted" },
      effect: { type: "none" },
    };
  }

  if (key === "q" || key === "\x03") {
    if (state.mode !== "list")
      return { state: { ...state, mode: "list", message: "" }, effect: { type: "none" } };
    return { state, effect: { type: "quit" } };
  }
  if (key === "esc")
    return { state: { ...state, mode: "list", message: "" }, effect: { type: "none" } };

  if (state.mode === "text") {
    const last = Math.max(0, state.textLength - 1);
    if (key === "j" || key === "down")
      return {
        state: { ...state, scroll: Math.min(last, state.scroll + 1) },
        effect: { type: "none" },
      };
    if (key === "k" || key === "up")
      return {
        state: { ...state, scroll: Math.max(0, state.scroll - 1) },
        effect: { type: "none" },
      };
    if (key === "pgdn" || key === " ")
      return {
        state: { ...state, scroll: Math.min(last, state.scroll + 10) },
        effect: { type: "none" },
      };
    if (key === "pgup")
      return {
        state: { ...state, scroll: Math.max(0, state.scroll - 10) },
        effect: { type: "none" },
      };
    if (key === "g") return { state: { ...state, scroll: 0 }, effect: { type: "none" } };
    return { state, effect: { type: "none" } };
  }

  if (state.mode === "ns") {
    const names = ctx.namespaces;
    const nsCursor = Math.min(state.nsCursor, Math.max(0, names.length - 1));
    if (key === "j" || key === "down")
      return {
        state: { ...state, nsCursor: Math.min(names.length - 1, nsCursor + 1) },
        effect: { type: "none" },
      };
    if (key === "k" || key === "up")
      return { state: { ...state, nsCursor: Math.max(0, nsCursor - 1) }, effect: { type: "none" } };
    if (key === "\r") {
      const picked = names[nsCursor];
      if (!picked) return { state: { ...state, mode: "list" }, effect: { type: "none" } };
      return {
        state: { ...state, ns: picked, all: false, board: "pod", cursor: 0, mode: "list" },
        effect: { type: "reload" },
      };
    }
    return { state, effect: { type: "none" } };
  }

  if (state.mode === "help") return { state, effect: { type: "none" } };
  return listMode(state, key, ctx.rows);
}
