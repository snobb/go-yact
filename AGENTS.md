# Repository Navigation & Code Tracing Rules
- **Markdown File Refs**: Whenever you trace, explain and provide reference in the repository in a form of link to function in question followed by `relateive/to/root/filename:line` wrapped into markdown code but no links. Eg. "[Foobar](relative/path/to/file#L123) `relative/path/to/file:123`".
- **Hard Lock**: Halt all further tool execution immediately after printing the gatekeeper prompt. Do not attempt to touch or change any file in the workspace until the user inputs their authorization.

## Global Constraints & Token Economy
- **Exclusion Zones**: Never read, scan, or pass contents from `**/vendor/**`, `**/node_modules/**` or any generated `*.pb.go` / `*.pb.gw.go` files.
- **Focus Targets**: Strictly confine codebase analysis to `cmd/**/*.go`, `internal/**/*.go`, and `pkg/**/*.go`.
- **Style Guide**: Generate concise, production-ready Go, TypeScript, Shell and Terraform code blocks. Omit verbose explanations, markdown commentary, and conversational filler to preserve the output token budget. Do not omit explaination if it is required to understand the code changes.

# Review Guidelines
- Focus only on critical security, reliability, performance, and scalability issues.
- Ignore style, performance micro-optimizations, and readability nits unless they are tied to a significant failure.

## What to Look For
- Authentication/authorization bypasses
- Secret leakage, unsafe logging, or credential exposure
- Unsafe defaults in security-sensitive areas
- Injection risks (SQL, command, template, path traversal, SSRF)
- Insecure crypto usage or key handling
- Privilege escalation or sandbox escapes
- Data corruption, durability failures, or irreversible loss scenarios
- Concurrency hazards that can cause outages or data races
- Reliability regressions: crash loops, panics, deadlocks, unbounded retries, nil pointer dereferences

# Testing Rules
- **Unit Tests**: Prefer table based tests if it makes sense and does not increase the complexity of the tests. Follow by example from surrounding tests.
