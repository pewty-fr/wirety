# Build stage
FROM node:26-alpine AS builder

WORKDIR /app

# Copy package files
COPY package*.json ./

# Install dependencies
RUN npm ci

# Copy source code
COPY . .

# Build the application
RUN npm run build

# Production stage
FROM nginx:alpine

# Copy built assets from builder stage
COPY --from=builder /app/dist /usr/share/nginx/html

# nginx configuration: the SPA, and /api proxied to the server when
# API_PROXY_ENABLED=true (see docker/40-wirety-api-proxy.sh).
COPY docker/default.conf /etc/nginx/conf.d/default.conf
COPY --chmod=755 docker/40-wirety-api-proxy.sh /docker-entrypoint.d/40-wirety-api-proxy.sh
RUN install -d -o nginx -g nginx /etc/nginx/wirety.d

EXPOSE 8080

CMD ["nginx", "-g", "daemon off;"]
