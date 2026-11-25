package kube

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"
)

const planTTL = 5 * time.Minute

var planIDPattern = regexp.MustCompile(`^[0-9a-f]{32}$`)

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

// ValidPlanID is true for minted ids: exactly 32 lowercase hex characters.
func ValidPlanID(id string) bool {
	return planIDPattern.MatchString(id)
}

func plansDir() string {
	return filepath.Join(StateDir(), "plans")
}

// planPath resolves id under ~/.kube-shell/plans. Rejects anything that escapes.
func planPath(id string) (string, bool) {
	if !ValidPlanID(id) {
		return "", false
	}
	base := plansDir()
	full := filepath.Clean(filepath.Join(base, id+".json"))
	rel, err := filepath.Rel(base, full)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", false
	}
	return full, true
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
	path, ok := planPath(id)
	if !ok {
		return "", os.ErrInvalid
	}
	if err := WritePrivate(path, string(raw)); err != nil {
		return "", err
	}
	return id, nil
}

// ConsumePlan accepts a record once when it matches the command about to run.
// The file is renamed first so two overlapping consumers cannot both succeed.
func ConsumePlan(planID string, expected PlanExpect, now time.Time) (bool, string) {
	path, ok := planPath(planID)
	if !ok {
		return false, "plan expired or unknown"
	}
	claimed := path + ".claimed"
	if err := os.Rename(path, claimed); err != nil {
		return false, "plan expired or unknown"
	}
	raw, err := os.ReadFile(claimed)
	if err != nil {
		_ = os.Remove(claimed)
		return false, "plan expired or unknown"
	}
	var stored PlanRecord
	if err := json.Unmarshal(raw, &stored); err != nil || stored.Expires <= now.UnixMilli() {
		_ = os.Remove(claimed)
		return false, "plan expired or unknown"
	}
	if stored.Context != expected.Context || stored.Namespace != expected.Namespace || !sameArgv(stored.Argv, expected.Argv) {
		_ = os.Rename(claimed, path)
		return false, "plan does not match this command"
	}
	_ = os.Remove(claimed)
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
