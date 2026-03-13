# ── Build stage ──────────────────────────────────────────────────────────────
FROM node:20-alpine AS builder
WORKDIR /app
COPY package.json ./
RUN npm install
COPY . .
ARG PUBLIC_API_URL=http://backend:8080
ENV PUBLIC_API_URL=$PUBLIC_API_URL
RUN npm run build

# ── Runtime stage — serve with node ──────────────────────────────────────────
FROM node:20-alpine
WORKDIR /app
COPY --from=builder /app/dist ./dist
COPY --from=builder /app/node_modules ./node_modules
COPY --from=builder /app/package.json .
EXPOSE 3000
CMD ["node", "./dist/server/entry.mjs"]
