package database

import (
	"context"
	"fmt"
)

type ClabelStore struct {
	db *DB
}

func NewClabelStore() *ClabelStore {
	return &ClabelStore{
		db: defaultDB,
	}
}

type Clabel struct {
	ID             int64       `bun:",pk,autoincrement" json:"id"`
	RepositoryID   int64       `bun:",notnull" json:"repository_id"`
	Repository     *Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	Path           string      `bun:",notnull" json:"path"`
	Ref            string      `bun:",notnull" json:"ref"`
	Label          string      `bun:",notnull" json:"label"`
	AnnotationPath string      `json:"annotation_path"`
	times
}

func (s *ClabelStore) FindByPath(ctx context.Context, repoID int64, path string, ref string) (Clabel, error) {
	var clabel Clabel
	err := s.db.Operator.Core.NewSelect().
		Model(&clabel).
		Where("repository_id = ? and path = ? and ref = ?", repoID, path, ref).
		Scan(ctx)
	if err != nil {
		return clabel, err
	}
	return clabel, nil
}

func (s *ClabelStore) BatchCreate(ctx context.Context, clabels []Clabel) error {
	result, err := s.db.Operator.Core.NewInsert().
		Model(&clabels).
		Exec(ctx)
	if err != nil {
		return err
	}

	return assertAffectedXRows(int64(len(clabels)), result, err)
}

func (s *ClabelStore) Create(ctx context.Context, clabel Clabel) error {
	_, err := s.db.Operator.Core.NewInsert().Model(&clabel).Exec(ctx)
	if err != nil {
		return err
	}
	return nil
}

func (s *ClabelStore) CreateOrUpdate(ctx context.Context, clabel Clabel) error {
	oldClabel, err := s.FindByPath(ctx, clabel.RepositoryID, clabel.Path, clabel.Ref)

	if err == nil {
		_, err = s.db.Operator.Core.NewUpdate().
			Model(&clabel).
			Where("id = ?", oldClabel.ID).
			Exec(ctx)
		if err != nil {
			return fmt.Errorf("update clabel in db failed, error: %w", err)
		}
		return nil
	}

	_, err = s.db.Operator.Core.NewInsert().Model(&clabel).Exec(ctx)
	if err != nil {
		return fmt.Errorf("insert clabel in db failed, error: %w", err)
	}

	return nil
}
