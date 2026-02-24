ALTER TABLE mints DROP COLUMN burnable;
ALTER TABLE mints DROP COLUMN burn_authority;
ALTER TABLE unconfirmed_mints DROP COLUMN burnable;
ALTER TABLE unconfirmed_mints DROP COLUMN burn_authority;
DROP TABLE IF EXISTS unconfirmed_token_burns;
