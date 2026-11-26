// Package kube holds shared domain types (Cluster, Plan, …), state paths,
// and test fixtures used by the feature packages.
//
// Implementation is split by concern:
//
//	parse    — tokenize, grammar, catalog, completion scripts
//	policy   — policy.json, Authorize, plans, confirm
//	redact   — secret redaction for argv and kubectl stdout
//	session  — history, audit, runbook recording, shell scope
//	live     — kubectl process + completion cache
//	plan     — PlanCommand, blast radius, ask/diagnose
//	complete — word completion
//	view     — interactive resource table
//	mcp      — MCP JSON-RPC server
//	undo     — undo snapshots
//	sandbox  — local kind sandbox
package kube
