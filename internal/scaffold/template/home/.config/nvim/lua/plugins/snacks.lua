-- Copyright 2026 BitWise Media Group Ltd
-- SPDX-License-Identifier: MIT

return {
  "folke/snacks.nvim",
  opts = {
    image = {},
    picker = {
      sources = {
        explorer = {
          hidden = true,
          ignored = false,
        },
        files = {
          hidden = true,
          ignored = false,
        },
        sessions = {
          supports_live = false,
          layout = {
            layout = {
              box = "horizontal",
              width = 0.8,
              min_width = 120,
              height = 0.8,
              {
                box = "vertical",
                border = true,
                title = " Sessions ",
                { win = "input", height = 1, border = "bottom" },
                { win = "list", border = "none" },
              },
              { win = "preview", title = " Status ", border = true, width = 0.5 },
            },
          },
          finder = function(_, ctx)
            local active = vim.fn.systemlist("tmux display-message -p '#{session_name}' 2>/dev/null")[1] or ""
            local raw = vim.fn.systemlist("tmux list-sessions -F '#{session_name} #{session_path}' 2>/dev/null")
            -- Worktree location from the active dotty profile: an absolute
            -- shared root, or a directory name inside each repository.
            local wt = vim.env.DOTTY_WORKTREES or ".worktrees"
            local is_worktree
            if wt:sub(1, 1) == "/" or wt:sub(1, 1) == "~" then
              local root = vim.fn.expand(wt) .. "/"
              is_worktree = function(p) return p:sub(1, #root) == root end
            else
              local marker = "/" .. wt .. "/"
              is_worktree = function(p) return p:find(marker, 1, true) ~= nil end
            end

            local parents_by_name = {}
            local parent_list = {}
            local worktree_list = {}

            for _, line in ipairs(raw) do
              local name, path = line:match("([^ ]+) (.+)")
              if name and path then
                local is_active = name == active
                if not is_active and is_worktree(path) then
                  table.insert(worktree_list, { name = name, path = path })
                else
                  local item = {
                    name = name,
                    kind = is_active and "active" or "parent",
                    tree = true,
                    text = name .. " " .. path,
                    data = { path = path, active = is_active },
                  }
                  parents_by_name[name] = item
                  table.insert(parent_list, item)
                end
              end
            end

            table.sort(parent_list, function(a, b)
              if a.kind == "active" and b.kind ~= "active" then
                return true
              elseif b.kind == "active" and a.kind ~= "active" then
                return false
              end
              return a.name < b.name
            end)

            local children_by_parent = {}
            local orphans = {}

            for _, wt in ipairs(worktree_list) do
              local best_parent, best_len = nil, 0
              for pname in pairs(parents_by_name) do
                local prefix = pname .. "-"
                if wt.name:sub(1, #prefix) == prefix and #pname > best_len then
                  best_parent = pname
                  best_len = #pname
                end
              end

              local item = {
                name = wt.name,
                kind = "worktree",
                tree = true,
                text = wt.name .. " " .. wt.path,
                data = { path = wt.path },
              }

              if best_parent then
                children_by_parent[best_parent] = children_by_parent[best_parent] or {}
                table.insert(children_by_parent[best_parent], item)
              else
                table.insert(orphans, item)
              end
            end

            local items = {}
            local align_1 = 0

            for _, parent in ipairs(parent_list) do
              align_1 = math.max(align_1, #parent.name)
              table.insert(items, parent)
              local children = children_by_parent[parent.name]
              if children then
                table.sort(children, function(a, b)
                  return a.name < b.name
                end)
                for i, child in ipairs(children) do
                  child.parent = parent
                  child.last = (i == #children)
                  align_1 = math.max(align_1, #child.name)
                  table.insert(items, child)
                end
              end
            end

            if #orphans > 0 then
              table.sort(orphans, function(a, b)
                return a.name < b.name
              end)
              local header = {
                name = "(orphans)",
                kind = "header",
                tree = true,
                text = "(orphans)",
              }
              align_1 = math.max(align_1, #header.name)
              table.insert(items, header)
              for i, orphan in ipairs(orphans) do
                orphan.kind = "orphan"
                orphan.parent = header
                orphan.last = (i == #orphans)
                align_1 = math.max(align_1, #orphan.name)
                table.insert(items, orphan)
              end
            end

            ctx.picker.align_1 = align_1
            return items
          end,
          format = function(item, picker)
            local ret = {}
            vim.list_extend(ret, Snacks.picker.format.tree(item, picker))
            local tree_w = vim.api.nvim_strwidth(ret[#ret] and ret[#ret][1] or "")
            local name_hl
            if item.kind == "header" then
              name_hl = "SnacksPickerDimmed"
            elseif item.kind == "orphan" then
              name_hl = "DiagnosticWarn"
            elseif item.kind == "active" then
              name_hl = "SnacksPickerSpecial"
            elseif item.kind == "worktree" then
              name_hl = "SnacksPickerDir"
            else
              name_hl = "SnacksPickerFile"
            end
            local label = item.kind == "active" and (item.name .. " (current)") or item.name
            ret[#ret + 1] = { Snacks.picker.util.align(label, picker.align_1 + 12 - tree_w), name_hl }
            if item.data and item.data.path then
              ret[#ret + 1] = { vim.fn.fnamemodify(item.data.path, ":~"), "SnacksPickerComment" }
            end
            return ret
          end,
          confirm = function(picker, item)
            picker:close()
            if not item or not item.data or not item.data.path or item.data.active then
              return
            end
            vim.fn.system(string.format("tmux switch-client -t '%s'", item.name))
          end,
          preview = function(ctx)
            ctx.preview:reset()
            local item = ctx.item
            if not item or not item.data or not item.data.path then
              ctx.preview:set_title("No selection")
              return
            end
            ctx.preview:set_title(item.name)
            local lines = vim.fn.systemlist({ "git", "-C", item.data.path, "status", "-sb" })
            if vim.v.shell_error ~= 0 then
              lines = { "(not a git repo)", item.data.path }
            end
            ctx.preview:set_lines(lines)
          end,
        },
        snippets = {
          supports_live = false,
          preview = "preview",
          format = function(item, picker)
            local name = Snacks.picker.util.align(item.name, picker.align_1 + 5)
            return {
              { name, item.ft == "" and "Conceal" or "DiagnosticWarn" },
              { item.description },
            }
          end,
          finder = function(_, ctx)
            local snippets = {}
            for _, snip in ipairs(require("luasnip").get_snippets().all) do
              snip.ft = ""
              table.insert(snippets, snip)
            end
            for _, snip in ipairs(require("luasnip").get_snippets(vim.bo.ft)) do
              snip.ft = vim.bo.ft
              table.insert(snippets, snip)
            end
            local align_1 = 0
            for _, snip in pairs(snippets) do
              align_1 = math.max(align_1, #snip.name)
            end
            ctx.picker.align_1 = align_1
            local items = {}
            for _, snip in pairs(snippets) do
              local docstring = snip:get_docstring()
              if type(docstring) == "table" then
                docstring = table.concat(docstring)
              end
              local name = snip.name
              local description = table.concat(snip.description)
              description = name == description and "" or description
              table.insert(items, {
                text = name .. " " .. description, -- search string
                name = name,
                description = description,
                trigger = snip.trigger,
                ft = snip.ft,
                preview = {
                  ft = snip.ft,
                  text = docstring,
                },
              })
            end
            return items
          end,
          confirm = function(picker, item)
            picker:close()
            --
            local expand = {}
            require("luasnip").available(function(snippet)
              if snippet.trigger == item.trigger then
                table.insert(expand, snippet)
              end
              return snippet
            end)
            if #expand > 0 then
              vim.cmd(":startinsert!")
              vim.defer_fn(function()
                require("luasnip").snip_expand(expand[1])
              end, 50)
            else
              Snacks.notify.warn("No snippet to expand")
            end
          end,
        },
      },
    },
  },
  keys = {
    -- swap the defaults to use cwd instead of root
    { "<leader><space>", LazyVim.pick("files", { root = false }), desc = "Find Files (cwd)" },
    { "<leader>E", "<leader>fe", desc = "Explorer Snacks (cwd)", remap = true },
    { "<leader>e", "<leader>fE", desc = "Explorer Snacks (root dir)", remap = true },
    {
      "<leader>fs",
      function()
        Snacks.picker.pick("sessions")
      end,
      desc = "Find Sessions (tmux)",
    },
    {
      "<leader>fx",
      function()
        Snacks.picker.pick("snippets")
      end,
      desc = "Find Snippets",
    },
  },
}
