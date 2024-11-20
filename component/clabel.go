package component

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"log/slog"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
)

func NewClabelComponent(config *config.Config) (*ClabelComponent, error) {
	cc := &ClabelComponent{}
	cc.cs = database.NewClabelStore()
	cc.rs = database.NewRepoStore()
	rc, _ := NewRepoComponent(config)
	cc.rc = rc
	return cc, nil
}

type ClabelComponent struct {
	cs *database.ClabelStore
	rs *database.RepoStore
	rc *RepoComponent
}

func (cc *ClabelComponent) Upsert(ctx *gin.Context, req *types.UpsertOneClabelReq) error {
	slog.Debug("upsert clabel get request", slog.String("namespace", req.Namespace), slog.String("filePath", req.Path))

	repo, err := cc.rs.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return fmt.Errorf("failed to find repo, error: %w", err)
	}
	if repo.RepositoryType != types.DatasetRepo {
		return fmt.Errorf("invalid repo type: %s", req.RepoType)
	}

	permission, err := cc.rc.getUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanWrite {
		return ErrUnauthorized
	}

	clabel := &database.Clabel{
		RepositoryID:   repo.ID,
		Repository:     repo,
		Path:           req.Path,
		Label:          req.Label,
		AnnotationPath: req.AnnotationPath,
		Ref:            req.Ref,
	}

	err = cc.cs.CreateOrUpdate(ctx, *clabel)
	if err != nil {
		return err
	}
	return nil
}

func (cc *ClabelComponent) ClabelInfo(ctx *gin.Context, req *types.GetClabelReq) (*types.Clabel, error) {
	repo, err := cc.rs.FindByPath(ctx, req.RepoType, req.Namespace, req.Name)
	if err != nil {
		return nil, fmt.Errorf("failed to find repo, error: %w", err)
	}

	permission, err := cc.rc.getUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return nil, fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanRead {
		return nil, ErrUnauthorized
	}

	if req.Ref == "" {
		req.Ref = repo.DefaultBranch
	}

	clabel, err := cc.cs.FindByPath(ctx, repo.ID, req.Path, req.Ref)
	if err != nil {
		if err.Error() == "sql: no rows in result set" {
			return nil, nil
		}
		return nil, fmt.Errorf("failed to find clabel, error: %w", err)
	}
	return &types.Clabel{
		Path:           clabel.Path,
		Ref:            clabel.Ref,
		Label:          clabel.Label,
		AnnotationPath: clabel.AnnotationPath,
	}, nil
}
