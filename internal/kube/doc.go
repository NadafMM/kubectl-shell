// Package kube is the shared core for the kube CLI: parsing, policy, plans,
// redaction, and session state.
//
// Feature areas live in subpackages:
//
//	live      — kubectl process + completion cache
//	plan      — PlanCommand, blast radius, ask/diagnose
//	complete  — word completion
//	view      — interactive resource table
//	mcp       — MCP JSON-RPC server
//	undo      — object undo snapshots
//	sandbox   — local kind sandbox
package kube
