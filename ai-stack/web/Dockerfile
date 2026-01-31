# =============================================================================
# Personal AI OS - Web UI
# =============================================================================
FROM nginx:alpine

# Copy static files
COPY index.html /usr/share/nginx/html/
COPY nginx.conf /etc/nginx/conf.d/default.conf

# Health check
HEALTHCHECK --interval=30s --timeout=3s \
    CMD wget -q --spider http://localhost/ || exit 1

EXPOSE 80
