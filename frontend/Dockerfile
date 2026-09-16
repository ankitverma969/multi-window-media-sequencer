# ==============================================================================
# Production Dockerfile for React Frontend (EVA Bharat Media Sequencer)
# Uses Node.js to build and serve the production bundle via Vite Preview.
# Completely eliminates any Nginx dependency.
# ==============================================================================
FROM node:20-alpine

WORKDIR /app

# Cache dependencies
COPY package*.json ./
RUN npm ci

# Copy application source code
COPY . .

# Build optimized production bundle to /app/dist
RUN npm run build

EXPOSE 5173

HEALTHCHECK --interval=15s --timeout=5s --start-period=5s --retries=3 \
    CMD wget -q --spider http://localhost:5173/ || exit 1

# Serve production assets directly via Vite Preview
CMD ["npm", "run", "preview", "--", "--host", "0.0.0.0", "--port", "5173"]
