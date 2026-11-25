package kube

import "strings"

// LeadingFlags are kube's own flags before the kubectl verb.
type LeadingFlags struct {
	Yes            bool
	Policy         string
	PlanID         string
	ConfirmContext string
	Sandbox        bool
	Sample         bool
	Record         string
	Rest           []string
	Error          string
}

var valueFlags = map[string]bool{
	"--policy": true, "--plan-id": true, "--confirm-context": true, "--record": true,
}

// ParseLeadingFlags peels kube flags from argv. Unknown leading flags error;
// flags after the first command word belong to kubectl.
func ParseLeadingFlags(argv []string) LeadingFlags {
	parsed := LeadingFlags{Rest: []string{}}
	for i := 0; i < len(argv); i++ {
		arg := argv[i]
		if len(parsed.Rest) > 0 {
			parsed.Rest = append(parsed.Rest, arg)
			continue
		}
		if arg == "--live" {
			continue
		}
		if arg == "--help" || arg == "-h" || arg == "--version" || arg == "-v" {
			parsed.Rest = append(parsed.Rest, arg)
			continue
		}
		if arg == "--sample" {
			parsed.Sample = true
			continue
		}
		if arg == "--yes" || arg == "-y" {
			parsed.Yes = true
			continue
		}
		if arg == "--sandbox" {
			parsed.Sandbox = true
			continue
		}
		eq := strings.Index(arg, "=")
		name := arg
		if eq >= 0 {
			name = arg[:eq]
		}
		if valueFlags[name] {
			var value string
			if eq == -1 {
				if i+1 >= len(argv) || strings.HasPrefix(argv[i+1], "-") {
					parsed.Error = "kube: " + name + " needs a value"
					return parsed
				}
				i++
				value = argv[i]
			} else {
				value = arg[eq+1:]
				if value == "" {
					parsed.Error = "kube: " + name + " needs a value"
					return parsed
				}
			}
			switch name {
			case "--policy":
				parsed.Policy = value
			case "--plan-id":
				parsed.PlanID = value
			case "--confirm-context":
				parsed.ConfirmContext = value
			case "--record":
				parsed.Record = value
			}
			continue
		}
		if strings.HasPrefix(arg, "-") {
			parsed.Error = "unknown flag " + arg + "; flags after the command belong to kubectl"
			return parsed
		}
		parsed.Rest = append(parsed.Rest, arg)
	}
	return parsed
}
