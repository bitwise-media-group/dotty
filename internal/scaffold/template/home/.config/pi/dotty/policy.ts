/**
 * Copyright 2026 BitWise Media Group Ltd
 * SPDX-License-Identifier: MIT
 */

// dotty's policy gate for Pi, which has no approvals or sandbox of its own. It gives
// Pi the confinement Claude Code gets from its permissions and sandbox blocks in
// settings.json, built from the same lists:
//
//   sandbox.json  srt (Anthropic's sandbox-runtime) settings rendered per profile —
//                 the single source of truth for the filesystem and network rules.
//                 Every bash command runs inside `srt --settings sandbox.json`, and
//                 the gate applies its denyRead/denyWrite to Pi's in-process tools.
//   policy.json   the bash allowlist (Claude's permissions.allow prefixes) and the
//                 denied-command patterns (credential paths, security, fnox).
//
// Tool rules, mirroring Claude Code:
//   read/grep/find/ls  blocked on the credential set (denyRead), otherwise allowed
//   write/edit         blocked on denyWrite and the credential set, allowed under /tmp
//                      and the worktrees root, otherwise asked
//   bash               blocked on a denied pattern; allowed when it matches an allowlist
//                      prefix and carries no shell metacharacter; otherwise asked
//   anything else      asked
//   no UI (-p, json)   whatever would be asked is blocked — the gate fails closed
//
// Bash fails closed too: without srt on PATH (run `mise install`), or on a platform srt
// cannot sandbox, every bash command is blocked rather than run unconfined. `pi
// --no-sandbox` drops only the srt wrapper; the gate still applies.
//
// The gate loads only because settings.json lists it under "extensions" (rendered when
// the profile is hardened); it lives outside the auto-discovered extensions/ directory
// so an unhardened profile carries it inert.

import { accessSync, constants, readdirSync, readFileSync, realpathSync } from "node:fs";
import { homedir } from "node:os";
import { basename, delimiter, dirname, isAbsolute, join, resolve, sep } from "node:path";
import {
	createBashToolDefinition,
	createLocalBashOperations,
	type ExtensionAPI,
	type ExtensionContext,
	getAgentDir,
} from "@earendil-works/pi-coding-agent";

interface SandboxSettings {
	filesystem: { denyRead: string[]; denyWrite: string[] };
}

interface Policy {
	bash: { allow: string[]; deny: { pattern: string; reason: string }[] };
	readOnlyTools: string[];
}

type Verdict = { kind: "allow" } | { kind: "block"; reason: string } | { kind: "ask"; title: string; detail: string };

type PathRule = (path: string) => boolean;

const home = homedir();

// Characters that let one allow-listed command smuggle another (chaining, pipes,
// substitution, redirection, subshells). A command carrying any of them is asked.
const SHELL_META = /[;&|$`<>()\n\r]/;

// Read from the agent directory rather than this file's own location: the per-profile
// sandbox.json reaches it through the active-profile link, and a loader that resolves
// symlinks would otherwise look for it beside the shared policy.ts in the repository.
const policyDir = join(getAgentDir(), "dotty");
const sandboxPath = join(policyDir, "sandbox.json");

function expandHome(path: string): string {
	if (path === "~") return home;
	if (path.startsWith("~/")) return join(home, path.slice(2));
	return path;
}

// Resolve a tool's path argument the way Pi's own tools do: an optional @ prefix, ~
// expansion, relative to the session's working directory.
function toolPath(raw: unknown, cwd: string): string {
	let path = typeof raw === "string" && raw !== "" ? raw : ".";
	if (path.startsWith("@")) path = path.slice(1);
	path = expandHome(path);
	return isAbsolute(path) ? resolve(path) : resolve(cwd, path);
}

// Resolve symlinks on the longest existing prefix, so a write is judged where it would
// land — a link into the dotfiles repository, or a not-yet-created file beneath one.
function canonical(path: string): string {
	const rest: string[] = [];
	let current = path;
	for (;;) {
		try {
			return join(realpathSync(current), ...rest.reverse());
		} catch {
			const parent = dirname(current);
			if (parent === current) return path;
			rest.push(basename(current));
			current = parent;
		}
	}
}

function within(path: string, root: string): boolean {
	return path === root || path.startsWith(root.endsWith(sep) ? root : root + sep);
}

function globToRegExp(glob: string): RegExp {
	let re = "";
	for (let i = 0; i < glob.length; i++) {
		const c = glob[i];
		if (c === "*" && glob[i + 1] === "*") {
			i++;
			if (glob[i + 1] === "/") {
				i++;
				re += "(?:.*/)?";
			} else {
				re += ".*";
			}
		} else if (c === "*") {
			re += "[^/]*";
		} else if (c === "?") {
			re += "[^/]";
		} else {
			re += c.replace(/[.+^${}()|[\]\\]/g, "\\$&");
		}
	}
	return new RegExp((glob.startsWith("/") ? "^" : "(?:^|/)") + re + "$");
}

// A rule matches a path when the path is the rule's target or lies beneath it. A
// literal root also matches through its canonical form (/tmp is /private/tmp on
// macOS); a glob such as **/.env* matches the path or any of its ancestors.
function pathRule(spec: string): PathRule {
	const expanded = expandHome(spec);
	if (!/[*?]/.test(expanded)) {
		const root = resolve(expanded);
		const real = canonical(root);
		return (path) => within(path, root) || within(path, real);
	}
	const re = globToRegExp(expanded);
	return (path) => {
		for (let p = path; ; p = dirname(p)) {
			if (re.test(p)) return true;
			if (dirname(p) === p) return false;
		}
	};
}

function matchesAny(rules: PathRule[], path: string): boolean {
	const real = canonical(path);
	return rules.some((rule) => rule(path) || rule(real));
}

// Claude-style command patterns: a trailing * matches anything, otherwise exact.
function commandRule(pattern: string): RegExp {
	return new RegExp(`^${pattern.split("*").map((s) => s.replace(/[.+?^${}()|[\]\\]/g, "\\$&")).join(".*")}$`);
}

function shellQuote(s: string): string {
	return `'${s.replace(/'/g, `'\\''`)}'`;
}

function findOnPath(name: string): string | undefined {
	for (const dir of (process.env.PATH ?? "").split(delimiter)) {
		if (dir === "") continue;
		const candidate = join(dir, name);
		try {
			accessSync(candidate, constants.X_OK);
			return candidate;
		} catch {
			// not here
		}
	}
	return undefined;
}

// The agent directory and the real location of everything in it — the dotfiles files
// its entries link to — so neither the gate nor its settings can be rewritten through
// a link. sandbox.json denies the same to bash.
function protectedRules(agentDir: string): PathRule[] {
	const rules = [pathRule(agentDir)];
	for (const dir of [agentDir, join(agentDir, "dotty"), join(agentDir, "extensions"), join(agentDir, "themes")]) {
		let names: string[];
		try {
			names = readdirSync(dir);
		} catch {
			continue;
		}
		for (const name of names) rules.push(pathRule(canonical(join(dir, name))));
	}
	return rules;
}

// The worktrees root `dotty git worktree` uses: an absolute or ~-prefixed root, or a
// directory name inside each repository.
function worktreesRule(): PathRule {
	const setting = process.env.DOTTY_WORKTREES || "~/.local/share/dotty/worktrees";
	if (setting.startsWith("~") || isAbsolute(setting)) return pathRule(setting);
	return pathRule(`**/${setting}`);
}

function loadPolicy() {
	const sandbox = JSON.parse(readFileSync(sandboxPath, "utf8")) as SandboxSettings;
	const policy = JSON.parse(readFileSync(join(policyDir, "policy.json"), "utf8")) as Policy;
	const denyRead = sandbox.filesystem.denyRead.map(pathRule);
	return {
		denyRead,
		denyReadRoots: sandbox.filesystem.denyRead
			.filter((spec) => !/[*?]/.test(spec))
			.map((spec) => resolve(expandHome(spec))),
		denyWrite: [...sandbox.filesystem.denyWrite.map(pathRule), ...denyRead, ...protectedRules(getAgentDir())],
		autoWrite: [pathRule("/tmp"), worktreesRule()],
		bashAllow: policy.bash.allow.map(commandRule),
		bashDeny: policy.bash.deny.map((d) => ({ re: new RegExp(d.pattern, "i"), reason: d.reason })),
		readOnlyTools: new Set(policy.readOnlyTools),
	};
}

export default function (pi: ExtensionAPI) {
	pi.registerFlag("no-sandbox", {
		description: "Run bash without dotty's srt sandbox (the policy gate still applies)",
		type: "boolean",
		default: false,
	});

	let policy: ReturnType<typeof loadPolicy> | undefined;
	let loadError = "";
	try {
		policy = loadPolicy();
	} catch (err) {
		loadError = `dotty policy failed to load from ${policyDir}: ${err instanceof Error ? err.message : String(err)}`;
	}

	const srt = findOnPath("srt");
	const supported = process.platform === "darwin" || process.platform === "linux";
	let sandboxed = true; // flags are readable only once the session starts

	pi.on("session_start", async (_event, ctx) => {
		sandboxed = pi.getFlag("no-sandbox") !== true;
		if (!ctx.hasUI) return;
		if (loadError) ctx.ui.notify(`${loadError} — every tool call is blocked`, "error");
		else if (!sandboxed) ctx.ui.notify("bash runs without the srt sandbox (--no-sandbox)", "warning");
		else if (sandboxError()) ctx.ui.notify(`${sandboxError()} — bash is blocked`, "error");
	});

	function sandboxError(): string {
		if (!supported) return `srt cannot sandbox bash on ${process.platform}`;
		if (!srt) return "srt (sandbox-runtime) is not on PATH; run `mise install`";
		return "";
	}

	// The command line bash -c runs: srt re-parses cmd with bash -c inside the sandbox.
	function wrap(command: string): string {
		if (!sandboxed) return command;
		const err = sandboxError();
		if (err) throw new Error(err);
		return `${shellQuote(srt as string)} --settings ${shellQuote(sandboxPath)} -c ${shellQuote(command)}`;
	}

	pi.registerTool(
		createBashToolDefinition(process.cwd(), {
			spawnHook: ({ command, cwd, env }) => ({ command: wrap(command), cwd, env }),
		}),
	);

	// The user's own ! commands skip the gate (they typed them) but not the sandbox.
	pi.on("user_bash", () => {
		if (!sandboxed) return undefined;
		const err = loadError || sandboxError();
		if (err) return { result: { output: `blocked: ${err}\n`, exitCode: 1, cancelled: false, truncated: false } };
		const local = createLocalBashOperations();
		return { operations: { exec: (command, cwd, options) => local.exec(wrap(command), cwd, options) } };
	});

	function judge(toolName: string, input: Record<string, unknown>, cwd: string): Verdict {
		if (!policy) return { kind: "block", reason: loadError };
		switch (toolName) {
			case "read":
			case "grep":
			case "find":
			case "ls": {
				const path = toolPath(input.path, cwd);
				if (matchesAny(policy.denyRead, path)) {
					return { kind: "block", reason: `${path} is a credential path dotty's policy denies reading` };
				}
				// grep and find recurse with --hidden, so a search rooted above a credential
				// directory would walk into it.
				if (toolName === "grep" || toolName === "find") {
					const real = canonical(path);
					const inside = policy.denyReadRoots.find((root) => within(root, path) || within(root, real));
					if (inside) return { kind: "block", reason: `searching ${path} would reach ${inside}; narrow the path` };
				}
				return { kind: "allow" };
			}
			case "write":
			case "edit": {
				const path = toolPath(input.path, cwd);
				if (matchesAny(policy.denyWrite, path)) {
					return { kind: "block", reason: `${path} is write-protected by dotty's policy` };
				}
				if (matchesAny(policy.autoWrite, path)) return { kind: "allow" };
				return { kind: "ask", title: `Allow Pi to ${toolName} this file?`, detail: path };
			}
			case "bash": {
				const command = typeof input.command === "string" ? input.command : "";
				const denied = policy.bashDeny.find((d) => d.re.test(command));
				if (denied) return { kind: "block", reason: `blocked by dotty policy: ${denied.reason}` };
				if (sandboxed && sandboxError()) return { kind: "block", reason: sandboxError() };
				const trimmed = command.trim();
				if (!SHELL_META.test(trimmed) && policy.bashAllow.some((re) => re.test(trimmed))) return { kind: "allow" };
				return { kind: "ask", title: sandboxed ? "Run this command?" : "Run this command (unsandboxed)?", detail: command };
			}
			default:
				if (policy.readOnlyTools.has(toolName)) return { kind: "allow" };
				return { kind: "ask", title: `Allow Pi to use ${toolName}?`, detail: JSON.stringify(input, null, 2) };
		}
	}

	pi.on("tool_call", async (event, ctx: ExtensionContext) => {
		let verdict: Verdict;
		try {
			verdict = judge(event.toolName, (event.input ?? {}) as Record<string, unknown>, ctx.cwd);
		} catch (err) {
			verdict = { kind: "block", reason: `dotty policy error: ${err instanceof Error ? err.message : String(err)}` };
		}
		switch (verdict.kind) {
			case "allow":
				return undefined;
			case "block":
				return { block: true, reason: verdict.reason };
			case "ask":
				if (!ctx.hasUI) {
					return { block: true, reason: "dotty policy needs approval for this call, and Pi has no UI to ask" };
				}
				if (await ctx.ui.confirm(verdict.title, verdict.detail)) return undefined;
				return { block: true, reason: "denied by the user" };
		}
	});
}
