package indexing

import (
	"context"
	dbsql "github.com/gtsteffaniak/filebrowser/backend/internal/database/sql"
)

func (idx *Index) Photos(ctx context.Context, prefix, query, kind string, before int64, after string, limit int, favorites []string, onlyFavorites bool) ([]dbsql.SearchItem, error) {
	rows, err := idx.db.PhotoRows(ctx, idx.Name, prefix, query, kind, before, after, limit, favorites, onlyFavorites)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []dbsql.SearchItem{}
	for rows.Next() {
		var v dbsql.SearchItem
		v.Source = idx.Name
		if err = rows.Scan(&v.Path, &v.Name, &v.Size, &v.CreatedTime, &v.ModTime, &v.Type, &v.IsDir, &v.HasPreview); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
