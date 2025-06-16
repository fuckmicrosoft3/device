package api


import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/gorilla/mux"
	"github.com/sirupsen/logrus"

	"example.com/backstage/services/truck/internal/metrics"
	"example.com/backstage/services/truck/internal/service"
)

// Handler defines the API handler
type Handler struct {
	operationService      service.OperationService
	operationGroupService service.OperationGroupService
	operationEventService service.OperationEventService
}

// NewHandler creates a new API handler
func NewHandler(
	operationService service.OperationService,
	operationGroupService service.OperationGroupService,
	operationEventService service.OperationEventService,
) *Handler {
	return &Handler{
		operationService:      operationService,
		operationGroupService: operationGroupService,
		operationEventService: operationEventService,
	}
}

// RegisterRoutes registers API routes
func (h *Handler) RegisterRoutes(r *mux.Router) {
	// Operation routes
	r.HandleFunc("/ops/op/{device_uid}", h.GetActiveOperation).Methods(http.MethodGet)
	r.HandleFunc("/ops/op/{device_uid}/events", h.RecordOperationEvent).Methods(http.MethodPost)
	
	// Operation group routes
	r.HandleFunc("/ops/opg/{truck_uid}", h.GetActiveOperationGroup).Methods(http.MethodGet)
	r.HandleFunc("/ops/opg/{truck_uid}/events", h.RecordOperationEvent).Methods(http.MethodPost)
	
	// Metrics and health endpoints
	r.HandleFunc("/metrics", MetricsHandler).Methods(http.MethodGet)
	r.HandleFunc("/health", HealthHandler).Methods(http.MethodGet)
}

// GetActiveOperation gets the active operation for a device
func (h *Handler) GetActiveOperation(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	deviceMCU := vars["device_uid"]
	
	// Update metrics
	collector := metrics.GetMetricsCollector()
	
	if deviceMCU == "" {
		WriteError(w, ErrInvalidRequest)
		collector.RecordError(metrics.ErrorTypeValidation)
		return
	}
	
	operation, err := h.operationService.FindActiveByDeviceMCU(r.Context(), deviceMCU)
	if err != nil {
		WriteError(w, ErrNotFound)
		collector.RecordError(metrics.ErrorTypeInternal)
		return
	}
	
	if operation == nil {
		WriteError(w, ErrNotFound)
		collector.RecordError(metrics.ErrorTypeInternal)
		return
	}
	
	writeJSONResponse(w, http.StatusOK, operation)
}

// GetActiveOperationGroup gets the active operation group for a truck
func (h *Handler) GetActiveOperationGroup(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	truckMCU := vars["truck_uid"]
	
	// Update metrics
	collector := metrics.GetMetricsCollector()
	
	if truckMCU == "" {
		WriteError(w, ErrInvalidRequest)
		collector.RecordError(metrics.ErrorTypeValidation)
		return
	}
	
	group, err := h.operationGroupService.FindActiveByTransportMCU(r.Context(), truckMCU)
	if err != nil {
		WriteError(w, ErrNotFound)
		collector.RecordError(metrics.ErrorTypeInternal)
		return
	}
	
	if group == nil {
		WriteError(w, ErrNotFound)
		collector.RecordError(metrics.ErrorTypeInternal)
		return
	}
	
	// Update active operation groups gauge
	collector.SetActiveOperationGroups(1) 
	
	writeJSONResponse(w, http.StatusOK, group)
}

// RecordOperationEvent records an operation event
func (h *Handler) RecordOperationEvent(w http.ResponseWriter, r *http.Request) {
	var req service.RecordEventRequest
	
	// Update metrics
	collector := metrics.GetMetricsCollector()
	startTime := time.Now()
	
	// Parse the request body
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		WriteError(w, NewValidationError("Invalid request body"))
		collector.RecordError(metrics.ErrorTypeValidation)
		return
	}
	
	// Get device MCU from URL
	vars := mux.Vars(r)
	deviceMCU := vars["device_uid"]
	truckMCU := vars["truck_uid"]
	
	// Use the appropriate MCU
	if deviceMCU != "" {
		req.DeviceMCU = deviceMCU
	} else if truckMCU != "" {
		req.DeviceMCU = truckMCU
	} else {
		WriteError(w, NewValidationError("Device or truck ID is required"))
		collector.RecordError(metrics.ErrorTypeValidation)
		return
	}
	
	// Record the event
	if err := h.operationEventService.RecordEvent(r.Context(), &req); err != nil {
		logrus.WithError(err).Error("Failed to record operation event")
		WriteError(w, ErrInternalServer)
		collector.RecordError(metrics.ErrorTypeInternal)
		return
	}
	
	// Record metrics for the event processing
	collector.RecordOperation(metrics.OperationTypeEventProcessing, time.Since(startTime))
	
	// Return success
	writeJSONResponse(w, http.StatusOK, map[string]interface{}{
		"success": true,
	})
}

// writeJSONResponse writes a JSON response
func writeJSONResponse(w http.ResponseWriter, statusCode int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	
	if err := json.NewEncoder(w).Encode(data); err != nil {
		logrus.WithError(err).Error("Failed to encode JSON response")
	}
}