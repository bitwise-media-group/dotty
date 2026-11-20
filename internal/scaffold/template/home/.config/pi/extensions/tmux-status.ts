/**
 * Copyright 2026 BitWise Media Group Ltd
 * SPDX-License-Identifier: MIT
 */

// Drive the shared `dotty tmux set-status` indicator from Pi's lifecycle events, mirroring
// the Claude Code Stop/Notification/UserPromptSubmit hooks so a Pi pane flags the same tmux
// window state (calm peach ● while waiting, bold red 󰂚 when it needs you). Pi's own OSC 7501
// status does not pass through tmux, so this hook is what reaches the window.
//
// Pi auto-loads every `extensions/*.ts` under its agent directory, so this file needs no
// settings entry.
//
// Event -> state mapping (see docs/extensions.md in the Pi package):
//   agent_start      -> clear      (a run began; the agent is busy)
//   agent_settled    -> waiting    (Pi will not continue on its own; your move)
//   ui_prompt_start  -> attention  (a dialog — such as dotty's policy gate — blocks on you)
//   ui_prompt_end    -> clear      (the dialog was answered)
//   session_shutdown -> clear
//
// dotty swallows every tmux/tty error and always exits 0; a missing dotty or a failed spawn
// is ignored here too, so a status blip can never disrupt a Pi session.

import { execFile } from "node:child_process";
import type { ExtensionAPI } from "@earendil-works/pi-coding-agent";

export default function (pi: ExtensionAPI) {
	const set = (state: string) => {
		try {
			execFile("dotty", ["tmux", "set-status", state], () => {
				// ignore — the indicator is best-effort
			});
		} catch {
			// ignore — the indicator is best-effort
		}
	};

	pi.on("agent_start", () => set("clear"));
	pi.on("agent_settled", () => set("waiting"));
	pi.on("ui_prompt_start", () => set("attention"));
	pi.on("ui_prompt_end", () => set("clear"));
	pi.on("session_shutdown", () => set("clear"));
}
