ALTER TABLE mints ADD COLUMN burnable BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE mints ADD COLUMN burn_authority TEXT NOT NULL DEFAULT 'owner_only';
ALTER TABLE unconfirmed_mints ADD COLUMN burnable BOOLEAN NOT NULL DEFAULT FALSE;
ALTER TABLE unconfirmed_mints ADD COLUMN burn_authority TEXT NOT NULL DEFAULT 'owner_only';

CREATE TABLE IF NOT EXISTS unconfirmed_token_burns (
    id TEXT PRIMARY KEY,
    hash TEXT NOT NULL UNIQUE,
    mint_hash TEXT NOT NULL,
    burn_quantity INTEGER NOT NULL,
    burner_address TEXT NOT NULL,
    public_key TEXT NOT NULL,
    signature TEXT NOT NULL,
    created_at TIMESTAMP NOT NULL DEFAULT CURRENT_TIMESTAMP
);
