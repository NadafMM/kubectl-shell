package parse

func PlainText(value string) string {
	var out []byte
	for i := 0; i < len(value); i++ {
		if value[i] == 0x1b {
			if i+1 < len(value) && value[i+1] == ']' {
				i += 2
				for i < len(value) && value[i] != 0x07 && !(i+1 < len(value) && value[i] == 0x1b && value[i+1] == '\\') {
					i++
				}
				if i+1 < len(value) && value[i] == 0x1b && value[i+1] == '\\' {
					i++
				}
			} else if i+1 < len(value) && value[i+1] == '[' {
				i += 2
				for i < len(value) && (value[i] < 0x40 || value[i] > 0x7e) {
					i++
				}
			}
			continue
		}
		if value[i] < 0x20 || (value[i] >= 0x7f && value[i] <= 0x9f) {
			continue
		}
		out = append(out, value[i])
	}
	return string(out)
}

// CompletionValue drops values that contain terminal or shell metacharacters.
func CompletionValue(value string) (string, bool) {
	if PlainText(value) != value {
		return "", false
	}
	for _, r := range value {
		switch r {
		case '`', '$', ';', '|', '\n':
			return "", false
		}
	}
	return value, true
}
