# APEX.BUILD Preview Container - Python
# Secure, minimal image for Python application previews
# Supports Flask, Django, FastAPI, and static file serving

FROM python:3.12-slim-bookworm

# Create non-root user for security
RUN groupadd -r sandbox && \
    useradd -r -g sandbox -d /home/sandbox -s /sbin/nologin sandbox && \
    mkdir -p /home/sandbox && \
    chown -R sandbox:sandbox /home/sandbox

# Install essential packages
RUN apt-get update && \
    apt-get install -y --no-install-recommends \
        curl \
        ca-certificates && \
    rm -rf /var/lib/apt/lists/* /tmp/* /var/tmp/*

# Create app directory with proper permissions
RUN mkdir -p /app && chown -R sandbox:sandbox /app

# Set working directory
WORKDIR /app

# Copy project files with sandbox ownership
COPY --chown=sandbox:sandbox . .

# Install dependencies if requirements.txt exists
RUN if [ -f requirements.txt ]; then \
      pip install --no-cache-dir --user -r requirements.txt 2>/dev/null || true; \
    fi && \
    rm -rf /root/.cache /tmp/*

# Set Python path to include user packages
ENV PATH="/home/sandbox/.local/bin:$PATH"
ENV PYTHONUNBUFFERED=1
ENV PYTHONDONTWRITEBYTECODE=1

# Switch to non-root user
USER sandbox

# Expose default port
EXPOSE 5000

# Health check
HEALTHCHECK --interval=30s --timeout=3s --start-period=10s --retries=3 \
    CMD curl -f http://localhost:5000/ || exit 1

# Start server - detect framework and entry point
CMD if [ -f "app.py" ] && grep -q "Flask\|flask" app.py 2>/dev/null; then \
      exec python -m flask run --host=0.0.0.0 --port=5000; \
    elif [ -f "main.py" ] && grep -q "FastAPI\|fastapi" main.py 2>/dev/null; then \
      exec python -m uvicorn main:app --host=0.0.0.0 --port=5000; \
    elif [ -f "manage.py" ]; then \
      exec python manage.py runserver 0.0.0.0:5000 --noreload; \
    elif [ -f "app.py" ]; then \
      exec python app.py; \
    elif [ -f "main.py" ]; then \
      exec python main.py; \
    elif [ -f "server.py" ]; then \
      exec python server.py; \
    else \
      exec python -m http.server 5000 --bind 0.0.0.0; \
    fi
