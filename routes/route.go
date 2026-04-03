package routes

import (
	"net/http"
	"petroapp/models"
	"petroapp/server"

	"github.com/gin-gonic/gin"
	"github.com/go-playground/validator/v10"
)

var validate *validator.Validate

func init() {
	validate = validator.New()
	validate.RegisterValidation("isodate", models.ValidateDate)
}

func RegisterRoutes(r *gin.Engine, s *server.Server) {
	r.POST("/transfers", postTransfers(s))
	r.GET("/stations/:station_id/summary", getStationSummary(s))
}

func postTransfers(s *server.Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		var req models.TransferRequest
		if err := c.ShouldBindJSON(&req); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "malformed request body", "detail": err.Error()})
			return
		}

		var inserted, duplicates, invalid int
		for _, event := range req.Events {
			if err := validate.Struct(event); err != nil {
				s.Logger.WithField("event_id", event.EventId).WithError(err).Warn("invalid event skipped")
				invalid++
				continue
			}

			ok, err := s.Database.InsertEvent(event)
			if err != nil {
				s.Logger.WithField("event_id", event.EventId).WithError(err).Error("insert failed")
				c.JSON(http.StatusInternalServerError, gin.H{"error": "internal server error"})
				return
			}
			if ok {
				inserted++
			} else {
				duplicates++
			}
		}

		c.JSON(http.StatusOK, models.TransferResponse{
			Inserted:   inserted,
			Duplicates: duplicates,
			Invalid:    invalid,
		})
	}
}

func getStationSummary(s *server.Server) gin.HandlerFunc {
	return func(c *gin.Context) {
		stationId := c.Param("station_id")

		events, err := s.Database.GetStationEventsByStationId(stationId)
		if err != nil {
			c.JSON(http.StatusNotFound, gin.H{"error": "station not found"})
			return
		}

		var totalApproved float64
		for _, e := range events {
			if e.Status == "approved" {
				totalApproved += e.Amount
			}
		}

		c.JSON(http.StatusOK, models.StationSummary{
			StationId:           stationId,
			TotalApprovedAmount: totalApproved,
			EventsCount:         len(events),
		})
	}
}
