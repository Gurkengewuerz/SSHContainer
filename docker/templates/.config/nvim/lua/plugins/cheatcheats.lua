local M = { 'smartinellimarco/nvcheatsheet.nvim' }

M.opts = {
}

function M.config(_, opts)
    local nvcheatsheet = require('nvcheatsheet')

    nvcheatsheet.setup(opts)

    -- You can also close it with <Esc>
    vim.keymap.set('n', '<F1>', nvcheatsheet.toggle)
end

return M
