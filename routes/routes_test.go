package routes_test

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"

	"petroapp/db"
	"petroapp/models"
	"petroapp/routes"
	"petroapp/server"

	"github.com/gin-gonic/gin"
	"github.com/sirupsen/logrus"
)

func setupRouter() *gin.Engine {
	database := db.NewInMemoryDB()
	logger := logrus.New()
	logger.SetOutput(io.Discard)
	srv := &server.Server{Database: database, Logger: logger}
	gin.SetMode(gin.TestMode)
	r := gin.New()
	routes.RegisterRoutes(r, srv)
	return r
}

func postTransfers(r *gin.Engine, events []models.Event) *httptest.ResponseRecorder {
	body, _ := json.Marshal(models.TransferRequest{Events: events})
	req := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

func getSummary(r *gin.Engine, stationId string) *httptest.ResponseRecorder {
	req := httptest.NewRequest(http.MethodGet, "/stations/"+stationId+"/summary", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	return w
}

// Test 1: batch insert returns correct inserted/duplicates counts
func TestBatchInsertCounts(t *testing.T) {
	r := setupRouter()
	events := []models.Event{
		{EventId: "e1", StationId: "S1", Amount: 10, Status: "approved", CreatedAt: "2026-01-01T00:00:00Z"},
		{EventId: "e2", StationId: "S1", Amount: 20, Status: "approved", CreatedAt: "2026-01-01T00:00:00Z"},
		{EventId: "e1", StationId: "S1", Amount: 10, Status: "approved", CreatedAt: "2026-01-01T00:00:00Z"}, // duplicate
	}
	w := postTransfers(r, events)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp models.TransferResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Inserted != 2 || resp.Duplicates != 1 {
		t.Errorf("expected inserted=2 duplicates=1, got %+v", resp)
	}
}

// Test 2: duplicate event doesn't change totals
func TestDuplicateDoesNotChangeTotals(t *testing.T) {
	r := setupRouter()
	event := models.Event{EventId: "e1", StationId: "S1", Amount: 50, Status: "approved", CreatedAt: "2026-01-01T00:00:00Z"}
	postTransfers(r, []models.Event{event})
	postTransfers(r, []models.Event{event})

	w := getSummary(r, "S1")
	var summary models.StationSummary
	json.NewDecoder(w.Body).Decode(&summary)
	if summary.TotalApprovedAmount != 50 {
		t.Errorf("expected total=50, got %f", summary.TotalApprovedAmount)
	}
	if summary.EventsCount != 1 {
		t.Errorf("expected count=1, got %d", summary.EventsCount)
	}
}

// Test 3: out-of-order arrival still produces the same totals
func TestOutOfOrderTotals(t *testing.T) {
	r := setupRouter()
	events := []models.Event{
		{EventId: "e3", StationId: "S2", Amount: 30, Status: "approved", CreatedAt: "2026-01-03T00:00:00Z"},
		{EventId: "e1", StationId: "S2", Amount: 10, Status: "approved", CreatedAt: "2026-01-01T00:00:00Z"},
		{EventId: "e2", StationId: "S2", Amount: 20, Status: "approved", CreatedAt: "2026-01-02T00:00:00Z"},
	}
	postTransfers(r, events)

	w := getSummary(r, "S2")
	var summary models.StationSummary
	json.NewDecoder(w.Body).Decode(&summary)
	if summary.TotalApprovedAmount != 60 {
		t.Errorf("expected total=60, got %f", summary.TotalApprovedAmount)
	}
}

// Test 4: concurrent ingestion of same event_id doesn't double-insert
func TestConcurrentInsertNoDuplicate(t *testing.T) {
	r := setupRouter()
	event := models.Event{EventId: "e-concurrent", StationId: "S3", Amount: 100, Status: "approved", CreatedAt: "2026-01-01T00:00:00Z"}

	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			postTransfers(r, []models.Event{event})
		}()
	}
	wg.Wait()

	w := getSummary(r, "S3")
	var summary models.StationSummary
	json.NewDecoder(w.Body).Decode(&summary)
	if summary.TotalApprovedAmount != 100 {
		t.Errorf("expected total=100, got %f", summary.TotalApprovedAmount)
	}
	if summary.EventsCount != 1 {
		t.Errorf("expected count=1, got %d", summary.EventsCount)
	}
}

// Test 5: summary endpoint returns correct totals per station (only approved counts toward amount)
func TestSummaryPerStation(t *testing.T) {
	r := setupRouter()
	events := []models.Event{
		{EventId: "e1", StationId: "SA", Amount: 100, Status: "approved", CreatedAt: "2026-01-01T00:00:00Z"},
		{EventId: "e2", StationId: "SA", Amount: 50, Status: "pending", CreatedAt: "2026-01-01T00:00:00Z"},
		{EventId: "e3", StationId: "SB", Amount: 200, Status: "approved", CreatedAt: "2026-01-01T00:00:00Z"},
	}
	postTransfers(r, events)

	wA := getSummary(r, "SA")
	var summaryA models.StationSummary
	json.NewDecoder(wA.Body).Decode(&summaryA)
	if summaryA.TotalApprovedAmount != 100 {
		t.Errorf("SA: expected approved total=100, got %f", summaryA.TotalApprovedAmount)
	}
	if summaryA.EventsCount != 2 {
		t.Errorf("SA: expected count=2 (all statuses), got %d", summaryA.EventsCount)
	}

	wB := getSummary(r, "SB")
	var summaryB models.StationSummary
	json.NewDecoder(wB.Body).Decode(&summaryB)
	if summaryB.TotalApprovedAmount != 200 {
		t.Errorf("SB: expected approved total=200, got %f", summaryB.TotalApprovedAmount)
	}
}

// Test 6: invalid events are skipped, valid ones are inserted (partial accept)
func TestValidationPartialAccept(t *testing.T) {
	r := setupRouter()
	events := []models.Event{
		{EventId: "valid-1", StationId: "S4", Amount: 100, Status: "approved", CreatedAt: "2026-01-01T00:00:00Z"},
		{EventId: "", StationId: "S4", Amount: 50, Status: "approved", CreatedAt: "2026-01-01T00:00:00Z"},           // missing event_id
		{EventId: "bad-date", StationId: "S4", Amount: 50, Status: "approved", CreatedAt: "not-a-date"},              // bad created_at
		{EventId: "negative", StationId: "S4", Amount: -10, Status: "approved", CreatedAt: "2026-01-01T00:00:00Z"},  // negative amount
	}
	w := postTransfers(r, events)
	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
	var resp models.TransferResponse
	json.NewDecoder(w.Body).Decode(&resp)
	if resp.Inserted != 1 {
		t.Errorf("expected inserted=1, got %d", resp.Inserted)
	}
	if resp.Invalid != 3 {
		t.Errorf("expected invalid=3, got %d", resp.Invalid)
	}
}

// Test 7: malformed JSON body returns 400
func TestMalformedBodyReturns400(t *testing.T) {
	r := setupRouter()
	req := httptest.NewRequest(http.MethodPost, "/transfers", bytes.NewBufferString(`{not valid json`))
	req.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

// Test 8: summary for unknown station returns 404
func TestUnknownStationReturns404(t *testing.T) {
	r := setupRouter()
	w := getSummary(r, "UNKNOWN")
	if w.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", w.Code)
	}
}
