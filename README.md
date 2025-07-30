# Envoy External Authorization Service for reCAPTCHA Enterprise

A high-performance external authorization service for Envoy Proxy that validates reCAPTCHA Enterprise tokens. This service integrates with Envoy's `ext_authz` filter to provide reCAPTCHA Enterprise validation for your APIs.

## Features

- **Enterprise-grade security**: Google Cloud reCAPTCHA Enterprise integration
- **High performance**: HTTP/gRPC-based authorization with caching
- **Resilient**: Circuit breaker pattern with graceful degradation
- **Observable**: Full OpenTelemetry integration with traces, metrics, and logs
- **Configurable**: Environment-based configuration
- **Mock mode**: Development-friendly testing without Google API calls
- **Containerized**: Ready for Kubernetes deployment
- **Safe tracing**: Panic-protected OpenTelemetry integration

## Architecture

```
Client Request → Envoy Proxy → ext_authz Filter → This Service → Google reCAPTCHA Enterprise API
                                                      ↓
                                              Cache (Redis)
                                                      ↓
                                              Circuit Breaker
                                                      ↓
                                              OpenTelemetry (SignOz)
                                                      ↓
                                              Service Account Auth
```

### Request Flow

1. Client sends request with `X-Recaptcha-Token` header
2. Envoy intercepts and calls this authorization service
3. Service authenticates with Google Cloud using service account
4. Service validates token with Google's reCAPTCHA Enterprise API
5. Returns ALLOW/DENY decision to Envoy
6. Envoy forwards or blocks the request accordingly

## Configuration

### Environment Variables

| Variable | Description | Default | Required |
|----------|-------------|---------|----------|
| `RECAPTCHA_PROJECT_ID` | Google Cloud project ID | - | Yes |
| `RECAPTCHA_SITE_KEY` | reCAPTCHA site key | - | Yes |
| `RECAPTCHA_ACTION` | Expected action name | authz | No |
| `RECAPTCHA_V3_THRESHOLD` | Score threshold (0.0-1.0) for Enterprise | 0.5 | No |
| `GOOGLE_SERVICE_ACCOUNT_KEY` | Base64 encoded service account JSON | - | Yes |
| `GOOGLE_API_TIMEOUT_SECONDS` | Timeout for Google API calls | 5 | No |
| `CACHE_TTL_SECONDS` | Cache TTL for successful validations | 30 | No |
| `CACHE_FAILED_TTL_SECONDS` | Cache TTL for failed validations | 300 | No |
| `REDIS_URL` | Redis connection URL | redis://localhost:6379 | Yes |
| `FAILURE_MODE` | Failure mode (fail_open/fail_closed) | fail_open | No |
| `CIRCUIT_BREAKER_ENABLED` | Enable circuit breaker | true | No |
| `CIRCUIT_BREAKER_FAILURE_THRESHOLD` | Failures before opening circuit | 5 | No |
| `CIRCUIT_BREAKER_RECOVERY_TIME_SECONDS` | Recovery time for circuit breaker | 60 | No |
| `HEALTH_CHECK_INTERVAL_SECONDS` | Health check interval | 30 | No |
| `OTEL_ENDPOINT` | OpenTelemetry endpoint (gRPC) | - | No |
| `OTEL_SERVICE_NAME` | Service name for telemetry | recaptcha-authz | No |
| `LOG_LEVEL` | Log level (debug, info, warn, error) | info | No |
| `MOCK_MODE` | Enable mock mode for development | false | No |

### Example Configuration

```bash
RECAPTCHA_PROJECT_ID=your-project-id
RECAPTCHA_SITE_KEY=your_site_key_here
RECAPTCHA_ACTION=authz
RECAPTCHA_V3_THRESHOLD=0.7
GOOGLE_SERVICE_ACCOUNT_KEY=eyJ0eXBlIjoic2VydmljZV9hY2NvdW50Iiwi...
GOOGLE_API_TIMEOUT_SECONDS=5
CACHE_TTL_SECONDS=30
REDIS_URL=redis://localhost:6379
FAILURE_MODE=fail_open
CIRCUIT_BREAKER_ENABLED=true
OTEL_ENDPOINT=signoz:4317
MOCK_MODE=false
```

### Service Account Setup

The service requires a Google Cloud service account with reCAPTCHA Enterprise permissions:

1. **Create service account:**
   ```bash
   gcloud iam service-accounts create recaptcha-authz \
       --display-name="reCAPTCHA Authorization Service"
   ```

2. **Grant reCAPTCHA Enterprise permissions:**
   ```bash
   gcloud projects add-iam-policy-binding YOUR_PROJECT_ID \
       --member="serviceAccount:recaptcha-authz@YOUR_PROJECT_ID.iam.gserviceaccount.com" \
       --role="roles/recaptchaenterprise.agent"
   ```

3. **Create and encode service account key:**
   ```bash
   gcloud iam service-accounts keys create key.json \
       --iam-account=recaptcha-authz@YOUR_PROJECT_ID.iam.gserviceaccount.com
   
   base64 -i key.json | tr -d '\n'
   ```

4. **Set environment variable:**
   ```bash
   export GOOGLE_SERVICE_ACCOUNT_KEY="eyJ0eXBlIjoic2VydmljZV9hY2NvdW50Iiwi..."
   ```

## API Endpoints

### Authorization Endpoint

The service implements the Envoy `ext_authz` filter interface, supporting both HTTP and gRPC protocols.

**HTTP Mode:**
- **Port**: 8000
- **Method**: POST
- **Headers**: `X-Recaptcha-Token` (required)

**gRPC Mode:**
- **Port**: 9000
- **Service**: `envoy.service.auth.v3.Authorization`
- **Method**: `Check`

**Request Headers:**
- `X-Recaptcha-Token`: The reCAPTCHA Enterprise token to validate

**Response:**
- **200 OK**: Request allowed
- **403 Forbidden**: Request denied

**Response Headers:**
- `X-Ext-Authz-Check-Result`: `allowed|denied`
- `X-Ext-Authz-Check-Received`: Request details

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
        uri: "http://recaptcha-authz:8000"
        cluster: "recaptcha_authz"
        timeout: 2s
      authorization_request:
        allowed_headers:
          patterns:
          - exact: "x-recaptcha-token"
      authorization_response:
        allowed_upstream_headers:
          patterns:
          - exact: "x-ext-authz-check-result"
          - exact: "x-ext-authz-check-received"
```

### gRPC Mode

```yaml
http_filters:
- name: envoy.filters.http.ext_authz
  typed_config:
    "@type": type.googleapis.com/envoy.extensions.filters.http.ext_authz.v3.ExtAuthz
    transport_api_version: V3
    grpc_service:
      envoy_grpc:
        cluster_name: "recaptcha_authz_grpc"
      timeout: 2s
```

## Development

### Prerequisites

- Go 1.24+
- Docker
- Nix (for development environment)
- Google Cloud service account with reCAPTCHA Enterprise permissions

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

3. **Run in mock mode (for development):**
   ```bash
   just run-mock
   ```
   
   Mock mode bypasses Google API calls and uses predefined responses for testing.

4. **Run with Docker:**
   ```bash
   just docker-compose
   ```

### Testing

The service can be tested using curl or grpcurl:

**HTTP Mode:**
```bash
curl -X POST http://localhost:8000 \
  -H "X-Recaptcha-Token: your_token_here" \
  -v
```

**Test with justfile:**
```bash
just test-curl-http
just test-curl-grpc
```

**gRPC Mode (requires grpcurl):**
```bash
grpcurl -plaintext \
  -d '{"attributes": {"request": {"http": {"headers": {"x-recaptcha-token": "your_token_here"}}}}}' \
  localhost:9000 envoy.service.auth.v3.Authorization/Check
```

**Using justfile:**
```bash
just test-curl-http
just test-curl-grpc
```

## Deployment

### Docker

```bash
docker build -t recaptcha-authz .
docker run -p 8000:8000 -p 9000:9000 \
  -e RECAPTCHA_PROJECT_ID=your-project-id \
  -e RECAPTCHA_SITE_KEY=your_site_key \
  -e GOOGLE_SERVICE_ACCOUNT_KEY=eyJ0eXBlIjoic2VydmljZV9hY2NvdW50Iiwi... \
  -e RECAPTCHA_ACTION=authz \
  recaptcha-authz --http=8000 --grpc=9000
```

### Kubernetes

1. **Create secret with service account key:**
   ```yaml
   apiVersion: v1
   kind: Secret
   metadata:
     name: recaptcha-authz-secrets
   type: Opaque
   data:
     google-service-account-key: <base64-encoded-service-account-json>
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

## Recent Improvements

### Safe Tracing
- **Panic protection**: OpenTelemetry integration with comprehensive panic recovery
- **Graceful degradation**: Service continues working even if telemetry fails
- **Nil checks**: Robust handling of nil pointers throughout the codebase

### Service Account Authentication
- **Base64 encoding**: Kubernetes-friendly service account key storage
- **Automatic decoding**: Service automatically decodes and uses service account credentials
- **Permission management**: Clear documentation for required IAM roles

### Error Handling
- **Nil pointer protection**: Comprehensive nil checks in reCAPTCHA validation
- **Circuit breaker safety**: Enhanced circuit breaker with proper state management
- **Graceful failures**: Service handles all error scenarios without panicking

## Security Considerations

- **Secret management**: Use Kubernetes secrets for sensitive data
- **Network security**: Restrict access to authorization service
- **Rate limiting**: Implement at Envoy level
- **Monitoring**: Monitor for abuse and unusual patterns
- **Service account security**: Rotate service account keys regularly

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests
5. Submit a pull request

## License

MIT License - see LICENSE file for details. 