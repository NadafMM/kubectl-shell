export type ObjectKind =
  | "pod"
  | "deployment"
  | "service"
  | "replicaset"
  | "statefulset"
  | "daemonset"
  | "job"
  | "cronjob"
  | "configmap"
  | "secret"
  | "ingress"
  | "pvc"
  | "serviceaccount"
  | "hpa"
  | "endpoints"
  | "poddisruptionbudget"
  | "networkpolicy"
  | "resourcequota"
  | "limitrange"
  | "storageclass"
  | "priorityclass"
  | "customresourcedefinition";

export type KObject = {
  kind: ObjectKind;
  name: string;
  ns: string;
  age: string;
  labels: Record<string, string>;
  status?: string;
  svcType?: string;
  containers?: { name: string }[];
};

export type Cluster = {
  current: string;
  contexts: { name: string; cluster: string; user: string; namespace: string }[];
  namespaces: { name: string; status: string; age: string }[];
  nodes: {
    name: string;
    status: string;
    roles: string;
    age: string;
    version: string;
    ip: string;
    schedulable: boolean;
  }[];
  pvs: {
    name: string;
    capacity: string;
    mode: string;
    status: string;
    claim: string;
    age: string;
  }[];
  events: {
    ns: string;
    last: string;
    type: string;
    reason: string;
    object: string;
    message: string;
  }[];
  objects: KObject[];
};

export type Resource = {
  name: string;
  short: string[];
  singular: string;
  kind: ObjectKind | "namespace" | "node" | "pv" | "event";
  detail: string;
  weight: number;
  namespaced: boolean;
  api: string;
  apiKind: string;
};

export const RESOURCES: Resource[] = [
  {
    name: "pods",
    short: ["po"],
    singular: "pod",
    kind: "pod",
    detail: "List or manage pods",
    weight: 100,
    namespaced: true,
    api: "v1",
    apiKind: "Pod",
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
    apiKind: "Deployment",
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
    apiKind: "Service",
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
    apiKind: "Namespace",
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
    apiKind: "Node",
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
    apiKind: "ConfigMap",
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
    apiKind: "Secret",
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
    apiKind: "Ingress",
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
    apiKind: "Event",
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
    apiKind: "PersistentVolumeClaim",
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
    apiKind: "PersistentVolume",
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
    apiKind: "ServiceAccount",
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
    apiKind: "ReplicaSet",
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
    apiKind: "StatefulSet",
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
    apiKind: "DaemonSet",
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
    apiKind: "Job",
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
    apiKind: "CronJob",
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
    apiKind: "HorizontalPodAutoscaler",
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
    apiKind: "Endpoints",
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
    apiKind: "PodDisruptionBudget",
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
    apiKind: "NetworkPolicy",
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
    apiKind: "ResourceQuota",
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
    apiKind: "LimitRange",
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
    apiKind: "StorageClass",
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
    apiKind: "PriorityClass",
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
    apiKind: "CustomResourceDefinition",
  },
];

const SINGULAR: Record<string, string> = Object.fromEntries(
  RESOURCES.flatMap((r) => [[r.singular, r.name], ...r.short.map((s) => [s, r.name] as const)]),
);

export function resolveResource(token: string): Resource | undefined {
  const raw = token.split("/")[0]?.toLowerCase() ?? "";
  const name = SINGULAR[raw] ?? raw;
  return RESOURCES.find((r) => r.name === name || r.short.includes(raw) || r.singular === raw);
}

export type FlagDef = { name: string; detail: string; takes?: string; weight: number };

const GLOBAL_FLAGS: FlagDef[] = [
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
  { name: "--help", detail: "Help for this command", weight: 18 },
];

const VERB_FLAGS: Record<string, FlagDef[]> = {
  get: [
    { name: "-w", detail: "Watch for changes", weight: 70 },
    { name: "--watch", detail: "Watch for changes", weight: 48 },
    { name: "--show-labels", detail: "Print labels column", weight: 44 },
    { name: "--sort-by", detail: "Sort by a JSON path", takes: "sort", weight: 36 },
    { name: "--chunk-size", detail: "Return large lists in chunks", takes: "replicas", weight: 8 },
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
    { name: "--all-containers", detail: "Every container in the pod", weight: 24 },
  ],
  exec: [
    { name: "-it", detail: "Allocate a TTY and stdin", weight: 94 },
    { name: "-i", detail: "Pass stdin to the container", weight: 40 },
    { name: "-t", detail: "Allocate a TTY", weight: 36 },
    { name: "-c", detail: "Container name", takes: "container", weight: 70 },
    { name: "--container", detail: "Container name", takes: "container", weight: 30 },
    { name: "--stdin", detail: "Pass stdin", weight: 16 },
    { name: "--tty", detail: "Allocate a TTY", weight: 16 },
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
    { name: "--prune", detail: "Delete objects missing from the file", weight: 14 },
  ],
  delete: [
    { name: "-f", detail: "Delete resources in a file", takes: "file", weight: 80 },
    { name: "--filename", detail: "Delete resources in a file", takes: "file", weight: 20 },
    { name: "--force", detail: "Immediate deletion", weight: 50 },
    { name: "--grace-period", detail: "Seconds to wait before delete", takes: "grace", weight: 40 },
    { name: "--now", detail: "Grace period of 1 second", weight: 30 },
    { name: "--all", detail: "Delete every object of this type", weight: 28 },
    { name: "--dry-run", detail: "Don't persist changes", takes: "dryrun", weight: 36 },
    { name: "--wait", detail: "Wait until deletion finishes", weight: 12 },
  ],
  create: [
    {
      name: "--dry-run",
      detail: "Print the object instead of creating it",
      takes: "dryrun",
      weight: 70,
    },
    { name: "--image", detail: "Container image", takes: "image", weight: 80 },
    { name: "--from-literal", detail: "Key=value pair", takes: "literal", weight: 60 },
    { name: "--port", detail: "Service or container port", takes: "port", weight: 50 },
    { name: "-o", detail: "Output format", takes: "output", weight: 40 },
  ],
  scale: [{ name: "--replicas", detail: "Desired replica count", takes: "replicas", weight: 96 }],
  rollout: [
    { name: "--to-revision", detail: "Revision to roll back to", takes: "revision", weight: 60 },
    { name: "--revision", detail: "Revision to inspect", takes: "revision", weight: 40 },
    { name: "--timeout", detail: "How long to wait", takes: "timeout", weight: 30 },
  ],
  "port-forward": [{ name: "--address", detail: "Bind address", takes: "address", weight: 50 }],
  expose: [
    { name: "--port", detail: "Service port", takes: "port", weight: 80 },
    { name: "--target-port", detail: "Container port", takes: "port", weight: 60 },
    { name: "--type", detail: "Service type", takes: "svctype", weight: 50 },
    { name: "--name", detail: "Service name", takes: "name", weight: 30 },
  ],
  set: [{ name: "--dry-run", detail: "Don't persist changes", takes: "dryrun", weight: 20 }],
  run: [
    { name: "--image", detail: "Container image", takes: "image", weight: 90 },
    { name: "--restart", detail: "Restart policy", takes: "restart", weight: 50 },
    { name: "-it", detail: "Interactive TTY", weight: 40 },
    { name: "--rm", detail: "Delete the pod when it exits", weight: 36 },
    { name: "--port", detail: "Expose a container port", takes: "port", weight: 24 },
    { name: "--dry-run", detail: "Don't persist changes", takes: "dryrun", weight: 20 },
  ],
  drain: [
    { name: "--ignore-daemonsets", detail: "Skip DaemonSet pods", weight: 80 },
    { name: "--delete-emptydir-data", detail: "Delete emptyDir data", weight: 60 },
    { name: "--force", detail: "Continue even if unmanaged pods remain", weight: 40 },
    { name: "--grace-period", detail: "Seconds to wait", takes: "grace", weight: 30 },
  ],
  wait: [
    { name: "--for", detail: "Condition to wait for", takes: "for", weight: 90 },
    { name: "--timeout", detail: "Give up after this duration", takes: "timeout", weight: 50 },
  ],
  debug: [
    { name: "-it", detail: "Interactive TTY", weight: 70 },
    { name: "--image", detail: "Debug image", takes: "image", weight: 60 },
    {
      name: "--target",
      detail: "Container to share the namespace with",
      takes: "container",
      weight: 40,
    },
  ],
  diff: [
    { name: "-f", detail: "File to diff", takes: "file", weight: 90 },
    { name: "-k", detail: "Kustomize directory", takes: "dir", weight: 40 },
  ],
  version: [{ name: "--client", detail: "Client version only", weight: 80 }],
  "api-resources": [
    { name: "--namespaced", detail: "Only namespaced resources", weight: 40 },
    { name: "--api-group", detail: "Filter by API group", takes: "group", weight: 30 },
  ],
  config: [
    { name: "--current", detail: "Edit the current context", weight: 80 },
    {
      name: "--namespace",
      detail: "Default namespace for the context",
      takes: "namespace",
      weight: 70,
    },
    { name: "--minify", detail: "Only the current context", weight: 30 },
  ],
  kustomize: [],
  label: [{ name: "--overwrite", detail: "Replace an existing label", weight: 40 }],
  annotate: [{ name: "--overwrite", detail: "Replace an existing annotation", weight: 40 }],
  patch: [
    { name: "-p", detail: "Patch document", takes: "literal", weight: 50 },
    { name: "--type", detail: "strategic, merge, or json", takes: "patchtype", weight: 40 },
  ],
};

export function flagCatalog(verb?: string): FlagDef[] {
  const extra = verb ? (VERB_FLAGS[verb] ?? []) : [];
  const map = new Map<string, FlagDef>();
  for (const flag of [...extra, ...GLOBAL_FLAGS]) {
    if (!map.has(flag.name)) map.set(flag.name, flag);
  }
  return [...map.values()];
}

export function takesOf(flag: string, verb?: string): string | undefined {
  const name = flag.includes("=") ? flag.slice(0, flag.indexOf("=")) : flag;
  return flagCatalog(verb).find((flagDef) => flagDef.name === name)?.takes;
}

export const VERBS: { name: string; detail: string; weight: number }[] = [
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
  { name: "replace", detail: "Replace a resource from a file", weight: 10 },
];

export const SUBS: Record<string, { value: string; detail: string; weight: number }[]> = {
  config: [
    { value: "get-contexts", detail: "List kubeconfig contexts", weight: 100 },
    { value: "use-context", detail: "Switch the current context", weight: 96 },
    { value: "current-context", detail: "Print the current context", weight: 80 },
    { value: "view", detail: "Show the merged kubeconfig", weight: 70 },
    { value: "set-context", detail: "Change a context entry", weight: 74 },
    { value: "get-clusters", detail: "List clusters", weight: 40 },
    { value: "delete-context", detail: "Delete a context", weight: 30 },
  ],
  rollout: [
    { value: "status", detail: "See if a rollout is done", weight: 100 },
    { value: "restart", detail: "Rolling restart", weight: 94 },
    { value: "history", detail: "List revisions", weight: 80 },
    { value: "undo", detail: "Roll back to a revision", weight: 86 },
    { value: "pause", detail: "Pause a rollout", weight: 40 },
    { value: "resume", detail: "Resume a rollout", weight: 36 },
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
    { value: "quota", detail: "Create a resource quota", weight: 30 },
  ],
  set: [
    { value: "image", detail: "Update a container image", weight: 100 },
    { value: "env", detail: "Update environment variables", weight: 80 },
    { value: "resources", detail: "Set CPU and memory", weight: 60 },
  ],
  top: [
    { value: "pods", detail: "Pod CPU and memory", weight: 100 },
    { value: "nodes", detail: "Node CPU and memory", weight: 90 },
  ],
  completion: [
    { value: "bash", detail: "Bash completion script", weight: 80 },
    { value: "zsh", detail: "Zsh completion script", weight: 100 },
    { value: "fish", detail: "Fish completion script", weight: 70 },
    { value: "powershell", detail: "PowerShell completion script", weight: 40 },
  ],
};

export const EXPLAIN: Record<string, { detail: string; doc: string }> = {
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
  status      <Object>  most recently observed status`,
  },
  "pods.metadata": {
    detail: "Object metadata",
    doc: `FIELD: metadata <Object>

  name        <string>
  namespace   <string>
  labels      <map[string]string>
  annotations <map[string]string>`,
  },
  "pods.spec": {
    detail: "Desired pod behavior",
    doc: `FIELD: spec <Object>

  containers          <[]Object>  required
  initContainers      <[]Object>
  restartPolicy       <string>    Always, OnFailure, or Never
  serviceAccountName  <string>
  nodeName            <string>
  volumes             <[]Object>`,
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
  readinessProbe  <Object>`,
  },
  "pods.spec.containers.image": {
    detail: "Container image reference",
    doc: `FIELD: image <string>

Container image name. Empty means the image is not updated by a controller.`,
  },
  "pods.status": {
    detail: "Observed pod status",
    doc: `FIELD: status <Object>

  phase      <string>  Pending, Running, Succeeded, Failed
  podIP      <string>
  startTime  <string>
  containerStatuses <[]Object>`,
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
  status    <Object>`,
  },
  "deployments.spec": {
    detail: "Deployment specification",
    doc: `FIELD: spec <Object>

  replicas  <integer>
  selector  <Object>  must match the pod template
  template  <Object>  pod template
  strategy  <Object>  RollingUpdate or Recreate`,
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
  status    <Object>`,
  },
  "services.spec": {
    detail: "Service specification",
    doc: `FIELD: spec <Object>

  type       <string>  ClusterIP, NodePort, or LoadBalancer
  selector   <map[string]string>
  ports      <[]Object>
  clusterIP  <string>`,
  },
};

export function currentContext(cluster: Cluster) {
  return cluster.contexts.find((c) => c.name === cluster.current) ?? cluster.contexts[0];
}

export function namespaceOf(cluster: Cluster): string {
  return currentContext(cluster)?.namespace || "default";
}

export function promptOf(cluster: Cluster): string {
  const ctx = currentContext(cluster);
  if (!ctx) return "none:default";
  return `${ctx.name}:${ctx.namespace}`;
}
