# Deployment Guide

## Dockerfile Structure

This repository contains multiple Dockerfiles for different purposes:

### Root Level (Production/Deployment)
- `Dockerfile.zencrm.deploy` - ZenCRM deployment (build context: root)
- `Dockerfile.allfunds.deploy` - Allfunds deployment (build context: root)

These are used for:
- DigitalOcean App Platform
- Docker Compose
- Any CI/CD pipeline

**Build context must be repository root** to access both `shared/` and `services/` directories.

### Service Level (Development)
- `services/zencrm/Dockerfile` - Local ZenCRM builds
- `services/allfunds/Dockerfile` - Local Allfunds builds

These are for local development only and require root build context.

## DigitalOcean App Platform

### Option 1: Using App Spec (Recommended)

```bash
# Create app from spec
doctl apps create --spec .do/app.yaml

# Update existing app
doctl apps update <app-id> --spec .do/app.yaml
```

### Option 2: Manual Setup

1. **Create New App**
   - Go to DigitalOcean dashboard
   - Click "Create" → "Apps"
   - Select GitHub repository: `Agusmazzeo/MCP-Servers`

2. **Configure ZenCRM Service**
   - Name: `zencrm-mcp`
   - Dockerfile path: `Dockerfile.zencrm.deploy`
   - Build context: `/` (root)
   - HTTP port: `8080`
   - Health check path: `/health`
   - Environment variables:
     - `ZENCRM_API_URL` (secret)
     - `ZENCRM_API_KEY` (secret)

3. **Configure Allfunds Service**
   - Name: `allfunds-mcp`
   - Dockerfile path: `Dockerfile.allfunds.deploy`
   - Build context: `/` (root)
   - HTTP port: `8081`
   - Health check path: `/health`
   - Environment variables:
     - `GRAPHQL_URL` = `https://api.allfunds.dev/graphql`
     - `EMAIL` (secret)
     - `PASSWORD` (secret)

4. **Deploy**
   - Review settings
   - Click "Create Resources"
   - Wait for deployment

### Environment Variables

Set these as encrypted secrets in DigitalOcean:

**ZenCRM:**
```
ZENCRM_API_URL=<your-zencrm-api-url>
ZENCRM_API_KEY=<your-api-key>
```

**Allfunds:**
```
GRAPHQL_URL=https://api.allfunds.dev/graphql
EMAIL=<your-allfunds-email>
PASSWORD=<your-allfunds-password>
```

### Troubleshooting

**Build fails with "no such file or directory":**
- Ensure build context is set to `/` (root)
- Verify Dockerfile path is correct (e.g., `Dockerfile.allfunds.deploy`)
- Check that all paths in Dockerfile are relative to root

**Service won't start:**
- Check environment variables are set correctly
- Verify health check endpoint `/health` is accessible
- Review application logs in DigitalOcean dashboard

## Docker Compose (Local)

```bash
# Create .env file with credentials
cp .env.example .env
# Edit .env with your values

# Start all services
docker-compose up --build

# Start specific service
docker-compose up zencrm
docker-compose up allfunds

# Stop all services
docker-compose down
```

## Direct Docker Build

```bash
# ZenCRM
docker build -f Dockerfile.zencrm.deploy -t zencrm-mcp .
docker run -p 8080:8080 \
  -e ZENCRM_API_URL=$ZENCRM_API_URL \
  -e ZENCRM_API_KEY=$ZENCRM_API_KEY \
  zencrm-mcp

# Allfunds
docker build -f Dockerfile.allfunds.deploy -t allfunds-mcp .
docker run -p 8081:8081 \
  -e GRAPHQL_URL=https://api.allfunds.dev/graphql \
  -e EMAIL=$EMAIL \
  -e PASSWORD=$PASSWORD \
  allfunds-mcp
```

## Health Checks

Both services expose health check endpoints:

- ZenCRM: `http://localhost:8080/health`
- Allfunds: `http://localhost:8081/health`

Expected response:
```json
{
  "status": "healthy",
  "server": "zencrm",
  "version": "1.0.0"
}
```

## Monitoring

### DigitalOcean Metrics
- CPU usage
- Memory usage
- Request rate
- Response time

### Application Logs
Access via DigitalOcean dashboard or CLI:
```bash
doctl apps logs <app-id> --type run
```

## Scaling

Adjust in `.do/app.yaml`:
```yaml
instance_count: 2  # Number of instances
instance_size_slug: basic-xs  # Instance size
```

Available sizes:
- `basic-xxs` - 512 MB RAM, 0.25 vCPU
- `basic-xs` - 1 GB RAM, 1 vCPU
- `basic-s` - 2 GB RAM, 1 vCPU
- `basic-m` - 4 GB RAM, 2 vCPU
