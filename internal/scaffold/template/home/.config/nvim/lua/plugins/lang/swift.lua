-- Copyright 2026 BitWise Media Group Ltd
-- SPDX-License-Identifier: MIT

-- Swift support adapted for LazyVim from:
-- https://www.swift.org/documentation/articles/zero-to-swift-nvim.html
--
-- Requires a Swift toolchain on PATH (sourcekit-lsp + lldb-dap). Neither is
-- available via Mason; use the versions shipped with the toolchain.
-- On macOS, lldb-dap is resolved via xcrun when possible.

---Locate the Swift-capable lldb-dap binary.
---@return string command path, or empty string when not found
local function find_lldb_dap()
  -- macOS: prefer the Xcode / Command Line Tools lldb-dap.
  if vim.fn.executable("xcrun") == 1 then
    local result = vim.system({ "xcrun", "--find", "lldb-dap" }, { text = true }):wait()
    if result.code == 0 then
      local path = vim.fn.trim(result.stdout)
      if path ~= "" and vim.fn.executable(path) == 1 then
        return path
      end
    end
  end

  -- Fallback: lldb-dap from the Swift toolchain on PATH.
  if vim.fn.executable("lldb-dap") == 1 then
    return "lldb-dap"
  end

  return ""
end

return {
  {
    "nvim-treesitter/nvim-treesitter",
    opts = { ensure_installed = { "swift" } },
  },
  {
    "neovim/nvim-lspconfig",
    opts = {
      servers = {
        -- Bundled with the Swift toolchain; nvim-lspconfig supplies root
        -- markers, language IDs, and the dynamicRegistration capabilities
        -- called out in the official guide.
        sourcekit = {
          mason = false,
          filetypes = { "swift" },
        },
      },
    },
  },
  {
    "stevearc/conform.nvim",
    optional = true,
    opts = {
      formatters_by_ft = {
        -- Official swift-format (Swift 6+). LazyVim's lsp_format = "fallback"
        -- uses sourcekit-lsp when the binary is missing.
        swift = { "swift" },
      },
    },
  },
  {
    "mfussenegger/nvim-dap",
    optional = true,
    opts = function()
      local command = find_lldb_dap()
      if command == "" then
        -- No Swift toolchain / lldb-dap; skip adapter registration silently.
        return
      end

      local dap = require("dap")
      dap.adapters["lldb-dap"] = {
        type = "executable",
        name = "lldb-dap",
        command = command,
        options = {
          -- Uncomment to enable lldb-dap logging (useful for bug reports).
          -- env = { LLDBDAP_LOG = "/tmp/lldb-dap.log" },
        },
      }

      dap.configurations.swift = {
        {
          name = "Launch program",
          type = "lldb-dap",
          request = "launch",
          program = function()
            return require("dap.utils").pick_file({ executables = true })
          end,
          cwd = "${workspaceFolder}",
        },
        {
          name = "Launch program with arguments",
          type = "lldb-dap",
          request = "launch",
          program = function()
            return require("dap.utils").pick_file({ executables = true })
          end,
          cwd = "${workspaceFolder}",
          args = function()
            local args_str = vim.fn.input("Arguments: ")
            return require("dap.utils").splitstr(args_str)
          end,
        },
        {
          name = "Attach to program",
          type = "lldb-dap",
          request = "attach",
          pid = function()
            return require("dap.utils").pick_process()
          end,
        },
      }
    end,
  },
}
