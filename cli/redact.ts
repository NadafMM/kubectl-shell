const PAIR_FLAGS = ["--from-literal", "--from-file"];
const VALUE_FLAGS = ["--docker-password", "--password", "--token", "--client-secret"];
const SENSITIVE_KEY = /^(password|token|secret|apikey|api-key|client-secret|client_secret)$/i;

function hidePair(value: string): string {
  const eq = value.indexOf("=");
  if (eq < 0) return "***";
  return `${value.slice(0, eq)}=***`;
}

function matchFlag(token: string, flags: string[]): string | undefined {
  return flags.find((flag) => token === flag || token.startsWith(`${flag}=`));
}

function redactAssignment(token: string): string {
  const eq = token.indexOf("=");
  if (eq < 0) return token;
  const key = token.slice(0, eq);
  if (SENSITIVE_KEY.test(key)) return `${key}=***`;
  return token;
}

export function redactLine(line: string): string {
  const tokens = line.split(" ");
  const out: string[] = [];
  for (let i = 0; i < tokens.length; i += 1) {
    const token = tokens[i] ?? "";
    const pair = matchFlag(token, PAIR_FLAGS);
    if (pair) {
      if (token === pair) {
        out.push(token);
        const next = tokens[i + 1];
        if (next !== undefined) {
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
        if (tokens[i + 1] !== undefined) {
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
