package store

import (
	"context"
	"encoding/csv"
	"fmt"
	"log"
	"os"
)

func (s *TokenisationStore) DebugPrintStore(ctx context.Context) {
	// Get table names
	rows, err := s.DB.Query(`SELECT name FROM sqlite_master WHERE type='table' AND name NOT LIKE 'sqlite_%';`)
	if err != nil {
		log.Fatal(err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			log.Fatal(err)
		}

		tables = append(tables, tableName)
	}

	for _, table := range tables {
		fmt.Printf("### TABLE: %s ###\n", table)

		// Query all data
		dataRows, err := s.DB.Query("SELECT * FROM " + table)
		if err != nil {
			log.Printf("Failed to query table %s: %v\n", table, err)
			continue
		}

		// Get column names
		columns, err := dataRows.Columns()
		if err != nil {
			log.Fatal(err)
		}

		// Write CSV to stdout
		writer := csv.NewWriter(os.Stdout)
		writer.Write(columns) // header

		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		for dataRows.Next() {
			for i := range columns {
				valuePtrs[i] = &values[i]
			}

			dataRows.Scan(valuePtrs...)

			record := make([]string, len(columns))
			for i, val := range values {
				if val == nil {
					record[i] = ""
				} else {
					record[i] = fmt.Sprintf("%v", val)
				}
			}
			writer.Write(record)
		}

		writer.Flush()
		fmt.Println()
	}
}
