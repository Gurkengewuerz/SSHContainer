local function open_nvim_term(data)
    -- buffer is a real file on the disk
    local real_file = vim.fn.filereadable(data.file) == 1

    -- buffer is a [No Name]
    local no_name = data.file == "" and vim.bo[data.buf].buftype == ""

    if not real_file and not no_name then
        return
    end


    --vim.cmd([[ ToggleTermToggleAll ]])

    -- list current buffers
    local buffers = vim.api.nvim_list_bufs()

    -- check if toggleterm buffer exists. If not then create one by vim.cmd [[ exe 1 . "ToggleTerm" ]]
    local toggleterm_exists = false
    for _, buf in ipairs(buffers) do
        local buf_name = vim.api.nvim_buf_get_name(buf)
        if buf_name:find("toggleterm#") then
            toggleterm_exists = true
            break
        end
    end

    if not toggleterm_exists then
        vim.cmd([[ exe 1 . "ToggleTerm" ]])
    end
end

vim.api.nvim_create_autocmd({ "VimEnter" }, { callback = open_nvim_term })

return {
    {
        "akinsho/toggleterm.nvim",
        cmd = "ToggleTerm",
        build = ":ToggleTerm",
        keys = { { "<F4>", "<cmd>ToggleTerm<cr>", desc = "Toggle floating terminal" } },
        opts = {
            open_mapping = [[<F4>]],
            direction = "horizontal",
            shade_filetypes = {},
            hide_numbers = true,
            insert_mappings = true,
            terminal_mappings = true,
            start_in_insert = true,
            close_on_exit = true,
        },
    },
}
