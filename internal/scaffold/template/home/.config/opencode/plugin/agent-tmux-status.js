/**
 * Copyright 2026 BitWise Media Group Ltd
 * SPDX-License-Identifier: MIT
 */

// Drive the shared `dotty tmux set-status` indicator from opencode's event bus, mirroring the
// Claude Code Stop/Notification/UserPromptSubmit hooks so an opencode pane flags the same
// tmux window state (calm peach ● while waiting, bold red 󰂚 when it needs you).
//
// opencode auto-loads every `plugin/*.js` under `~/.config/opencode/`, so dropping this file
// in is all the wiring required — there is no entry to add to `opencode.jsonc`.
//
// Event -> state mapping (see https://opencode.ai/docs/plugins/):
//   session.idle        -> waiting    (the agent finished its turn; your move)
//   permission.updated  -> attention  (the agent is blocked on an approval)
//   message.updated/user -> clear     (you sent a new prompt; the agent is busy again)
//
// The leaf command is no-op-safe (dotty swallows every tmux/tty error and always exits 0),
// and we additionally swallow any error here so a status blip can never disrupt an
// opencode session.

export const AgentTmuxStatus = async ({ $ }) => {
  const set = async (state) => {
    try {
      await $`dotty tmux set-status ${state}`.quiet().nothrow();
    } catch {
      // ignore — the indicator is best-effort
    }
  };

  return {
    event: async ({ event }) => {
      switch (event.type) {
        case "permission.updated":
          await set("attention");
          break;
        case "session.idle":
          await set("waiting");
          break;
        case "message.updated":
          if (event.properties?.info?.role === "user") await set("clear");
          break;
      }
    },
  };
};
