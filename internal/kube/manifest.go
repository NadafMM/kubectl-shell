package kube

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

// ManifestRef is one named object from a manifest file.
type ManifestRef struct {
	Kind      string
	Name      string
	Namespace string
}

// ManifestRefs reads file (or a directory of yaml/json files) into refs.
func ManifestRefs(file string) (refs []ManifestRef, errMsg string, ok bool) {
	stat, err := os.Stat(file)
	if err != nil {
		return nil, fsErrString(err), false
	}
	if stat.IsDir() {
		entries, err := os.ReadDir(file)
		if err != nil {
			return nil, fsErrString(err), false
		}
		var all []ManifestRef
		for _, entry := range entries {
			name := entry.Name()
			if !manifestNameRe.MatchString(name) {
				continue
			}
			nested, nestedErr, nestedOK := ManifestRefs(filepath.Join(file, name))
			if !nestedOK {
				return nil, nestedErr, false
			}
			all = append(all, nested...)
		}
		if all == nil {
			all = []ManifestRef{}
		}
		return all, "", true
	}
	raw, err := os.ReadFile(file)
	if err != nil {
		return nil, fsErrString(err), false
	}
	return ParseManifest(string(raw)), "", true
}

func fsErrString(err error) string {
	if err == nil {
		return ""
	}
	var pe *os.PathError
	if errors.As(err, &pe) && pe.Err != nil {
		if errno, ok := pe.Err.(syscall.Errno); ok {
			switch errno {
			case syscall.ENOENT:
				return "ENOENT"
			case syscall.EACCES:
				return "EACCES"
			case syscall.EPERM:
				return "EPERM"
			case syscall.EISDIR:
				return "EISDIR"
			}
			return errno.Error()
		}
		return pe.Err.Error()
	}
	return err.Error()
}

var (
	manifestNameRe = regexp.MustCompile(`(?i)\.(ya?ml|json)$`)
	yamlDocSplitRe = regexp.MustCompile(`\r?\n---\r?\n`)
	yamlKeyRe      = regexp.MustCompile(`^([A-Za-z0-9_]+):\s*(.*)$`)
	yamlCommentRe  = regexp.MustCompile(`\s+#`)
)

// ParseManifest extracts kind/name/namespace refs from JSON or multi-doc YAML.
func ParseManifest(text string) []ManifestRef {
	trimmed := strings.TrimSpace(text)
	if trimmed == "" {
		return []ManifestRef{}
	}
	if strings.HasPrefix(trimmed, "{") || strings.HasPrefix(trimmed, "[") {
		var value any
		if err := json.Unmarshal([]byte(trimmed), &value); err != nil {
			return []ManifestRef{}
		}
		return jsonRefs(value)
	}
	var refs []ManifestRef
	for _, doc := range yamlDocSplitRe.Split(trimmed, -1) {
		if ref, ok := yamlRef(doc); ok {
			refs = append(refs, ref)
		}
	}
	if refs == nil {
		refs = []ManifestRef{}
	}
	return refs
}

func jsonRefs(value any) []ManifestRef {
	switch v := value.(type) {
	case []any:
		var refs []ManifestRef
		for _, item := range v {
			refs = append(refs, jsonRefs(item)...)
		}
		if refs == nil {
			return []ManifestRef{}
		}
		return refs
	case map[string]any:
		if items, ok := v["items"].([]any); ok {
			return jsonRefs(items)
		}
		kind, _ := v["kind"].(string)
		meta, _ := v["metadata"].(map[string]any)
		if kind == "" || meta == nil {
			return []ManifestRef{}
		}
		name, _ := meta["name"].(string)
		if name == "" {
			return []ManifestRef{}
		}
		ns, _ := meta["namespace"].(string)
		return []ManifestRef{{Kind: kind, Name: name, Namespace: ns}}
	default:
		return []ManifestRef{}
	}
}

func yamlRef(doc string) (ManifestRef, bool) {
	var kind, name string
	var namespace *string
	inMeta := false
	metaIndent := 0
	for _, raw := range strings.Split(doc, "\n") {
		raw = strings.TrimSuffix(raw, "\r")
		trimmed := strings.TrimSpace(raw)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		indent := len(raw) - len(strings.TrimLeft(raw, " \t"))
		if inMeta && indent <= metaIndent {
			inMeta = false
		}
		match := yamlKeyRe.FindStringSubmatch(trimmed)
		if match == nil {
			continue
		}
		key := match[1]
		value := stripYAMLValue(match[2])
		if !inMeta && indent == 0 && key == "kind" && value != "" {
			kind = value
		}
		if !inMeta && indent == 0 && key == "metadata" {
			inMeta = true
			metaIndent = indent
			continue
		}
		if inMeta && indent > metaIndent {
			if key == "name" && value != "" && name == "" {
				name = value
			}
			if key == "namespace" && value != "" && namespace == nil {
				ns := value
				namespace = &ns
			}
		}
	}
	if kind == "" || name == "" {
		return ManifestRef{}, false
	}
	ref := ManifestRef{Kind: kind, Name: name}
	if namespace != nil {
		ref.Namespace = *namespace
	}
	return ref, true
}

func stripYAMLValue(value string) string {
	cut := strings.TrimSpace(yamlCommentRe.Split(value, 2)[0])
	if len(cut) >= 2 {
		if (cut[0] == '"' && cut[len(cut)-1] == '"') || (cut[0] == '\'' && cut[len(cut)-1] == '\'') {
			return cut[1 : len(cut)-1]
		}
	}
	return cut
}
