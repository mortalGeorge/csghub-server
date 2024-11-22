package handler

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"log/slog"
	"net/http"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
	"slices"
)

func NewClabelHandler(config *config.Config) (*ClabelHandler, error) {
	clabelComponent, err := component.NewClabelComponent(config)
	if err != nil {
		return nil, err
	}
	repoHandler, err := NewRepoHandler(config)
	if err != nil {
		return nil, err
	}
	datasetComponent, err := component.NewDatasetComponent(config)
	return &ClabelHandler{
		c:  clabelComponent,
		rh: repoHandler,
		dc: datasetComponent,
	}, nil
}

type ClabelHandler struct {
	c  *component.ClabelComponent
	rh *RepoHandler
	dc *component.DatasetComponent
}

func (h *ClabelHandler) UpsertClabel(ctx *gin.Context) {
	userName := httpbase.GetCurrentUser(ctx)
	if userName == "" {
		httpbase.UnauthorizedError(ctx, component.ErrUserNotFound)
		return
	}
	var req *types.UpsertOneClabelReq
	if err := ctx.ShouldBindJSON(&req); err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}

	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)
	req.Path = filePath
	req.CurrentUser = userName
	req.Namespace = namespace
	req.Name = name
	req.RepoType = common.RepoTypeFromContext(ctx)
	req.Ref = ctx.Query("ref")

	err = h.c.Upsert(ctx, req)
	if err != nil {
		slog.Error("Failed to upsert clabel", "error", err)
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, req)
}

func (h *ClabelHandler) BatchUpsert(ctx *gin.Context) {
	h.rh.handleDownload(ctx, true)
}

func (h *ClabelHandler) ClabelInfo(ctx *gin.Context) {
	namespace, name, err := common.GetNamespaceAndNameFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	currentUser := httpbase.GetCurrentUser(ctx)
	filePath := ctx.Param("file_path")
	filePath = convertFilePathFromRoute(filePath)

	req := &types.GetClabelReq{
		Path:        filePath,
		Namespace:   namespace,
		Name:        name,
		Ref:         ctx.Query("ref"),
		CurrentUser: currentUser,
		RepoType:    common.RepoTypeFromContext(ctx),
	}

	clabel, err := h.c.ClabelInfo(ctx, req)
	if err != nil {
		slog.Error("failed to get Clabel info", slog.Any("req", req), slog.Any("error", err))
		httpbase.ServerError(ctx, err)
		return
	}
	httpbase.OK(ctx, clabel)
}

func (h *ClabelHandler) IndexFile(ctx *gin.Context) {
	filter := new(types.CmccFilesFilter)
	filter.Tags = parseTagReqs(ctx)
	filter.Username = httpbase.GetCurrentUser(ctx)
	per, page, err := common.GetPerAndPageFromContext(ctx)
	if err != nil {
		slog.Error("Bad request format", "error", err)
		httpbase.BadRequest(ctx, err.Error())
		return
	}
	filter = h.getFilterFromContext(ctx, filter)
	if !slices.Contains[[]string](Sorts, filter.Sort) {
		msg := fmt.Sprintf("sort parameter must be one of %v", Sorts)
		slog.Error("Bad request format,", slog.String("error", msg))
		ctx.JSON(http.StatusBadRequest, gin.H{"message": msg})
		return
	}

	if filter.Source != "" && !slices.Contains[[]string](Sources, filter.Source) {
		msg := fmt.Sprintf("source parameter must be one of %v", Sources)
		slog.Error("Bad request format,", slog.String("error", msg))
		ctx.JSON(http.StatusBadRequest, gin.H{"message": msg})
		return
	}

	files, total, err := h.c.IndexFile(ctx, filter, per, page)
	if err != nil {
		httpbase.ServerError(ctx, err)
		return
	}
	respData := gin.H{
		"data":  files,
		"total": total,
	}
	ctx.JSON(http.StatusOK, respData)
}

func (h *ClabelHandler) getFilterFromContext(ctx *gin.Context, filter *types.CmccFilesFilter) *types.CmccFilesFilter {
	filter.Search = ctx.Query("search")
	filter.Sort = ctx.Query("sort")
	if filter.Sort == "" {
		filter.Sort = "recently_update"
	}
	filter.Source = ctx.Query("source")
	filter.FileSearch = ctx.Query("file_search")
	return filter
}
