package http

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	pkglogger "github.com/boilerplate/microservice-boilerplate/pkg/logger"
	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.uber.org/zap"

	apiresp "github.com/boilerplate/microservice-boilerplate/api/http/v1/response"
	"github.com/boilerplate/microservice-boilerplate/internal/product/domain/request"
	_ "github.com/boilerplate/microservice-boilerplate/internal/product/domain/response"
	"github.com/boilerplate/microservice-boilerplate/internal/product/usecase"
	"github.com/boilerplate/microservice-boilerplate/pkg/apperror"
	"github.com/boilerplate/microservice-boilerplate/pkg/queryutil"
)

var tracer = otel.Tracer("product/handler")

type ProductHandler struct {
	getUC    usecase.GetProductUseCase
	listUC   usecase.ListProductsUseCase
	createUC usecase.CreateProductUseCase
	updateUC usecase.UpdateProductUseCase
	deleteUC usecase.DeleteProductUseCase
	log      *zap.Logger
}

func NewProductHandler(
	getUC usecase.GetProductUseCase,
	listUC usecase.ListProductsUseCase,
	createUC usecase.CreateProductUseCase,
	updateUC usecase.UpdateProductUseCase,
	deleteUC usecase.DeleteProductUseCase,
	log *zap.Logger,
) *ProductHandler {
	return &ProductHandler{
		getUC:    getUC,
		listUC:   listUC,
		createUC: createUC,
		updateUC: updateUC,
		deleteUC: deleteUC,
		log:      log,
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
// @Failure      404  {object}  apiresp.APIResponse[apiresp.Empty]
// @Failure      500  {object}  apiresp.APIResponse[apiresp.Empty]
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
// @Description  Returns a paginated list of products. Filters use field:operator:value; sort uses field:direction.
// @Tags         products
// @Produce      json
// @Param        page       query     int     false  "Page number (default 1)"
// @Param        page_size  query     int     false  "Items per page (default 20)"
// @Param        search     query     string  false  "Substring search across name and description"
// @Param        filter     query     string  false  "Repeatable filter: name:eq:Widget, price:gte:10, stock:lte:50"
// @Param        sort       query     string  false  "Sort: name:asc, price:desc, stock:asc, created_at:desc, updated_at:desc"
// @Param        preload    query     string  false  "Comma-separated relations to eager-load"
// @Success      200  {object}  apiresp.APIResponse[apiresp.ListData[response.ProductResponse]]
// @Failure      400  {object}  apiresp.APIResponse[apiresp.Empty]
// @Failure      500  {object}  apiresp.APIResponse[apiresp.Empty]
// @Router       /api/v1/products [get]
func (h *ProductHandler) List(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "product.handler.List")
	defer span.End()

	tin := apiresp.ContextTime(c, "tin")
	correlationID := c.GetString("correlationid")
	var req request.ListProductsRequest
	if err := queryutil.Bind(c.Request.URL.Query(), &req); err != nil {
		log := pkglogger.FromContext(ctx, h.log).With(zap.String("operation", "ListProducts"))
		details := apperror.Classify(err)
		log.Warn("request decoding rejected", zap.String("reason", details.Reason),
			zap.String("validation_field", details.ValidationField), zap.String("validation_rule", details.ValidationRule))
		pkglogger.DebugPayload(log, "request decoding input", "request", req)
		pkglogger.DebugPayload(log, "request decoding response", "response", map[string]any{"error": "invalid_query"})
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		_ = c.Error(err)
		return
	}
	span.SetAttributes(
		attribute.Int("page", req.Page),
		attribute.Int("page_size", req.PageSize),
		attribute.String("search", req.Search),
		attribute.String("sort", req.Sort),
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
// @Failure      400   {object}  apiresp.APIResponse[apiresp.Empty]
// @Failure      500   {object}  apiresp.APIResponse[apiresp.Empty]
// @Router       /api/v1/products [post]
func (h *ProductHandler) Create(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "product.handler.Create")
	defer span.End()

	tin := apiresp.ContextTime(c, "tin")
	correlationID := c.GetString("correlationid")

	var req request.CreateProductRequest
	if err := decodeProductJSON(c.Request.Body, &req); err != nil {
		status, reason := decodingFailure(err)
		h.logDecodeFailure(ctx, "CreateProduct", reason, req)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		c.JSON(status, apiresp.NewError(correlationID, tin, time.Now(), reason))
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
// @Failure      400   {object}  apiresp.APIResponse[apiresp.Empty]
// @Failure      404   {object}  apiresp.APIResponse[apiresp.Empty]
// @Failure      409   {object}  apiresp.APIResponse[apiresp.Empty]
// @Failure      500   {object}  apiresp.APIResponse[apiresp.Empty]
// @Router       /api/v1/products/{id} [put]
func (h *ProductHandler) Update(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "product.handler.Update")
	defer span.End()
	span.SetAttributes(attribute.String("product.id", c.Param("id")))

	tin := apiresp.ContextTime(c, "tin")
	correlationID := c.GetString("correlationid")

	var req request.UpdateProductRequest
	if err := decodeProductJSON(c.Request.Body, &req); err != nil {
		status, reason := decodingFailure(err)
		h.logDecodeFailure(ctx, "UpdateProduct", reason, req)
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		c.JSON(status, apiresp.NewError(correlationID, tin, time.Now(), reason))
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

func decodeProductJSON(body io.Reader, target any) error {
	decoder := json.NewDecoder(body)
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		if err != nil {
			return err
		}
		return errors.New("request must contain exactly one JSON value")
	}
	return nil
}

func decodingFailure(err error) (int, string) {
	var tooLarge *http.MaxBytesError
	if errors.As(err, &tooLarge) {
		return http.StatusRequestEntityTooLarge, "request_body_too_large"
	}
	return http.StatusBadRequest, "invalid_json"
}

func (h *ProductHandler) logDecodeFailure(ctx context.Context, operation, reason string, input any) {
	log := pkglogger.FromContext(ctx, h.log).With(zap.String("operation", operation))
	log.Warn("request decoding rejected", zap.String("reason", reason))
	pkglogger.DebugPayload(log, "request decoding input", "request", input)
	pkglogger.DebugPayload(log, "request decoding response", "response", map[string]any{"error": reason})
}

// Delete deletes a product by ID.
//
// @Summary      Delete product
// @Description  Deletes a product by ID and invalidates its cache entry.
// @Tags         products
// @Produce      json
// @Param        id  path      string  true  "Product UUID"
// @Success      204 {object}  apiresp.APIResponse[apiresp.Empty]
// @Failure      404 {object}  apiresp.APIResponse[apiresp.Empty]
// @Failure      500 {object}  apiresp.APIResponse[apiresp.Empty]
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
