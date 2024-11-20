package migrations

import (
	"context"
	"fmt"
	"github.com/uptrace/bun"
	"opencsg.com/csghub-server/builder/store/database"
)

func init() {
	Migrations.MustRegister(func(ctx context.Context, db *bun.DB) error {
		err := createTables(ctx, db, Clabel{})
		if err != nil {
			return err
		}
		_, err = db.NewCreateIndex().
			Model((*Clabel)(nil)).
			Index("idx_files_id").
			Column("repository_id", "path", "ref").
			Exec(ctx)
		fmt.Print(" [up migration] create table clabel")
		return nil
	}, func(ctx context.Context, db *bun.DB) error {
		err := dropTables(ctx, db, Clabel{})
		if err != nil {
			return err
		}
		return nil
	})

}

type Clabel struct {
	ID             int64                `bun:",pk,autoincrement" json:"id"`
	RepositoryID   int64                `bun:",notnull" json:"repository_id"`
	Repository     *database.Repository `bun:"rel:belongs-to,join:repository_id=id" json:"repository"`
	Path           string               `bun:",notnull" json:"path"`
	Ref            string               `bun:",notnull" json:"ref"`
	Label          string               `bun:",notnull" json:"label"`
	AnnotationPath string               `json:"annotation_path"`
	times
}
