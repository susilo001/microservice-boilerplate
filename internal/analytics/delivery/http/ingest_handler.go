package http

import (
	"errors"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"

	apiresp "github.com/boilerplate/microservice-boilerplate/api/http/v1/response"
	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/entity"
	"github.com/boilerplate/microservice-boilerplate/internal/analytics/domain/request"
	"github.com/boilerplate/microservice-boilerplate/internal/analytics/usecase"
)

var tracer = otel.Tracer("analytics/handler")

type IngestHandler struct {
	publishUC usecase.PublishEventUseCase
}

func NewIngestHandler(publishUC usecase.PublishEventUseCase) *IngestHandler {
	return &IngestHandler{publishUC: publishUC}
}

// Ingest accepts an analytics event and hands it to Kafka. It returns 202 the
// moment the event is published — never after a DB write. See AGENTS.md:
// "internal/analytics ingest handler never performs a synchronous Postgres call."
//
// @Summary      Ingest analytics event
// @Description  Publishes an event to Kafka asynchronously and returns immediately.
// @Tags         analytics
// @Accept       json
// @Produce      json
// @Param        body  body      request.IngestEventRequest  true  "Event payload"
// @Success      202   {object}  apiresp.APIResponse[struct{}]
// @Failure      400   {object}  apiresp.APIResponse[struct{}]
// @Failure      503   {object}  apiresp.APIResponse[struct{}]
// @Router       /api/v1/events [post]
func (h *IngestHandler) Ingest(c *gin.Context) {
	ctx, span := tracer.Start(c.Request.Context(), "analytics.handler.Ingest")
	defer span.End()

	tin := apiresp.ContextTime(c, "tin")
	correlationID := c.GetString("correlationid")

	var req request.IngestEventRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		c.JSON(http.StatusBadRequest, apiresp.NewError(correlationID, tin, time.Now(), err.Error()))
		return
	}

	if err := h.publishUC.Execute(ctx, req); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())

		status := http.StatusServiceUnavailable
		if isValidationErr(err) {
			status = http.StatusBadRequest
		}
		c.JSON(status, apiresp.NewError(correlationID, tin, time.Now(), err.Error()))
		return
	}

	c.JSON(http.StatusAccepted, apiresp.NewSuccess(correlationID, tin, time.Now(), struct{}{}))
}

func isValidationErr(err error) bool {
	return errors.Is(err, entity.ErrEventIDRequired) ||
		errors.Is(err, entity.ErrUserIDRequired) ||
		errors.Is(err, entity.ErrEventTypeRequired)
}
