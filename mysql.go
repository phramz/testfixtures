package testfixtures

import (
	"database/sql"
	"fmt"
)

// MySQL is the MySQL helper for this package
type MySQL struct {
	baseHelper
	tableChecksums map[string]int64
}

func (*MySQL) paramType() int {
	return paramTypeQuestion
}

func (*MySQL) quoteKeyword(str string) string {
	return fmt.Sprintf("`%s`", str)
}

func (*MySQL) databaseName(q queryable) (dbName string) {
	q.QueryRow("SELECT DATABASE()").Scan(&dbName)
	return
}

func (h *MySQL) tableNames(q queryable) ([]string, error) {
	query := `
		SELECT table_name
		FROM information_schema.tables
		WHERE table_schema=?;
	`
	rows, err := q.Query(query, h.databaseName(q))
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var table string
		if err = rows.Scan(&table); err != nil {
			return nil, err
		}
		tables = append(tables, table)
	}
	if err = rows.Err(); err != nil {
		return nil, err
	}
	return tables, nil

}

func (h *MySQL) disableReferentialIntegrity(db *sql.DB, loadFn loadFunction) (err error) {
	// re-enable after load
	defer func() {
		if _, err2 := db.Exec("SET FOREIGN_KEY_CHECKS = 1"); err2 != nil && err == nil {
			err = err2
		}
	}()

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err = tx.Exec("SET FOREIGN_KEY_CHECKS = 0"); err != nil {
		return err
	}

	if err = loadFn(tx); err != nil {
		return err
	}

	return tx.Commit()
}

func (h *MySQL) isTableModified(q queryable, tableName string) (bool, error) {
	checksum, err := h.getChecksum(q, tableName)
	if err != nil {
		return true, err
	}
	previousChecksum, ok := h.tableChecksums[tableName]
	return !ok || checksum != previousChecksum, nil
}

func (h *MySQL) tablesLoaded(q queryable) error {
	if h.tableChecksums != nil {
		return nil
	}
	tableNames, err := h.tableNames(q)
	if err != nil {
		return err
	}
	h.tableChecksums = make(map[string]int64, len(tableNames))
	for _, tableName := range tableNames {
		h.tableChecksums[tableName], err = h.getChecksum(q, tableName)
		if err != nil {
			return err
		}
	}
	return nil
}

func (h *MySQL) getChecksum(q queryable, tableName string) (int64, error) {
	row := q.QueryRow("CHECKSUM TABLE " + h.quoteKeyword(tableName))
	var table string
	var checksum int64
	err := row.Scan(&table, &checksum)
	return checksum, err
}
