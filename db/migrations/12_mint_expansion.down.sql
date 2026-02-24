DROP TABLE IF EXISTS unconfirmed_mint_expansions;

ALTER TABLE unconfirmed_mints DROP COLUMN allow_expansion;
ALTER TABLE mints DROP COLUMN allow_expansion;
