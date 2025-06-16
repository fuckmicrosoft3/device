
## API Endpoints

### Health Status
- **URL**: `/health`
- **Method**: GET
- **Purpose**: Quick check if the service is healthy
- **Refresh Rate**: Every 15-30 seconds
- **Response Format**:
  ```json
  {
    "status": true,  // Overall health (true = healthy, false = unhealthy)
    "details": {     // Individual component health
      "database_write": true,
      "database_read": true,
      "elasticsearch": true,
      "cache": true
    }
  }
  ```

### Detailed Metrics
- **URL**: `/metrics`
- **Method**: GET
- **Purpose**: Comprehensive metrics for dashboards
- **Refresh Rate**: Every 30-60 seconds
- **Response Format**: See below

## Metrics Overview

The `/metrics` endpoint returns a JSON object with the following sections:

```json
{
  "uptime_seconds": 3600,
  "counters": { ... },
  "gauges": { ... },
  "timers": { ... },
  "error_rates": { ... },
  "health_checks": { ... }
}
```

## Metric Types and Visualization Recommendations

### 1. Health Checks
Health checks indicate component status (true = healthy, false = unhealthy).

**Visualization Recommendation**: 
- Status indicators/traffic lights (green/red)
- Health history timeline (status over time)

**Key Health Metrics**:
- `database_write`: Write database connection
- `database_read`: Read database connection
- `elasticsearch`: Elasticsearch connection
- `cache`: Redis cache connection

### 2. Counters
Counters track cumulative values that only increase.

**Visualization Recommendation**:
- Large number displays with trends
- Stacked area charts for related counters

**Key Counter Metrics**:
- `dispense_sessions_created`: Total sessions created
- `sales_processed`: Total sales processed
- `sales_created_immediate`: Sales created immediately
- `sales_created_reconciliation`: Sales created during reconciliation
- `service_bus_messages_received`: Messages received from Azure
- `total_sales_amount`: Cumulative sales amount (KSH)
- `paid_sales_count`: Number of paid sales
- `free_sales_count`: Number of free sales
- `reconciliation_runs`: Number of reconciliation jobs executed

### 3. Gauges
Gauges track point-in-time values that can increase or decrease.

**Visualization Recommendation**:
- Gauges with thresholds
- Line charts showing trends

**Key Gauge Metrics**:
- `unprocessed_sessions`: Sessions waiting to be processed
- `active_db_transactions`: Current active database transactions
- `last_reconciliation_success_count`: Successes in last reconciliation
- `last_reconciliation_error_count`: Errors in last reconciliation
- `goroutines`: Current number of goroutines

### 4. Timers
Timers track operation durations in milliseconds.

**Visualization Recommendation**:
- Bar charts or line charts with thresholds
- Heatmaps for response time distribution

**Key Timer Metrics**:
Each timer provides these values:
- `count`: Number of measurements
- `total_time_ms`: Total time spent
- `average_time_ms`: Average time per operation
- `min_time_ms`: Fastest operation
- `max_time_ms`: Slowest operation

**Important Timers**:
- `create_dispense_session`: Time to create a dispense session
- `retrieve_sale_context`: Time to retrieve sale context data
- `db_transaction_time`: Database transaction execution time
- `index_sale_elastic`: Time to index sale in Elasticsearch
- `immediate_processing_total`: Total immediate processing time
- `process_service_bus_message`: Service bus message processing time
- `reconcile_sales_total`: Total reconciliation job time

### 5. Error Rates
Error rates track success/failure percentages.

**Visualization Recommendation**:
- Percentage gauges with thresholds
- Line charts showing error trends

**Key Error Rate Metrics**:
Each error rate provides:
- `total`: Total operations
- `errors`: Failed operations
- `error_rate`: Percentage of errors

**Important Error Rates**:
- `dispense_session_creation`: Success rate of session creation
- `immediate_sale_processing`: Success rate of immediate processing
- `db_transaction`: Success rate of database transactions
- `index_sale_elastic`: Success rate of Elasticsearch indexing
- `process_service_bus_message`: Success rate of message processing

## Recommended Dashboard Layouts

### 1. Overview Dashboard
- System Health Status (all components)
- Uptime
- Active Sessions (unprocessed count)
- Today's Sales Count
- Today's Sales Amount
- Recent Error Rates
- Message Processing Rate

### 2. Performance Dashboard
- Operation Timers (all key operations)
- Database Performance
- Elasticsearch Performance
- API Response Times
- Resource Usage (goroutines)

### 3. Business Metrics Dashboard
- Sales Volume Trends
- Revenue Trends (paid sales)
- Free vs. Paid Sales Ratio
- Sales by Time of Day
- Reconciliation Performance

## Implementation Notes

1. **Polling Frequency**:
   - Health checks: 15-30 seconds
   - Metrics data: 30-60 seconds
   - Avoid excessive polling to prevent service load

2. **Data Visualization**:
   - Use appropriate thresholds for each metric
   - Add visual alerts for values outside normal ranges
   - Include trend indicators (up/down arrows)

3. **Time Range Selection**:
   - Allow selection of different time ranges
   - Default to last hour for most metrics
   - Provide day/week/month views for business metrics

4. **Error Highlighting**:
   - Prominently display error rates > 1%
   - Provide drill-down capability for error details

5. **Responsive Design**:
   - Ensure dashboards work on desktop and tablets
   - Consider a simplified mobile view for on-call monitoring

## API Response Example

```json
{
  "uptime_seconds": 3600,
  "counters": {
    "dispense_sessions_created": 1250,
    "sales_processed": 1200,
    "sales_created_immediate": 1150,
    "sales_created_reconciliation": 50,
    "total_sales_amount": 456000
  },
  "gauges": {
    "unprocessed_sessions": 5,
    "active_db_transactions": 2,
    "goroutines": 24
  },
  "timers": {
    "create_dispense_session": {
      "count": 1250,
      "total_time_ms": 125000,
      "average_time_ms": 100.0,
      "min_time_ms": 50,
      "max_time_ms": 450
    },
    "immediate_processing_total": {
      "count": 1150,
      "total_time_ms": 345000,
      "average_time_ms": 300.0,
      "min_time_ms": 200,
      "max_time_ms": 800
    }
  },
  "error_rates": {
    "dispense_session_creation": {
      "total": 1250,
      "errors": 0,
      "error_rate": 0.0
    },
    "immediate_sale_processing": {
      "total": 1250,
      "errors": 100,
      "error_rate": 8.0
    }
  },
  "health_checks": {
    "database_write": true,
    "database_read": true,
    "elasticsearch": true,
    "cache": true
  }
}
```