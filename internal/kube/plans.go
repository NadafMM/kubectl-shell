package kube

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"time"
)

const planTTL = 5 * time.Minute

type PlanRecord struct {
	PlanID    string   `json:"planId"`
	Context   string   `json:"context"`
	Namespace string   `json:"namespace"`
	Argv      []string `json:"argv"`
	Expires   int64    `json:"expires"`
}

type PlanExpect struct {
	Context   string
	Namespace string
	Argv      []string
}

func planPath(id string) string {
	return filepath.Join(StateDir(), "plans", id+".json")
}

// MintPlan stores a short-lived record an agent can pass back with --plan-id.
func MintPlan(record PlanExpect, now time.Time) (string, error) {
	buf := make([]byte, 16)
	if _, err := rand.Read(buf); err != nil {
		return "", err
	}
	id := hex.EncodeToString(buf)
	stored := PlanRecord{
		PlanID: id, Context: record.Context, Namespace: record.Namespace,
		Argv: record.Argv, Expires: now.Add(planTTL).UnixMilli(),
	}
	if stored.Argv == nil {
		stored.Argv = []string{}
	}
	raw, err := json.Marshal(stored)
	if err != nil {
		return "", err
	}
	if err := writePrivate(planPath(id), string(raw)); err != nil {
		return "", err
	}
	return id, nil
}

// ConsumePlan accepts a record once when it matches the command about to run.
func ConsumePlan(planID string, expected PlanExpect, now time.Time) (bool, string) {
	raw, err := os.ReadFile(planPath(planID))
	if err != nil {
		return false, "plan expired or unknown"
	}
	var stored PlanRecord
	if err := json.Unmarshal(raw, &stored); err != nil || stored.Expires <= now.UnixMilli() {
		_ = os.Remove(planPath(planID))
		return false, "plan expired or unknown"
	}
	if stored.Context != expected.Context || stored.Namespace != expected.Namespace || !sameArgv(stored.Argv, expected.Argv) {
		return false, "plan does not match this command"
	}
	_ = os.Remove(planPath(planID))
	return true, ""
}

func sameArgv(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
