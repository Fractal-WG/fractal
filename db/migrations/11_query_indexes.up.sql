CREATE INDEX IF NOT EXISTS mints_hash_idx
    ON mints (hash);
CREATE INDEX IF NOT EXISTS mints_public_key_idx
    ON mints (public_key);
CREATE INDEX IF NOT EXISTS mints_owner_address_idx
    ON mints (owner_address);

CREATE INDEX IF NOT EXISTS unconfirmed_mints_public_key_idx
    ON unconfirmed_mints (public_key);
CREATE INDEX IF NOT EXISTS unconfirmed_mints_owner_address_idx
    ON unconfirmed_mints (owner_address);

CREATE INDEX IF NOT EXISTS sell_offers_mint_hash_idx
    ON sell_offers (mint_hash);
CREATE INDEX IF NOT EXISTS sell_offers_mint_hash_offerer_idx
    ON sell_offers (mint_hash, offerer_address);

CREATE INDEX IF NOT EXISTS buy_offers_mint_hash_idx
    ON buy_offers (mint_hash);
CREATE INDEX IF NOT EXISTS buy_offers_mint_hash_seller_idx
    ON buy_offers (mint_hash, seller_address);

CREATE INDEX IF NOT EXISTS invoices_mint_buyer_idx
    ON invoices (mint_hash, buyer_address);
CREATE INDEX IF NOT EXISTS invoices_mint_seller_idx
    ON invoices (mint_hash, seller_address);
CREATE INDEX IF NOT EXISTS invoices_buyer_idx
    ON invoices (buyer_address);
CREATE INDEX IF NOT EXISTS invoices_seller_idx
    ON invoices (seller_address);

CREATE INDEX IF NOT EXISTS unconfirmed_invoices_mint_buyer_idx
    ON unconfirmed_invoices (mint_hash, buyer_address);
