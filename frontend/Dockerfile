# syntax=docker/dockerfile:1.7

FROM --platform=$BUILDPLATFORM node:20-alpine AS builder
WORKDIR /app
COPY package*.json ./
RUN --mount=type=cache,target=/root/.npm npm ci || npm install
COPY . .
RUN npm run build

FROM nginx:1.27-alpine AS final
RUN apk add --no-cache jq
COPY --from=builder /app/build /usr/share/nginx/html
COPY .docker/nginx.conf /etc/nginx/conf.d/default.conf
COPY .docker/inject-env.sh /docker-entrypoint.d/01-inject-env.sh
RUN chmod +x /docker-entrypoint.d/01-inject-env.sh
EXPOSE 80
