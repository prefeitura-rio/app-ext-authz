# Envoy External Authorization Service for reCAPTCHA Enterprise

A high-performance external authorization service for Envoy Proxy that validates reCAPTCHA Enterprise tokens. This service integrates with Envoy's `ext_authz` filter to provide reCAPTCHA Enterprise validation for your APIs.

## Features

- **Enterprise-grade security**: Google Cloud reCAPTCHA Enterprise integration
- **High performance**: HTTP-based authorization with caching
- **Resilient**: Circuit breaker pattern with graceful degradation
- **Observable**: Full OpenTelemetry integration with traces, metrics, and logs
- **Configurable**: Environment-based configuration
- **Tested**: Comprehensive test suite with mocks
- **Containerized**: Ready for Kubernetes deployment

## Architecture

```
Client Request → Envoy Proxy → ext_authz Filter → This Service → Google reCAPTCHA Enterprise API
                                                      ↓
                                              Cache (Redis)
                                                      ↓
                                              Circuit Breaker
                                                      ↓
                                              OpenTelemetry (SignOz)
```

### Request Flow

1. Client sends request with `X-Recaptcha-Token` header
2. Envoy intercepts and calls this authorization service
3. Service validates token with Google's reCAPTCHA Enterprise API
4. Returns ALLOW/DENY decision to Envoy
5. Envoy forwards or blocks the request accordingly

## Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `RECAPTCHA_PROJECT_ID` | Google Cloud project ID | - | Yes |
| `RECAPTCHA_SITE_KEY` | reCAPTCHA site key | - | Yes |
| `RECAPTCHA_ACTION` | Expected action name | authz | No |
| `RECAPTCHA_V3_THRESHOLD` | Score threshold (0.0-1.0) for Enterprise | 0.5 | No |
| `GOOGLE_API_TIMEOUT_SECONDS` | Timeout for Google API calls | 5 | No |
| `CACHE_TTL_SECONDS` | Cache TTL for successful validations | 30 | No |
| `CACHE_FAILED_TTL_SECONDS` | Cache TTL for failed validations | 300 | No |
| `REDIS_URL` | Redis connection URL | redis://localhost:6379 | Yes |
| `FAILURE_MODE` | Failure mode (fail_open/fail_closed) | fail_open | No |
| `CIRCUIT_BREAKER_ENABLED` | Enable circuit breaker | true | No |
| `CIRCUIT_BREAKER_FAILURE_THRESHOLD` | Failures before opening circuit | 5 | No |
| `CIRCUIT_BREAKER_RECOVERY_TIME_SECONDS` | Recovery time for circuit breaker | 60 | No |
| `HEALTH_CHECK_INTERVAL_SECONDS` | Health check interval | 30 | No |
| `OTEL_ENDPOINT` | OpenTelemetry endpoint | - | No |
| `OTEL_SERVICE_NAME` | Service name for telemetry | recaptcha-authz | No |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | info | No |
| `PORT` | HTTP server port | 8080 | No |

### Example Configuration

```bash
RECAPTCHA_PROJECT_ID=your-project-id
RECAPTCHA_SITE_KEY=your_site_key_here
RECAPTCHA_ACTION=authz
RECAPTCHA_V3_THRESHOLD=0.7
GOOGLE_API_TIMEOUT_SECONDS=5
CACHE_TTL_SECONDS=30
REDIS_URL=redis://localhost:6379
FAILURE_MODE=fail_open
CIRCUIT_BREAKER_ENABLED=true
OTEL_ENDPOINT=http://signoz:4317
```

## API Endpoints

### Authorization Endpoint

**POST** `/authz`

This is the main endpoint that Envoy calls for authorization decisions.

**Request Headers:**
- `X-Recaptcha-Token`: The reCAPTCHA token to validate

**Response:**
- **200 OK**: Request allowed
- **403 Forbidden**: Request denied
- **500 Internal Server Error**: Service error

**Response Headers:**
- `X-Recaptcha-Status`: `valid|invalid|degraded|timeout`
- `X-Recaptcha-Score`: Score value (Enterprise)
- `X-Recaptcha-Cache`: `hit|miss`

### Health Check

**GET** `/health`

Returns service health status.

**Response:**
```json
{
  "status": "healthy",
  "timestamp": "2024-01-01T00:00:00Z",
  "google_api": "healthy",
  "circuit_breaker": "closed"
}
```

### Metrics

**GET** `/metrics`

Prometheus metrics endpoint.

## Envoy Configuration

### HTTP Mode

```yaml
http_filters:
- name: envoy.filters.http.ext_authz
  typed_config:
    "@type": type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz
    transport_api_version: V3
    http_service:
      server_uri:
        uri: "http://recaptcha-authz:8080"
        cluster: "recaptcha_authz"
        timeout: 2s
      authorization_request:
        allowed_headers:
          patterns:
          - exact: "x-recaptcha-token"
      authorization_response:
        allowed_upstream_headers:
          patterns:
          - exact: "x-recaptcha-status"
          - exact: "x-recaptcha-score"
          - exact: "x-recaptcha-cache"
```

## Development

### Prerequisites

- Go 1.21+
- Docker
- Nix (for development environment)

### Local Development

1. **Setup with direnv (recommended):**
   ```bash
   # Install direnv if not already installed
   # macOS: brew install direnv
   # Linux: sudo apt install direnv
   
   # Allow direnv in this directory
   direnv allow
   
   # This will automatically load the Nix flake and environment variables
   ```

2. **Setup with Nix (manual):**
   ```bash
   nix develop
   ```

2. **Run locally:**
   ```bash
   just run
   ```

3. **Run tests:**
   ```bash
   just test
   ```

4. **Run with Docker:**
   ```bash
   just docker-build
   just docker-run
   ```

### Testing

The project includes comprehensive tests:

- **Unit tests**: Core validation logic
- **Integration tests**: HTTP endpoints
- **Load tests**: Performance testing with mocks
- **Mock mode**: Bypass Google API for development

Run all tests:
```bash
just test
just test-load
```

## Deployment

### Docker

```bash
docker build -t recaptcha-authz .
docker run -p 8080:8080 \
  -e RECAPTCHA_PROJECT_ID=your-project-id \
  -e RECAPTCHA_SITE_KEY=your_site_key \
  -e RECAPTCHA_ACTION=authz \
  recaptcha-authz
```

### Kubernetes

1. **Create secret (if needed for additional configuration):**
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: recaptcha-config
   type: Opaque
   data:
     # Add any additional secrets if needed
   ```

2. **Deploy service:**
   ```bash
   kubectl apply -f k8s/
   ```

3. **Scaling Strategy:**
   - **HPA (Horizontal Pod Autoscaler)**: Scales based on CPU/memory usage
   - **VPA (Vertical Pod Autoscaler)**: Optimizes resource requests/limits
   - **PDB (Pod Disruption Budget)**: Ensures availability during scaling
   - **Redis**: Shared cache across all pods for better performance

## Monitoring

### Metrics

Key metrics available:
- `recaptcha_requests_total`: Total requests processed
- `recaptcha_validations_total`: Validation attempts
- `recaptcha_cache_hits_total`: Cache hit rate
- `recaptcha_google_api_duration_seconds`: Google API response time
- `recaptcha_circuit_breaker_state`: Circuit breaker status

### Alerts

Recommended alerts:
- High error rate (>5%)
- Circuit breaker trips
- Google API timeouts
- High response latency (>2s)

## Failure Handling

### Circuit Breaker

The service implements a circuit breaker pattern:
- **Closed**: Normal operation
- **Open**: Stop calling Google API, return degraded responses
- **Half-open**: Test Google API before resuming normal operation

### Graceful Degradation

When Google API is unavailable:
- Return `ALLOW` with `X-Recaptcha-Status: degraded`
- Continue serving requests to prevent complete outage
- Monitor and alert on degraded state

## Security Considerations

- **Secret management**: Use Kubernetes secrets for sensitive data
- **Network security**: Restrict access to authorization service
- **Rate limiting**: Implement at Envoy level
- **Monitoring**: Monitor for abuse and unusual patterns

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details. 