# Authentication System for Govee Monitoring System

This guide explains the authentication system implemented in the Govee Monitoring System to ensure that only authorized clients can send data to the server.

## Overview

The authentication system uses API keys to authenticate clients. Each client needs a valid API key to send data to the server.

## Authentication Types

1. **Admin API Key**: Has full access to all server functions including API key management
2. **Client-Specific API Keys**: Tied to specific client IDs
3. **Default API Key**: Optional shared key that can be used by all clients

## Server Configuration

When starting the server, you can configure authentication with these flags:

| Flag | Default | Description |
|------|---------|-------------|
| `-auth` | true | Enable API key authentication |
| `-admin-key` | auto-generated | Admin API key (generated if empty) |
| `-default-key` | auto-generated | Default API key for all clients (generated if empty) |
| `-allow-default` | false | Allow the default API key to be used |

### Examples

**Starting the server with authentication enabled:**
```bash
./govee-server -auth=true
```
The server will automatically generate and display the admin API key.

**Starting the server with pre-defined keys:**
```bash
./govee-server -admin-key=admin123 -default-key=default123 -allow-default=true
```

**Disabling authentication (not recommended for production):**
```bash
./govee-server -auth=false
```

## Client Configuration

Clients must provide their API key when sending data to the server:

```bash
./govee-client -server=http://server:8080/readings -apikey=YOUR_API_KEY -id=client-name
```

If no API key is provided, the client will warn you that server communications may fail.

**Note**: Authentication is not required when running the client in local or discovery mode:
```bash
./govee-client -local=true
# OR
./govee-client -discover
```

## API Key Management

API keys can be managed through the server's API (requires admin API key):

### List all API keys

```
GET /api/keys
Header: X-API-Key: <admin_key>
```

Example using curl:
```bash
curl -H "X-API-Key: your_admin_key" http://server:8080/api/keys
```

### Create a new API key

```
POST /api/keys
Header: X-API-Key: <admin_key>
Body: {"client_id": "client-name"}
```

Example using curl:
```bash
curl -X POST -H "X-API-Key: your_admin_key" -H "Content-Type: application/json" \
  -d '{"client_id": "client-bedroom"}' \
  http://server:8080/api/keys
```

The server will respond with the newly created API key:
```json
{
  "api_key": "abc123def456ghi789jkl0",
  "client_id": "client-bedroom"
}
```

### Delete an API key

```
DELETE /api/keys?key=<api_key_to_delete>
Header: X-API-Key: <admin_key>
```

Example using curl:
```bash
curl -X DELETE -H "X-API-Key: your_admin_key" \
  http://server:8080/api/keys?key=abc123def456ghi789jkl0
```

## Authentication Flow

1. Client obtains an API key (either from admin or using default key)
2. Client sends data to server with API key in `X-API-Key` header
3. Server validates the API key:
   - If admin key: full access granted
   - If client-specific key: validates that client ID matches
   - If default key (and allowed): access granted
4. If authentication fails, server returns 401 Unauthorized

## Client ID Validation

For client-specific API keys, the server validates that the client ID in the request matches the one associated with the API key. This prevents clients from impersonating each other.

For example, if an API key is created for "client-bedroom", it can only be used to send data with that client ID. If a different client ID is used with this API key, the server will reject the request.

## Security Considerations

- API keys are stored in the server's data directory if persistence is enabled
- Admin API key grants full access, so keep it secure
- Client-specific keys can only be used with their associated client ID
- The default API key (if enabled) is less secure as it can be used by any client
- In production environments:
  - Use HTTPS to encrypt API key transmission
  - Generate strong API keys (the server does this by default)
  - Disable the default API key option if possible
  - Regularly rotate API keys for better security

## Troubleshooting Authentication Issues

### Server-side issues:

1. **Check if authentication is enabled:**
   - Verify the `-auth` flag is set to `true`
   - Check server logs for authentication status

2. **Verify API keys are loaded:**
   - Check server logs for "Loaded X API keys from storage"
   - Restart server if keys are not loading

3. **Inspect authentication failures in logs:**
   - Look for "Authentication failed" messages
   - Check for client ID mismatches

### Client-side issues:

1. **Verify API key is being sent:**
   - Check client logs for authentication failures
   - Ensure `-apikey` flag is set correctly

2. **Check client ID:**
   - Make sure client ID matches the one associated with the API key
   - If using custom client ID, ensure it's the same one registered with the API key

3. **Test with default key if allowed:**
   - If server allows default key, try using it for testing

## Best Practices

1. **Create specific keys for each client:** Avoid sharing API keys between clients
2. **Use meaningful client IDs:** Name clients based on location or purpose for easier management
3. **Secure admin key:** Store the admin key securely and limit its use to administrative tasks
4. **Monitor authentication failures:** Regularly check server logs for unusual authentication patterns
