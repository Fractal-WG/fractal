ALTER TABLE mints ADD COLUMN allow_expansion BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE unconfirmed_mints ADD COLUMN allow_expansion BOOLEAN NOT NULL DEFAULT FALSE;

CREATE TABLE IF NOT EXISTS unconfirmed_mint_expansions (
    id TEXT PRIMARY KEY,
    hash TEXT NOT NULL UNIQUE,
    mint_hash TEXT NOT NULL,
    additional_supply INTEGER NOT NULL,
    owner_address TEXT NOT NULL,
    public_key TEXT NOT NULL,
    signature TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
