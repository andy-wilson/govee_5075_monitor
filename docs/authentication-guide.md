# Authentication guide for Govee 5075 Monitor

**Version 2.0** - Enhanced Security Features

This guide explains the security features of the Govee 5075 Monitoring System, covering API key authentication, HTTPS encryption, and additional security improvements in v2.0.

## Security Overview

The system implements multiple layers of security:

1. **API Key Authentication** - Verifies that clients are authorized to communicate with the server
2. **HTTPS/TLS Encryption** - Encrypts all data in transit between clients and the server
3. **Input Validation** - Prevents XSS, path traversal, and injection attacks (v2.0)
4. **Security Headers** - Comprehensive HTTP security headers (v2.0)
5. **Rate Limiting** - Prevents abuse and DoS attacks

For maximum security, all features should be enabled and properly configured.

### New in v2.0

- **Cryptographically Secure API Keys**: Generated using `crypto/rand` for unpredictability
- **XSS Prevention**: Device names and inputs are validated and sanitized
- **Security Headers**: CSP, HSTS, X-Frame-Options, and more
- **Enhanced Health Checks**: Monitor security status via `/health` endpoint
- **Audit Capabilities**: Better logging for security events

## API Key Authentication

### How Authentication Works

The Govee Monitoring System uses API keys to control access to server resources:

- Each client must provide a valid API key with every request
- API keys are associated with specific client IDs
- The server validates both the API key and the client ID

### Authentication Types

The system supports three categories of API keys:

1. **Admin API Key**: Has full access to all server functions including API key management
2. **Client-Specific API Keys**: Tied to specific client IDs, allows clients to send data
3. **Default API Key**: Optional shared key that can be used by all clients (less secure)

### Setting Up Authentication

#### Server Configuration

When starting the server, enable authentication with these flags:

```bash
./govee-server -auth=true -admin-key=YOUR_ADMIN_KEY
```

Available authentication flags:

| Flag | Default | Description |
|------|---------|-------------|
| `-auth` | true | Enable API key authentication |
| `-admin-key` | auto-generated | Admin API key (generated if empty) |
| `-default-key` | auto-generated | Default API key for all clients |
| `-allow-default` | false | Allow the default API key to be used |

#### Managing API Keys

You can manage client API keys using the API (requires admin key):

**List all API keys:**
```bash
curl -H "X-API-Key: <admin_key>" http://server:8080/api/keys
```

**Create a new API key:**
```bash
curl -X POST -H "X-API-Key: <admin_key>" -H "Content-Type: application/json" \
  -d '{"client_id": "client-kitchen"}' \
  http://server:8080/api/keys
```

**Delete an API key:**
```bash
curl -X DELETE -H "X-API-Key: <admin_key>" \
  http://server:8080/api/keys?key=<api_key_to_delete>
```

#### Client Configuration

Clients must provide their API key when sending data:

```bash
./govee-client -server=http://server:8080/readings -apikey=YOUR_API_KEY -id=client-kitchen
```

## HTTPS Encryption

HTTPS (HTTP Secure) uses Transport Layer Security (TLS) to encrypt all communications:

- Prevents eavesdropping on sensitive data (API keys, readings)
- Verifies the identity of the server to prevent man-in-the-middle attacks
- Works alongside API key authentication for a layered security approach

### Setting Up HTTPS

#### 1. Generate Certificates

You can use the provided script to generate self-signed certificates:

```bash
./generate_cert.sh --name your-server-hostname --ip your-server-ip
```

This creates:
- A Certificate Authority (CA) certificate (`ca.crt`)
- A server certificate (`cert.pem`)
- A private key (`key.pem`)

For production, consider obtaining certificates from a trusted certificate authority.

#### 2. Configure the Server for HTTPS

Enable HTTPS on the server:

```bash
./govee-server -https=true -cert=./certs/cert.pem -key=./certs/key.pem
```

HTTPS flags:

| Flag | Default | Description |
|------|---------|-------------|
| `-https` | false | Enable HTTPS |
| `-cert` | cert.pem | Path to TLS certificate file |
| `-key` | key.pem | Path to TLS key file |

#### 3. Configure Clients for HTTPS

Clients must be configured to use HTTPS and verify the server's certificate:

```bash
./govee-client -server=https://server:8080/readings -ca-cert=./certs/ca.crt -apikey=YOUR_API_KEY
```

HTTPS client flags:

| Flag | Default | Description |
|------|---------|-------------|
| `-ca-cert` | "" | Path to CA certificate file |
| `-insecure` | false | Skip certificate verification (NOT recommended for production) |

## Using Both Security Layers Together

For maximum security, enable both authentication and HTTPS:

### Server Configuration

```bash
./govee-server -https=true -cert=./certs/cert.pem -key=./certs/key.pem -auth=true
```

### Client Configuration

```bash
./govee-client -server=https://server:8080/readings \
  -ca-cert=./certs/ca.crt \
  -apikey=YOUR_API_KEY \
  -id=client-kitchen
```

### Docker Configuration

For Docker deployments:

```yaml
services:
  govee-server:
    # ...other settings...
    volumes:
      - ./certs:/app/certs
    environment:
      - HTTPS=true
      - CERT=/app/certs/cert.pem
      - KEY=/app/certs/key.pem
      - AUTH=true
      - ADMIN_KEY=${ADMIN_KEY:-admin-key-here}

  govee-client:
    # ...other settings...
    environment:
      - SERVER_URL=https://server:8080/readings
      - APIKEY=${CLIENT_APIKEY:-your_api_key_here}
      - CA_CERT=/app/certs/ca.crt
    volumes:
      - ./certs:/app/certs
```

## Testing Security Configuration

### Testing Authentication

1. **Try with invalid API key:**
   ```bash
   ./govee-client -server=http://server:8080/readings -apikey=INVALID_KEY
   ```
   
   You should see an "Authentication failed" error.

2. **Try with wrong client ID:**
   ```bash
   ./govee-client -server=http://server:8080/readings -apikey=VALID_KEY -id=wrong-client-id
   ```
   
   You should see a "Client ID mismatch" error.

### Testing HTTPS

1. **Verify certificate details:**
   ```bash
   openssl x509 -in certs/cert.pem -text -noout
   ```
   
   Check that the hostname matches your server.

2. **Test HTTPS connection:**
   ```bash
   curl -v --cacert certs/ca.crt https://server:8080/health
   ```
   
   You should see "OK" with TLS handshake details.

## Troubleshooting

### Authentication Issues

1. **"Unauthorized: API key required"**
   - Verify API key is being included in the request header
   - Check that the `-apikey` flag is set correctly

2. **"Unauthorized: Invalid API key"**
   - Verify the API key exists on the server
   - Try regenerating the API key

3. **"Unauthorized: Client ID mismatch"**
   - Ensure client ID matches the one registered with the API key
   - Check for typos or case sensitivity issues

### HTTPS/TLS Issues

1. **Certificate verification failure**
   - Verify server hostname matches the certificate's Common Name or SAN
   - Ensure CA certificate is correctly provided to the client
   - Check certificate expiration dates

2. **"Connection refused" or timeouts**
   - Verify the server is listening on the correct port
   - Check firewall rules for HTTPS port
   - Try using curl with `-k` to bypass verification for testing

## Security Best Practices

1. **Use both authentication and HTTPS** for defense in depth
2. **Generate unique API keys** for each client
3. **Rotate API keys periodically** for better security
4. **Use trusted certificates** in production environments
5. **Keep private keys secure** with appropriate permissions
6. **Monitor authentication logs** for unusual activity
7. **Disable the default API key** in production
8. **Use environment variables** for API keys in Docker deployments
9. **Back up certificates and keys** securely
10. **Implement rate limiting** to prevent brute force attacks

## Appendix: Endpoint Security

| Endpoint | Auth Required | Description |
|----------|---------------|-------------|
| `/readings` | Yes | Add/get sensor readings |
| `/devices` | Yes | Get device information |
| `/clients` | Yes | Get client information |
| `/stats` | Yes | Get statistics |
| `/dashboard/data` | Yes | Dashboard data |
| `/api/keys` | Admin only | Manage API keys |
| `/health` | No | Health check endpoint |
| `/` | No | Static dashboard files |
