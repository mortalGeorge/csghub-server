package component

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"log/slog"
	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"path/filepath"
)

func NewClabelComponent(config *config.Config) (*ClabelComponent, error) {
	cc := &ClabelComponent{}
	cc.cs = database.NewClabelStore()
	cc.rs = database.NewRepoStore()
	rc, _ := NewRepoComponent(config)
	cc.repo = rc
	var err error
	cc.git, err = git.NewGitServer(config)
	if err != nil {
		newError := fmt.Errorf("fail to create git server,error:%w", err)
		slog.Error(newError.Error())
		return nil, newError
	}
	return cc, nil
}

type ClabelComponent struct {
	cs   *database.ClabelStore
	rs   *database.RepoStore
	repo *RepoComponent
	git  gitserver.GitServer
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

	permission, err := cc.repo.getUserRepoPermission(ctx, req.CurrentUser, repo)
	if err != nil {
		return fmt.Errorf("failed to get user repo permission, error: %w", err)
	}
	if !permission.CanWrite {
		return ErrUnauthorized
	}

	getFileRawReq := gitserver.GetRepoInfoByPathReq{
		Namespace: req.Namespace,
		Name:      req.Name,
		Ref:       req.Ref,
		Path:      req.Path,
		RepoType:  req.RepoType,
	}
	_, err = cc.git.GetRepoFileRaw(ctx, getFileRawReq)
	if err != nil {
		return fmt.Errorf("failed to get repo file, error: %w", err)
	}

	clabel := &database.Clabel{
		RepositoryID:   repo.ID,
		Repository:     repo,
		Path:           req.Path,
		Label:          req.Label,
		AnnotationPath: req.AnnotationPath,
		Ref:            req.Ref,
		RepoNamespace:  req.Namespace,
		RepoName:       req.Name,
		FileName:       filepath.Base(req.Path),
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

	permission, err := cc.repo.getUserRepoPermission(ctx, req.CurrentUser, repo)
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

func (cc *ClabelComponent) IndexFile(ctx *gin.Context, filter *types.CmccFilesFilter, per, page int) (files []types.Clabel, total int, err error) {
	repoFilter := &types.RepoFilter{
		Tags:     filter.Tags,
		Sort:     filter.Sort,
		Search:   filter.Search,
		Source:   filter.Source,
		Username: filter.Username,
	}
	repos, _, err := cc.repo.PublicToUser(ctx, types.DatasetRepo, filter.Username, repoFilter, 0, 1)
	if err != nil {
		newError := fmt.Errorf("failed to get public dataset repos,error:%w", err)
		return nil, 0, newError
	}
	var repoIDs []int64
	for _, repo := range repos {
		repoIDs = append(repoIDs, repo.ID)
	}

	clabes, total, err := cc.cs.PublicToUser(ctx, repoIDs, filter, per, page)

	for _, clabe := range clabes {
		files = append(files, types.Clabel{
			RepositoryID:   clabe.RepositoryID,
			Path:           clabe.Path,
			Ref:            clabe.Ref,
			Label:          clabe.Label,
			AnnotationPath: clabe.AnnotationPath,
			RepoNamespace:  clabe.RepoNamespace,
			RepoName:       clabe.RepoName,
			FileName:       clabe.FileName,
		})
	}
	return
}
