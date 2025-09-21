# API Reference

This page contains comprehensive API documentation for our platform.

## Authentication
All API requests require authentication using Bearer tokens.

```bash
curl -H "Authorization: Bearer YOUR_TOKEN" https://api.example.com/endpoint
```

## Base URL
```
https://api.example.com/v1
```

## Endpoints

### Users
- **GET /api/users** - Retrieve all users
- **GET /api/users/{id}** - Retrieve specific user
- **POST /api/users** - Create new user
- **PUT /api/users/{id}** - Update existing user
- **DELETE /api/users/{id}** - Delete user

### Projects
- **GET /api/projects** - List all projects
- **POST /api/projects** - Create new project

## Response Format
All responses are returned in JSON format:

```json
{
  "success": true,
  "data": {},
  "message": "Operation completed successfully"
}
```

## Error Handling
Standard HTTP status codes are used:
- 200: Success
- 400: Bad Request
- 401: Unauthorized
- 404: Not Found
- 500: Internal Server Error