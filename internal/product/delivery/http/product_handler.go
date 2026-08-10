package http

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"

	apiresp "github.com/boilerplate/microservice-boilerplate/api/http/v1/response"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/product/usecase"
)

var tracer = otel.Tracer("product/handler")

type ProductHandler struct {
	getUC    usecase.GetProductUseCase
	listUC   usecase.ListProductsUseCase
	createUC usecase.CreateProductUseCase
	updateUC usecase.UpdateProductUseCase
	deleteUC usecase.DeleteProductUseCase
}

func NewProductHandler(
	getUC usecase.GetProductUseCase,
	listUC usecase.ListProductsUseCase,
	createUC usecase.CreateProductUseCase,
	updateUC usecase.UpdateProductUseCase,
	deleteUC usecase.DeleteProductUseCase,
) *ProductHandler {
	return &ProductHandler{
		getUC:    getUC,
		listUC:   listUC,
		createUC: createUC,
		updateUC: updateUC,
		deleteUC: deleteUC,
	}
}

// GetByID returns a single product by its UUID.
//
// @Summary      Get product by ID
// @Description  Returns a single product identified by its UUID.
// @Tags         products
// @Produce      json
// @Param        id   path      string  true  "Product UUID"
// @Success      200  {object}  apiresp.APIResponse[response.ProductResponse]
// @Failure      404  {object}  apiresp.APIResponse[struct{}]
// @Failure      500  {object}  apiresp.APIResponse[struct{}]
// @Router       /api/v1/products/{id} [get]
func (h *ProductHandler) GetByID(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "product.handler.GetByID")
	defer span.End()
	span.SetAttributes(attribute.String("product.id", c.Param("id")))

	tin := apiresp.ContextTime(c, "tin")
	correlationID := c.GetString("correlationid")

	res, err := h.getUC.Execute(ctx, c.Param("id"))
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, apiresp.NewSuccess(correlationID, tin, time.Now(), res))
}

// List returns a paginated list of products.
//
// @Summary      List products
// @Description  Returns a paginated list of products. Supports substring search across name/description, whitelisted sort fields, and optional relation preloading.
// @Tags         products
// @Produce      json
// @Param        page       query     int     false  "Page number (default 1)"
// @Param        page_size  query     int     false  "Items per page (default 20)"
// @Param        search     query     string  false  "Substring search across name and description"
// @Param        sort_by    query     string  false  "Sort field: name, price, stock, created_at, updated_at"
// @Param        sort_dir   query     string  false  "Sort direction: asc (default) or desc"
// @Param        preload    query     string  false  "Comma-separated relations to eager-load"
// @Success      200  {object}  apiresp.APIResponse[apiresp.ListData[response.ProductResponse]]
// @Failure      400  {object}  apiresp.APIResponse[struct{}]
// @Failure      500  {object}  apiresp.APIResponse[struct{}]
// @Router       /api/v1/products [get]
func (h *ProductHandler) List(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "product.handler.List")
	defer span.End()

	tin := apiresp.ContextTime(c, "tin")
	correlationID := c.GetString("correlationid")
	req := request.ListProductsRequest{
		Page:     queryInt(c, "page", 1),
		PageSize: queryInt(c, "page_size", 20),
		Search:   c.Query("search"),
		SortBy:   c.Query("sort_by"),
		SortDir:  c.Query("sort_dir"),
		Preload:  queryList(c, "preload"),
	}
	span.SetAttributes(
		attribute.Int("page", req.Page),
		attribute.Int("page_size", req.PageSize),
		attribute.String("search", req.Search),
		attribute.String("sort_by", req.SortBy),
	)

	start := time.Now()
	res, total, err := h.listUC.Execute(ctx, req)
	finish := time.Now()

	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		_ = c.Error(err)
		return
	}
	span.SetAttributes(attribute.Int64("total", total))
	c.JSON(http.StatusOK, apiresp.NewList(correlationID, tin, time.Now(), res, req.Page, req.PageSize, total, start, finish))
}

// Create creates a new product.
//
// @Summary      Create product
// @Description  Creates a new product and publishes a ProductCreated event.
// @Tags         products
// @Accept       json
// @Produce      json
// @Param        body  body      request.CreateProductRequest  true  "Product payload"
// @Success      201   {object}  apiresp.APIResponse[response.ProductResponse]
// @Failure      400   {object}  apiresp.APIResponse[struct{}]
// @Failure      500   {object}  apiresp.APIResponse[struct{}]
// @Router       /api/v1/products [post]
func (h *ProductHandler) Create(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "product.handler.Create")
	defer span.End()

	tin := apiresp.ContextTime(c, "tin")
	correlationID := c.GetString("correlationid")

	var req request.CreateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		c.JSON(http.StatusBadRequest, apiresp.NewError(correlationID, tin, time.Now(), err.Error()))
		return
	}

	res, err := h.createUC.Execute(ctx, req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		_ = c.Error(err)
		return
	}
	span.SetAttributes(attribute.String("product.id", res.ID))
	c.JSON(http.StatusCreated, apiresp.NewSuccess(correlationID, tin, time.Now(), res))
}

// Update updates an existing product.
//
// @Summary      Update product
// @Description  Updates an existing product by ID and publishes a ProductUpdated event.
// @Tags         products
// @Accept       json
// @Produce      json
// @Param        id    path      string                        true  "Product UUID"
// @Param        body  body      request.UpdateProductRequest  true  "Product payload"
// @Success      200   {object}  apiresp.APIResponse[response.ProductResponse]
// @Failure      400   {object}  apiresp.APIResponse[struct{}]
// @Failure      404   {object}  apiresp.APIResponse[struct{}]
// @Failure      500   {object}  apiresp.APIResponse[struct{}]
// @Router       /api/v1/products/{id} [put]
func (h *ProductHandler) Update(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "product.handler.Update")
	defer span.End()
	span.SetAttributes(attribute.String("product.id", c.Param("id")))

	tin := apiresp.ContextTime(c, "tin")
	correlationID := c.GetString("correlationid")

	var req request.UpdateProductRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		c.JSON(http.StatusBadRequest, apiresp.NewError(correlationID, tin, time.Now(), err.Error()))
		return
	}

	res, err := h.updateUC.Execute(ctx, c.Param("id"), req)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusOK, apiresp.NewSuccess(correlationID, tin, time.Now(), res))
}

// Delete deletes a product by ID.
//
// @Summary      Delete product
// @Description  Deletes a product by ID and invalidates its cache entry.
// @Tags         products
// @Produce      json
// @Param        id  path      string  true  "Product UUID"
// @Success      204 {object}  apiresp.APIResponse[struct{}]
// @Failure      404 {object}  apiresp.APIResponse[struct{}]
// @Failure      500 {object}  apiresp.APIResponse[struct{}]
// @Router       /api/v1/products/{id} [delete]
func (h *ProductHandler) Delete(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "product.handler.Delete")
	defer span.End()
	span.SetAttributes(attribute.String("product.id", c.Param("id")))

	tin := apiresp.ContextTime(c, "tin")
	correlationID := c.GetString("correlationid")

	if err := h.deleteUC.Execute(ctx, c.Param("id")); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		_ = c.Error(err)
		return
	}
	c.JSON(http.StatusNoContent, apiresp.NewSuccess(correlationID, tin, time.Now(), struct{}{}))
}

func queryInt(c *gin.Context, key string, defaultVal int) int {
	s := c.Query(key)
	if s == "" {
		return defaultVal
	}
	v, err := strconv.Atoi(s)
	if err != nil || v < 1 {
		return defaultVal
	}
	return v
}

// queryList splits a comma-separated query param into a trimmed, non-empty slice.
func queryList(c *gin.Context, key string) []string {
	s := c.Query(key)
	if s == "" {
		return nil
	}
	parts := strings.Split(s, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
