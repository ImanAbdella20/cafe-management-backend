package menu

import (
	"errors"
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

type Handler struct {
	service Service
}

func NewHandler(service Service) *Handler {
	return &Handler{service: service}
}

func (h *Handler) CreateItem(c *gin.Context) {
	req, err := bindCreateItemRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	item, err := h.service.CreateItem(c.Request.Context(), req)
	if err != nil {
		if errors.Is(err, ErrMenuItemNameRequired) || errors.Is(err, ErrMenuCategoryRequired) || errors.Is(err, ErrInvalidImageFormat) || errors.Is(err, ErrImageTooLarge) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrMenuCategoryNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusCreated, CreateMenuItemResponse{
		Message: "Item added.",
		Item:    item,
	})
}

func (h *Handler) GetItems(c *gin.Context) {
	items, err := h.service.GetItems(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, items)
}

func (h *Handler) UpdateItem(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	req, err := bindUpdateItemRequest(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	err = h.service.UpdateItem(c.Request.Context(), id, req)
	if err != nil {
		if errors.Is(err, ErrMenuItemNameRequired) || errors.Is(err, ErrInvalidImageFormat) || errors.Is(err, ErrImageTooLarge) {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		if errors.Is(err, ErrMenuItemNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}

func bindCreateItemRequest(c *gin.Context) (CreateMenuItemRequest, error) {
	if isMultipartRequest(c.ContentType()) {
		categoryID, err := strconv.Atoi(strings.TrimSpace(c.PostForm("category_id")))
		if err != nil {
			return CreateMenuItemRequest{}, fmt.Errorf("invalid category id")
		}

		image, err := readImageFromForm(c, "image")
		if err != nil {
			return CreateMenuItemRequest{}, err
		}

		return CreateMenuItemRequest{
			CategoryID:  categoryID,
			Name:        c.PostForm("name"),
			Description: c.PostForm("description"),
			ImageURL:    c.PostForm("image_url"),
			Image:       image,
		}, nil
	}

	var req CreateMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return CreateMenuItemRequest{}, err
	}

	return req, nil
}

func bindUpdateItemRequest(c *gin.Context) (UpdateMenuItemRequest, error) {
	if isMultipartRequest(c.ContentType()) {
		image, err := readImageFromForm(c, "image")
		if err != nil {
			return UpdateMenuItemRequest{}, err
		}

		return UpdateMenuItemRequest{
			Name:        c.PostForm("name"),
			Description: c.PostForm("description"),
			ImageURL:    c.PostForm("image_url"),
			RemoveImage: parseBoolFormValue(c.PostForm("remove_image")),
			Image:       image,
		}, nil
	}

	var req UpdateMenuItemRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		return UpdateMenuItemRequest{}, err
	}

	return req, nil
}

func readImageFromForm(c *gin.Context, fieldName string) (*UploadedImage, error) {
	fileHeader, err := c.FormFile(fieldName)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return nil, nil
		}
		return nil, err
	}

	return ReadUploadedImage(fileHeader)
}

func isMultipartRequest(contentType string) bool {
	return strings.HasPrefix(strings.ToLower(strings.TrimSpace(contentType)), "multipart/form-data")
}

func parseBoolFormValue(value string) bool {
	trimmed := strings.ToLower(strings.TrimSpace(value))
	return trimmed == "1" || trimmed == "true" || trimmed == "yes" || trimmed == "on"
}

func (h *Handler) DeleteItem(c *gin.Context) {
	id, err := strconv.Atoi(c.Param("id"))
	if err != nil || id <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid item id"})
		return
	}

	err = h.service.DeleteItem(c.Request.Context(), id)
	if err != nil {
		if errors.Is(err, ErrMenuItemNotFound) {
			c.JSON(http.StatusNotFound, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		return
	}

	c.Status(http.StatusOK)
}
