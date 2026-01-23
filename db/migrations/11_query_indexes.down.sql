DROP INDEX IF EXISTS mints_hash_idx;
DROP INDEX IF EXISTS mints_public_key_idx;
DROP INDEX IF EXISTS mints_owner_address_idx;

DROP INDEX IF EXISTS unconfirmed_mints_public_key_idx;
DROP INDEX IF EXISTS unconfirmed_mints_owner_address_idx;

DROP INDEX IF EXISTS sell_offers_mint_hash_idx;
DROP INDEX IF EXISTS sell_offers_mint_hash_offerer_idx;

DROP INDEX IF EXISTS buy_offers_mint_hash_idx;
DROP INDEX IF EXISTS buy_offers_mint_hash_seller_idx;

DROP INDEX IF EXISTS invoices_mint_buyer_idx;
DROP INDEX IF EXISTS invoices_mint_seller_idx;
DROP INDEX IF EXISTS invoices_buyer_idx;
DROP INDEX IF EXISTS invoices_seller_idx;

DROP INDEX IF EXISTS unconfirmed_invoices_mint_buyer_idx;
