return {
    {
        "neovim/nvim-lspconfig",
        opts = {
            servers = {
                ruff = {
                    mason = false,
                },
                ruff_lsp = {
                    mason = false,
                },
            },
        },
    },
}
