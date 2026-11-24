#!/usr/bin/env node
"use strict";
var __create = Object.create;
var __defProp = Object.defineProperty;
var __getOwnPropDesc = Object.getOwnPropertyDescriptor;
var __getOwnPropNames = Object.getOwnPropertyNames;
var __getProtoOf = Object.getPrototypeOf;
var __hasOwnProp = Object.prototype.hasOwnProperty;
var __copyProps = (to, from, except, desc) => {
  if (from && typeof from === "object" || typeof from === "function") {
    for (let key of __getOwnPropNames(from))
      if (!__hasOwnProp.call(to, key) && key !== except)
        __defProp(to, key, { get: () => from[key], enumerable: !(desc = __getOwnPropDesc(from, key)) || desc.enumerable });
  }
  return to;
};
var __toESM = (mod, isNodeMode, target) => (target = mod != null ? __create(__getProtoOf(mod)) : {}, __copyProps(
  // If the importer is in node compatibility mode or this is not an ESM
  // file that has been converted to a CommonJS file using a Babel-
  // compatible transform (i.e. "__esModule" has not been set), then set
  // "default" to the CommonJS "module.exports" for node compatibility.
  isNodeMode || !mod || !mod.__esModule ? __defProp(target, "default", { value: mod, enumerable: true }) : target,
  mod
));

// cli/main.ts
var import_node_fs6 = __toESM(require("node:fs"), 1);
var import_node_path5 = __toESM(require("node:path"), 1);
var import_node_readline = __toESM(require("node:readline"), 1);

// src/lib/kube/shell-meta.ts
var SHELL_COMMANDS = [
  { value: "view", detail: "Full-screen resource list", weight: 98, aliases: ["ui"] },
  { value: "plan", detail: "What a command will change", weight: 92 },
  { value: "ask", detail: "Turn a question into kubectl lines", weight: 90 },
  { value: "live", detail: "Reconnect to the cluster", weight: 86 },
  { value: "reset", detail: "Refresh names from the cluster", weight: 84 },
  { value: "help", detail: "List shell commands", weight: 50 },
  { value: "clear", detail: "Clear the screen", weight: 40 },
  { value: "exit", detail: "Leave the shell", weight: 30, aliases: ["quit"] }
];
var VIEW_ARGS = [
  { value: "pods", detail: "Pods board", kind: "command", weight: 100 },
  { value: "deploy", detail: "Deployments board", kind: "command", weight: 96 },
  { value: "svc", detail: "Services board", kind: "command", weight: 92 },
  { value: "nodes", detail: "Nodes board", kind: "command", weight: 88 },
  { value: "ns", detail: "Namespaces board", kind: "command", weight: 84 },
  { value: "all", detail: "Every namespace", kind: "command", weight: 80 },
  { value: "-A", detail: "Every namespace", kind: "command", weight: 80 },
  { value: "--all-namespaces", detail: "Every namespace", kind: "command", weight: 78 }
];
function namesOf(command) {
  return [command.value, ...command.aliases ?? []];
}
var CLI_WORDS = /* @__PURE__ */ new Set(["--help", "-h", "--version", "-v", "completion", "sample"]);
function metaOf(line) {
  const trimmed = line.trim();
  if (trimmed.startsWith("?")) return "ask";
  const word = trimmed.split(/\s+/)[0];
  if (!word) return void 0;
  if (CLI_WORDS.has(word)) return word;
  for (const command of SHELL_COMMANDS) {
    if (namesOf(command).includes(word)) return word;
  }
  return void 0;
}
function shellMetaFirstWord(first) {
  if (!first) return false;
  const lower = first.toLowerCase();
  for (const command of SHELL_COMMANDS) {
    for (const name of namesOf(command)) {
      if (lower === name) return true;
      if (name.startsWith(lower) && lower.length >= 2) return true;
    }
  }
  return false;
}
function shellCommandCandidates(prefixToken) {
  return SHELL_COMMANDS.map((command) => ({
    value: command.value,
    detail: command.detail,
    kind: "command",
    weight: command.weight,
    aliases: command.aliases
  })).filter((item) => {
    const p = prefixToken.toLowerCase();
    const names = [item.value, ...item.aliases ?? []];
    return names.some((name) => !p || name === p || name.startsWith(p) || p.startsWith(name));
  });
}

// src/lib/kube/model.ts
var RESOURCES = [
  {
    name: "pods",
    short: ["po"],
    singular: "pod",
    kind: "pod",
    detail: "List or manage pods",
    weight: 100,
    namespaced: true,
    api: "v1",
    apiKind: "Pod"
  },
  {
    name: "deployments",
    short: ["deploy"],
    singular: "deployment",
    kind: "deployment",
    detail: "Declarative pod updates",
    weight: 96,
    namespaced: true,
    api: "apps/v1",
    apiKind: "Deployment"
  },
  {
    name: "services",
    short: ["svc"],
    singular: "service",
    kind: "service",
    detail: "Stable network endpoints",
    weight: 92,
    namespaced: true,
    api: "v1",
    apiKind: "Service"
  },
  {
    name: "namespaces",
    short: ["ns"],
    singular: "namespace",
    kind: "namespace",
    detail: "Cluster namespaces",
    weight: 88,
    namespaced: false,
    api: "v1",
    apiKind: "Namespace"
  },
  {
    name: "nodes",
    short: ["no"],
    singular: "node",
    kind: "node",
    detail: "Cluster nodes",
    weight: 84,
    namespaced: false,
    api: "v1",
    apiKind: "Node"
  },
  {
    name: "configmaps",
    short: ["cm"],
    singular: "configmap",
    kind: "configmap",
    detail: "Configuration keys",
    weight: 80,
    namespaced: true,
    api: "v1",
    apiKind: "ConfigMap"
  },
  {
    name: "secrets",
    short: [],
    singular: "secret",
    kind: "secret",
    detail: "Sensitive configuration",
    weight: 76,
    namespaced: true,
    api: "v1",
    apiKind: "Secret"
  },
  {
    name: "ingresses",
    short: ["ing"],
    singular: "ingress",
    kind: "ingress",
    detail: "HTTP routes into the cluster",
    weight: 74,
    namespaced: true,
    api: "networking.k8s.io/v1",
    apiKind: "Ingress"
  },
  {
    name: "events",
    short: ["ev"],
    singular: "event",
    kind: "event",
    detail: "Recent cluster events",
    weight: 72,
    namespaced: true,
    api: "v1",
    apiKind: "Event"
  },
  {
    name: "persistentvolumeclaims",
    short: ["pvc"],
    singular: "persistentvolumeclaim",
    kind: "pvc",
    detail: "Storage claims",
    weight: 68,
    namespaced: true,
    api: "v1",
    apiKind: "PersistentVolumeClaim"
  },
  {
    name: "persistentvolumes",
    short: ["pv"],
    singular: "persistentvolume",
    kind: "pv",
    detail: "Cluster volumes",
    weight: 64,
    namespaced: false,
    api: "v1",
    apiKind: "PersistentVolume"
  },
  {
    name: "serviceaccounts",
    short: ["sa"],
    singular: "serviceaccount",
    kind: "serviceaccount",
    detail: "Pod identities",
    weight: 62,
    namespaced: true,
    api: "v1",
    apiKind: "ServiceAccount"
  },
  {
    name: "replicasets",
    short: ["rs"],
    singular: "replicaset",
    kind: "replicaset",
    detail: "Pod replicas",
    weight: 58,
    namespaced: true,
    api: "apps/v1",
    apiKind: "ReplicaSet"
  },
  {
    name: "statefulsets",
    short: ["sts"],
    singular: "statefulset",
    kind: "statefulset",
    detail: "Stable-identity workloads",
    weight: 56,
    namespaced: true,
    api: "apps/v1",
    apiKind: "StatefulSet"
  },
  {
    name: "daemonsets",
    short: ["ds"],
    singular: "daemonset",
    kind: "daemonset",
    detail: "One pod per node",
    weight: 54,
    namespaced: true,
    api: "apps/v1",
    apiKind: "DaemonSet"
  },
  {
    name: "jobs",
    short: ["job"],
    singular: "job",
    kind: "job",
    detail: "Run-to-completion tasks",
    weight: 52,
    namespaced: true,
    api: "batch/v1",
    apiKind: "Job"
  },
  {
    name: "cronjobs",
    short: ["cj"],
    singular: "cronjob",
    kind: "cronjob",
    detail: "Scheduled jobs",
    weight: 50,
    namespaced: true,
    api: "batch/v1",
    apiKind: "CronJob"
  },
  {
    name: "horizontalpodautoscalers",
    short: ["hpa"],
    singular: "horizontalpodautoscaler",
    kind: "hpa",
    detail: "Scale on CPU or custom metrics",
    weight: 48,
    namespaced: true,
    api: "autoscaling/v2",
    apiKind: "HorizontalPodAutoscaler"
  },
  {
    name: "endpoints",
    short: ["ep"],
    singular: "endpoints",
    kind: "endpoints",
    detail: "Service backend addresses",
    weight: 30,
    namespaced: true,
    api: "v1",
    apiKind: "Endpoints"
  },
  {
    name: "poddisruptionbudgets",
    short: ["pdb"],
    singular: "poddisruptionbudget",
    kind: "poddisruptionbudget",
    detail: "Voluntary disruption limits",
    weight: 28,
    namespaced: true,
    api: "policy/v1",
    apiKind: "PodDisruptionBudget"
  },
  {
    name: "networkpolicies",
    short: ["netpol"],
    singular: "networkpolicy",
    kind: "networkpolicy",
    detail: "Pod traffic rules",
    weight: 26,
    namespaced: true,
    api: "networking.k8s.io/v1",
    apiKind: "NetworkPolicy"
  },
  {
    name: "resourcequotas",
    short: ["quota"],
    singular: "resourcequota",
    kind: "resourcequota",
    detail: "Namespace resource caps",
    weight: 24,
    namespaced: true,
    api: "v1",
    apiKind: "ResourceQuota"
  },
  {
    name: "limitranges",
    short: ["limits"],
    singular: "limitrange",
    kind: "limitrange",
    detail: "Default container limits",
    weight: 22,
    namespaced: true,
    api: "v1",
    apiKind: "LimitRange"
  },
  {
    name: "storageclasses",
    short: ["sc"],
    singular: "storageclass",
    kind: "storageclass",
    detail: "Dynamic provisioners",
    weight: 20,
    namespaced: false,
    api: "storage.k8s.io/v1",
    apiKind: "StorageClass"
  },
  {
    name: "priorityclasses",
    short: ["pc"],
    singular: "priorityclass",
    kind: "priorityclass",
    detail: "Pod scheduling priority",
    weight: 16,
    namespaced: false,
    api: "scheduling.k8s.io/v1",
    apiKind: "PriorityClass"
  },
  {
    name: "customresourcedefinitions",
    short: ["crd"],
    singular: "customresourcedefinition",
    kind: "customresourcedefinition",
    detail: "Custom API types",
    weight: 18,
    namespaced: false,
    api: "apiextensions.k8s.io/v1",
    apiKind: "CustomResourceDefinition"
  }
];
var SINGULAR = Object.fromEntries(
  RESOURCES.flatMap((r) => [[r.singular, r.name], ...r.short.map((s) => [s, r.name])])
);
function resolveResource(token) {
  const raw = token.split("/")[0]?.toLowerCase() ?? "";
  const name = SINGULAR[raw] ?? raw;
  return RESOURCES.find((r) => r.name === name || r.short.includes(raw) || r.singular === raw);
}
var GLOBAL_FLAGS = [
  { name: "--namespace", detail: "Namespace to query", takes: "namespace", weight: 90 },
  { name: "-n", detail: "Namespace to query", takes: "namespace", weight: 92 },
  { name: "--context", detail: "Kubeconfig context", takes: "context", weight: 70 },
  { name: "--kubeconfig", detail: "Path to kubeconfig", takes: "file", weight: 20 },
  { name: "-A", detail: "All namespaces", weight: 80 },
  { name: "--all-namespaces", detail: "All namespaces", weight: 60 },
  { name: "-o", detail: "Output format", takes: "output", weight: 88 },
  { name: "--output", detail: "Output format", takes: "output", weight: 66 },
  { name: "-l", detail: "Label selector", takes: "selector", weight: 84 },
  { name: "--selector", detail: "Label selector", takes: "selector", weight: 50 },
  { name: "--field-selector", detail: "Field selector", takes: "field", weight: 40 },
  { name: "-v", detail: "Log verbosity", takes: "level", weight: 12 },
  { name: "--request-timeout", detail: "Request timeout", takes: "timeout", weight: 14 },
  { name: "--as", detail: "Impersonate a username", takes: "user", weight: 10 },
  { name: "-h", detail: "Help for this command", weight: 8 },
  { name: "--help", detail: "Help for this command", weight: 18 }
];
var VERB_FLAGS = {
  get: [
    { name: "-w", detail: "Watch for changes", weight: 70 },
    { name: "--watch", detail: "Watch for changes", weight: 48 },
    { name: "--show-labels", detail: "Print labels column", weight: 44 },
    { name: "--sort-by", detail: "Sort by a JSON path", takes: "sort", weight: 36 },
    { name: "--chunk-size", detail: "Return large lists in chunks", takes: "replicas", weight: 8 }
  ],
  describe: [{ name: "--show-events", detail: "Include related events", weight: 30 }],
  logs: [
    { name: "-f", detail: "Follow log stream", weight: 90 },
    { name: "--follow", detail: "Follow log stream", weight: 60 },
    { name: "--tail", detail: "Lines from the end of the logs", takes: "tail", weight: 86 },
    { name: "-c", detail: "Container name", takes: "container", weight: 80 },
    { name: "--container", detail: "Container name", takes: "container", weight: 40 },
    { name: "-p", detail: "Logs of the previous container", weight: 50 },
    { name: "--previous", detail: "Logs of the previous container", weight: 34 },
    { name: "--since", detail: "Only logs newer than a duration", takes: "since", weight: 46 },
    { name: "--timestamps", detail: "Prefix each line with a timestamp", weight: 28 },
    { name: "--all-containers", detail: "Every container in the pod", weight: 24 }
  ],
  exec: [
    { name: "-it", detail: "Allocate a TTY and stdin", weight: 94 },
    { name: "-i", detail: "Pass stdin to the container", weight: 40 },
    { name: "-t", detail: "Allocate a TTY", weight: 36 },
    { name: "-c", detail: "Container name", takes: "container", weight: 70 },
    { name: "--container", detail: "Container name", takes: "container", weight: 30 },
    { name: "--stdin", detail: "Pass stdin", weight: 16 },
    { name: "--tty", detail: "Allocate a TTY", weight: 16 }
  ],
  apply: [
    { name: "-f", detail: "File or directory to apply", takes: "file", weight: 96 },
    { name: "--filename", detail: "File or directory to apply", takes: "file", weight: 40 },
    { name: "-k", detail: "Kustomize directory", takes: "dir", weight: 70 },
    { name: "--kustomize", detail: "Kustomize directory", takes: "dir", weight: 24 },
    { name: "--dry-run", detail: "Don't persist changes", takes: "dryrun", weight: 64 },
    { name: "--force", detail: "Delete and re-create if needed", weight: 20 },
    { name: "-R", detail: "Process the directory recursively", weight: 18 },
    { name: "--server-side", detail: "Server-side apply", weight: 22 },
    { name: "--prune", detail: "Delete objects missing from the file", weight: 14 }
  ],
  delete: [
    { name: "-f", detail: "Delete resources in a file", takes: "file", weight: 80 },
    { name: "--filename", detail: "Delete resources in a file", takes: "file", weight: 20 },
    { name: "--force", detail: "Immediate deletion", weight: 50 },
    { name: "--grace-period", detail: "Seconds to wait before delete", takes: "grace", weight: 40 },
    { name: "--now", detail: "Grace period of 1 second", weight: 30 },
    { name: "--all", detail: "Delete every object of this type", weight: 28 },
    { name: "--dry-run", detail: "Don't persist changes", takes: "dryrun", weight: 36 },
    { name: "--wait", detail: "Wait until deletion finishes", weight: 12 }
  ],
  create: [
    {
      name: "--dry-run",
      detail: "Print the object instead of creating it",
      takes: "dryrun",
      weight: 70
    },
    { name: "--image", detail: "Container image", takes: "image", weight: 80 },
    { name: "--from-literal", detail: "Key=value pair", takes: "literal", weight: 60 },
    { name: "--port", detail: "Service or container port", takes: "port", weight: 50 },
    { name: "-o", detail: "Output format", takes: "output", weight: 40 }
  ],
  scale: [{ name: "--replicas", detail: "Desired replica count", takes: "replicas", weight: 96 }],
  rollout: [
    { name: "--to-revision", detail: "Revision to roll back to", takes: "revision", weight: 60 },
    { name: "--revision", detail: "Revision to inspect", takes: "revision", weight: 40 },
    { name: "--timeout", detail: "How long to wait", takes: "timeout", weight: 30 }
  ],
  "port-forward": [{ name: "--address", detail: "Bind address", takes: "address", weight: 50 }],
  expose: [
    { name: "--port", detail: "Service port", takes: "port", weight: 80 },
    { name: "--target-port", detail: "Container port", takes: "port", weight: 60 },
    { name: "--type", detail: "Service type", takes: "svctype", weight: 50 },
    { name: "--name", detail: "Service name", takes: "name", weight: 30 }
  ],
  set: [{ name: "--dry-run", detail: "Don't persist changes", takes: "dryrun", weight: 20 }],
  run: [
    { name: "--image", detail: "Container image", takes: "image", weight: 90 },
    { name: "--restart", detail: "Restart policy", takes: "restart", weight: 50 },
    { name: "-it", detail: "Interactive TTY", weight: 40 },
    { name: "--rm", detail: "Delete the pod when it exits", weight: 36 },
    { name: "--port", detail: "Expose a container port", takes: "port", weight: 24 },
    { name: "--dry-run", detail: "Don't persist changes", takes: "dryrun", weight: 20 }
  ],
  drain: [
    { name: "--ignore-daemonsets", detail: "Skip DaemonSet pods", weight: 80 },
    { name: "--delete-emptydir-data", detail: "Delete emptyDir data", weight: 60 },
    { name: "--force", detail: "Continue even if unmanaged pods remain", weight: 40 },
    { name: "--grace-period", detail: "Seconds to wait", takes: "grace", weight: 30 }
  ],
  wait: [
    { name: "--for", detail: "Condition to wait for", takes: "for", weight: 90 },
    { name: "--timeout", detail: "Give up after this duration", takes: "timeout", weight: 50 }
  ],
  debug: [
    { name: "-it", detail: "Interactive TTY", weight: 70 },
    { name: "--image", detail: "Debug image", takes: "image", weight: 60 },
    {
      name: "--target",
      detail: "Container to share the namespace with",
      takes: "container",
      weight: 40
    }
  ],
  diff: [
    { name: "-f", detail: "File to diff", takes: "file", weight: 90 },
    { name: "-k", detail: "Kustomize directory", takes: "dir", weight: 40 }
  ],
  version: [{ name: "--client", detail: "Client version only", weight: 80 }],
  "api-resources": [
    { name: "--namespaced", detail: "Only namespaced resources", weight: 40 },
    { name: "--api-group", detail: "Filter by API group", takes: "group", weight: 30 }
  ],
  config: [
    { name: "--current", detail: "Edit the current context", weight: 80 },
    {
      name: "--namespace",
      detail: "Default namespace for the context",
      takes: "namespace",
      weight: 70
    },
    { name: "--minify", detail: "Only the current context", weight: 30 }
  ],
  kustomize: [],
  label: [{ name: "--overwrite", detail: "Replace an existing label", weight: 40 }],
  annotate: [{ name: "--overwrite", detail: "Replace an existing annotation", weight: 40 }],
  patch: [
    { name: "-p", detail: "Patch document", takes: "literal", weight: 50 },
    { name: "--type", detail: "strategic, merge, or json", takes: "patchtype", weight: 40 }
  ]
};
function flagCatalog(verb) {
  const extra = verb ? VERB_FLAGS[verb] ?? [] : [];
  const map = /* @__PURE__ */ new Map();
  for (const flag of [...extra, ...GLOBAL_FLAGS]) {
    if (!map.has(flag.name)) map.set(flag.name, flag);
  }
  return [...map.values()];
}
function takesOf(flag, verb) {
  const name = flag.includes("=") ? flag.slice(0, flag.indexOf("=")) : flag;
  return flagCatalog(verb).find((flagDef) => flagDef.name === name)?.takes;
}
var VERBS = [
  { name: "get", detail: "List or fetch resources", weight: 100 },
  { name: "describe", detail: "Show detailed resource state", weight: 94 },
  { name: "logs", detail: "Print container logs", weight: 90 },
  { name: "exec", detail: "Run a command in a container", weight: 86 },
  { name: "apply", detail: "Apply a manifest", weight: 84 },
  { name: "delete", detail: "Delete resources", weight: 82 },
  { name: "create", detail: "Create a resource", weight: 78 },
  { name: "rollout", detail: "Manage a rollout", weight: 76 },
  { name: "scale", detail: "Set replica count", weight: 74 },
  { name: "port-forward", detail: "Forward a local port", weight: 72 },
  { name: "config", detail: "Read or switch kubeconfig", weight: 70 },
  { name: "explain", detail: "Document an API field", weight: 68 },
  { name: "edit", detail: "Edit a live object", weight: 60 },
  { name: "top", detail: "CPU and memory usage", weight: 64 },
  { name: "auth", detail: "Inspect permissions", weight: 58 },
  { name: "diff", detail: "Diff a file against the cluster", weight: 52 },
  { name: "wait", detail: "Wait for a condition", weight: 50 },
  { name: "set", detail: "Change image, env, or resources", weight: 56 },
  { name: "expose", detail: "Turn a workload into a Service", weight: 48 },
  { name: "run", detail: "Run an image in a pod", weight: 46 },
  { name: "debug", detail: "Attach a debug container", weight: 44 },
  { name: "label", detail: "Update labels", weight: 40 },
  { name: "annotate", detail: "Update annotations", weight: 36 },
  { name: "patch", detail: "Patch live fields", weight: 38 },
  { name: "api-resources", detail: "List supported resource types", weight: 42 },
  { name: "api-versions", detail: "List API versions", weight: 22 },
  { name: "cluster-info", detail: "Control plane endpoints", weight: 28 },
  { name: "version", detail: "Client and server version", weight: 26 },
  { name: "cordon", detail: "Mark a node unschedulable", weight: 32 },
  { name: "uncordon", detail: "Mark a node schedulable", weight: 30 },
  { name: "drain", detail: "Evict pods for node maintenance", weight: 34 },
  { name: "taint", detail: "Taint a node", weight: 24 },
  { name: "cp", detail: "Copy files in or out of a container", weight: 20 },
  { name: "kustomize", detail: "Render a kustomization", weight: 18 },
  { name: "completion", detail: "Shell completion script", weight: 16 },
  { name: "help", detail: "Help for a command", weight: 14 },
  { name: "proxy", detail: "Proxy the API server locally", weight: 12 },
  { name: "replace", detail: "Replace a resource from a file", weight: 10 }
];
var SUBS = {
  config: [
    { value: "get-contexts", detail: "List kubeconfig contexts", weight: 100 },
    { value: "use-context", detail: "Switch the current context", weight: 96 },
    { value: "current-context", detail: "Print the current context", weight: 80 },
    { value: "view", detail: "Show the merged kubeconfig", weight: 70 },
    { value: "set-context", detail: "Change a context entry", weight: 74 },
    { value: "get-clusters", detail: "List clusters", weight: 40 },
    { value: "delete-context", detail: "Delete a context", weight: 30 }
  ],
  rollout: [
    { value: "status", detail: "See if a rollout is done", weight: 100 },
    { value: "restart", detail: "Rolling restart", weight: 94 },
    { value: "history", detail: "List revisions", weight: 80 },
    { value: "undo", detail: "Roll back to a revision", weight: 86 },
    { value: "pause", detail: "Pause a rollout", weight: 40 },
    { value: "resume", detail: "Resume a rollout", weight: 36 }
  ],
  auth: [{ value: "can-i", detail: "Check if an action is allowed", weight: 100 }],
  create: [
    { value: "namespace", detail: "Create a namespace", weight: 90 },
    { value: "deployment", detail: "Create a deployment", weight: 100 },
    { value: "configmap", detail: "Create a ConfigMap", weight: 80 },
    { value: "secret", detail: "Create a secret", weight: 76 },
    { value: "service", detail: "Create a service", weight: 74 },
    { value: "job", detail: "Create a job", weight: 60 },
    { value: "cronjob", detail: "Create a cronjob", weight: 50 },
    { value: "serviceaccount", detail: "Create a service account", weight: 48 },
    { value: "ingress", detail: "Create an ingress", weight: 46 },
    { value: "quota", detail: "Create a resource quota", weight: 30 }
  ],
  set: [
    { value: "image", detail: "Update a container image", weight: 100 },
    { value: "env", detail: "Update environment variables", weight: 80 },
    { value: "resources", detail: "Set CPU and memory", weight: 60 }
  ],
  top: [
    { value: "pods", detail: "Pod CPU and memory", weight: 100 },
    { value: "nodes", detail: "Node CPU and memory", weight: 90 }
  ],
  completion: [
    { value: "bash", detail: "Bash completion script", weight: 80 },
    { value: "zsh", detail: "Zsh completion script", weight: 100 },
    { value: "fish", detail: "Fish completion script", weight: 70 },
    { value: "powershell", detail: "PowerShell completion script", weight: 40 }
  ]
};
var EXPLAIN = {
  pods: {
    detail: "Pod is a group of containers",
    doc: `KIND:     Pod
VERSION:  v1

DESCRIPTION:
    Pod is a collection of containers that can run on a host.

FIELDS:
  apiVersion  <string>
  kind        <string>
  metadata    <Object>
  spec        <Object>  desired behavior of the pod
  status      <Object>  most recently observed status`
  },
  "pods.metadata": {
    detail: "Object metadata",
    doc: `FIELD: metadata <Object>

  name        <string>
  namespace   <string>
  labels      <map[string]string>
  annotations <map[string]string>`
  },
  "pods.spec": {
    detail: "Desired pod behavior",
    doc: `FIELD: spec <Object>

  containers          <[]Object>  required
  initContainers      <[]Object>
  restartPolicy       <string>    Always, OnFailure, or Never
  serviceAccountName  <string>
  nodeName            <string>
  volumes             <[]Object>`
  },
  "pods.spec.containers": {
    detail: "Containers that run in the pod",
    doc: `FIELD: containers <[]Object>

  name            <string>  DNS_LABEL, required
  image           <string>
  command         <[]string>
  args            <[]string>
  ports           <[]Object>
  env             <[]Object>
  resources       <Object>  requests and limits
  livenessProbe   <Object>
  readinessProbe  <Object>`
  },
  "pods.spec.containers.image": {
    detail: "Container image reference",
    doc: `FIELD: image <string>

Container image name. Empty means the image is not updated by a controller.`
  },
  "pods.status": {
    detail: "Observed pod status",
    doc: `FIELD: status <Object>

  phase      <string>  Pending, Running, Succeeded, Failed
  podIP      <string>
  startTime  <string>
  containerStatuses <[]Object>`
  },
  deployments: {
    detail: "Declarative updates for Pods",
    doc: `KIND:     Deployment
VERSION:  apps/v1

DESCRIPTION:
    Deployment enables declarative updates for Pods and ReplicaSets.

FIELDS:
  metadata  <Object>
  spec      <Object>
  status    <Object>`
  },
  "deployments.spec": {
    detail: "Deployment specification",
    doc: `FIELD: spec <Object>

  replicas  <integer>
  selector  <Object>  must match the pod template
  template  <Object>  pod template
  strategy  <Object>  RollingUpdate or Recreate`
  },
  services: {
    detail: "Named set of endpoints",
    doc: `KIND:     Service
VERSION:  v1

DESCRIPTION:
    Service is a named abstraction of a software service.

FIELDS:
  metadata  <Object>
  spec      <Object>
  status    <Object>`
  },
  "services.spec": {
    detail: "Service specification",
    doc: `FIELD: spec <Object>

  type       <string>  ClusterIP, NodePort, or LoadBalancer
  selector   <map[string]string>
  ports      <[]Object>
  clusterIP  <string>`
  }
};
function currentContext(cluster) {
  return cluster.contexts.find((c) => c.name === cluster.current) ?? cluster.contexts[0];
}
function namespaceOf(cluster) {
  return currentContext(cluster)?.namespace || "default";
}
function promptOf(cluster) {
  const ctx = currentContext(cluster);
  if (!ctx) return "none:default";
  return `${ctx.name}:${ctx.namespace}`;
}

// src/lib/kube/grammar.ts
var import_node_fs = __toESM(require("node:fs"), 1);
var import_node_path = __toESM(require("node:path"), 1);

// src/lib/kube/parse.ts
function tokenize(line) {
  const tokens = [];
  let i = 0;
  while (i < line.length) {
    while (i < line.length && line[i] === " ") i += 1;
    if (i >= line.length) break;
    const start = i;
    let text = "";
    const quote = line[i] === '"' || line[i] === "'" ? line[i] : "";
    if (quote) {
      text += line[i];
      i += 1;
      while (i < line.length && line[i] !== quote) {
        text += line[i];
        i += 1;
      }
      if (i < line.length) {
        text += line[i];
        i += 1;
      }
    } else {
      while (i < line.length && line[i] !== " ") {
        text += line[i];
        i += 1;
      }
    }
    tokens.push({ text, start, end: i });
  }
  return tokens;
}
function locate(line, cursor) {
  const tokens = tokenize(line);
  const c = Math.max(0, Math.min(cursor, line.length));
  for (let i = 0; i < tokens.length; i += 1) {
    const token = tokens[i];
    if (c < token.start) return { start: c, end: c, index: i, tokens };
    if (c <= token.end) return { start: token.start, end: token.end, index: i, tokens };
  }
  return { start: c, end: c, index: tokens.length, tokens };
}
function positionalWords(tokens) {
  const words = [];
  const dashWords = [];
  let after = false;
  for (let i = 0; i < tokens.length; i += 1) {
    const token = tokens[i];
    if (after) {
      dashWords.push(token);
      continue;
    }
    if (token === "--") {
      after = true;
      continue;
    }
    if (token.startsWith("-") && token !== "-") {
      if (!token.includes("=") && takesOf(token, words[0])) i += 1;
      continue;
    }
    words.push(token);
  }
  return { words, afterDashDash: after, dashWords };
}
function scopeOf(tokens, cluster) {
  let context = cluster.current;
  let ns = currentContext(cluster).namespace;
  let all = false;
  for (let i = 0; i < tokens.length; i += 1) {
    const token = tokens[i];
    if (token === "-A" || token === "--all-namespaces") {
      all = true;
      continue;
    }
    if (token === "--context" && tokens[i + 1]) {
      context = tokens[i + 1];
      i += 1;
      const found = cluster.contexts.find((c) => c.name === context);
      if (found) ns = found.namespace;
      continue;
    }
    if (token.startsWith("--context=")) {
      context = token.slice("--context=".length);
      const found = cluster.contexts.find((c) => c.name === context);
      if (found) ns = found.namespace;
      continue;
    }
    if ((token === "-n" || token === "--namespace") && tokens[i + 1]) {
      ns = tokens[i + 1];
      i += 1;
      continue;
    }
    if (token.startsWith("--namespace=")) ns = token.slice("--namespace=".length);
  }
  return { ns, all, context };
}
function verbOf(args) {
  return positionalWords(args).words[0];
}
function flagsForVerb(verb) {
  return flagCatalog(verb);
}
function quoteArg(text) {
  const hasSingle = text.includes("'");
  const hasDouble = text.includes('"');
  if (!text.includes(" ") && !hasSingle && !hasDouble) return text;
  if (hasSingle && hasDouble) return null;
  if (hasDouble) return `'${text}'`;
  return `"${text}"`;
}
function unquote(text) {
  if (text.length >= 2 && (text.startsWith('"') && text.endsWith('"') || text.startsWith("'") && text.endsWith("'"))) {
    return text.slice(1, -1);
  }
  return text;
}
function flagValue(tokens, names) {
  for (let i = 0; i < tokens.length; i += 1) {
    const token = tokens[i];
    for (const name of names) {
      if (token === name) return tokens[i + 1];
      if (token.startsWith(`${name}=`)) return token.slice(name.length + 1);
    }
  }
  return void 0;
}

// src/lib/kube/grammar.ts
function resourceItems(filter) {
  return RESOURCES.filter((resource) => filter ? filter(resource) : true).map((resource) => ({
    value: resource.name,
    detail: resource.detail,
    kind: "resource",
    weight: resource.weight,
    aliases: [resource.singular, ...resource.short]
  }));
}
function inScope(ns, scope) {
  return scope.all || ns === scope.ns;
}
function namesOf2(resource, scope, cluster) {
  if (resource.kind === "namespace") {
    return cluster.namespaces.map((item) => ({
      value: item.name,
      detail: item.status,
      kind: "namespace",
      weight: 50
    }));
  }
  if (resource.kind === "node") {
    return cluster.nodes.map((item) => ({
      value: item.name,
      detail: item.schedulable ? item.roles : "unschedulable",
      kind: "node",
      weight: 50
    }));
  }
  if (resource.kind === "pv") {
    if (resource.name !== "persistentvolumes") return [];
    return cluster.pvs.map((item) => ({
      value: item.name,
      detail: `${item.capacity} ${item.status}`,
      kind: "volume",
      weight: 40
    }));
  }
  if (resource.kind === "event") {
    if (resource.name !== "events") return [];
    return cluster.events.filter((item) => inScope(item.ns, scope)).map((item) => ({
      value: `${item.reason.toLowerCase()}.${item.object.replace("/", "-")}`,
      detail: item.message,
      kind: "event",
      weight: 20
    }));
  }
  return cluster.objects.filter(
    (item) => item.kind === resource.kind && (!resource.namespaced || inScope(item.ns, scope))
  ).map((item) => ({
    value: item.name,
    detail: scope.all ? `${item.ns} \xB7 ${item.status ?? item.kind}` : item.status ?? item.svcType ?? item.kind,
    kind: item.kind,
    weight: item.status === "CrashLoopBackOff" || item.status === "Pending" ? 70 : 50
  }));
}
function slashNames(typeToken, namePrefix, scope, cluster) {
  const resource = resolveResource(typeToken);
  if (!resource) return [];
  return namesOf2(resource, scope, cluster).filter((item) => item.value.toLowerCase().startsWith(namePrefix.toLowerCase())).map((item) => ({ ...item, value: `${typeToken}/${item.value}` }));
}
function workloadSlash(kinds, scope, cluster) {
  const out = [];
  for (const kind of kinds) {
    const resource = RESOURCES.find((item) => item.kind === kind && item.name.endsWith("s")) ?? RESOURCES.find((item) => item.kind === kind);
    if (!resource) continue;
    for (const item of namesOf2(resource, scope, cluster)) {
      out.push({
        value: `${resource.singular}/${item.value}`,
        detail: item.detail,
        kind: resource.singular,
        weight: (item.weight ?? 40) - 4
      });
    }
  }
  return out;
}
function manifestCandidates(prefix, root = process.cwd()) {
  const normalized = prefix.replace(/\\/g, "/");
  const slash = normalized.lastIndexOf("/");
  const dirPart = slash >= 0 ? normalized.slice(0, slash) : "";
  const dir = import_node_path.default.resolve(root, dirPart);
  const relative = import_node_path.default.relative(root, dir);
  if (relative.startsWith("..") || import_node_path.default.isAbsolute(relative)) return [];
  let entries;
  try {
    entries = import_node_fs.default.readdirSync(dir, { withFileTypes: true });
  } catch {
    return [];
  }
  const out = [];
  for (const entry of entries) {
    if (entry.name.startsWith(".") || entry.name === "node_modules" || entry.isSymbolicLink())
      continue;
    const rel = (dirPart ? `${dirPart}/${entry.name}` : entry.name).replace(/\\/g, "/");
    if (!rel.toLowerCase().startsWith(normalized.toLowerCase())) continue;
    if (entry.isDirectory())
      out.push({ value: `${rel}/`, detail: "Directory", kind: "file", weight: 35 });
    else if (/\.ya?ml$/i.test(entry.name))
      out.push({ value: rel, detail: "Manifest", kind: "file", weight: 40 });
  }
  return out;
}
function flagItems(verb) {
  return flagsForVerb(verb).map((flag) => ({
    value: flag.name,
    detail: flag.detail,
    kind: "flag",
    weight: flag.weight
  }));
}
function ports() {
  return [
    { value: "8080:80", detail: "Local 8080 to pod 80", kind: "port", weight: 80 },
    { value: "8443:443", detail: "Local 8443 to pod 443", kind: "port", weight: 50 },
    { value: "3000:3000", detail: "Grafana-style forward", kind: "port", weight: 40 },
    { value: "9090:9090", detail: "Prometheus-style forward", kind: "port", weight: 36 }
  ];
}
function targets(scope, cluster, kinds) {
  const types = resourceItems((resource) => kinds.includes(resource.kind));
  return [...types, ...workloadSlash(kinds, scope, cluster)];
}
function afterResource(rest, scope, cluster, verb) {
  if (rest.length === 0) return resourceItems();
  if (rest.length === 1) {
    const resource = resolveResource(rest[0] ?? "");
    if (!resource || rest[0]?.includes("/")) return flagItems(verb);
    const names = namesOf2(resource, scope, cluster);
    return names.length > 0 ? names : flagItems(verb);
  }
  return flagItems(verb);
}
var NAME_VERBS = /* @__PURE__ */ new Set([
  "get",
  "describe",
  "delete",
  "edit",
  "label",
  "annotate",
  "patch",
  "wait"
]);
var FLAG_VERBS = /* @__PURE__ */ new Set([
  "completion",
  "version",
  "api-resources",
  "api-versions",
  "cluster-info",
  "proxy",
  "run"
]);
var FILL = {
  config(ctx) {
    const { rest, cluster, verb } = ctx;
    const sub = rest[0];
    if (sub === "use-context" || sub === "delete-context") {
      if (rest.length === 1) {
        return cluster.contexts.map((item) => ({
          value: item.name,
          detail: item.cluster,
          kind: "context",
          weight: 50
        }));
      }
    }
    if (sub === "set-context" && rest.length === 1) {
      return [
        ...cluster.contexts.map((item) => ({
          value: item.name,
          detail: item.cluster,
          kind: "context",
          weight: 40
        })),
        { value: "--current", detail: "Modify the active context", kind: "flag", weight: 80 },
        { value: "--namespace", detail: "Set the default namespace", kind: "flag", weight: 70 }
      ];
    }
    return flagItems(verb);
  },
  rollout(ctx) {
    const { rest, scope, cluster, verb } = ctx;
    const sub = rest[0] ?? "";
    const after = rest.slice(1);
    if (!sub) return [];
    if (after.length === 0)
      return targets(scope, cluster, ["deployment", "statefulset", "daemonset"]);
    if (after.length === 1 && !after[0]?.includes("/")) {
      const resource = resolveResource(after[0] ?? "");
      if (resource) return namesOf2(resource, scope, cluster);
    }
    return flagItems(verb);
  },
  auth(ctx) {
    const { rest, verb } = ctx;
    if (rest.length === 1) {
      return ["get", "list", "watch", "create", "update", "patch", "delete", "*"].map(
        (value, index) => ({
          value,
          detail: "Verb to check",
          kind: "verb",
          weight: 80 - index
        })
      );
    }
    if (rest.length === 2) return resourceItems();
    return flagItems(verb);
  },
  create(ctx) {
    if (ctx.rest.length === 0) return [];
    return flagItems(ctx.verb);
  },
  set(ctx) {
    const { rest, scope, cluster, verb } = ctx;
    const sub = rest[0];
    const after = rest.slice(1);
    if (!sub) return [];
    if (after.length === 0)
      return targets(scope, cluster, ["deployment", "statefulset", "daemonset"]);
    if (sub === "image" && after.length >= 1) return [];
    return flagItems(verb);
  },
  top() {
    return flagItems("get");
  },
  logs: fillPodish,
  exec: fillPodish,
  debug: fillPodish,
  cp: fillPodish,
  "port-forward"(ctx) {
    const { rest, scope, cluster, verb } = ctx;
    if (rest.length === 0) {
      const pods = namesOf2(resolveResource("pods"), scope, cluster);
      return [...pods, ...workloadSlash(["service", "deployment"], scope, cluster)];
    }
    if (rest.length === 1) return ports();
    return flagItems(verb);
  },
  scale(ctx) {
    const { rest, scope, cluster, verb } = ctx;
    if (rest.length === 0)
      return targets(scope, cluster, ["deployment", "statefulset", "replicaset"]);
    if (rest.length === 1 && !rest[0]?.includes("/")) {
      const resource = resolveResource(rest[0] ?? "");
      if (resource) return namesOf2(resource, scope, cluster);
    }
    return flagItems(verb);
  },
  expose(ctx) {
    const { rest, scope, cluster, verb } = ctx;
    if (rest.length === 0)
      return resourceItems(
        (resource) => ["deployment", "pod", "service", "replicaset"].includes(resource.kind)
      );
    if (rest.length === 1) {
      const resource = resolveResource(rest[0] ?? "");
      if (resource) return namesOf2(resource, scope, cluster);
    }
    return flagItems(verb);
  },
  explain(ctx) {
    if (ctx.rest.length === 0) {
      return Object.entries(EXPLAIN).map(([value, info], index) => ({
        value,
        detail: info.detail,
        kind: "field",
        weight: 100 - index
      }));
    }
    return [];
  },
  cordon: fillNode,
  uncordon: fillNode,
  drain: fillNode,
  taint: fillNode,
  apply: fillManifest,
  diff: fillManifest,
  replace: fillManifest,
  help() {
    return VERBS.map((item) => ({
      value: item.name,
      detail: item.detail,
      kind: "verb",
      weight: item.weight
    }));
  },
  kustomize(ctx) {
    return manifestCandidates(ctx.prefix).filter((item) => item.value.endsWith("/"));
  }
};
function fillPodish(ctx) {
  const { verb, rest, scope, cluster } = ctx;
  if (rest.length === 0) {
    const pods = namesOf2(resolveResource("pods"), scope, cluster).map((item) => ({
      ...item,
      weight: 80
    }));
    if (verb === "cp") {
      return pods.map((item) => ({
        ...item,
        value: `${item.value}:/app`,
        detail: "pod path",
        kind: "path"
      }));
    }
    return [...pods, ...workloadSlash(["deployment", "statefulset", "job"], scope, cluster)];
  }
  return flagItems(verb);
}
function fillNode(ctx) {
  if (ctx.rest.length === 0) return namesOf2(resolveResource("nodes"), ctx.scope, ctx.cluster);
  return flagItems(ctx.verb);
}
function fillManifest(ctx) {
  if (ctx.rest.length === 0 && !ctx.prefix.startsWith("-")) {
    return [
      { value: "-f", detail: "Apply or diff a file", kind: "flag", weight: 100 },
      { value: "-k", detail: "Kustomize directory", kind: "flag", weight: 70 },
      ...manifestCandidates(ctx.prefix)
    ];
  }
  return flagItems(ctx.verb);
}
function candidatesFor(verb, rest, _args, scope, cluster, prefix) {
  const subs = SUBS[verb];
  if (subs && rest.length === 0 && !prefix.includes("/")) {
    return subs.map((sub) => ({
      value: sub.value,
      detail: sub.detail,
      kind: "command",
      weight: sub.weight
    }));
  }
  const fill = FILL[verb];
  if (fill) return fill({ verb, rest, scope, cluster, prefix });
  if (FLAG_VERBS.has(verb)) return flagItems(verb);
  if (NAME_VERBS.has(verb)) return afterResource(rest, scope, cluster, verb);
  return flagItems(verb);
}

// src/lib/kube/complete.ts
var LIMIT = 24;
function rank(prefix, items) {
  const p = prefix.toLowerCase();
  const scored = [];
  for (const item of items) {
    const value = item.value.toLowerCase();
    const aliases = (item.aliases ?? []).map((alias) => alias.toLowerCase());
    let score;
    if (!p) score = item.weight ?? 1;
    else if (value === p) score = 1200 + (item.weight ?? 0) / 1e3;
    else if (aliases.includes(p)) score = 1100;
    else if (value.startsWith(p))
      score = 800 - Math.min(value.length, 180) + (item.weight ?? 0) / 100;
    else if (aliases.some((alias) => alias.startsWith(p))) score = 760 + (item.weight ?? 0) / 100;
    else if (p.length >= 2 && (value.includes(p) || aliases.some((alias) => alias.includes(p))))
      score = 420;
    else continue;
    scored.push({ item, score });
  }
  scored.sort((a, b) => b.score - a.score || a.item.value.localeCompare(b.item.value));
  const unique = [];
  const seen = /* @__PURE__ */ new Set();
  for (const row of scored) {
    if (seen.has(row.item.value)) continue;
    seen.add(row.item.value);
    unique.push(row.item);
    if (unique.length >= LIMIT) break;
  }
  if (unique.length === 0 && p.length >= 3) {
    const near = items.map((item) => ({ item, dist: distance(p, item.value.toLowerCase()) })).filter((row) => row.dist <= 2).sort((a, b) => a.dist - b.dist || a.item.value.localeCompare(b.item.value)).slice(0, 6).map((row) => row.item);
    return near;
  }
  return unique;
}
function distance(a, b) {
  if (Math.abs(a.length - b.length) > 2) return 9;
  const dp = Array.from({ length: a.length + 1 }, () => new Array(b.length + 1).fill(0));
  for (let i = 0; i <= a.length; i += 1) dp[i][0] = i;
  for (let j = 0; j <= b.length; j += 1) dp[0][j] = j;
  for (let i = 1; i <= a.length; i += 1) {
    for (let j = 1; j <= b.length; j += 1) {
      const cost = a[i - 1] === b[j - 1] ? 0 : 1;
      dp[i][j] = Math.min(dp[i - 1][j] + 1, dp[i][j - 1] + 1, dp[i - 1][j - 1] + cost);
    }
  }
  return dp[a.length][b.length];
}
function valueItems(kind, args, cluster, scope, prefix = "") {
  switch (kind) {
    case "namespace":
      return cluster.namespaces.map((item) => ({
        value: item.name,
        detail: "Namespace",
        kind: "value",
        weight: 40
      }));
    case "context":
      return cluster.contexts.map((item) => ({
        value: item.name,
        detail: item.cluster,
        kind: "context",
        weight: 40
      }));
    case "output":
      return ["wide", "yaml", "json", "name", "jsonpath", "custom-columns"].map((value, index) => ({
        value,
        detail: "Output format",
        kind: "value",
        weight: 80 - index
      }));
    case "container": {
      const pods = referencedPods(args, scope, cluster);
      const names = /* @__PURE__ */ new Set();
      for (const item of pods)
        for (const container of item.containers ?? []) names.add(container.name);
      if (names.size === 0) {
        for (const item of cluster.objects) {
          if (item.kind === "pod" && inScope(item.ns, scope)) {
            for (const container of item.containers ?? []) names.add(container.name);
          }
        }
      }
      return [...names].map((value) => ({ value, detail: "Container", kind: "value", weight: 40 }));
    }
    case "file":
    case "dir":
      return manifestCandidates(prefix);
    case "selector":
      return selectorItems(scope, cluster);
    case "replicas":
      return ["1", "2", "3", "4", "5"].map((value) => ({
        value,
        detail: "Replica count",
        kind: "value",
        weight: 40
      }));
    case "tail":
      return ["20", "50", "100", "200", "500"].map((value, index) => ({
        value,
        detail: "Log lines",
        kind: "value",
        weight: 50 - index
      }));
    case "dryrun":
      return [
        { value: "client", detail: "Validate locally", kind: "value", weight: 60 },
        { value: "server", detail: "Validate on the API server", kind: "value", weight: 50 },
        { value: "none", detail: "Persist the change", kind: "value", weight: 20 }
      ];
    case "image":
      return [];
    case "port":
      return ["80", "443", "8080", "3000", "9090"].map((value) => ({
        value,
        detail: "Port",
        kind: "value",
        weight: 30
      }));
    case "for":
      return ["condition=ready", "condition=available", "condition=complete", "delete"].map(
        (value, index) => ({
          value,
          detail: "Wait condition",
          kind: "value",
          weight: 40 - index
        })
      );
    case "sort":
      return [".metadata.name", ".metadata.creationTimestamp", ".status.phase"].map((value) => ({
        value,
        detail: "JSON path",
        kind: "value",
        weight: 20
      }));
    case "since":
      return ["5s", "15m", "1h", "24h"].map((value) => ({
        value,
        detail: "Duration",
        kind: "value",
        weight: 20
      }));
    case "address":
      return ["127.0.0.1", "localhost", "0.0.0.0"].map((value) => ({
        value,
        detail: "Bind address",
        kind: "value",
        weight: 20
      }));
    case "revision":
      return ["1", "2"].map((value) => ({ value, detail: "Revision", kind: "value", weight: 20 }));
    case "svctype":
      return ["ClusterIP", "NodePort", "LoadBalancer"].map((value) => ({
        value,
        detail: "Service type",
        kind: "value",
        weight: 30
      }));
    case "restart":
      return ["Always", "OnFailure", "Never"].map((value) => ({
        value,
        detail: "Restart policy",
        kind: "value",
        weight: 30
      }));
    case "literal":
      return [
        { value: "LOG_LEVEL=info", detail: "Example pair", kind: "value", weight: 20 },
        { value: "color=blue", detail: "Example pair", kind: "value", weight: 16 }
      ];
    case "timeout":
      return ["30s", "60s", "5m"].map((value) => ({
        value,
        detail: "Duration",
        kind: "value",
        weight: 20
      }));
    case "grace":
      return ["0", "30"].map((value) => ({ value, detail: "Seconds", kind: "value", weight: 20 }));
    case "level":
      return ["0", "2", "4", "6", "8"].map((value) => ({
        value,
        detail: "Verbosity",
        kind: "value",
        weight: 10
      }));
    case "field":
      return [
        { value: "status.phase=Running", detail: "Running pods", kind: "value", weight: 30 },
        { value: "status.phase=Pending", detail: "Pending pods", kind: "value", weight: 20 }
      ];
    case "user":
      return ["system:admin", "alice"].map((value) => ({
        value,
        detail: "User",
        kind: "value",
        weight: 10
      }));
    case "group":
      return ["apps", "batch", ""].filter(Boolean).map((value) => ({ value, detail: "API group", kind: "value", weight: 10 }));
    case "patchtype":
      return ["strategic", "merge", "json"].map((value) => ({
        value,
        detail: "Patch type",
        kind: "value",
        weight: 20
      }));
    case "name":
      return [];
    default:
      return [];
  }
}
function selectorItems(scope, cluster) {
  const seen = /* @__PURE__ */ new Set();
  const items = [];
  for (const object of cluster.objects) {
    if (!inScope(object.ns, scope)) continue;
    for (const [key, value] of Object.entries(object.labels)) {
      const pair = `${key}=${value}`;
      if (seen.has(pair)) continue;
      seen.add(pair);
      items.push({ value: pair, detail: "Label selector", kind: "value", weight: 30 });
    }
  }
  return items;
}
function referencedPods(args, scope, cluster) {
  const words = positionalWords(args).words;
  const names = new Set(words);
  return cluster.objects.filter(
    (object) => object.kind === "pod" && inScope(object.ns, scope) && names.has(object.name)
  );
}
function shellWords() {
  return [
    { value: "bash", detail: "Interactive shell", kind: "command", weight: 100 },
    { value: "sh", detail: "POSIX shell", kind: "command", weight: 80 },
    { value: "ls", detail: "List files", kind: "command", weight: 70 },
    { value: "env", detail: "Print environment", kind: "command", weight: 64 },
    { value: "printenv", detail: "Print environment", kind: "command", weight: 40 },
    { value: "pwd", detail: "Working directory", kind: "command", weight: 50 },
    { value: "whoami", detail: "Current user", kind: "command", weight: 46 },
    { value: "ps", detail: "Process list", kind: "command", weight: 30 },
    { value: "cat", detail: "Print a file", kind: "command", weight: 28 }
  ];
}
function complete(line, cursor, cluster) {
  const loc = locate(line, cursor);
  const prior = loc.tokens.slice(0, loc.index).map((token) => token.text);
  const prefix = line.slice(loc.start, cursor);
  const base = { start: loc.start, end: loc.end };
  if (prior.length === 0) {
    return {
      ...base,
      candidates: rank(prefix, [
        {
          value: "kubectl",
          detail: "Kubernetes CLI",
          kind: "command",
          weight: 100,
          aliases: ["k"]
        },
        ...shellCommandCandidates(prefix),
        { value: "help", detail: "How Tab completion works here", kind: "command", weight: 10 }
      ])
    };
  }
  const bin = prior[0];
  if (bin === "view" || bin === "ui") {
    const tail = prior.slice(1);
    if (tail.length === 0) {
      return { ...base, candidates: rank(prefix, [...VIEW_ARGS]) };
    }
  }
  if (bin !== "kubectl" && bin !== "k" && bin !== "help") {
    const shell = shellCommandCandidates(bin ?? "");
    if (shell.length > 0) {
      return { ...base, candidates: rank(prefix, shell) };
    }
    return {
      ...base,
      candidates: rank(prefix, [
        ...shellCommandCandidates(bin ?? ""),
        { value: "kubectl", detail: "Commands start with kubectl", kind: "command", weight: 60 }
      ])
    };
  }
  if (bin === "help") {
    return {
      ...base,
      candidates: rank(
        prefix,
        VERBS.map((item) => ({
          value: item.name,
          detail: item.detail,
          kind: "verb",
          weight: item.weight
        }))
      )
    };
  }
  const args = prior.slice(1);
  const scope = scopeOf(["kubectl", ...args], cluster);
  const equals = /^(--?[A-Za-z0-9-]+)=(.*)$/.exec(prefix);
  if (equals) {
    const flag = equals[1] ?? "";
    const kind = takesOf(flag, verbOf(args));
    const values = kind ? valueItems(kind, args, cluster, scope, equals[2] ?? "") : [];
    return {
      ...base,
      candidates: rank(
        prefix,
        values.map((item) => ({ ...item, value: `${flag}=${item.value}` }))
      )
    };
  }
  if (prefix.startsWith("-")) {
    return { ...base, candidates: rank(prefix, flagItems(verbOf(args))) };
  }
  const previous = args[args.length - 1];
  const verb = verbOf(args);
  if (previous && takesOf(previous, verb) && !previous.includes("=")) {
    const kind = takesOf(previous, verb);
    return {
      ...base,
      candidates: rank(prefix, valueItems(kind, args.slice(0, -1), cluster, scope, prefix))
    };
  }
  const parsed = positionalWords(args);
  if (parsed.afterDashDash) return { ...base, candidates: rank(prefix, shellWords()) };
  const words = parsed.words;
  if (words.length === 0) {
    return {
      ...base,
      candidates: rank(
        prefix,
        VERBS.map((item) => ({
          value: item.name,
          detail: item.detail,
          kind: "verb",
          weight: item.weight
        }))
      )
    };
  }
  const rest = words.slice(1);
  if (prefix.includes("/") && rest.length === 0) {
    const slash = prefix.indexOf("/");
    const typeToken = prefix.slice(0, slash);
    const namePrefix = prefix.slice(slash + 1);
    return { ...base, candidates: rank(prefix, slashNames(typeToken, namePrefix, scope, cluster)) };
  }
  return {
    ...base,
    candidates: rank(prefix, candidatesFor(verb ?? "", rest, args, scope, cluster, prefix))
  };
}

// cli/confirm.ts
function allowYes(yes, argc, tty) {
  if (!yes) return false;
  if (argc === 0 && tty) return false;
  return true;
}
function readAnswer(chunks) {
  let answer = "";
  while (chunks.length > 0) {
    const next = chunks.shift() ?? "";
    const newline = next.search(/[\r\n]/);
    if (newline >= 0) {
      answer += next.slice(0, newline);
      const rest = next.slice(newline + 1);
      if (rest) chunks.unshift(rest);
      break;
    }
    answer += next;
  }
  const first = answer.trimStart()[0];
  return first === "y" || first === "Y";
}
function readConfirm(fd, read, warn) {
  const chunks = [];
  const buf = Buffer.alloc(1);
  try {
    for (; ; ) {
      const n = read(fd, buf);
      if (n <= 0) break;
      const text = buf.toString("utf8", 0, n);
      chunks.push(text);
      if (text === "\n" || text === "\r") break;
    }
  } catch (error) {
    const err = error;
    warn(`
kube: could not read the answer (${err.code ?? err.message}); not running it.
`);
    return false;
  }
  return readAnswer(chunks);
}

// cli/engine.ts
function toEngine(line, cursor, mode) {
  const c = Math.max(0, Math.min(cursor, line.length));
  const match = /^(\s*)(\S*)/.exec(line);
  const lead = match?.[1] ?? "";
  const first = match?.[2] ?? "";
  const firstEnd = lead.length + first.length;
  if (mode === "shell" && (first === "kube" || first === "kubectl-shell")) {
    if (c <= firstEnd) return { line, cursor: c, delta: 0, skip: true };
    const delta2 = "kubectl".length - first.length;
    return {
      line: `${lead}kubectl${line.slice(firstEnd)}`,
      cursor: c + delta2,
      delta: delta2,
      skip: false
    };
  }
  if (first === "kubectl" || first === "k" || first === "help") {
    return { line, cursor: c, delta: 0, skip: false };
  }
  if (shellMetaFirstWord(first)) {
    return { line, cursor: c, delta: 0, skip: false };
  }
  if (!first) {
    const engineLine = `${lead}kubectl `;
    return {
      line: engineLine,
      cursor: engineLine.length,
      delta: engineLine.length - lead.length,
      skip: false
    };
  }
  const delta = "kubectl ".length;
  return {
    line: `${lead}kubectl ${line.slice(lead.length)}`,
    cursor: c <= lead.length ? c : c + delta,
    delta,
    skip: false
  };
}
function spliceToken(line, start, end, value, pad) {
  const after = line.slice(end);
  const spaced = pad && !/[=/]$/.test(value);
  if (!spaced || after.startsWith(" ")) {
    const next = line.slice(0, start) + value + after;
    const cursor = start + value.length + (spaced && after.startsWith(" ") ? 1 : 0);
    return { line: next, cursor };
  }
  return {
    line: line.slice(0, start) + value + " " + after,
    cursor: start + value.length + 1
  };
}

// cli/terminal.ts
var SLEEP = new Int32Array(new SharedArrayBuffer(4));
function sleepMs(ms) {
  Atomics.wait(SLEEP, 0, 0, ms);
}
function refreshLine(rl) {
  rl._refreshLine?.();
}
function plainText(value) {
  let out = "";
  for (let i = 0; i < value.length; i += 1) {
    const code = value.charCodeAt(i);
    if (code === 27) {
      const next = value[i + 1];
      if (next === "]") {
        i += 2;
        while (i < value.length && value.charCodeAt(i) !== 7 && value.slice(i, i + 2) !== "\x1B\\")
          i += 1;
        if (value.slice(i, i + 2) === "\x1B\\") i += 1;
      } else if (next === "[") {
        i += 2;
        while (i < value.length && (value.charCodeAt(i) < 64 || value.charCodeAt(i) > 126))
          i += 1;
      }
      continue;
    }
    if (code < 32 || code >= 127 && code <= 159) continue;
    out += value[i];
  }
  return out;
}
function completionValue(value) {
  if (plainText(value) !== value) return null;
  if (/[`$;|\n]/.test(value)) return null;
  return value;
}

// cli/completion.ts
function completionScript(shell) {
  if (shell === "bash") {
    return `# bash completion for kube
_kube_completion() {
  local IFS=$'\\n'
  COMPREPLY=()
  mapfile -t COMPREPLY < <(kube __complete --cursor "$COMP_POINT" --line "$COMP_LINE" 2>/dev/null)
  if [[ \${#COMPREPLY[@]} -eq 1 ]]; then
    local one="\${COMPREPLY[0]}"
    if [[ "$one" != */ && "$one" != *= ]]; then
      COMPREPLY[0]="$one "
    fi
  fi
}
complete -o nospace -F _kube_completion kube
`;
  }
  if (shell === "zsh") {
    return `#compdef kube
_kube() {
  local -a vals
  vals=("\${(@f)$(kube __complete --cursor "$CURSOR" --line "$BUFFER" 2>/dev/null)}")
  (( \${#vals} )) || return
  if (( \${#vals} == 1 )); then
    local suf=' '
    [[ \${vals[1]} == */ || \${vals[1]} == *= ]] && suf=''
    compadd -S "$suf" -- "\${vals[1]}"
  else
    compadd -- "\${vals[@]}"
  fi
}
if (( \${+functions[compdef]} )); then
  compdef _kube kube
fi
`;
  }
  if (shell === "fish") {
    return `function __kube_complete
  kube __complete --cursor (commandline -C) --line (commandline -b)
end
complete -c kube -f -a '(__kube_complete)'
`;
  }
  return "";
}

// cli/manifest.ts
var import_node_fs2 = __toESM(require("node:fs"), 1);
var import_node_path2 = __toESM(require("node:path"), 1);
function manifestRefs(file) {
  let stat;
  try {
    stat = import_node_fs2.default.statSync(file);
  } catch (error) {
    const err = error;
    return { ok: false, error: err.code ?? err.message };
  }
  if (stat.isDirectory()) {
    const refs = [];
    for (const name of import_node_fs2.default.readdirSync(file)) {
      if (!/\.(ya?ml|json)$/i.test(name)) continue;
      const nested = manifestRefs(import_node_path2.default.join(file, name));
      if (!nested.ok) return nested;
      refs.push(...nested.refs);
    }
    return { ok: true, refs };
  }
  try {
    return { ok: true, refs: parseManifest(import_node_fs2.default.readFileSync(file, "utf8")) };
  } catch (error) {
    const err = error;
    return { ok: false, error: err.code ?? err.message };
  }
}
function parseManifest(text) {
  const trimmed = text.trim();
  if (!trimmed) return [];
  if (trimmed.startsWith("{") || trimmed.startsWith("[")) {
    try {
      return jsonRefs(JSON.parse(trimmed));
    } catch {
      return [];
    }
  }
  const refs = [];
  for (const doc of trimmed.split(/\r?\n---\r?\n/)) {
    const ref = yamlRef(doc);
    if (ref) refs.push(ref);
  }
  return refs;
}
function jsonRefs(value) {
  if (Array.isArray(value)) return value.flatMap(jsonRefs);
  if (!value || typeof value !== "object") return [];
  const doc = value;
  if (Array.isArray(doc.items)) return jsonRefs(doc.items);
  if (!doc.kind || !doc.metadata?.name) return [];
  return [{ kind: doc.kind, name: doc.metadata.name, namespace: doc.metadata.namespace }];
}
function yamlRef(doc) {
  let kind = "";
  let name = "";
  let namespace;
  let inMeta = false;
  let metaIndent = 0;
  for (const raw of doc.split(/\r?\n/)) {
    if (!raw.trim() || raw.trimStart().startsWith("#")) continue;
    const indent = raw.length - raw.trimStart().length;
    const line = raw.trim();
    if (inMeta && indent <= metaIndent) inMeta = false;
    const match = /^([A-Za-z0-9_]+):\s*(.*)$/.exec(line);
    if (!match) continue;
    const key = match[1];
    const value = strip(match[2] ?? "");
    if (!inMeta && indent === 0 && key === "kind" && value) kind = value;
    if (!inMeta && indent === 0 && key === "metadata") {
      inMeta = true;
      metaIndent = indent;
      continue;
    }
    if (inMeta && indent > metaIndent) {
      if (key === "name" && value && !name) name = value;
      if (key === "namespace" && value && namespace === void 0) namespace = value;
    }
  }
  if (!kind || !name) return null;
  return { kind, name, namespace };
}
function strip(value) {
  const cut = value.split(/\s+#/)[0]?.trim() ?? "";
  if (cut.length >= 2 && (cut.startsWith('"') && cut.endsWith('"') || cut.startsWith("'") && cut.endsWith("'"))) {
    return cut.slice(1, -1);
  }
  return cut;
}

// cli/blast.ts
var BLAST_BUDGET_MS = 3e3;
var subjects = /* @__PURE__ */ new WeakMap();
function skipped(reason) {
  return { status: "skipped", reason, objects: [], objectCount: 0, services: [] };
}
function unknown(reason) {
  return { status: "unknown", reason, objects: [], objectCount: 0, services: [] };
}
function noun(kind, count) {
  return count === 1 ? kind : `${kind}s`;
}
function orderAndCap(objects) {
  const counts = /* @__PURE__ */ new Map();
  for (const object of objects) {
    const key = `${object.namespace}\0${object.kind}`;
    const group = counts.get(key) ?? { namespace: object.namespace, kind: object.kind, count: 0 };
    group.count += 1;
    counts.set(key, group);
  }
  const ordered = objects.map((object, index) => ({ object, index }));
  ordered.sort((a, b) => a.object.namespace.localeCompare(b.object.namespace) || a.index - b.index);
  const groups = [...counts.values()].sort(
    (a, b) => a.namespace.localeCompare(b.namespace) || a.kind.localeCompare(b.kind)
  );
  return { shown: ordered.slice(0, 20).map((item) => item.object), groups };
}
function pack(objects, partial, extra = {}) {
  const { shown, groups } = orderAndCap(objects);
  return remember(
    { status: "ok", objects: shown, objectCount: objects.length, services: [], ...extra },
    { ...partial, groups }
  );
}
function formatOk(blast) {
  const lines = ["blast"];
  const subject = subjects.get(blast);
  const groups = subject?.groups ?? [];
  if (blast.objectCount === 0 || groups.length === 0) {
    const kind = subject?.kind ?? "object";
    const ns = subject?.namespace ?? "default";
    lines.push(`  0 ${noun(kind, 0)} in namespace ${ns}${fromSuffix(blast, ns, kind)}`);
  } else {
    for (const group of groups) {
      lines.push(
        `  ${group.count} ${noun(group.kind, group.count)} in namespace ${group.namespace}${fromSuffix(blast, group.namespace, group.kind)}`
      );
    }
    for (const object of blast.objects) lines.push(objectLine(blast, object));
    if (blast.objectCount > 20) lines.push(`  and ${blast.objectCount - 20} more`);
  }
  const multi = new Set(groups.map((group) => group.namespace)).size > 1;
  for (const service of blast.services) lines.push(serviceLine(blast, service, multi));
  if (blast.serviceNote) lines.push(`  ${blast.serviceNote}`);
  for (const pdb of blast.pdbs ?? []) {
    const word = pdb.disruptionsAllowed === 1 ? "disruption" : "disruptions";
    lines.push(`  pdb/${pdb.name} allows ${pdb.disruptionsAllowed} ${word}`);
  }
  if (blast.pdbNote) lines.push(`  ${blast.pdbNote}`);
  for (const hpa of blast.hpas ?? []) {
    lines.push(
      `  hpa/${hpa.name} keeps ${hpa.target} between ${hpa.minReplicas} and ${hpa.maxReplicas}`
    );
  }
  if (blast.dryRun?.status === "ok") lines.push(`  dry-run: ${blast.dryRun.count ?? 0} objects`);
  if (blast.dryRun?.status === "unknown") {
    lines.push(`  dry-run: unknown (${blast.dryRun.reason ?? "failed"})`);
  }
  return lines.join("\n");
}
function objectLine(blast, object) {
  const owner = blast.owners?.find(
    (item) => item.pod === object.name && item.namespace === object.namespace
  );
  const owned = owner ? ` owned by ${owner.kind}/${owner.name}` : "";
  return `  ${object.kind}/${object.name}${owned}`;
}
function fromSuffix(blast, namespace, kind) {
  if (kind !== "pod" || !blast.controllers?.length) return "";
  const names = blast.controllers.filter((item) => item.namespace === namespace).map((item) => `${item.kind}/${item.name}`);
  return names.length > 0 ? `, from ${names.join(", ")}` : "";
}
function deletedPodsIn(blast, namespace) {
  const groups = subjects.get(blast)?.groups;
  if (groups) {
    return groups.filter((group) => group.namespace === namespace && group.kind === "pod").reduce((sum, group) => sum + group.count, 0);
  }
  return blast.objects.filter((object) => object.kind === "pod" && object.namespace === namespace).length;
}
function serviceLine(blast, service, multi) {
  const where = multi ? ` in namespace ${service.namespace}` : "";
  const deleted = deletedPodsIn(blast, service.namespace);
  const matched = service.matchedPods === deleted && deleted > 1 ? `matches all ${service.matchedPods}` : `matches ${service.matchedPods}`;
  const ready = service.readyPodsRemaining === 0 ? "would have 0 ready pods" : `would keep ${service.readyPodsRemaining} ready pod${service.readyPodsRemaining === 1 ? "" : "s"}`;
  return `  service/${service.name}${where} ${matched} and ${ready}`;
}
function formatBlast(blast) {
  if (blast.status === "skipped" && blast.reason === "not a delete") return "";
  if (blast.status === "skipped" && blast.reason === "no name or selector") {
    return "blast: not expanded (no name or selector)";
  }
  if (blast.status === "skipped") return `blast: skipped (${blast.reason ?? ""})`;
  if (blast.status === "unknown") return `blast: unknown (${blast.reason ?? ""})`;
  return formatOk(blast);
}
function itemsOf(json, expectedKind) {
  if (!json || typeof json !== "object") return null;
  const doc = json;
  if (expectedKind && doc.kind === expectedKind && doc.metadata?.name) return [doc];
  if (!expectedKind && doc.kind && doc.metadata?.name) return [doc];
  if (typeof doc.kind !== "string" || doc.kind !== "List" && !doc.kind.endsWith("List")) {
    return null;
  }
  if (!Array.isArray(doc.items)) return null;
  const items = [];
  for (const item of doc.items) {
    if (!item || typeof item !== "object") return null;
    const object = item;
    if (!object.metadata?.name) return null;
    if (expectedKind && object.kind !== expectedKind) return null;
    if (!expectedKind && !object.kind) return null;
    items.push(object);
  }
  return items;
}
function askedOf(args) {
  let namespace;
  let context;
  let allNamespaces = false;
  for (let i = 0; i < args.length; i += 1) {
    const token = args[i];
    if (token === "-A" || token === "--all-namespaces" || token === "-A=true" || token === "--all-namespaces=true") {
      allNamespaces = true;
    } else if (token.startsWith("-n=")) namespace = token.slice("-n=".length);
    else if ((token === "-n" || token === "--namespace") && args[i + 1]) namespace = args[++i];
    else if (token.startsWith("--namespace=")) namespace = token.slice("--namespace=".length);
    else if (token === "--context" && args[i + 1]) context = args[++i];
    else if (token.startsWith("--context=")) context = token.slice("--context=".length);
  }
  return { namespace, allNamespaces, context };
}
function askedFlags(asked) {
  const flags = [];
  if (asked.namespace) flags.push("-n", asked.namespace);
  if (asked.context) flags.push("--context", asked.context);
  if (asked.allNamespaces) flags.push("-A");
  return flags;
}
function withContext(flags, context) {
  return context ? [...flags, "--context", context] : flags;
}
function targetOf(args) {
  const rest = positionalWords(args).words.slice(1);
  const selector = flagValue(args, ["-l", "--selector"]);
  const all = args.some((token2) => token2 === "--all" || token2 === "--all=true");
  const slash = rest.filter((word) => word.includes("/"));
  if (slash.length > 0) {
    if (slash.length !== rest.length) return "mixed";
    const kinds = slash.map((word) => word.slice(0, word.indexOf("/")));
    const names2 = slash.map((word) => word.slice(word.indexOf("/") + 1));
    if (new Set(kinds).size !== 1 || names2.some((name) => name.length === 0)) return "mixed";
    const token2 = kinds[0];
    return { token: token2, names: names2, resource: resolveResource(token2), selector: void 0, all: false };
  }
  const token = rest[0];
  if (!token) return null;
  const resource = resolveResource(token);
  const names = rest.slice(1);
  if (!all && !selector && names.length === 0) return null;
  if (all) return { token, names: [], resource, selector: void 0, all: true };
  if (selector) return { token, names: [], resource, selector, all: false };
  return { token, names, resource, selector: void 0, all: false };
}
function remember(blast, subject) {
  subjects.set(blast, subject);
  return blast;
}
function expired(fetch) {
  return fetch.now() - fetch.started >= BLAST_BUDGET_MS;
}
function jsonArgs(resource, middle, scope) {
  return ["get", resource, ...middle, ...scope, "-o", "json", "--request-timeout=4s"];
}
function fetchJson(fetch, args) {
  if (expired(fetch)) return "timeout";
  const result = fetch.getJson(args);
  if (expired(fetch)) return "timeout";
  return result;
}
function scopeFlags(namespaces) {
  if (namespaces.length > 1) return ["-A"];
  const ns = namespaces[0];
  return ns ? ["-n", ns] : [];
}
function stringMap(value) {
  if (!value || typeof value !== "object" || Array.isArray(value)) return null;
  const entries = Object.entries(value);
  if (entries.length === 0) return null;
  const map = {};
  for (const [key, entry] of entries) {
    if (typeof entry !== "string") return null;
    map[key] = entry;
  }
  return map;
}
function labelsMatch(selector, labels) {
  return Object.entries(selector).every(([key, value]) => labels[key] === value);
}
function podRef(item, fallbackNs) {
  const name = item.metadata?.name;
  if (!name) return null;
  return {
    namespace: item.metadata?.namespace || fallbackNs,
    name,
    labels: item.metadata?.labels ?? {},
    ready: item.status?.conditions?.some(
      (condition) => condition.type === "Ready" && condition.status === "True"
    ) === true
  };
}
var CONTROLLERS = /* @__PURE__ */ new Set(["Deployment", "StatefulSet", "DaemonSet", "ReplicaSet"]);
function selectorOf(item) {
  const raw = item.spec?.selector;
  if (!raw || typeof raw !== "object" || Array.isArray(raw)) return null;
  const selector = raw;
  if (Array.isArray(selector.matchExpressions) && selector.matchExpressions.length > 0) return null;
  return stringMap(selector.matchLabels);
}
function labelSelector(labels) {
  return Object.keys(labels).sort().map((key) => `${key}=${labels[key]}`).join(",");
}
function expandControllers(items, controllers, fetch, asked, fallbackNs, partial) {
  const named = { controllers: controllers.slice(0, 20) };
  const selectors = [];
  const namespaces = /* @__PURE__ */ new Set();
  for (const item of items) {
    const selector = selectorOf(item);
    if (!selector) return withExtras(pack(controllers, partial, named), [], fetch, asked);
    selectors.push(selector);
    namespaces.add(item.metadata?.namespace || fallbackNs);
  }
  const encoded = new Set(selectors.map(labelSelector));
  if (encoded.size !== 1 || namespaces.size > 1 && !asked.allNamespaces) {
    return withExtras(pack(controllers, partial, named), [], fetch, asked);
  }
  const scope = asked.allNamespaces ? withContext(["-A"], asked.context) : withContext(scopeFlags([...namespaces]), asked.context);
  const result = fetchJson(fetch, jsonArgs("pods", ["-l", [...encoded][0]], scope));
  if (result === "timeout") return unknown("timed out");
  if (!result.ok) return unknown(`kubectl get pods: ${result.error || "failed"}`);
  const podItems = itemsOf(result.json, "Pod");
  if (!podItems) return unknown("kubectl get pods: not a Pod object or list");
  const objects = podItems.flatMap((item) => {
    const name = item.metadata?.name;
    if (!name) return [];
    return [{ kind: "pod", namespace: item.metadata?.namespace || fallbackNs, name }];
  });
  const pods = podItems.flatMap((item) => {
    const pod = podRef(item, fallbackNs);
    return pod ? [pod] : [];
  });
  const podBase = pack(objects, { kind: "pod", namespace: fallbackNs }, named);
  const withServices = attachServices(podBase, pods, fetch, fallbackNs, {
    context: asked.context,
    reuse: false
  });
  if (withServices.status !== "ok") return withServices;
  return withExtras(remember(withServices, subjects.get(podBase)), pods, fetch, asked);
}
function serviceNote(base, error) {
  return { ...base, services: [], serviceNote: `services unknown (${error || "failed"})` };
}
function attachServices(base, pods, fetch, fallbackNs, options) {
  if (pods.length === 0) return base;
  if (pods.length > 200) {
    return {
      ...base,
      services: [],
      serviceNote: `service check skipped (${pods.length} pods exceeds 200)`
    };
  }
  const namespaces = [...new Set(pods.map((pod) => pod.namespace))];
  if (namespaces.length > 5) {
    return {
      ...base,
      services: [],
      serviceNote: `service check skipped (${namespaces.length} namespaces exceeds 5)`
    };
  }
  const services = fetchJson(
    fetch,
    jsonArgs("services", [], withContext(scopeFlags(namespaces), options.context))
  );
  if (services === "timeout") return serviceNote(base, "timed out");
  if (!services.ok) return serviceNote(base, services.error);
  const items = itemsOf(services.json, "Service");
  if (!items) return serviceNote(base, "not a Service object or list");
  const matched = [];
  for (const item of items) {
    const selector = stringMap(item.spec?.selector);
    const name = item.metadata?.name;
    const ns = item.metadata?.namespace || (namespaces.length === 1 ? namespaces[0] : "");
    if (!selector || !name || !ns) continue;
    const matchedPods = pods.filter(
      (pod) => pod.namespace === ns && labelsMatch(selector, pod.labels)
    ).length;
    if (matchedPods === 0) continue;
    matched.push({ namespace: ns, name, selector, matchedPods });
  }
  if (matched.length === 0) return base;
  let alive = pods;
  if (!options.reuse) {
    const serviceNamespaces = [...new Set(matched.map((item) => item.namespace))];
    const survivors = fetchJson(
      fetch,
      jsonArgs("pods", [], withContext(scopeFlags(serviceNamespaces), options.context))
    );
    if (survivors === "timeout") return serviceNote(base, "timed out");
    if (!survivors.ok) return serviceNote(base, survivors.error);
    const survivorItems = itemsOf(survivors.json, "Pod");
    if (!survivorItems) return serviceNote(base, "not a Pod object or list");
    alive = survivorItems.flatMap((item) => {
      const pod = podRef(item, fallbackNs);
      return pod ? [pod] : [];
    });
  }
  const deleted = new Set(pods.map((pod) => `${pod.namespace}/${pod.name}`));
  return {
    ...base,
    services: matched.map((item) => ({
      namespace: item.namespace,
      name: item.name,
      matchedPods: item.matchedPods,
      readyPodsRemaining: alive.filter(
        (pod) => pod.namespace === item.namespace && !deleted.has(`${pod.namespace}/${pod.name}`) && labelsMatch(item.selector, pod.labels) && pod.ready
      ).length
    }))
  };
}
function carry(next, prev) {
  const subject = subjects.get(prev);
  if (subject) subjects.set(next, subject);
  return next;
}
function podBudget(pods) {
  return pods.length > 0 && pods.length <= 200 && new Set(pods.map((pod) => pod.namespace)).size <= 5;
}
function withPdbs(blast, pods, fetch, asked) {
  if (!podBudget(pods)) return blast;
  const namespaces = [...new Set(pods.map((pod) => pod.namespace))];
  const result = fetchJson(
    fetch,
    jsonArgs("pdb", [], withContext(scopeFlags(namespaces), asked.context))
  );
  if (result === "timeout") return carry({ ...blast, pdbNote: "pdbs unknown (timed out)" }, blast);
  if (!result.ok)
    return carry({ ...blast, pdbNote: `pdbs unknown (${result.error || "failed"})` }, blast);
  const items = itemsOf(result.json, "PodDisruptionBudget");
  if (!items) {
    return carry(
      { ...blast, pdbNote: "pdbs unknown (not a PodDisruptionBudget object or list)" },
      blast
    );
  }
  const pdbs = [];
  for (const item of items) {
    const selector = selectorOf(item);
    const name = item.metadata?.name;
    const ns = item.metadata?.namespace || (namespaces.length === 1 ? namespaces[0] : "");
    const allowed = item.status?.disruptionsAllowed;
    if (!selector || !name || !ns || typeof allowed !== "number") continue;
    const matched = pods.some((pod) => pod.namespace === ns && labelsMatch(selector, pod.labels));
    if (!matched) continue;
    pdbs.push({ namespace: ns, name, disruptionsAllowed: allowed });
  }
  return carry({ ...blast, pdbs }, blast);
}
function withHpas(blast, fetch, asked) {
  const controllers = blast.controllers ?? [];
  if (controllers.length === 0) return blast;
  const namespaces = [...new Set(controllers.map((item) => item.namespace))];
  if (namespaces.length > 5) return blast;
  const result = fetchJson(
    fetch,
    jsonArgs("hpa", [], withContext(scopeFlags(namespaces), asked.context))
  );
  if (result === "timeout" || !result.ok) return blast;
  const items = itemsOf(result.json, "HorizontalPodAutoscaler");
  if (!items) return blast;
  const hpas = [];
  for (const item of items) {
    const ref = item.spec?.scaleTargetRef;
    const name = item.metadata?.name;
    if (!ref?.name || !name) continue;
    const target = controllers.find(
      (controller) => controller.name === ref.name && controller.kind === (ref.kind ?? "").toLowerCase()
    );
    if (!target) continue;
    hpas.push({
      namespace: item.metadata?.namespace || target.namespace,
      name,
      minReplicas: item.spec?.minReplicas ?? 1,
      maxReplicas: item.spec?.maxReplicas ?? item.spec?.minReplicas ?? 1,
      target: `${target.kind}/${target.name}`
    });
  }
  if (hpas.length === 0) return blast;
  return carry({ ...blast, hpas }, blast);
}
function withExtras(blast, pods, fetch, asked) {
  return withHpas(withPdbs(blast, pods, fetch, asked), fetch, asked);
}
function dryRunArgs(args) {
  const next = [...args];
  if (!next.some((token) => token === "--dry-run" || token.startsWith("--dry-run="))) {
    next.push("--dry-run=server");
  }
  if (!next.includes("-o") && !next.some((token) => token.startsWith("--output"))) {
    next.push("-o", "json");
  }
  if (!next.some((token) => token === "--request-timeout" || token.startsWith("--request-timeout="))) {
    next.push("--request-timeout=4s");
  }
  return next;
}
function countOf(json) {
  if (!json || typeof json !== "object") return null;
  const doc = json;
  if (Array.isArray(doc.items)) return doc.items.length;
  if (doc.metadata?.name) return 1;
  return null;
}
function withDryRun(blast, args, fetch) {
  if (args[0] !== "delete" || blast.status === "skipped") return blast;
  const result = fetchJson(fetch, dryRunArgs(args));
  if (result === "timeout") {
    return carry({ ...blast, dryRun: { status: "unknown", reason: "timed out" } }, blast);
  }
  if (!result.ok) {
    return carry(
      { ...blast, dryRun: { status: "unknown", reason: result.error || "failed" } },
      blast
    );
  }
  const count = countOf(result.json);
  if (count === null) {
    return carry({ ...blast, dryRun: { status: "unknown", reason: "not a delete result" } }, blast);
  }
  return carry({ ...blast, dryRun: { status: "ok", count } }, blast);
}
function ownerOf(item, namespace, name) {
  const owner = item.metadata?.ownerReferences?.find((ref) => ref.controller) ?? item.metadata?.ownerReferences?.[0];
  if (!owner?.kind || !owner.name) return [];
  return [{ namespace, pod: name, kind: owner.kind.toLowerCase(), name: owner.name }];
}
function fromItems(items, resource, asked, fetch, fallbackNs, reuse) {
  const owners = [];
  const objects = items.flatMap((item) => {
    const name = item.metadata?.name;
    if (!name) return [];
    const kind = resource?.singular ?? (item.kind ?? "object").toLowerCase();
    const namespace = item.metadata?.namespace || fallbackNs;
    if (kind === "pod") owners.push(...ownerOf(item, namespace, name) ?? []);
    return [{ kind, namespace, name }];
  });
  const singular = resource?.singular ?? (objects.every((object) => object.kind === "pod") ? "pod" : objects[0]?.kind ?? "object");
  const partial = { kind: singular, namespace: fallbackNs };
  if (items.length > 0 && items.every((item) => CONTROLLERS.has(item.kind ?? ""))) {
    return expandControllers(items, objects, fetch, asked, fallbackNs, partial);
  }
  const base = pack(objects, partial, owners.length ? { owners } : {});
  if (singular !== "pod" && !(!resource && objects.some((object) => object.kind === "pod"))) {
    return base;
  }
  const pods = items.flatMap((item) => {
    if (item.kind !== "Pod" && resource?.singular !== "pod") return [];
    if (resource?.singular === "pod" || item.kind === "Pod") {
      const pod = podRef(item, fallbackNs);
      return pod ? [pod] : [];
    }
    return [];
  });
  const withServices = attachServices(base, pods, fetch, fallbackNs, {
    context: asked.context,
    reuse
  });
  if (withServices.status !== "ok") return withServices;
  return withExtras(remember(withServices, subjects.get(base)), pods, fetch, asked);
}
function isKustomize(args) {
  return args.some((token) => {
    const name = token.split("=", 1)[0];
    return name === "-k" || name === "--kustomize";
  });
}
function manifestFiles(args) {
  const files = [];
  for (let i = 0; i < args.length; i += 1) {
    const token = args[i];
    if (token === "-f" || token === "--filename") {
      if (args[i + 1]) files.push(args[i += 1]);
    } else if (token.startsWith("--filename=")) files.push(token.slice("--filename=".length));
    else if (token.startsWith("-f=")) files.push(token.slice("-f=".length));
  }
  return files;
}
function isDaemon(item) {
  return item.metadata?.ownerReferences?.some((owner) => owner.kind === "DaemonSet") === true;
}
function blastFiles(files, plan, asked, fetch) {
  const fallbackNs = asked.namespace || plan.namespace;
  const groups = /* @__PURE__ */ new Map();
  for (const file of files) {
    const read = manifestRefs(file);
    if (!read.ok) return unknown(`manifest: ${read.error}`);
    for (const ref of read.refs) {
      const namespace = ref.namespace || fallbackNs;
      const key = `${ref.kind.toLowerCase()}\0${namespace}`;
      const list = groups.get(key) ?? [];
      list.push({ ...ref, namespace });
      groups.set(key, list);
    }
  }
  if (groups.size === 0) return skipped("no name or selector");
  const items = [];
  for (const group of groups.values()) {
    const token = group[0].kind.toLowerCase();
    const resource = resolveResource(token);
    const namespace = group[0].namespace || fallbackNs;
    const result = fetchJson(
      fetch,
      jsonArgs(
        resource?.singular ?? token,
        group.map((ref) => ref.name),
        withContext(["-n", namespace], asked.context)
      )
    );
    if (result === "timeout")
      return items.length ? fromItems(items, void 0, asked, fetch, fallbackNs, false) : unknown("timed out");
    if (!result.ok) {
      return items.length ? fromItems(items, void 0, asked, fetch, fallbackNs, false) : unknown(`kubectl get ${token}: ${result.error || "failed"}`);
    }
    const parsed = itemsOf(result.json, resource?.apiKind);
    if (!parsed) {
      return items.length ? fromItems(items, void 0, asked, fetch, fallbackNs, false) : unknown(`kubectl get ${token}: not a ${resource?.apiKind ?? "object"} object or list`);
    }
    items.push(...parsed);
  }
  return fromItems(items, void 0, asked, fetch, fallbackNs, false);
}
function blastDrain(args, plan, fetch, asked) {
  const node = positionalWords(args).words[1];
  if (!node) return skipped("no name or selector");
  const fallbackNs = asked.namespace || plan.namespace;
  const scope = asked.namespace && !asked.allNamespaces ? ["-n", asked.namespace] : ["-A"];
  const result = fetchJson(
    fetch,
    jsonArgs(
      "pods",
      ["--field-selector", `spec.nodeName=${node}`],
      withContext(scope, asked.context)
    )
  );
  if (result === "timeout") return unknown("timed out");
  if (!result.ok) return unknown(`kubectl get pods: ${result.error || "failed"}`);
  const parsed = itemsOf(result.json, "Pod");
  if (!parsed) return unknown("kubectl get pods: not a Pod object or list");
  return fromItems(
    parsed.filter((item) => !isDaemon(item)),
    resolveResource("pod"),
    asked,
    fetch,
    fallbackNs,
    false
  );
}
function blastRadius(plan, args, getJson, clock) {
  const now = clock?.now ?? Date.now;
  const started = now();
  const fetch = { getJson, now, started };
  const asked = askedOf(args);
  const scaleZero = plan.verb === "scale" && flagValue(args, ["--replicas"]) === "0";
  if (plan.verb === "drain") return blastDrain(args, plan, fetch, asked);
  if (plan.verb !== "delete" && !scaleZero) return skipped("not a delete");
  if (isKustomize(args)) return skipped("kustomize delete (-k) is not expanded");
  const files = manifestFiles(args);
  if (files.some((file) => /^https?:\/\//i.test(file))) {
    return skipped("remote manifest URLs are not expanded");
  }
  if (files.length > 0) return withDryRun(blastFiles(files, plan, asked, fetch), args, fetch);
  const target = targetOf(args);
  if (target === "mixed") return skipped("mixed resource kinds");
  if (!target) return skipped("no name or selector");
  const middle = target.selector ? ["-l", target.selector] : target.all ? [] : target.names;
  const result = fetchJson(fetch, jsonArgs(target.token, middle, askedFlags(asked)));
  if (result === "timeout") return unknown("timed out");
  if (!result.ok) return unknown(`kubectl get ${target.token}: ${result.error || "failed"}`);
  const items = itemsOf(result.json, target.resource?.apiKind);
  if (!items) {
    const kind = target.resource?.apiKind ?? "object";
    return unknown(`kubectl get ${target.token}: not a ${kind} object or list`);
  }
  const finished = fromItems(
    items,
    target.resource,
    asked,
    fetch,
    asked.namespace || plan.namespace,
    target.all && (target.resource?.singular === "pod" || target.token === "pods" || target.token === "pod")
  );
  return withDryRun(finished, args, fetch);
}

// cli/reveal-output.ts
function outputFormatToken(token) {
  if (token === "-o" || token === "--output") return void 0;
  if (token.startsWith("-o=")) return token.slice(3);
  if (token.startsWith("--output=")) return token.slice("--output=".length);
  return void 0;
}
function isRevealingFormat(fmt) {
  if (fmt === "yaml" || fmt === "json" || fmt === "wide") return true;
  if (fmt.startsWith("jsonpath")) return true;
  if (fmt === "template" || fmt.startsWith("go-template")) return true;
  if (fmt.startsWith("custom-columns")) return true;
  return false;
}
function revealingOutputFormat(tokens) {
  const output = flagValue(tokens, ["-o", "--output"]);
  if (output && isRevealingFormat(output)) return true;
  return tokens.some((token) => {
    const inline = outputFormatToken(token);
    if (inline) return isRevealingFormat(inline);
    return token === "-o=wide" || token === "--output=wide";
  });
}
function hasFlag(tokens, name) {
  return tokens.some((token) => token === name || token.startsWith(`${name}=`));
}
function resourceTokenKind(token) {
  const resource = resolveResource(token);
  if (resource?.kind === "secret" || token === "secret" || token === "secrets") return "secret";
  if (resource?.kind === "configmap" || token === "configmap" || token === "configmaps")
    return "configmap";
  return null;
}
function sensitiveStdoutKind(tokens) {
  const { words } = positionalWords(tokens);
  const verb = words[0] ?? "";
  if (verb === "get" && revealingOutputFormat(tokens)) {
    for (let i = 1; i < words.length; i += 1) {
      const kind = resourceTokenKind(words[i]);
      if (kind) return kind;
    }
    return null;
  }
  if (verb === "describe") {
    return resourceTokenKind(words[1] ?? "");
  }
  if (verb === "config" && words[1] === "view" && hasFlag(tokens, "--raw")) return "kubeconfig";
  return null;
}

// cli/insight.ts
function argsOf(engineLine) {
  const tokens = tokenize(engineLine).map((token) => unquote(token.text));
  if (tokens[0] === "kubectl" || tokens[0] === "k") return tokens.slice(1);
  return tokens;
}
function has(tokens, name) {
  return tokens.some((token) => token === name || token.startsWith(`${name}=`));
}
function dryRun(tokens) {
  return tokens.some((token) => token === "--dry-run" || token.startsWith("--dry-run="));
}
function riskOf(verb, sub, tokens) {
  if (verb === "delete" || verb === "drain" || verb === "cordon" || verb === "taint") return "high";
  if (verb === "cp" || verb === "attach" || verb === "exec" || verb === "debug") return "high";
  if (verb === "port-forward" || verb === "proxy") return "high";
  if (verb === "certificate" && (sub === "approve" || sub === "deny")) return "high";
  if (verb === "config" && sub === "set-credentials") return "high";
  if (!dryRun(tokens) && (verb === "run" || verb === "expose")) return "high";
  if (verb === "scale" && flagValue(tokens, ["--replicas"]) === "0") return "high";
  if (verb === "rollout" && (sub === "undo" || sub === "restart")) return "high";
  if (verb === "replace" && has(tokens, "--force")) return "high";
  if (verb === "config" && (sub === "delete-context" || sub === "delete-cluster" || sub === "delete-user"))
    return "high";
  if (!dryRun(tokens) && ["apply", "create", "replace", "patch"].includes(verb)) return "high";
  if ([
    "scale",
    "edit",
    "label",
    "annotate",
    "set",
    "rollout",
    "run",
    "expose",
    "autoscale",
    "uncordon",
    "apply",
    "create",
    "replace",
    "patch"
  ].includes(verb)) {
    return "low";
  }
  if (verb === "config" && ["use-context", "set-context", "set-cluster", "set"].includes(sub))
    return "low";
  return "read";
}
function summaryFor(plan, sub, tokens) {
  const where = plan.allNamespaces ? "every namespace" : plan.namespace;
  const what = plan.targets.join(", ") || plan.verb;
  const reveal = sensitiveStdoutKind(tokens);
  if (reveal === "secret") {
    return `Reads ${what} in ${where}. kubectl may print Secret data on stdout; kube does not redact it unless KUBE_SHELL_REDACT_STDOUT=1 and stdout is not a TTY.`;
  }
  if (reveal === "configmap") {
    return `Reads ${what} in ${where}. kubectl may print ConfigMap data on stdout; kube does not redact it unless KUBE_SHELL_REDACT_STDOUT=1 and stdout is not a TTY.`;
  }
  if (reveal === "kubeconfig") {
    return `Prints the kubeconfig including client credentials. kube does not redact it unless KUBE_SHELL_REDACT_STDOUT=1 and stdout is not a TTY.`;
  }
  if (plan.verb === "port-forward") return `Opens a local port to ${what} in ${where}.`;
  if (plan.verb === "proxy") return `Starts a local API proxy using your kubeconfig credentials.`;
  if (plan.verb === "delete")
    return `Deletes ${what} in ${where}. A controller may recreate a pod.`;
  if (plan.verb === "drain") return `Drains ${what}. Pods are evicted off that node.`;
  if (plan.verb === "scale" && plan.risk === "high")
    return `Scales ${what} to zero replicas in ${where}.`;
  if (plan.verb === "rollout" && sub === "undo")
    return `Rolls ${what} back to the previous revision.`;
  if (plan.verb === "rollout" && sub === "restart") return `Restarts ${what} in ${where}.`;
  if (plan.verb === "cp") return `Copies files to or from ${what}.`;
  if (plan.verb === "attach") return `Attaches to a process in ${what}.`;
  if (plan.verb === "exec")
    return `Runs a process inside ${what}. The container filesystem can change.`;
  if (plan.verb === "debug") return `Starts a debug container for ${what}.`;
  if (plan.risk === "read") return `Reads ${what} in ${where}. Nothing in the cluster changes.`;
  if (plan.verb === "patch") return `Patches ${what} in ${where}.`;
  if (plan.verb === "apply" || plan.verb === "create" || plan.verb === "replace")
    return `Writes ${what} in ${where}.`;
  return `Changes ${what} in ${where}.`;
}
function planCommand(engineLine, cluster) {
  const args = argsOf(engineLine);
  const scope = scopeOf(args, cluster);
  const parsed = positionalWords(args);
  const verb = parsed.words[0] ?? "";
  const sub = parsed.words[1] ?? "";
  const resource = resolveResource(sub);
  const names = resource ? parsed.words.slice(2) : verb === "drain" || verb === "cordon" || verb === "uncordon" ? parsed.words.slice(1) : [];
  const kind = resource?.singular || (verb === "drain" ? "node" : "");
  const targets2 = names.length > 0 ? names.map((name) => kind ? `${kind}/${name}` : name) : sub ? [has(args, "--all") ? `all ${sub}` : sub] : [];
  const risk = riskOf(verb, resource ? parsed.words[2] ?? "" : sub, args);
  const plan = {
    command: ["kubectl", ...args].join(" "),
    verb,
    context: scope.context,
    namespace: scope.ns || namespaceOf(cluster),
    allNamespaces: scope.all,
    targets: targets2,
    mutating: risk !== "read",
    risk,
    summary: ""
  };
  plan.summary = summaryFor(
    plan,
    verb === "rollout" ? sub : resource ? parsed.words[1] ?? "" : sub,
    args
  );
  return plan;
}
function formatPlan(plan) {
  const where = plan.allNamespaces ? "all namespaces" : `namespace ${plan.namespace}`;
  return `${plan.command}
context ${plan.context} \xB7 ${where} \xB7 risk ${plan.risk}
${plan.summary}`;
}
function unhealthy(cluster) {
  const hit = cluster.objects.find(
    (item) => /CrashLoop|ImagePull|ErrImage|Error|Pending|BackOff|OOM/i.test(item.status ?? "")
  );
  if (!hit) return void 0;
  return { name: hit.name, kind: hit.kind, ns: hit.ns, status: hit.status ?? hit.kind };
}
function mentioned(question, cluster) {
  const q = question.toLowerCase();
  const ranked = [...cluster.objects].sort((a, b) => b.name.length - a.name.length);
  for (const item of ranked) {
    if (item.name.length >= 3 && q.includes(item.name.toLowerCase())) {
      return { name: item.name, kind: item.kind, ns: item.ns, status: item.status ?? item.kind };
    }
  }
  return void 0;
}
function nsSuffix(hit, cluster) {
  if (!hit) return "";
  if (hit.ns && hit.ns !== namespaceOf(cluster)) return ` -n ${hit.ns}`;
  return "";
}
function podish(hit) {
  if (!hit) return "pod";
  if (hit.kind === "pod") return "pod";
  return hit.kind;
}
function advise(question, cluster) {
  const q = question.trim().toLowerCase();
  const named = mentioned(question, cluster);
  const sick = named ?? unhealthy(cluster);
  const ns = nsSuffix(sick, cluster);
  const name = sick?.name ?? "<name>";
  const kind = podish(sick);
  const whyName = sick ? `${sick.name} is ${sick.status} in ${sick.ns}.` : "No matching object in this context.";
  if (!q) {
    return {
      why: "Ask about a pod, a crash, a pending schedule, or the current context.",
      commands: ["get pods", "get events"]
    };
  }
  if (/context|where am i|which cluster/.test(q)) {
    return {
      why: `Current context is ${cluster.current}, namespace ${namespaceOf(cluster)}.`,
      commands: ["config current-context", "config get-contexts"]
    };
  }
  if (/namespace|which ns|what ns/.test(q)) {
    return {
      why: `Namespace on ${cluster.current} is ${namespaceOf(cluster)}.`,
      commands: ["get ns", "config view --minify"]
    };
  }
  if (/crash|backoff|restart|oom|not starting|failing/.test(q)) {
    return {
      why: whyName,
      commands: [`describe ${kind} ${name}${ns}`, `logs ${name} --tail=80${ns}`, `get events${ns}`]
    };
  }
  if (/pending|unschedul|cannot schedule|can't schedule|insufficient/.test(q)) {
    return {
      why: whyName,
      commands: [`describe ${kind} ${name}${ns}`, "get nodes", `get events${ns}`]
    };
  }
  if (/imagepull|errimage|pull back|registry/.test(q)) {
    return { why: whyName, commands: [`describe ${kind} ${name}${ns}`, `get events${ns}`] };
  }
  if (/not ready|unhealthy|readiness|liveness|probe/.test(q)) {
    return { why: whyName, commands: [`describe ${kind} ${name}${ns}`, `get pods${ns}`] };
  }
  if (/\blogs?\b/.test(q)) {
    return { why: whyName, commands: [`logs ${name} --tail=100${ns}`] };
  }
  if (/event/.test(q)) {
    return {
      why: "Recent events explain most scheduling and probe failures.",
      commands: [`get events${ns}`]
    };
  }
  if (/node/.test(q)) {
    return {
      why: "Node pressure shows up as Pending pods and evictions.",
      commands: ["get nodes", "describe nodes"]
    };
  }
  if (/\btop\b|cpu|memory hog|using memory/.test(q)) {
    return {
      why: "Metrics need the metrics-server. If top fails, describe the pod instead.",
      commands: [`top pods${ns}`, "top nodes"]
    };
  }
  if (/endpoint|service .*down|svc .*down|can.?t connect/.test(q)) {
    return {
      why: "A service with no endpoints has no ready pods behind it.",
      commands: [`get endpoints${ns}`, `get pods${ns}`]
    };
  }
  if (/port-?forward|forward/.test(q)) {
    return {
      why: `Forward a local port to ${name}.`,
      commands: [`port-forward ${kind}/${name} 8080:80${ns}`]
    };
  }
  if (/what('?s| is) running|all pods|list pods|show pods/.test(q)) {
    return { why: whyName, commands: [`get pods${ns}`, `get deploy,svc${ns}`] };
  }
  return {
    why: whyName,
    commands: [`get pods${ns}`, `describe ${kind} ${name}${ns}`, `get events${ns}`]
  };
}
function formatAdvice(advice) {
  return [advice.why, "", ...advice.commands.map((line) => `kubectl ${line}`)].join("\n");
}

// cli/live.ts
var import_node_child_process = require("node:child_process");
var import_node_fs4 = __toESM(require("node:fs"), 1);
var import_node_path4 = __toESM(require("node:path"), 1);

// cli/private.ts
var import_node_fs3 = __toESM(require("node:fs"), 1);
var import_node_os = __toESM(require("node:os"), 1);
var import_node_path3 = __toESM(require("node:path"), 1);
function stateDir() {
  return import_node_path3.default.join(import_node_os.default.homedir(), ".kube-shell");
}
function writePrivate(file, data) {
  const dir = import_node_path3.default.dirname(file);
  import_node_fs3.default.mkdirSync(dir, { recursive: true, mode: 448 });
  import_node_fs3.default.chmodSync(dir, 448);
  const tmp = `${file}.${process.pid}.tmp`;
  import_node_fs3.default.writeFileSync(tmp, data, { mode: 384 });
  import_node_fs3.default.chmodSync(tmp, 384);
  import_node_fs3.default.renameSync(tmp, file);
  import_node_fs3.default.chmodSync(file, 384);
}
function tryWritePrivate(file, data) {
  try {
    writePrivate(file, data);
    return true;
  } catch {
    return false;
  }
}

// cli/redact-output.ts
function wantsRedactedStdout(args) {
  if (process.env.KUBE_SHELL_REDACT_STDOUT !== "1") return false;
  return sensitiveStdoutKind(args) !== null;
}
function redactJsonMap(block) {
  const out = {};
  for (const key of Object.keys(block)) out[key] = "***";
  return out;
}
function redactJsonValue(value) {
  if (Array.isArray(value)) return value.map(redactJsonValue);
  if (value && typeof value === "object") {
    const record = value;
    const out = {};
    let changed = false;
    for (const [key, child] of Object.entries(record)) {
      if (key === "data" || key === "stringData" || key === "binaryData") {
        if (child && typeof child === "object" && !Array.isArray(child)) {
          out[key] = redactJsonMap(child);
          changed = true;
          continue;
        }
      }
      if (key === "token" || key === "client-certificate-data" || key === "client-key-data" || key === "password") {
        out[key] = "***";
        changed = true;
        continue;
      }
      const next = redactJsonValue(child);
      out[key] = next;
      if (next !== child) changed = true;
    }
    return changed ? out : value;
  }
  return value;
}
function redactJson(stdout) {
  try {
    const parsed = JSON.parse(stdout);
    const redacted = redactJsonValue(parsed);
    if (redacted === parsed) return stdout;
    return `${JSON.stringify(redacted, null, 2)}
`;
  } catch {
    return stdout;
  }
}
var SENSITIVE_YAML_KEYS = /^(data|stringData|token|client-certificate-data|client-key-data|password|client-secret)$/;
function redactYaml(stdout) {
  const lines = stdout.split("\n");
  const out = [];
  let inData = false;
  let dataIndent = 0;
  for (const line of lines) {
    const trimmed = line.trimStart();
    const indent = line.length - trimmed.length;
    const keyMatch = /^([a-zA-Z0-9_.-]+):\s*(.*)$/.exec(trimmed);
    if (keyMatch && SENSITIVE_YAML_KEYS.test(keyMatch[1]) && keyMatch[2] === "") {
      inData = keyMatch[1] === "data" || keyMatch[1] === "stringData";
      dataIndent = indent;
      out.push(line);
      continue;
    }
    if (inData && trimmed && indent <= dataIndent && !/^(data|stringData):/.test(trimmed)) {
      inData = false;
    }
    if (keyMatch && SENSITIVE_YAML_KEYS.test(keyMatch[1]) && keyMatch[2] !== "") {
      out.push(`${line.slice(0, line.indexOf(":") + 1)} "***"`);
      continue;
    }
    if (inData && /:\s*$/.test(trimmed) === false && /:\s*.+/.test(trimmed)) {
      const colon = line.indexOf(":");
      if (colon >= 0) {
        out.push(`${line.slice(0, colon + 1)} "***"`);
        continue;
      }
    }
    out.push(line);
  }
  return out.join("\n");
}
function redactKubectlStdout(args, stdout) {
  if (!sensitiveStdoutKind(args)) return stdout;
  const trimmed = stdout.trim();
  if (trimmed.startsWith("{")) return redactJson(stdout);
  if (trimmed.includes("apiVersion:") || trimmed.includes("data:") || trimmed.includes("clusters:"))
    return redactYaml(stdout);
  return stdout;
}

// cli/live.ts
var TTL_MS = 12e3;
function cachePath() {
  return import_node_path4.default.join(stateDir(), "live.json");
}
var KIND = {
  Pod: "pod",
  Deployment: "deployment",
  Service: "service",
  ConfigMap: "configmap",
  Secret: "secret",
  Ingress: "ingress",
  PersistentVolumeClaim: "pvc",
  ServiceAccount: "serviceaccount",
  ReplicaSet: "replicaset",
  StatefulSet: "statefulset",
  DaemonSet: "daemonset",
  Job: "job",
  CronJob: "cronjob",
  HorizontalPodAutoscaler: "hpa",
  Endpoints: "endpoints",
  PodDisruptionBudget: "poddisruptionbudget",
  NetworkPolicy: "networkpolicy",
  ResourceQuota: "resourcequota",
  LimitRange: "limitrange",
  StorageClass: "storageclass",
  PriorityClass: "priorityclass",
  CustomResourceDefinition: "customresourcedefinition"
};
var memory = null;
var contextMemory = null;
function invocation() {
  const cmd = process.env.KUBE_SHELL_KUBECTL || "kubectl";
  const script = process.env.KUBE_SHELL_NODE_SCRIPT;
  return { cmd, prefix: script ? [script] : [] };
}
function run(args, timeout = 8e3) {
  const { cmd, prefix } = invocation();
  try {
    const stdout = (0, import_node_child_process.execFileSync)(cmd, [...prefix, ...args], {
      encoding: "utf8",
      timeout,
      maxBuffer: 16 * 1024 * 1024,
      stdio: ["ignore", "pipe", "pipe"]
    });
    return { code: 0, stdout };
  } catch (err) {
    const failed = err;
    const stdout = typeof failed.stdout === "string" ? failed.stdout : "";
    return { code: typeof failed.status === "number" ? failed.status : 1, stdout };
  }
}
function probeLive() {
  if (memory && Date.now() - memory.at < TTL_MS) return true;
  const result = run(["get", "ns", "--request-timeout=4s", "-o", "name"], 5e3);
  return result.code === 0 && result.stdout.trim().length > 0;
}
function readCache(key) {
  const now = Date.now();
  if (memory && memory.key === key && now - memory.at < TTL_MS) return memory.cluster;
  try {
    const parsed = JSON.parse(import_node_fs4.default.readFileSync(cachePath(), "utf8"));
    if (parsed.key === key && now - parsed.at < TTL_MS && parsed.cluster?.objects) {
      memory = parsed;
      return parsed.cluster;
    }
  } catch {
  }
  return null;
}
function writeCache(key, cluster) {
  const row = { at: Date.now(), key, cluster };
  memory = row;
  tryWritePrivate(cachePath(), JSON.stringify(row));
}
function invalidateLive() {
  memory = null;
  contextMemory = null;
  try {
    import_node_fs4.default.rmSync(cachePath(), { force: true });
  } catch {
  }
}
function contextCluster() {
  if (contextMemory && Date.now() - contextMemory.at < 6e4) return contextMemory.cluster;
  const view = run(["config", "view", "-o", "json"], 4e3);
  if (view.code !== 0 || !view.stdout.trim()) return null;
  let parsed;
  try {
    parsed = JSON.parse(view.stdout);
  } catch {
    return null;
  }
  const current = parsed["current-context"]?.trim() ?? "";
  if (!current) return null;
  const names = run(["config", "get-contexts", "-o", "name"], 4e3).stdout.split("\n").map((line) => line.trim()).filter(Boolean);
  if (!names.includes(current)) names.unshift(current);
  const namespaceOf2 = (name) => parsed.contexts?.find((item) => item.name === name)?.context?.namespace || "default";
  const cluster = {
    current,
    contexts: names.map((name) => ({
      name,
      cluster: parsed.contexts?.find((item) => item.name === name)?.context?.cluster || name,
      user: parsed.contexts?.find((item) => item.name === name)?.context?.user || name,
      namespace: namespaceOf2(name)
    })),
    namespaces: [],
    nodes: [],
    pvs: [],
    events: [],
    objects: []
  };
  contextMemory = { at: Date.now(), cluster };
  return cluster;
}
function contextArgs(context, current) {
  return context && context !== current ? ["--context", context] : [];
}
function itemsOf2(args) {
  const result = run([...args, "-o", "json", "--request-timeout=6s"], 8e3);
  if (result.code !== 0 || !result.stdout.trim()) return [];
  try {
    const parsed = JSON.parse(result.stdout);
    return parsed.items ?? [];
  } catch {
    return [];
  }
}
function ageOf(timestamp, now = Date.now()) {
  if (!timestamp) return "";
  const then = Date.parse(timestamp);
  if (!Number.isFinite(then)) return "";
  const seconds = Math.max(0, Math.round((now - then) / 1e3));
  if (seconds < 90) return `${seconds}s`;
  const minutes = Math.round(seconds / 60);
  if (minutes < 90) return `${minutes}m`;
  const hours = Math.round(minutes / 60);
  if (hours < 48) return `${hours}h`;
  return `${Math.round(hours / 24)}d`;
}
function detailOf(item) {
  const waiting = item.status?.containerStatuses?.find(
    (container) => container.state?.waiting?.reason
  )?.state?.waiting?.reason;
  if (waiting) return waiting;
  if (item.kind === "Deployment" || item.kind === "ReplicaSet" || item.kind === "StatefulSet") {
    return `${item.status?.readyReplicas ?? 0}/${item.status?.replicas ?? 0}`;
  }
  if (item.kind === "Service") return item.spec?.type || "Service";
  if (item.kind === "Secret") return item.type || "Secret";
  if (item.kind === "CronJob") return item.spec?.schedule || "CronJob";
  if (item.kind === "Node") return item.status?.nodeInfo?.kubeletVersion || "Ready";
  if (item.kind === "PersistentVolume")
    return `${item.spec?.capacity?.storage ?? ""} ${item.status?.phase ?? ""}`.trim();
  return item.status?.phase || item.kind || "";
}
function mapListItem(item) {
  const kind = KIND[item.kind ?? ""];
  const name = item.metadata?.name;
  if (!kind || !name) return null;
  return {
    kind,
    name,
    ns: item.metadata?.namespace || "default",
    age: ageOf(item.metadata?.creationTimestamp),
    labels: item.metadata?.labels ?? {},
    status: detailOf(item),
    svcType: item.kind === "Service" ? item.spec?.type : void 0,
    containers: containerNames(item)
  };
}
function containerNames(item) {
  const names = item.spec?.containers?.flatMap(
    (container) => container.name ? [container.name] : []
  );
  if (!names || names.length === 0) return void 0;
  return names.map((name) => ({ name }));
}
function mapNode(item) {
  const name = item.metadata?.name;
  if (!name) return null;
  if (item.kind && item.kind !== "Node") return null;
  const ready = item.status?.conditions?.find((condition) => condition.type === "Ready")?.status;
  const status = ready === "True" ? "Ready" : ready === "False" ? "NotReady" : "Unknown";
  return {
    name,
    status,
    roles: "",
    age: ageOf(item.metadata?.creationTimestamp),
    version: item.status?.nodeInfo?.kubeletVersion || "",
    ip: "",
    schedulable: item.spec?.unschedulable !== true
  };
}
function mapEvent(item, fallbackNs) {
  if (item.kind !== "Event") return null;
  const involved = item.involvedObject;
  const object = involved?.name ? `${(involved.kind || "object").toLowerCase()}/${involved.name}` : "";
  return {
    ns: item.metadata?.namespace || involved?.namespace || fallbackNs,
    last: ageOf(item.metadata?.creationTimestamp),
    type: item.type || "",
    reason: item.reason || "",
    object,
    message: item.message || ""
  };
}
var OBJECT_GROUPS = [
  ["pods", "services", "configmaps", "serviceaccounts", "persistentvolumeclaims"],
  ["secrets"],
  ["deployments.apps", "replicasets.apps", "statefulsets.apps", "daemonsets.apps"],
  ["jobs.batch", "cronjobs.batch"],
  ["ingresses.networking.k8s.io", "networkpolicies.networking.k8s.io"],
  ["horizontalpodautoscalers.autoscaling"],
  ["endpoints", "poddisruptionbudgets.policy", "resourcequotas", "limitranges"]
];
var PROJECTED_JSONPATH = '{range .items[*]}{.kind}{"\\t"}{.metadata.name}{"\\t"}{.metadata.namespace}{"\\t"}{.metadata.creationTimestamp}{"\\t"}{.type}{"\\t"}{.status.phase}{"\\t"}{.status.readyReplicas}{"\\t"}{.status.replicas}{"\\t"}{.spec.type}{"\\t"}{.spec.schedule}{"\\t"}{.spec.capacity.storage}{"\\t"}{.status.nodeInfo.kubeletVersion}{"\\t"}{.reason}{"\\t"}{.message}{"\\t"}{.involvedObject.kind}{"\\t"}{.involvedObject.name}{"\\t"}{.involvedObject.namespace}{"\\t"}{range .status.containerStatuses[*]}{.state.waiting.reason}{","}{end}{"\\t"}{range .spec.containers[*]}{.name}{","}{end}{range .spec.template.spec.containers[*]}{.name}{","}{end}{"\\t"}{range $k,$v := .metadata.labels}{$k}={$v}{","}{end}{"\\n"}{end}';
function parseProjected(stdout) {
  if (!stdout.trim()) return [];
  const items = [];
  for (const line of stdout.split("\n")) {
    if (!line.trim()) continue;
    const cols = line.split("	");
    if (cols.length !== 20) continue;
    const [
      kind,
      name,
      namespace,
      creationTimestamp,
      type,
      phase,
      readyReplicas,
      replicas,
      specType,
      schedule,
      storage,
      kubeletVersion,
      reason,
      message,
      involvedKind,
      involvedName,
      involvedNs,
      waitingRaw,
      containersRaw,
      labelsRaw
    ] = cols;
    const labels = {};
    if (labelsRaw) {
      for (const pair of labelsRaw.split(",")) {
        if (!pair) continue;
        const eq = pair.indexOf("=");
        if (eq >= 0) {
          labels[pair.slice(0, eq)] = pair.slice(eq + 1);
        }
      }
    }
    const containerStatuses = waitingRaw ? waitingRaw.split(",").filter(Boolean).map((r) => ({ state: { waiting: { reason: r } } })) : void 0;
    const item = {
      kind: kind || void 0,
      metadata: {
        name: name || void 0,
        namespace: namespace || void 0,
        creationTimestamp: creationTimestamp || void 0,
        labels
      },
      type: type || void 0,
      reason: reason || void 0,
      message: message || void 0,
      involvedObject: {
        kind: involvedKind || void 0,
        name: involvedName || void 0,
        namespace: involvedNs || void 0
      },
      spec: {
        type: specType || void 0,
        schedule: schedule || void 0,
        capacity: storage ? { storage } : void 0,
        containers: containersRaw ? containersRaw.split(",").filter(Boolean).map((name2) => ({ name: name2 })) : void 0
      },
      status: {
        phase: phase || void 0,
        readyReplicas: readyReplicas ? Number(readyReplicas) : void 0,
        replicas: replicas ? Number(replicas) : void 0,
        nodeInfo: kubeletVersion ? { kubeletVersion } : void 0,
        containerStatuses
      }
    };
    items.push(item);
  }
  return items;
}
function fetchProjectedLists(ctx, scope, groups) {
  const combinedName = groups.flat().join(",");
  const combined = run(
    [...ctx, "get", combinedName, ...scope, "-o", PROJECTED_JSONPATH, "--request-timeout=6s"],
    8e3
  );
  if (combined.code === 0) return [parseProjected(combined.stdout)];
  const lists = [];
  for (const group of groups) {
    const res = run(
      [...ctx, "get", group.join(","), ...scope, "-o", PROJECTED_JSONPATH, "--request-timeout=6s"],
      8e3
    );
    if (res.code === 0) {
      lists.push(parseProjected(res.stdout));
      continue;
    }
    for (const kind of group) {
      const single = run(
        [...ctx, "get", kind, ...scope, "-o", PROJECTED_JSONPATH, "--request-timeout=6s"],
        8e3
      );
      if (single.code === 0) lists.push(parseProjected(single.stdout));
    }
  }
  return lists;
}
function fetchEvents(ctx, ns, all) {
  const scope = all ? ["--all-namespaces"] : ["-n", ns || "default"];
  const items = itemsOf2([...ctx, "get", "events", ...scope]);
  return items.flatMap((item) => {
    const event = mapEvent(item, ns || "default");
    return event ? [event] : [];
  });
}
function fetchObjects(ctx, ns, all) {
  const scope = all ? ["--all-namespaces"] : ["-n", ns || "default"];
  const lists = fetchProjectedLists(ctx, scope, OBJECT_GROUPS);
  const objects = [];
  for (const items of lists) {
    for (const item of items) {
      const mapped = mapListItem(item);
      if (!mapped) continue;
      if (!mapped.ns) mapped.ns = ns || "default";
      objects.push(mapped);
    }
  }
  return { objects, events: fetchEvents(ctx, ns, all) };
}
function liveCluster(engineLine) {
  const base = contextCluster();
  if (!base) return null;
  const tokens = tokenize(engineLine).map((token) => unquote(token.text));
  const scope = scopeOf(tokens.length ? tokens : ["kubectl"], base);
  const key = `${base.current}|${scope.context}|${scope.all ? "*" : scope.ns}`;
  const cached = readCache(key);
  if (cached) return cached;
  const ctx = contextArgs(scope.context, base.current);
  const scoped = itemsOf2([
    ...ctx,
    "get",
    "ns,nodes,pv,storageclasses.storage.k8s.io,priorityclasses.scheduling.k8s.io,customresourcedefinitions.apiextensions.k8s.io"
  ]);
  const namespaces = scoped.filter((item) => item.kind === "Namespace" && item.metadata?.name).map((item) => ({
    name: item.metadata?.name ?? "",
    status: item.status?.phase || "Active",
    age: ageOf(item.metadata?.creationTimestamp)
  }));
  const nodes = scoped.flatMap(
    (item) => item.kind === "Node" ? [mapNode(item)].filter((node) => node !== null) : []
  );
  const pvs = scoped.flatMap((item) => {
    if (item.kind !== "PersistentVolume" || !item.metadata?.name) return [];
    return [
      {
        name: item.metadata.name,
        capacity: item.spec?.capacity?.storage || "",
        mode: "",
        status: item.status?.phase || "",
        claim: "",
        age: ageOf(item.metadata.creationTimestamp)
      }
    ];
  });
  const fetched = fetchObjects(ctx, scope.ns, scope.all);
  const clusterObjects = scoped.flatMap((item) => {
    const mapped = mapListItem(item);
    return mapped ? [mapped] : [];
  });
  const cluster = {
    ...base,
    namespaces: namespaces.length ? namespaces : base.namespaces,
    nodes,
    pvs,
    events: fetched.events,
    objects: [...clusterObjects, ...fetched.objects]
  };
  writeCache(key, cluster);
  return cluster;
}
function applyLiveContext(cluster) {
  const base = contextCluster();
  if (!base) return cluster;
  return { ...cluster, current: base.current, contexts: base.contexts };
}
function kubectlArgs(engineLine) {
  return tokenize(engineLine).slice(1).map((token) => unquote(token.text));
}
function stderrLine(stderr) {
  const line = stderr.split(/\r?\n/).map((item) => item.trim()).find(Boolean);
  return (line ?? "").slice(0, 120);
}
function runJson(args, timeoutMs = 3e3) {
  const { cmd, prefix } = invocation();
  const text = (value) => typeof value === "string" ? value : value ? value.toString("utf8") : "";
  try {
    const stdout = (0, import_node_child_process.execFileSync)(cmd, [...prefix, ...args], {
      encoding: "utf8",
      timeout: timeoutMs,
      maxBuffer: 16 * 1024 * 1024,
      stdio: ["ignore", "pipe", "pipe"]
    });
    try {
      return { ok: true, json: JSON.parse(stdout), error: "" };
    } catch {
      return { ok: false, json: null, error: "invalid JSON" };
    }
  } catch (err) {
    const failed = err;
    const line = stderrLine(text(failed.stderr));
    const error = line || (failed.code === "ETIMEDOUT" ? "timed out" : "failed");
    return { ok: false, json: null, error };
  }
}
function captureLive(args) {
  const { cmd, prefix } = invocation();
  try {
    const stdout = (0, import_node_child_process.execFileSync)(cmd, [...prefix, ...args], {
      encoding: "utf8",
      timeout: 8e3,
      maxBuffer: 4 * 1024 * 1024,
      stdio: ["ignore", "pipe", "pipe"]
    });
    return { code: 0, stdout, stderr: "" };
  } catch (err) {
    const failed = err;
    const text = (value) => typeof value === "string" ? value : value ? value.toString("utf8") : "";
    return {
      code: typeof failed.status === "number" ? failed.status : 1,
      stdout: text(failed.stdout),
      stderr: text(failed.stderr)
    };
  }
}
function runLive(args, options = {}) {
  if (wantsRedactedStdout(args) && !process.stdout.isTTY) {
    const { code, stdout, stderr } = captureLive(args);
    if (stderr) process.stderr.write(stderr);
    process.stdout.write(redactKubectlStdout(args, stdout));
    invalidateLive();
    return code;
  }
  const { cmd, prefix } = invocation();
  const stdin = process.stdin.isTTY || options.inheritStdin ? "inherit" : "ignore";
  const result = (0, import_node_child_process.spawnSync)(cmd, [...prefix, ...args], { stdio: [stdin, "inherit", "inherit"] });
  invalidateLive();
  if (result.error) {
    process.stderr.write(`kubectl: ${result.error.message}
`);
    return 127;
  }
  return result.status ?? 1;
}

// cli/tty.ts
var import_node_fs5 = __toESM(require("node:fs"), 1);
var RETRY = /* @__PURE__ */ new Set(["EAGAIN", "EWOULDBLOCK", "EINTR"]);
function openPromptInput(platform = process.platform, open = (file) => import_node_fs5.default.openSync(file, "r")) {
  const stdin = { fd: 0, close: () => {
  } };
  if (platform === "win32") return stdin;
  let fd;
  try {
    fd = open("/dev/tty");
  } catch {
    return stdin;
  }
  return { fd, close: () => import_node_fs5.default.closeSync(fd) };
}
function readSyncRetry(fd, buf, read = (f, b) => import_node_fs5.default.readSync(f, b, 0, b.length, null), sleep = sleepMs) {
  for (; ; ) {
    try {
      return read(fd, buf);
    } catch (error) {
      if (!RETRY.has(error.code ?? "")) throw error;
      sleep(10);
    }
  }
}

// cli/view-state.ts
function viewState(partial) {
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
    ...partial
  };
}
function listMode(state, key, rows) {
  const cursor = Math.min(state.cursor, Math.max(0, rows.length - 1));
  const base = { ...state, cursor };
  if (key === "j" || key === "down")
    return {
      state: { ...base, cursor: Math.min(rows.length - 1, cursor + 1) },
      effect: { type: "none" }
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
    const boards = ["pod", "deployment", "service", "node", "namespace"];
    return {
      state: { ...base, board: boards[Number(key) - 1], cursor: 0 },
      effect: { type: "none" }
    };
  }
  const row = rows[cursor];
  if (key === "\r") {
    if (!row) return { state: base, effect: { type: "none" } };
    if (row.kind === "namespace") {
      return {
        state: { ...base, ns: row.name, all: false, board: "pod", cursor: 0 },
        effect: { type: "reload" }
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
        effect: { type: "none" }
      };
    }
    if (state.yes) return { state: base, effect: { type: "delete", index: cursor } };
    return {
      state: { ...base, pendingIndex: cursor, mode: "confirm" },
      effect: { type: "arm-delete", index: cursor }
    };
  }
  return { state: base, effect: { type: "none" } };
}
function reduce(state, key, ctx) {
  if (state.commanding) {
    if (key === "\r" || key === "\n") {
      const word = state.command.trim().toLowerCase();
      const next = { ...state, commanding: false, command: "", mode: "list" };
      if (word === "q" || word === "quit") return { state: next, effect: { type: "quit" } };
      if (word === "all") return { state: { ...next, all: true }, effect: { type: "reload" } };
      if (word) {
        const boards = {
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
          namespaces: "namespace"
        };
        return {
          state: { ...next, board: boards[word] ?? "pod", cursor: 0 },
          effect: { type: "none" }
        };
      }
      return { state: next, effect: { type: "none" } };
    }
    if (key === "esc")
      return { state: { ...state, commanding: false, command: "" }, effect: { type: "none" } };
    if (key === "\x7F" || key === "del")
      return { state: { ...state, command: state.command.slice(0, -1) }, effect: { type: "none" } };
    if (key.length === 1 && key >= " ")
      return { state: { ...state, command: state.command + key }, effect: { type: "none" } };
    return { state, effect: { type: "none" } };
  }
  if (state.filtering) {
    if (key === "\r" || key === "\n" || key === "esc")
      return { state: { ...state, filtering: false }, effect: { type: "none" } };
    if (key === "\x7F" || key === "del")
      return {
        state: { ...state, filter: state.filter.slice(0, -1), cursor: 0 },
        effect: { type: "none" }
      };
    if (key.length === 1 && key >= " ")
      return {
        state: { ...state, filter: state.filter + key, cursor: 0 },
        effect: { type: "none" }
      };
    return { state, effect: { type: "none" } };
  }
  if (state.mode === "confirm") {
    if ((key === "y" || key === "Y") && state.pendingIndex !== null) {
      return {
        state: { ...state, mode: "list", pendingIndex: null },
        effect: { type: "delete", index: state.pendingIndex }
      };
    }
    return {
      state: { ...state, mode: "list", pendingIndex: null, message: "not deleted" },
      effect: { type: "none" }
    };
  }
  if (key === "q" || key === "") {
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
        effect: { type: "none" }
      };
    if (key === "k" || key === "up")
      return {
        state: { ...state, scroll: Math.max(0, state.scroll - 1) },
        effect: { type: "none" }
      };
    if (key === "pgdn" || key === " ")
      return {
        state: { ...state, scroll: Math.min(last, state.scroll + 10) },
        effect: { type: "none" }
      };
    if (key === "pgup")
      return {
        state: { ...state, scroll: Math.max(0, state.scroll - 10) },
        effect: { type: "none" }
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
        effect: { type: "none" }
      };
    if (key === "k" || key === "up")
      return { state: { ...state, nsCursor: Math.max(0, nsCursor - 1) }, effect: { type: "none" } };
    if (key === "\r") {
      const picked = names[nsCursor];
      if (!picked) return { state: { ...state, mode: "list" }, effect: { type: "none" } };
      return {
        state: { ...state, ns: picked, all: false, board: "pod", cursor: 0, mode: "list" },
        effect: { type: "reload" }
      };
    }
    return { state, effect: { type: "none" } };
  }
  if (state.mode === "help") return { state, effect: { type: "none" } };
  return listMode(state, key, ctx.rows);
}

// cli/view.ts
var BOARDS = [
  { id: "pod", label: "pods" },
  { id: "deployment", label: "deploy" },
  { id: "service", label: "svc" },
  { id: "node", label: "nodes" },
  { id: "namespace", label: "ns" }
];
function deleteEngineLine(row) {
  const parts = ["kubectl", "delete", row.kind, row.name];
  if (row.ns) parts.push("-n", row.ns);
  return parts.join(" ");
}
function runDelete(plan) {
  const result = captureLive(kubectlArgs(plan.command));
  return (result.stderr || result.stdout || "deleted").trim().split("\n")[0] ?? "deleted";
}
function boardFrom(word) {
  const token = (word ?? "pods").toLowerCase();
  if (["po", "pod", "pods"].includes(token)) return "pod";
  if (["deploy", "deployment", "deployments"].includes(token)) return "deployment";
  if (["svc", "service", "services"].includes(token)) return "service";
  if (["no", "node", "nodes"].includes(token)) return "node";
  if (["ns", "namespace", "namespaces"].includes(token)) return "namespace";
  return "pod";
}
function toneOf(status) {
  if (/CrashLoop|Error|ImagePull|BackOff|Failed|Evicted|NotReady/i.test(status)) return "bad";
  if (/Pending|Unknown/i.test(status)) return "warn";
  if (/Running|Active|Ready|Bound|Succeeded/i.test(status)) return "ok";
  return "plain";
}
function objectRow(item) {
  const info = item.kind === "service" ? item.svcType || "" : "";
  return {
    name: item.name,
    ns: item.ns,
    info,
    status: item.status || item.kind,
    age: item.age,
    kind: item.kind
  };
}
function byProblem(a, b) {
  const rank2 = (row) => toneOf(row.status) === "bad" ? 0 : toneOf(row.status) === "warn" ? 1 : 2;
  return rank2(a) - rank2(b) || a.ns.localeCompare(b.ns) || a.name.localeCompare(b.name);
}
function rowsOf(cluster, board, ns, all, filter) {
  const needle = filter.trim().toLowerCase();
  const keep = (row) => !needle || `${row.name} ${row.ns} ${row.status}`.toLowerCase().includes(needle);
  if (board === "node") {
    return cluster.nodes.map((node) => ({
      name: node.name,
      ns: "",
      info: node.roles || node.version,
      status: node.status,
      age: node.age,
      kind: "node"
    })).filter(keep).sort(byProblem);
  }
  if (board === "namespace") {
    return cluster.namespaces.map((item) => ({
      name: item.name,
      ns: "",
      info: "",
      status: item.status,
      age: item.age,
      kind: "namespace"
    })).filter(keep).sort(byProblem);
  }
  return cluster.objects.filter((item) => item.kind === board && (all || item.ns === ns)).map(objectRow).filter(keep).sort(byProblem);
}
function tableText(cluster, board, all) {
  const ns = namespaceOf(cluster);
  const rows = rowsOf(cluster, board, ns, all, "");
  const lines = [all ? "NAMESPACE  NAME  STATUS" : "NAME  STATUS"];
  for (const row of rows)
    lines.push(`${all && row.ns ? `${row.ns}  ` : ""}${row.name}  ${row.status}`);
  return lines.join("\n");
}
function fit(text, width) {
  const clean = text.replace(/\s+/g, " ");
  if (clean.length >= width) return clean.slice(0, Math.max(0, width));
  return clean.padEnd(width);
}
function paint(text, tone) {
  if (tone === "selected") return `\x1B[7m${text}\x1B[0m`;
  if (tone === "head") return `\x1B[1m${text}\x1B[0m`;
  if (tone === "bad") return `\x1B[31m${text}\x1B[0m`;
  if (tone === "warn") return `\x1B[33m${text}\x1B[0m`;
  if (tone === "ok") return `\x1B[32m${text}\x1B[0m`;
  return text;
}
function readKeys(fd) {
  const buf = Buffer.alloc(64);
  const n = readSyncRetry(fd, buf);
  if (n <= 0) return ["q"];
  const raw = buf.subarray(0, n).toString("utf8");
  const keys = [];
  for (let i = 0; i < raw.length; i += 1) {
    if (raw[i] !== "\x1B") {
      keys.push(raw[i] ?? "");
      continue;
    }
    const rest = raw.slice(i);
    if (rest.startsWith("\x1B[A")) {
      keys.push("up");
      i += 2;
    } else if (rest.startsWith("\x1B[B")) {
      keys.push("down");
      i += 2;
    } else if (rest.startsWith("\x1B[5~")) {
      keys.push("pgup");
      i += 3;
    } else if (rest.startsWith("\x1B[6~")) {
      keys.push("pgdn");
      i += 3;
    } else if (rest.startsWith("\x1B[3~")) {
      keys.push("del");
      i += 3;
    } else {
      keys.push("esc");
    }
  }
  return keys.filter(Boolean);
}
function lookupText(verb, row) {
  const args = verb === "logs" ? ["logs", row.name, "--tail=80"] : ["describe", row.kind, row.name];
  if (row.ns) args.push("-n", row.ns);
  const result = captureLive(args);
  return (result.stdout || result.stderr || "(no output)").replace(/\s+$/, "");
}
function openView(clusterIn, options) {
  if (!process.stdin.isTTY || !process.stdout.isTTY) {
    process.stdout.write(`${tableText(clusterIn, options.board, options.all)}
`);
    return clusterIn;
  }
  let cluster = clusterIn;
  let text = [];
  let pendingDelete = null;
  let state = viewState({
    board: options.board,
    all: options.all,
    ns: namespaceOf(cluster),
    yes: options.yes,
    nsCursor: Math.max(
      0,
      cluster.namespaces.findIndex((item) => item.name === namespaceOf(cluster))
    )
  });
  process.stdin.setRawMode(true);
  const input = openPromptInput();
  process.stdout.write("\x1B[?1049h\x1B[?25l");
  const rows = () => rowsOf(cluster, state.board, state.ns, state.all, state.filter);
  const reload = () => {
    const next = options.reload(state.all, state.ns);
    if (next) {
      cluster = next;
      state = { ...state, message: "refreshed" };
      return;
    }
    state = { ...state, message: "refresh failed" };
  };
  const draw = () => {
    const { board, all, ns, filter, filtering, mode, command, commanding } = state;
    let cursor = state.cursor;
    let nsCursor = state.nsCursor;
    const message = state.message;
    const scroll = state.scroll;
    const width = Math.max(40, process.stdout.columns || 80);
    const height = Math.max(12, process.stdout.rows || 24);
    const list = rows();
    if (cursor >= list.length) cursor = Math.max(0, list.length - 1);
    const lines = [];
    const label = BOARDS.find((item) => item.id === board)?.label ?? board;
    lines.push(
      paint(
        fit(
          ` kube  ${cluster.current}  ${all ? "all namespaces" : ns}  ${label}  ${list.length}`,
          width
        ),
        "head"
      )
    );
    if (mode === "help") {
      for (const line of [
        "j/k or arrows   move",
        "enter           describe",
        "l               logs, pods only",
        "d               delete, then y",
        "/               filter by name",
        "1-5             pods, deploy, svc, nodes, ns",
        "a               all namespaces",
        "n               pick a namespace",
        "r               refresh",
        ":pods :deploy   same as the number keys",
        "q               back to the shell"
      ]) {
        lines.push(fit(line, width));
      }
    } else if (mode === "text") {
      for (const line of text.slice(scroll, scroll + height - 3)) lines.push(fit(line, width));
    } else if (mode === "ns") {
      const names = cluster.namespaces.map((item) => item.name);
      if (nsCursor >= names.length) nsCursor = Math.max(0, names.length - 1);
      const room = height - 3;
      const start = Math.max(0, Math.min(nsCursor - 2, Math.max(0, names.length - room)));
      for (let i = start; i < Math.min(names.length, start + room); i += 1) {
        const name = names[i] ?? "";
        lines.push(
          paint(
            fit(`  ${name}${name === ns ? "  current" : ""}`, width),
            i === nsCursor ? "selected" : "plain"
          )
        );
      }
    } else {
      const showNs = all && board !== "node" && board !== "namespace";
      const nameW = Math.max(16, Math.min(36, width - (showNs ? 48 : 32)));
      lines.push(
        paint(
          fit(
            `${showNs ? fit("NAMESPACE", 16) : ""}${fit("NAME", nameW)}  ${fit("INFO", 16)}  STATUS`,
            width
          ),
          "head"
        )
      );
      const room = height - 4;
      const start = Math.max(
        0,
        Math.min(cursor - Math.floor(room / 2), Math.max(0, list.length - room))
      );
      for (let i = start; i < Math.min(list.length, start + room); i += 1) {
        const row = list[i];
        const body = `${showNs ? fit(row.ns, 16) : ""}${fit(row.name, nameW)}  ${fit(row.info, 16)}  ${row.status}  ${row.age}`;
        lines.push(paint(fit(body, width), i === cursor ? "selected" : toneOf(row.status)));
      }
    }
    while (lines.length < height - 1) lines.push(fit("", width));
    const hint = commanding ? `:${command}` : filtering ? `/${filter}` : mode === "confirm" ? message : message || "1 pods  2 deploy  3 svc  4 nodes  5 ns    enter describe   l logs   / filter   d delete   ? help   q quit";
    lines.push(paint(fit(hint, width), "head"));
    state.cursor = cursor;
    state.nsCursor = nsCursor;
    process.stdout.write(`\x1B[H\x1B[2J${lines.slice(0, height).join("\n")}`);
  };
  try {
    draw();
    for (; ; ) {
      for (const key of readKeys(input.fd)) {
        const list = rows();
        const step = reduce(state, key, {
          rows: list,
          namespaces: cluster.namespaces.map((item) => item.name)
        });
        state = step.state;
        const effect = step.effect;
        if (effect.type === "quit") return cluster;
        if (effect.type === "reload") reload();
        if (effect.type === "describe" || effect.type === "logs") {
          const row = list[effect.index];
          if (row) {
            text = lookupText(effect.type === "logs" ? "logs" : "describe", row).split("\n");
            state = { ...state, textLength: text.length, scroll: 0, mode: "text" };
          }
        }
        if (effect.type === "arm-delete") {
          const row = list[effect.index];
          if (row) {
            pendingDelete = planCommand(deleteEngineLine(row), cluster);
            state = {
              ...state,
              message: `${pendingDelete.risk} risk: ${pendingDelete.summary}  [y/N]`
            };
          }
        }
        if (effect.type === "delete") {
          const row = list[effect.index];
          const planned = pendingDelete ?? (row ? planCommand(deleteEngineLine(row), cluster) : null);
          pendingDelete = null;
          if (planned) {
            state = { ...state, message: runDelete(planned) };
            reload();
          }
        }
      }
      draw();
    }
  } finally {
    input.close();
    process.stdout.write("\x1B[?25h\x1B[?1049l");
    if (process.stdin.isTTY) process.stdin.setRawMode(false);
  }
}

// cli/commands.ts
function wantsBlast(plan) {
  return plan.verb === "delete" || plan.verb === "drain" || plan.verb === "scale" && plan.risk === "high";
}
function liveBlast(plan, engineLine) {
  const started = Date.now();
  const now = () => Date.now();
  const getJson = (args) => {
    const left = BLAST_BUDGET_MS - (now() - started);
    if (left <= 0) return { ok: false, json: null, error: "timed out" };
    return runJson(args, left);
  };
  return blastRadius(plan, kubectlArgs(engineLine), getJson, { now });
}
function runUserLine(line, cluster, host2, options = {}) {
  let trimmed = line.trim();
  const meta = metaOf(trimmed);
  if (meta === "exit" || meta === "quit") return { output: "", ok: true, cluster, exit: true };
  if (meta === "reset") {
    invalidateLive();
    const next = liveCluster("kubectl") ?? cluster;
    return { output: `refreshed names from ${next.current}`, ok: true, cluster: next };
  }
  if (meta === "sample") {
    return {
      output: "sample mode was removed. kube only runs against a reachable cluster.",
      ok: false,
      cluster
    };
  }
  if (meta === "live") {
    invalidateLive();
    if (!probeLive())
      return {
        output: "No live cluster. Check kubectl and the current context.",
        ok: false,
        cluster
      };
    const next = liveCluster("kubectl") ?? cluster;
    return { output: `live cluster ${next.current}`, ok: true, cluster: next };
  }
  if (meta === "clear") return { output: "\x1Bc", ok: true, cluster };
  if (meta === "help" || meta === "--help" || meta === "-h")
    return { output: host2.help, ok: true, cluster };
  if (meta === "--version" || meta === "-v") return { output: host2.version, ok: true, cluster };
  if (meta === "completion") {
    const shell = trimmed.split(/\s+/)[1] ?? "";
    const script = completionScript(shell);
    if (!script) return { output: "usage: kube completion bash|zsh|fish", ok: false, cluster };
    return { output: script, ok: true, cluster };
  }
  if (meta === "view" || meta === "ui") {
    const parts = trimmed.split(/\s+/).slice(1);
    const all = parts.includes("-A") || parts.includes("--all-namespaces") || parts.includes("all");
    const word = parts.find((part) => part !== "all" && !part.startsWith("-"));
    const next = openView(cluster, {
      board: boardFrom(word),
      all,
      yes: host2.yes,
      reload: (wantAll, namespace) => {
        invalidateLive();
        return liveCluster(wantAll ? "kubectl get pods -A" : `kubectl get pods -n ${namespace}`);
      }
    });
    return { output: "", ok: true, cluster: next };
  }
  if (meta === "plan") {
    const asJson = /(^|\s)--json(\s|$)/.test(trimmed);
    const body = trimmed.replace(/^plan\b\s*/, "").replace(/(^|\s)--json(\s|$)/g, " ").trim();
    if (!body) return { output: "usage: kube plan [--json] delete pod <name>", ok: false, cluster };
    const view2 = toEngine(body, body.length, "repl");
    const planned2 = planCommand(view2.line, cluster);
    const blast = wantsBlast(planned2) ? liveBlast(planned2, view2.line) : void 0;
    if (asJson) {
      const payload = blast ? { ...planned2, blast } : planned2;
      return { output: JSON.stringify(payload), ok: true, cluster };
    }
    const text = blast ? formatBlast(blast) : "";
    return {
      output: text ? `${formatPlan(planned2)}
${text}` : formatPlan(planned2),
      ok: true,
      cluster
    };
  }
  if (meta === "ask") {
    const asJson = /(^|\s)--json(\s|$)/.test(trimmed);
    const question = trimmed.replace(/^\?\s*/, "").replace(/^ask\s+/, "").replace(/(^|\s)--json(\s|$)/g, " ").trim().replace(/^["']|["']$/g, "");
    const advice = advise(question, cluster);
    if (asJson) return { output: JSON.stringify(advice), ok: true, cluster };
    if (!process.stdin.isTTY) return { output: formatAdvice(advice), ok: true, cluster };
    process.stderr.write(`${formatAdvice(advice)}
`);
    if (!host2.confirm("Run the first command? [y/N] ")) return { output: "", ok: true, cluster };
    const next = advice.commands[0];
    if (!next) return { output: "", ok: true, cluster };
    trimmed = next;
  }
  const view = toEngine(trimmed, trimmed.length, "repl");
  const planned = planCommand(view.line, cluster);
  if (planned.risk === "high" && !host2.yes) {
    const extra = wantsBlast(planned) ? formatBlast(liveBlast(planned, view.line)) : "";
    process.stderr.write(`${formatPlan(planned)}${extra ? `
${extra}` : ""}
`);
    if (!host2.confirm("Run it? [y/N] ")) return { output: "not run", ok: false, cluster };
  }
  const code = runLive(kubectlArgs(view.line), { inheritStdin: options.inheritStdin });
  return { output: "", ok: code === 0, cluster: applyLiveContext(cluster) };
}

// cli/redact.ts
var PAIR_FLAGS = ["--from-literal", "--from-file"];
var VALUE_FLAGS = ["--docker-password", "--password", "--token", "--client-secret"];
var SENSITIVE_KEY = /^(password|token|secret|apikey|api-key|client-secret|client_secret)$/i;
function hidePair(value) {
  const eq = value.indexOf("=");
  if (eq < 0) return "***";
  return `${value.slice(0, eq)}=***`;
}
function matchFlag(token, flags) {
  return flags.find((flag) => token === flag || token.startsWith(`${flag}=`));
}
function redactAssignment(token) {
  const eq = token.indexOf("=");
  if (eq < 0) return token;
  const key = token.slice(0, eq);
  if (SENSITIVE_KEY.test(key)) return `${key}=***`;
  return token;
}
function redactLine(line) {
  const tokens = line.split(" ");
  const out = [];
  for (let i = 0; i < tokens.length; i += 1) {
    const token = tokens[i] ?? "";
    const pair = matchFlag(token, PAIR_FLAGS);
    if (pair) {
      if (token === pair) {
        out.push(token);
        const next = tokens[i + 1];
        if (next !== void 0) {
          out.push(hidePair(next));
          i += 1;
        }
      } else {
        out.push(`${pair}=${hidePair(token.slice(pair.length + 1))}`);
      }
      continue;
    }
    const valueFlag = matchFlag(token, VALUE_FLAGS);
    if (valueFlag) {
      if (token === valueFlag) {
        out.push(token);
        if (tokens[i + 1] !== void 0) {
          out.push("***");
          i += 1;
        }
      } else {
        out.push(`${valueFlag}=***`);
      }
      continue;
    }
    out.push(redactAssignment(token));
  }
  return out.join(" ");
}

// cli/main.ts
var VERSION = true ? "0.5.1" : "0.0.0-dev";
var HISTORY = import_node_path5.default.join(stateDir(), "history");
var session = { yes: false };
function loadHistory() {
  try {
    return import_node_fs6.default.readFileSync(HISTORY, "utf8").split("\n").map((line) => line.trimEnd()).filter(Boolean).slice(-200).reverse();
  } catch {
    return [];
  }
}
var HISTORY_LIMIT = 200;
function trimHistoryFile() {
  try {
    const lines = import_node_fs6.default.readFileSync(HISTORY, "utf8").split("\n").filter(Boolean);
    if (lines.length <= HISTORY_LIMIT) return;
    writePrivate(HISTORY, `${lines.slice(-HISTORY_LIMIT).join("\n")}
`);
  } catch {
  }
}
function remember2(line) {
  try {
    const trimmed = redactLine(line.trim());
    if (!trimmed) return;
    if (!import_node_fs6.default.existsSync(HISTORY)) writePrivate(HISTORY, "");
    else import_node_fs6.default.chmodSync(HISTORY, 384);
    import_node_fs6.default.appendFileSync(HISTORY, `${trimmed}
`);
    trimHistoryFile();
  } catch {
  }
}
function cliHelp() {
  return `kube ${VERSION}

Tab completes against the current cluster. kube exits if kubectl cannot reach it.

  kube                         shell. Tab fills the next word
  kube get pods                one command, then exit
  kube view                    full-screen pods, logs, and describe
  kube plan delete pod <name>  what it will change, before it runs
  kube ask "why is it crashing"
  kube plan --json get pods    same plan, for an agent
  kube --yes delete pod <name> skip the confirmation

Inside the shell the kubectl prefix is optional. A line starting with ? is ask.
\`view\` and \`ui\` open the resource screen. Delete, drain, and scale-to-zero ask before they run.
\`live\` reconnects. \`reset\` refreshes names from the cluster.
Up and down walk history. Ctrl+C clears the line, Ctrl+D exits.

Install: brew tap NadafMM/tap && brew install kubectl-shell`;
}
function suggest(line, cursor, mode, fallback) {
  const view = toEngine(line, cursor, mode);
  const cluster = view.skip ? fallback : liveCluster(view.line) ?? fallback;
  if (view.skip)
    return { view, candidates: [], start: cursor, end: cursor, cluster };
  const found = complete(view.line, view.cursor, cluster);
  return {
    view,
    candidates: found.candidates,
    start: Math.max(0, found.start - view.delta),
    end: Math.max(0, found.end - view.delta),
    cluster
  };
}
function confirm(prompt) {
  if (session.yes) return true;
  process.stderr.write(prompt);
  if (!process.stdin.isTTY) return false;
  const input = openPromptInput();
  try {
    return readConfirm(input.fd, readSyncRetry, (text) => process.stderr.write(text));
  } finally {
    input.close();
  }
}
function host() {
  return { yes: session.yes, help: cliHelp(), version: VERSION, confirm };
}
function writeResult(result) {
  if (!result.output) return;
  const text = result.output.endsWith("\n") ? result.output : `${result.output}
`;
  if (result.ok) {
    process.stdout.write(text);
  } else {
    process.stderr.write(text);
  }
}
function visible(candidates) {
  return candidates.flatMap((item) => {
    const value = completionValue(item.value);
    if (value === null) return [];
    return [{ ...item, value, detail: plainText(item.detail) }];
  });
}
function printComplete(line, cursor, cluster, asJson) {
  const candidates = visible(suggest(line, cursor, "shell", cluster).candidates);
  if (asJson) {
    process.stdout.write(
      `${JSON.stringify(candidates.map(({ value, detail, kind }) => ({ value, detail, kind })))}
`
    );
    return;
  }
  process.stdout.write(candidates.map((item) => item.value).join("\n"));
  if (candidates.length) process.stdout.write("\n");
}
function fillTab(rl, getCluster, setCluster) {
  const found = suggest(rl.line, rl.cursor, "repl", getCluster());
  setCluster(found.cluster);
  const candidates = visible(found.candidates);
  const { start, end } = found;
  if (candidates.length === 0) {
    rl.output.write("\x07");
    return;
  }
  const top = candidates[0];
  const next = spliceToken(rl.line, start, end, top.value, true);
  if (next.line === rl.line && next.cursor === rl.cursor) {
    if (candidates.length > 1) {
      const width = Math.max(...candidates.map((item) => item.value.length));
      const body = candidates.map((item) => `  ${item.value.padEnd(width)}  ${item.detail}`).join("\r\n");
      rl.output.write(`\r
${body}\r
`);
    } else {
      rl.output.write("\x07");
    }
    return;
  }
  rl.line = next.line;
  rl.cursor = next.cursor;
}
function startRepl(clusterIn) {
  process.on("SIGINT", () => {
  });
  let cluster = clusterIn;
  const rl = import_node_readline.default.createInterface({
    input: process.stdin,
    output: process.stdout,
    terminal: true,
    history: loadHistory(),
    historySize: 200
  });
  rl.input.on(
    "keypress",
    (_str, key) => {
      if (!key || key.name !== "tab" || key.ctrl || key.meta) return;
      if (rl.cursor > 0 && rl.line[rl.cursor - 1] === "	") {
        rl.line = rl.line.slice(0, rl.cursor - 1) + rl.line.slice(rl.cursor);
        rl.cursor -= 1;
      }
      fillTab(
        rl,
        () => cluster,
        (next) => {
          cluster = next;
        }
      );
      refreshLine(rl);
    }
  );
  const prompt = () => {
    rl.setPrompt(`${promptOf(cluster)}$ `);
  };
  process.stdout.write(
    `kube  live cluster ${promptOf(cluster)}. Tab completes real names. \`help\` lists commands.
`
  );
  prompt();
  rl.prompt();
  rl.on("line", (input) => {
    remember2(input);
    rl.pause();
    if (process.stdin.isTTY) process.stdin.setRawMode(false);
    const result = runUserLine(input, cluster, host());
    if (process.stdin.isTTY) process.stdin.setRawMode(true);
    rl.resume();
    cluster = result.cluster;
    if (result.output === "\x1Bc") {
      import_node_readline.default.cursorTo(process.stdout, 0, 0);
      import_node_readline.default.clearScreenDown(process.stdout);
    } else {
      writeResult(result);
    }
    if (result.exit) {
      rl.close();
      return;
    }
    prompt();
    rl.prompt();
  });
  rl.on("close", () => {
    process.stdout.write("\n");
    process.exit(0);
  });
  rl.on("SIGINT", () => {
    if (rl.line) {
      rl.line = "";
      rl.cursor = 0;
      process.stdout.write("\n");
      prompt();
      rl.prompt();
      return;
    }
    process.stdout.write("\n");
    process.exit(0);
  });
}
function boot() {
  if (!probeLive()) {
    process.stderr.write(
      "kubectl could not reach a cluster. kube only runs against a reachable cluster.\n"
    );
    process.exit(1);
  }
  const cluster = liveCluster("kubectl");
  if (!cluster) {
    process.stderr.write(
      "kubectl could not reach a cluster. kube only runs against a reachable cluster.\n"
    );
    process.exit(1);
  }
  return cluster;
}
function takeFlags(argv) {
  let yes = false;
  const rest = [];
  for (const arg of argv) {
    if (rest.length === 0 && arg === "--sample") {
      process.stderr.write(
        "sample mode was removed. kube only runs against a reachable cluster.\n"
      );
      process.exit(1);
    }
    if (rest.length === 0 && arg === "--live") continue;
    if (rest.length === 0 && (arg === "--yes" || arg === "-y")) {
      yes = true;
      continue;
    }
    rest.push(arg);
  }
  return { yes, rest };
}
function blankCluster() {
  return {
    current: "",
    contexts: [],
    namespaces: [],
    nodes: [],
    pvs: [],
    events: [],
    objects: []
  };
}
function main() {
  const argv = process.argv.slice(2);
  if (argv[0] === "__complete") {
    const args = argv.slice(1);
    let cursor;
    let line2 = "";
    let asJson = false;
    for (let i = 0; i < args.length; i += 1) {
      const arg = args[i];
      if (arg === "--cursor") {
        cursor = Number(args[i + 1]);
        i += 1;
      } else if (arg === "--json") asJson = true;
      else if (arg === "--line") {
        line2 = args[i + 1] ?? "";
        break;
      }
    }
    const cluster2 = boot();
    printComplete(line2, Number.isFinite(cursor) ? cursor : line2.length, cluster2, asJson);
    return;
  }
  const { yes, rest } = takeFlags(argv);
  if (yes && !allowYes(yes, rest.length, process.stdin.isTTY === true)) {
    process.stderr.write("`--yes` applies to one command, not the shell.\n");
    process.exit(1);
  }
  session.yes = yes;
  const quoted = [];
  for (const arg of rest) {
    const next = quoteArg(arg);
    if (next === null) {
      process.stderr.write("cannot pass an argument that contains both quote styles\n");
      process.exit(1);
    }
    quoted.push(next);
  }
  const line = quoted.join(" ");
  const meta = metaOf(line);
  if (meta === "sample") {
    process.stderr.write("sample mode was removed. kube only runs against a reachable cluster.\n");
    process.exit(1);
  }
  if (meta === "help" || meta === "--help" || meta === "-h" || meta === "--version" || meta === "-v" || meta === "completion") {
    const result2 = runUserLine(line, blankCluster(), host());
    writeResult(result2);
    process.exit(result2.ok ? 0 : 1);
  }
  const cluster = boot();
  if (rest.length === 0) {
    if (!process.stdin.isTTY) {
      let current = cluster;
      const text = import_node_fs6.default.readFileSync(0, "utf8");
      let code = 0;
      for (const line2 of text.split("\n")) {
        if (!line2.trim()) continue;
        const result2 = runUserLine(line2, current, host());
        current = result2.cluster;
        if (result2.output !== "\x1Bc") writeResult(result2);
        if (!result2.ok) code = 1;
        if (result2.exit) break;
      }
      process.exit(code);
    }
    startRepl(cluster);
    return;
  }
  const inheritStdin = !process.stdin.isTTY;
  const result = runUserLine(line, cluster, host(), { inheritStdin });
  writeResult(result);
  process.exit(result.ok ? 0 : 1);
}

// cli/bin.ts
main();
