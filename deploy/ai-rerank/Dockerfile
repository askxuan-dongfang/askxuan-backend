# Shares the pinned Python 3.12 / FastEmbed 0.8.1 runtime with the embedding
# sidecar. Build deploy/ai-embedding first on a clean host.
FROM askxuan/knowledge-embedding:20260930
WORKDIR /app
COPY server.py .
ENV BIND=0.0.0.0 PORT=9981 MODEL_CACHE=/data/models
EXPOSE 9981
HEALTHCHECK --interval=30s --start-period=180s CMD python -c "import urllib.request; urllib.request.urlopen('http://127.0.0.1:9981/health', timeout=5)"
CMD ["python", "server.py"]
