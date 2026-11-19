package files

import (
	"errors"
	"io"
	"mime"
	"net/http"
	"strings"

	"github.com/Akrambahy/EduFlow/backend/internal/platform/httpx"
	"github.com/gin-gonic/gin"
)

type RequesterFunc func(c *gin.Context) Requester

type Handler struct {
	svc       *Service
	requester RequesterFunc
}

func NewHandler(svc *Service, requester RequesterFunc) *Handler {
	return &Handler{svc: svc, requester: requester}
}

func (h *Handler) Register(g *gin.RouterGroup, auth gin.HandlerFunc) {
	a := g.Group("/files", auth)
	a.POST("", h.upload)
	a.GET("/:id", h.metadata)
	a.GET("/:id/download", h.download)
	a.DELETE("/:id", h.delete)
}

func (h *Handler) upload(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.svc.MaxSize()+64<<10)
	reader, err := c.Request.MultipartReader()
	if err != nil {
		httpx.Error(c, http.StatusBadRequest, "invalid_body", "Expected a multipart/form-data body with a file field", nil)
		return
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			if isMaxBytes(err) {
				tooLarge(c, h.svc.MaxSize())
				return
			}
			httpx.Error(c, http.StatusBadRequest, "invalid_body", "Malformed multipart body", nil)
			return
		}
		if part.FormName() != "file" || part.FileName() == "" {
			_ = part.Close()
			continue
		}
		f, err := h.svc.Upload(c.Request.Context(), h.requester(c).UserID, part.FileName(), part)
		_ = part.Close()
		if writeError(c, err, h.svc.MaxSize()) {
			return
		}
		c.Header("Location", "/api/v1/files/"+f.ID)
		httpx.Data(c, http.StatusCreated, f)
		return
	}
	httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"file": "is required"})
}

func (h *Handler) metadata(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	f, err := h.svc.Authorized(c.Request.Context(), h.requester(c), id)
	if writeError(c, err, 0) {
		return
	}
	httpx.Data(c, http.StatusOK, f)
}

func (h *Handler) download(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	f, err := h.svc.Authorized(c.Request.Context(), h.requester(c), id)
	if writeError(c, err, 0) {
		return
	}
	rc, err := h.svc.Open(c.Request.Context(), f)
	if err != nil {
		httpx.Internal(c, err)
		return
	}
	defer rc.Close()

	c.Header("Content-Type", f.ContentType)
	c.Header("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": f.Name}))
	c.Header("X-Content-Type-Options", "nosniff")
	c.Header("Cache-Control", "private, no-store")
	c.Header("Content-Security-Policy", "default-src 'none'; sandbox")
	http.ServeContent(c.Writer, c.Request, "", f.CreatedAt, rc)
}

func (h *Handler) delete(c *gin.Context) {
	id, ok := httpx.PathUUID(c, "id")
	if !ok {
		return
	}
	if writeError(c, h.svc.Delete(c.Request.Context(), h.requester(c), id), 0) {
		return
	}
	httpx.NoContent(c)
}

func tooLarge(c *gin.Context, max int64) {
	httpx.Error(c, http.StatusRequestEntityTooLarge, "file_too_large", "The file exceeds the maximum allowed size", map[string]any{"maxBytes": max})
}

func writeError(c *gin.Context, err error, max int64) bool {
	switch {
	case err == nil:
		return false
	case errors.Is(err, ErrNotFound):
		httpx.NotFound(c)
	case errors.Is(err, ErrTooLarge), isMaxBytes(err):
		tooLarge(c, max)
	case errors.Is(err, ErrUnsupportedType):
		httpx.Error(c, http.StatusUnsupportedMediaType, "unsupported_file_type", "This file type is not allowed",
			map[string]any{"allowed": strings.Join(AllowedExtensions(), ", ")})
	case errors.Is(err, ErrEmpty):
		httpx.Error(c, http.StatusBadRequest, "validation_error", "Some fields are invalid", map[string]string{"file": "must not be empty"})
	case errors.Is(err, ErrInUse):
		httpx.Error(c, http.StatusConflict, "file_in_use", "The file is still attached to other records", nil)
	default:
		httpx.Internal(c, err)
	}
	return true
}
