package handler

import (
	"github.com/gin-gonic/gin"
	"log/slog"
	"opencsg.com/csghub-server/api/httpbase"
	"opencsg.com/csghub-server/common/config"
	"opencsg.com/csghub-server/common/types"
	"opencsg.com/csghub-server/common/utils/common"
	"opencsg.com/csghub-server/component"
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
	return &ClabelHandler{
		c:  clabelComponent,
		rh: repoHandler,
	}, nil
}

type ClabelHandler struct {
	c  *component.ClabelComponent
	rh *RepoHandler
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
