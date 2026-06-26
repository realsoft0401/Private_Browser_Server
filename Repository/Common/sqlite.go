package Common

import (
	"database/sql"

	SQLiteInfra "private_browser_server/Infrastructures/SQLite"
)

func DB() *sql.DB {
	return SQLiteInfra.DB()
}
