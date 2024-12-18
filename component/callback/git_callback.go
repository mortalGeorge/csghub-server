package callback

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"opencsg.com/csghub-server/builder/git"
	"opencsg.com/csghub-server/builder/git/gitserver"
	"opencsg.com/csghub-server/builder/rpc"
	"opencsg.com/csghub-server/builder/store/database"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/component"
)

// define GitCallbackComponent struct
type GitCallbackComponent struct {
	config       *config.Config
	gs           gitserver.GitServer
	tc           component.TagComponent
	modSvcClient rpc.ModerationSvcClient
	ms           database.ModelStore
	ds           database.DatasetStore
	sc           component.SpaceComponent
	ss           database.SpaceStore
	rs           database.RepoStore
	rrs          database.RepoRelationsStore
	mirrorStore  database.MirrorStore
	rrf          *database.RepositoriesRuntimeFrameworkStore
	rac          component.RuntimeArchitectureComponent
	ras          database.RuntimeArchitecturesStore
	rfs          database.RuntimeFrameworksStore
	ts           database.TagStore
	// set visibility if file content is sensitive
	setRepoVisibility bool
	pp                component.PromptComponent
	maxPromptFS       int64
}

// new CallbackComponent
func NewGitCallback(config *config.Config) (*GitCallbackComponent, error) {
	gs, err := git.NewGitServer(config)
	if err != nil {
		return nil, err
	}
	tc, err := component.NewTagComponent(config)
	if err != nil {
		return nil, err
	}
	ms := database.NewModelStore()
	ds := database.NewDatasetStore()
	ss := database.NewSpaceStore()
	rs := database.NewRepoStore()
	rrs := database.NewRepoRelationsStore()
	mirrorStore := database.NewMirrorStore()
	sc, err := component.NewSpaceComponent(config)
	ras := database.NewRuntimeArchitecturesStore()
	if err != nil {
		return nil, err
	}
	rrf := database.NewRepositoriesRuntimeFramework()
	rac, err := component.NewRuntimeArchitectureComponent(config)
	if err != nil {
		return nil, err
	}
	rfs := database.NewRuntimeFrameworksStore()
	ts := database.NewTagStore()
	pp, err := component.NewPromptComponent(config)
	if err != nil {
		return nil, err
	}
	var modSvcClient rpc.ModerationSvcClient
	if config.SensitiveCheck.Enable {
		modSvcClient = rpc.NewModerationSvcHttpClient(fmt.Sprintf("%s:%d", config.Moderation.Host, config.Moderation.Port))
	}
	return &GitCallbackComponent{
		config:       config,
		gs:           gs,
		tc:           tc,
		ms:           ms,
		ds:           ds,
		ss:           ss,
		sc:           sc,
		rs:           rs,
		rrs:          rrs,
		mirrorStore:  mirrorStore,
		modSvcClient: modSvcClient,
		rrf:          rrf,
		rac:          rac,
		ras:          ras,
		rfs:          rfs,
		pp:           pp,
		ts:           ts,
		maxPromptFS:  config.Dataset.PromptMaxJsonlFileSize,
	}, nil
}

// SetRepoVisibility sets a flag whether change repo's visibility if file content is sensitive
func (c *GitCallbackComponent) SetRepoVisibility(yes bool) {
	c.setRepoVisibility = yes
}

func (c *GitCallbackComponent) WatchSpaceChange(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	err := WatchSpaceChange(req, c.ss, c.sc).Run()
	if err != nil {
		slog.Error("watch space change failed", slog.Any("error", err))
		return err
	}
	return nil
}

func (c *GitCallbackComponent) WatchRepoRelation(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	err := WatchRepoRelation(req, c.rs, c.rrs, c.gs).Run()
	if err != nil {
		slog.Error("watch repo relation failed", slog.Any("error", err))
		return err
	}
	return nil
}

func (c *GitCallbackComponent) SetRepoUpdateTime(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	// split req.Repository.FullName by '/'
	splits := strings.Split(req.Repository.FullName, "/")
	fullNamespace, repoName := splits[0], splits[1]
	repoType, namespace, _ := strings.Cut(fullNamespace, "_")
	adjustedRepoType := types.RepositoryType(strings.TrimRight(repoType, "s"))
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	isMirrorRepo, err := c.rs.IsMirrorRepo(ctx, adjustedRepoType, namespace, repoName)
	if err != nil {
		slog.Error("failed to check if a mirror repo", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
		return err
	}
	if isMirrorRepo {
		updated, err := time.Parse(time.RFC3339, req.HeadCommit.Timestamp)
		if err != nil {
			slog.Error("Error parsing time:", slog.Any("error", err), slog.String("timestamp", req.HeadCommit.Timestamp))
			return err
		}
		err = c.rs.SetUpdateTimeByPath(ctx, adjustedRepoType, namespace, repoName, updated)
		if err != nil {
			slog.Error("failed to set repo update time", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
			return err
		}
		mirror, err := c.mirrorStore.FindByRepoPath(ctx, adjustedRepoType, namespace, repoName)
		if err != nil {
			slog.Error("failed to find repo mirror", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
			return err
		}
		mirror.LastUpdatedAt = time.Now()
		err = c.mirrorStore.Update(ctx, mirror)
		if err != nil {
			slog.Error("failed to update repo mirror last_updated_at", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
			return err
		}
	} else {
		err := c.rs.SetUpdateTimeByPath(ctx, adjustedRepoType, namespace, repoName, time.Now())
		if err != nil {
			slog.Error("failed to set repo update time", slog.Any("error", err), slog.String("repo_type", string(adjustedRepoType)), slog.String("namespace", namespace), slog.String("name", repoName))
			return err
		}
	}
	return nil
}

func (c *GitCallbackComponent) UpdateRepoInfos(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	commits := req.Commits
	ref := req.Ref
	// split req.Repository.FullName by '/'
	splits := strings.Split(req.Repository.FullName, "/")
	fullNamespace, repoName := splits[0], splits[1]
	repoType, namespace, _ := strings.Cut(fullNamespace, "_")

	var err error
	for _, commit := range commits {
		err = errors.Join(err, c.modifyFiles(ctx, repoType, namespace, repoName, ref, commit.Modified))
		err = errors.Join(err, c.removeFiles(ctx, repoType, namespace, repoName, ref, commit.Removed))
		err = errors.Join(err, c.addFiles(ctx, repoType, namespace, repoName, ref, commit.Added))
	}

	return err
}

func (c *GitCallbackComponent) SensitiveCheck(ctx context.Context, req *types.GiteaCallbackPushReq) error {
	// split req.Repository.FullName by '/'
	splits := strings.Split(req.Repository.FullName, "/")
	fullNamespace, repoName := splits[0], splits[1]
	repoType, namespace, _ := strings.Cut(fullNamespace, "_")
	adjustedRepoType := types.RepositoryType(strings.TrimRight(repoType, "s"))

	var err error
	if c.modSvcClient != nil {
		err = c.modSvcClient.SubmitRepoCheck(ctx, adjustedRepoType, namespace, repoName)
	}
	if err != nil {
		slog.Error("fail to submit repo sensitive check", slog.Any("error", err), slog.Any("repo_type", adjustedRepoType), slog.String("namespace", namespace), slog.String("name", repoName))
		return err
	}
	return nil
}

// modifyFiles method handles modified files, skip if not modify README.md
func (c *GitCallbackComponent) modifyFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	for _, fileName := range fileNames {
		slog.Debug("modify file", slog.String("file", fileName))
		// update model runtime
		c.updateModelRuntimeFrameworks(ctx, repoType, namespace, repoName, ref, fileName, false)
		// only care about readme file under root directory
		if fileName != types.ReadmeFileName {
			continue
		}

		content, err := c.getFileRaw(repoType, namespace, repoName, ref, fileName)
		if err != nil {
			return err
		}
		// should be only one README.md
		return c.updateMetaTags(ctx, repoType, namespace, repoName, ref, content)
	}
	return nil
}

func (c *GitCallbackComponent) removeFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	// handle removed files
	// delete tags
	for _, fileName := range fileNames {
		slog.Debug("remove file", slog.String("file", fileName))
		// update model runtime
		c.updateModelRuntimeFrameworks(ctx, repoType, namespace, repoName, ref, fileName, true)
		// only care about readme file under root directory
		if fileName == types.ReadmeFileName {
			// use empty content to clear all the meta tags
			const content string = ""
			adjustedRepoType := types.RepositoryType(strings.TrimSuffix(repoType, "s"))
			err := c.tc.ClearMetaTags(ctx, adjustedRepoType, namespace, repoName)
			if err != nil {
				slog.Error("failed to clear meta tags", slog.String("content", content),
					slog.String("repo", path.Join(namespace, repoName)), slog.String("ref", ref),
					slog.Any("error", err))
				return fmt.Errorf("failed to clear met tags,cause: %w", err)
			}
		} else {
			var tagScope database.TagScope
			switch repoType {
			case fmt.Sprintf("%ss", types.DatasetRepo):
				tagScope = database.DatasetTagScope
			case fmt.Sprintf("%ss", types.ModelRepo):
				tagScope = database.ModelTagScope
			case fmt.Sprintf("%ss", types.PromptRepo):
				tagScope = database.PromptTagScope
			default:
				return nil
				// case CodeRepoType:
				// 	tagScope = database.CodeTagScope
				// case SpaceRepoType:
				// 	tagScope = database.SpaceTagScope
			}
			err := c.tc.UpdateLibraryTags(ctx, tagScope, namespace, repoName, fileName, "")
			if err != nil {
				slog.Error("failed to remove Library tag", slog.String("namespace", namespace),
					slog.String("name", repoName), slog.String("ref", ref), slog.String("fileName", fileName),
					slog.Any("error", err))
				return fmt.Errorf("failed to remove Library tag, cause: %w", err)
			}
		}
	}
	return nil
}

func (c *GitCallbackComponent) addFiles(ctx context.Context, repoType, namespace, repoName, ref string, fileNames []string) error {
	for _, fileName := range fileNames {
		slog.Debug("add file", slog.String("file", fileName))
		// update model runtime
		c.updateModelRuntimeFrameworks(ctx, repoType, namespace, repoName, ref, fileName, false)
		// only care about readme file under root directory
		if fileName == types.ReadmeFileName {
			content, err := c.getFileRaw(repoType, namespace, repoName, ref, fileName)
			if err != nil {
				return err
			}
			err = c.updateMetaTags(ctx, repoType, namespace, repoName, ref, content)
			if err != nil {
				return err
			}
		} else {
			var tagScope database.TagScope
			switch repoType {
			case fmt.Sprintf("%ss", types.DatasetRepo):
				tagScope = database.DatasetTagScope
			case fmt.Sprintf("%ss", types.ModelRepo):
				tagScope = database.ModelTagScope
			case fmt.Sprintf("%ss", types.PromptRepo):
				tagScope = database.PromptTagScope
			default:
				return nil
				// case CodeRepoType:
				// 	tagScope = database.CodeTagScope
				// case SpaceRepoType:
				// 	tagScope = database.SpaceTagScope
			}
			err := c.tc.UpdateLibraryTags(ctx, tagScope, namespace, repoName, "", fileName)
			if err != nil {
				slog.Error("failed to add Library tag", slog.String("namespace", namespace),
					slog.String("name", repoName), slog.String("ref", ref), slog.String("fileName", fileName),
					slog.Any("error", err))
				return fmt.Errorf("failed to add Library tag, cause: %w", err)
			}
		}
	}
	return nil
}

func (c *GitCallbackComponent) updateMetaTags(ctx context.Context, repoType, namespace, repoName, ref, content string) error {
	var (
		err      error
		tagScope database.TagScope
	)
	switch repoType {
	case fmt.Sprintf("%ss", types.DatasetRepo):
		tagScope = database.DatasetTagScope
	case fmt.Sprintf("%ss", types.ModelRepo):
		tagScope = database.ModelTagScope
	case fmt.Sprintf("%ss", types.PromptRepo):
		tagScope = database.PromptTagScope
	default:
		return nil
		// TODO: support code and space
		// case CodeRepoType:
		// 	tagScope = database.CodeTagScope
		// case SpaceRepoType:
		// 	tagScope = database.SpaceTagScope
	}
	_, err = c.tc.UpdateMetaTags(ctx, tagScope, namespace, repoName, content)
	if err != nil {
		slog.Error("failed to update meta tags", slog.String("namespace", namespace),
			slog.String("content", content), slog.String("repo", repoName), slog.String("ref", ref),
			slog.Any("error", err))
		return fmt.Errorf("failed to update met tags,cause: %w", err)
	}
	slog.Info("update meta tags success", slog.String("repo", path.Join(namespace, repoName)), slog.String("type", repoType))
	return nil
}

func (c *GitCallbackComponent) getFileRaw(repoType, namespace, repoName, ref, fileName string) (string, error) {
	var (
		content string
		err     error
	)
	repoType = strings.TrimRight(repoType, "s")
	getFileRawReq := gitserver.GetRepoInfoByPathReq{
		Namespace: namespace,
		Name:      repoName,
		Ref:       ref,
		Path:      fileName,
		RepoType:  types.RepositoryType(repoType),
	}
	content, err = c.gs.GetRepoFileRaw(context.Background(), getFileRawReq)
	if err != nil {
		slog.Error("failed to get file content", slog.String("namespace", namespace),
			slog.String("file", fileName), slog.String("repo", repoName), slog.String("ref", ref),
			slog.Any("error", err))
		return "", fmt.Errorf("failed to get file content,cause: %w", err)
	}
	slog.Debug("get file content success", slog.String("repoType", repoType), slog.String("namespace", namespace),
		slog.String("file", fileName), slog.String("repo", repoName), slog.String("ref", ref))

	return content, nil
}

func (c *GitCallbackComponent) updateModelRuntimeFrameworks(ctx context.Context, repoType, namespace, repoName, ref, fileName string, deleteAction bool) {
	slog.Debug("update model relation for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("repoType", repoType), slog.Any("fileName", fileName), slog.Any("branch", ref))
	// must be model repo and config.json
	if repoType != fmt.Sprintf("%ss", types.ModelRepo) || fileName != component.ConfigFileName || ref != ("refs/heads/"+component.MainBranch) {
		return
	}
	repo, err := c.rs.FindByPath(ctx, types.ModelRepo, namespace, repoName)
	if err != nil || repo == nil {
		slog.Warn("fail to query repo for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("error", err))
		return
	}
	// delete event
	if deleteAction {
		err := c.rrf.DeleteByRepoID(ctx, repo.ID)
		if err != nil {
			slog.Warn("fail to remove repo runtimes for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("repoid", repo.ID), slog.Any("error", err))
		}
		return
	}
	arch, err := c.rac.GetArchitectureFromConfig(ctx, namespace, repoName)
	if err != nil {
		slog.Warn("fail to get config.json content for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("error", err))
		return
	}
	slog.Debug("get arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("arch", arch))
	//add resource tag, like ascend
	runtime_framework_tags, _ := c.ts.GetTagsByScopeAndCategories(ctx, "model", []string{"runtime_framework", "resource"})
	fields := strings.Split(repo.Path, "/")
	c.rac.AddResourceTag(ctx, runtime_framework_tags, fields[1], repo.ID)
	runtimes, err := c.ras.ListByRArchNameAndModel(ctx, arch, fields[1])
	// to do check resource models
	if err != nil {
		slog.Warn("fail to get runtime ids by arch for git callback", slog.Any("arch", arch), slog.Any("error", err))
		return
	}
	slog.Debug("get runtimes by arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("arch", arch), slog.Any("runtimes", runtimes))
	var frameIDs []int64
	for _, runtime := range runtimes {
		frameIDs = append(frameIDs, runtime.RuntimeFrameworkID)
	}
	slog.Debug("get new frame ids for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("frameIDs", frameIDs))
	newFrames, err := c.rfs.ListByIDs(ctx, frameIDs)
	if err != nil {
		slog.Warn("fail to get runtime frameworks for git callback", slog.Any("arch", arch), slog.Any("error", err))
		return
	}
	slog.Debug("get new frames by arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("newFrames", newFrames))
	var newFrameMap map[string]string = make(map[string]string)
	for _, frame := range newFrames {
		newFrameMap[string(frame.ID)] = string(frame.ID)
	}
	slog.Debug("get new frame map by arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("newFrameMap", newFrameMap))
	oldRepoRuntimes, err := c.rrf.GetByRepoIDs(ctx, repo.ID)
	if err != nil {
		slog.Warn("fail to get repo runtimes for git callback", slog.Any("repo.ID", repo.ID), slog.Any("error", err))
		return
	}
	slog.Debug("get old frames by arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("oldRepoRuntimes", oldRepoRuntimes))
	var oldFrameMap map[string]string = make(map[string]string)
	// get map
	for _, runtime := range oldRepoRuntimes {
		oldFrameMap[string(runtime.RuntimeFrameworkID)] = string(runtime.RuntimeFrameworkID)
	}
	slog.Debug("get old frame map by arch for git callback", slog.Any("namespace", namespace), slog.Any("repoName", repoName), slog.Any("oldFrameMap", oldFrameMap))
	// remove incorrect relation
	for _, old := range oldRepoRuntimes {
		// check if it need remove
		_, exist := newFrameMap[string(old.RuntimeFrameworkID)]
		if !exist {
			// remove incorrect relations
			err := c.rrf.Delete(ctx, old.RuntimeFrameworkID, repo.ID, old.Type)
			if err != nil {
				slog.Warn("fail to delete old repo runtimes for git callback", slog.Any("repo.ID", repo.ID), slog.Any("runtime framework id", old.RuntimeFrameworkID), slog.Any("error", err))
			}
			// remove runtime framework tags
			c.rac.RemoveRuntimeFrameworkTag(ctx, runtime_framework_tags, repo.ID, old.RuntimeFrameworkID)
		}
	}

	// add new relation
	for _, new := range newFrames {
		// check if it need add
		_, exist := oldFrameMap[string(new.ID)]
		if !exist {
			// add new relations
			err := c.rrf.Add(ctx, new.ID, repo.ID, new.Type)
			if err != nil {
				slog.Warn("fail to add new repo runtimes for git callback", slog.Any("repo.ID", repo.ID), slog.Any("runtime framework id", new.ID), slog.Any("error", err))
			}
			// add runtime framework and resource tags
			err = c.rac.AddRuntimeFrameworkTag(ctx, runtime_framework_tags, repo.ID, new.ID)
			if err != nil {
				slog.Warn("fail to add runtime framework tag for git callback", slog.Any("repo.ID", repo.ID), slog.Any("runtime framework id", new.ID), slog.Any("error", err))
			}
		}
	}

}
