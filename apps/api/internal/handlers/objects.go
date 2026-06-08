package handlers

import (
	"errors"
	"net/http"

	"github.com/carissafarry/tag-me/api/internal/models"
	"github.com/carissafarry/tag-me/api/internal/services"
	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

type ObjectHandler struct {
	service   *services.ObjectService
	qrService *services.QRCodeService
}

func NewObjectHandler(service *services.ObjectService) *ObjectHandler {
	return &ObjectHandler{service: service}
}

func NewObjectHandlerWithQR(service *services.ObjectService, qrService *services.QRCodeService) *ObjectHandler {
	return &ObjectHandler{
		service:   service,
		qrService: qrService,
	}
}

func (h *ObjectHandler) CreateObject(c *gin.Context) {
	ownerID, err := getUserIDUUID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
			Code:  "unauthorized",
		})
		return
	}

	var req models.CreateObjectRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: err.Error(),
			Code:  "validation_error",
		})
		return
	}

	obj, err := h.service.CreateObject(c.Request.Context(), ownerID, &req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to create object",
			Code:  "internal_error",
		})
		return
	}

	c.JSON(http.StatusCreated, obj)
}

func (h *ObjectHandler) GetObjects(c *gin.Context) {
	ownerID, err := getUserIDUUID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Code:  "unauthorized",
			Error: "unauthorized",
		})
		return
	}

	filters := make(map[string]string)
	if name := c.Query("name"); name != "" {
		filters["name"] = name
	}
	if objectType := c.Query("object_type"); objectType != "" {
		filters["object_type"] = objectType
	}

	objects, err := h.service.GetObjects(c.Request.Context(), ownerID, filters)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to list objects",
			Code:  "internal_error",
		})
		return
	}

	if objects == nil {
		objects = []models.Object{}
	}
	
	c.JSON(http.StatusOK, gin.H{
		"objects": objects,
		"total": len(objects),
	})
}

func (h *ObjectHandler) GetObjectDetail(c *gin.Context) {
	ownerID, err := getUserIDUUID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
			Code:  "unauthorized",
		})
		return
	}

	objectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid object id",
			Code:  "invalid_object_id",
		})
		return
	}

	obj, err := h.service.GetObject(c.Request.Context(), ownerID, objectID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to get object",
			Code:  "internal_error",
		})
		return
	}

	if obj == nil {
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Error: "object not found",
			Code:  "object_not_found",
		})
		return
	}

	c.JSON(http.StatusOK, obj)
}

func (h *ObjectHandler) DeleteObject(c *gin.Context) {
	ownerID, err := getUserIDUUID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Error: "unauthorized",
			Code:  "unauthorized",
		})
		return
	}

	objectID, err := uuid.Parse(c.Param("id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Error: "invalid object id",
			Code:  "invalid_object_id",
		})
		return
	}

	err = h.service.DeleteObject(c.Request.Context(), ownerID, objectID)
	if err != nil {
		if errors.Is(err, services.ErrConversationActive) {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Error: err.Error(),
				Code:  "conversation_active",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Error: "failed to delete object",
			Code:  "internal_error",
		})
		return
	}

	c.JSON(http.StatusNoContent, nil)
}

// QR CODE handlers

func (h *ObjectHandler) GenerateQRCode(c *gin.Context) {
	var req models.GenerateQRRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Code:  "validation_error",
			Error: services.ErrInvalidRequest.Error(),
		})
		return
	}

	ownerID, err := getUserIDUUID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Code:  "unauthorized",
			Error: "unauthorized",
		})
		return
	}

	qr, err := h.qrService.GenerateQRCode(c.Request.Context(), ownerID, req.ObjectID)
	if err != nil {
		if errors.Is(err, services.ErrObjectNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Code:  "object_not_found",
				Error: "object not found",
			})
			return
		}
		if errors.Is(err, services.ErrQRCodeGenerationInProgress) {
			c.JSON(http.StatusConflict, models.ErrorResponse{
				Code:  "generation_in_progress",
				Error: "qr code generation already in progress",
			})
			return
		}
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Code:  "internal_error",
			Error: "failed to generate qr code",
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, models.GenerateQRResponse{
		ID:        qr.ID,
		QRToken:   qr.QRToken,
		IsActive:  qr.IsActive,
		CreatedAt: formatJakartaTime(qr.CreatedAt),
	})
}

func (h *ObjectHandler) GetQRImage(c *gin.Context) {
	token := c.Param("token")
	if token == "" {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Code:  "invalid_token",
			Error: "qr token is required",
		})
		return
	}

	qrImage, err := h.qrService.GenerateQRCodeImage(c.Request.Context(), token)
	if err != nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Code:  "internal_error",
			Error: "failed to get qr code image",
			Detail: err.Error(),
		})
		return
	}

	// Return image as base64 in JSON response
	c.JSON(http.StatusOK, gin.H{
		"qr_image": qrImage,
	})
}

func (h *ObjectHandler) GetQRCode(c *gin.Context) {
	if h.qrService == nil {
		c.JSON(http.StatusInternalServerError, models.ErrorResponse{
			Code:  "internal_error",
			Error: "qr code service not initialized",
		})
		return
	}

	ownerID, err := getUserIDUUID(c)
	if err != nil {
		c.JSON(http.StatusUnauthorized, models.ErrorResponse{
			Code:  "unauthorized",
			Error: "unauthorized",
		})
		return
	}

	objectID, err := uuid.Parse(c.Param("object_id"))
	if err != nil {
		c.JSON(http.StatusBadRequest, models.ErrorResponse{
			Code:  "invalid_object_id",
			Error: "invalid object id",
		})
		return
	}

	qr, err := h.qrService.GetQRCode(c.Request.Context(), ownerID, objectID)
	if err != nil {
		if errors.Is(err, services.ErrObjectNotFound) {
			c.JSON(http.StatusNotFound, models.ErrorResponse{
				Code:  "object_not_found",
				Error: "object not found",
			})
			return
		}
		c.JSON(http.StatusNotFound, models.ErrorResponse{
			Code:  "qr_code_not_found",
			Error: "qr code not found",
		})
		return
	}

	c.JSON(http.StatusOK, models.GetQRResponse{
		ID:        qr.ID,
		QRToken:   qr.QRToken,
		ObjectID:  qr.ObjectID,
		IsActive:  qr.IsActive,
		CreatedAt: formatJakartaTime(qr.CreatedAt),
	})
}
