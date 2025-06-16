
## Available Endpoints

### Health Status Endpoint
- **URL**: `/health`
- **Method**: `GET`
- **Description**: Provides a quick overview of service health with key indicators
- **Use Case**: Status dashboards, alerts, service health indicators

### Detailed Metrics Endpoint
- **URL**: `/metrics`
- **Method**: `GET`
- **Description**: Provides comprehensive metrics for all aspects of the service
- **Use Case**: Detailed dashboards, performance analysis, troubleshooting

## Data Structure

### Health Endpoint Response

```json
{
  "status": {
    "healthy": true,
    "uptime_seconds": 3600
  },
  "metrics": {
    "total_requests": 5000,
    "error_rate": 0.01,
    "operations_completed": 150,
    "operations_failed": 2,
    "messages_processed": 200,
    "messages_error": 1
  }
}
```

### Metrics Endpoint Response

```json
{
  "uptime_seconds": 3600,
  "counters": {
    "http_requests_total": 5000,
    "http_requests_success_total": 4950,
    "http_requests_error_total": 50,
    "operations_created_total": 160,
    "operations_completed_total": 150,
    "operations_failed_total": 2,
    "messages_sent_total": 300,
    "messages_received_total": 210,
    "messages_processed_total": 200,
    "messages_error_total": 1,
    "db_queries_total": 8500,
    "db_queries_error_total": 5,
    "errors_total": 58
  },
  "gauges": {
    "active_operations": 8,
    "active_operation_groups": 2,
    "pending_messages": 10,
    "system_memory_bytes": 52428800,
    "cpu_usage_percent": 12.5
  },
  "request_counts": {
    "/ops/op/{device_uid}": 2000,
    "/ops/op/{device_uid}/events": 1500,
    "/ops/opg/{truck_uid}": 1000,
    "/ops/opg/{truck_uid}/events": 500
  },
  "request_latencies_ms": {
    "/ops/op/{device_uid}": 45.2,
    "/ops/op/{device_uid}/events": 120.5,
    "/ops/opg/{truck_uid}": 52.1,
    "/ops/opg/{truck_uid}/events": 135.3
  },
  "operation_counts": {
    "create": 160,
    "update": 320,
    "complete": 150,
    "failed": 2,
    "cancel": 8,
    "event_processing": 480
  },
  "operation_latencies_ms": {
    "create": 75.3,
    "update": 90.2,
    "complete": 110.5,
    "failed": 250.8,
    "cancel": 45.6,
    "event_processing": 85.4
  },
  "message_bus_counts": {
    "send": 300,
    "receive": 210,
    "complete": 200,
    "reject": 10
  },
  "message_bus_latencies_ms": {
    "send": 65.2,
    "receive": 12.8,
    "complete": 8.5,
    "reject": 7.2
  },
  "database_query_counts": {
    "select": 6000,
    "insert": 800,
    "update": 1650,
    "delete": 50
  },
  "database_latencies_ms": {
    "select": 35.2,
    "insert": 75.8,
    "update": 80.3,
    "delete": 60.1
  },
  "error_counts": {
    "http": 50,
    "validation": 32,
    "database": 5,
    "message_bus": 1,
    "internal": 12
  },
  "runtime": {
    "goroutines": 25,
    "memory": {
      "alloc_bytes": 52428800,
      "total_alloc_bytes": 152428800,
      "sys_bytes": 72428800,
      "heap_objects": 42500,
      "gc_cycles": 85
    }
  }
}
```

## Recommended Visualizations

### 1. Service Health Dashboard

- **Components**:
  - Service Status Indicator (Boolean health status)
  - Uptime Counter/Gauge
  - Error Rate Gauge (with thresholds: <1% green, 1-5% yellow, >5% red)
  - Active Operations Count (gauge or number)
  - Active Operation Groups Count (gauge or number)

### 2. Operations Dashboard

- **Components**:
  - Operations Creation Rate (line chart over time)
  - Operations Completion Rate (line chart over time)
  - Operation Success Rate (pie chart: completed vs. failed)
  - Operation Latencies (bar chart by operation type)
  - Active Operations (time series)

### 3. API Performance Dashboard

- **Components**:
  - Request Volume by Endpoint (bar chart)
  - Latency by Endpoint (bar chart)
  - HTTP Status Code Distribution (pie chart)
  - Top 5 Slowest Endpoints (sorted bar chart)
  - Request Error Rate (time series)

### 4. Message Bus Dashboard

- **Components**:
  - Messages Sent/Received (line chart over time)
  - Message Processing Success Rate (gauge)
  - Message Processing Latency (line chart)
  - Pending Messages (gauge with thresholds)

### 5. Database Performance Dashboard

- **Components**:
  - Query Volume by Type (stacked bar chart)
  - Query Latency by Type (bar chart)
  - Database Error Rate (gauge)
  - Top Slow Queries (if available)

### 6. Error Analysis Dashboard

- **Components**:
  - Error Distribution by Type (pie chart)
  - Error Trend (line chart over time)
  - Error Rate by Operation Type (bar chart)
  - Top Error Sources (sorted list)

## Implementation Tips

### Polling Frequency

- **Health Endpoint**: Poll every 10-30 seconds
- **Metrics Endpoint**: Poll every 1-5 minutes
  
### Time Series Data

When building time series visualizations:
- Store timestamp with each metrics poll
- Calculate rates of change for counters
- Keep 24 hours of raw data and aggregate older data

### Alert Thresholds

Consider setting alerts for:
- Service unhealthy status
- Error rate > 5%
- Operation latency > 200ms
- Message processing errors > 1%
- Database errors > 0.1%

### Dashboard Layout

- Group related metrics together
- Use consistent color schemes (e.g., red for errors, green for success)
- Include timestamp of last update
- Add option to adjust time range (last hour, day, week)

