package store

import "database/sql"

func approximateTableCountPostgres(db *sql.DB, table string) (int, error) {
	var count sql.NullInt64
	err := db.QueryRow(`
		SELECT COALESCE(reltuples::bigint, 0)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE c.relname = $1 AND n.nspname = current_schema()
	`, table).Scan(&count)
	if err == sql.ErrNoRows {
		return 0, nil
	}
	if err != nil {
		return 0, err
	}
	if !count.Valid {
		return 0, nil
	}
	return int(count.Int64), nil
}
