FROM python:3.12-slim
WORKDIR /app
COPY requirements.txt .
ARG APT_MIRROR=deb.debian.org
ARG PIP_INDEX_URL=https://pypi.org/simple
RUN sed -i "s|deb.debian.org|${APT_MIRROR}|g" /etc/apt/sources.list.d/debian.sources
RUN apt-get update && apt-get install -y --no-install-recommends ca-certificates libgomp1 && rm -rf /var/lib/apt/lists/*
RUN pip install --index-url "$PIP_INDEX_URL" --no-cache-dir -r requirements.txt && useradd -m -u 10001 embedding && mkdir -p /data/models && chown embedding /data/models
COPY server.py .
USER embedding
ENV BIND=0.0.0.0 PORT=9981 MODEL_CACHE=/data/models
EXPOSE 9981
HEALTHCHECK --interval=30s --start-period=180s CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:9981/health', timeout=5)"
CMD ["python", "server.py"]
