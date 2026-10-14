package sql

import (
	"context"
	"database/sql"
	"strings"
)

// PhotoRows pages the existing file index; it never stores media in a new DB.
// substr provides literal prefix matching even for folders containing * or %.
func (db *IndexDB) PhotoRows(ctx context.Context, source, prefix, query, kind string, before int64, after string, limit int, favorites []string, onlyFavorites bool) (*sql.Rows, error) {
	statement := `SELECT path,name,size,created_time,mod_time,type,is_dir,has_preview FROM index_items WHERE source=? AND is_dir=0 AND substr(path,1,length(?))=? AND (type LIKE 'image/%' OR type LIKE 'video/%' OR lower(name) GLOB '*.heic' OR lower(name) GLOB '*.heif')`
	args := []any{source, prefix, prefix}
	if kind == "image" {
		statement += ` AND (type LIKE 'image/%' OR lower(name) GLOB '*.heic' OR lower(name) GLOB '*.heif')`
	}
	if kind == "video" {
		statement += ` AND type LIKE 'video/%'`
	}
	if query != "" {
		statement += ` AND instr(lower(name),lower(?))>0`
		args = append(args, query)
	}
	if after != "" {
		statement += ` AND (mod_time<? OR (mod_time=? AND path>?))`
		args = append(args, before, before, after)
	}
	if onlyFavorites {
		if len(favorites) == 0 {
			statement += ` AND 0=1`
		} else {
			statement += ` AND path IN (` + strings.TrimSuffix(strings.Repeat("?,", len(favorites)), ",") + `)`
			for _, p := range favorites {
				args = append(args, p)
			}
		}
	}
	statement += ` ORDER BY mod_time DESC,path ASC LIMIT ?`
	args = append(args, limit)
	return db.QueryContext(ctx, statement, args...)
}
