CREATE INDEX IF NOT EXISTS token_balances_mint_hash_quantity_idx
    ON token_balances (mint_hash, quantity);
