ALTER TABLE mints ADD COLUMN current_supply INTEGER NOT NULL DEFAULT 0;
ALTER TABLE unconfirmed_mints ADD COLUMN current_supply INTEGER NOT NULL DEFAULT 0;

UPDATE mints SET current_supply = fraction_count WHERE current_supply = 0;
UPDATE unconfirmed_mints SET current_supply = fraction_count WHERE current_supply = 0;
