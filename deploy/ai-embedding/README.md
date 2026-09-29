# Private Chinese embeddings

FastEmbed 0.8.1 (Apache-2.0) and BAAI/bge-small-zh-v1.5. Model weights download on first boot into the persistent model cache. Source: https://github.com/qdrant/fastembed ; model: https://huggingface.co/BAAI/bge-small-zh-v1.5 . Follow upstream model terms when distributing cached weights.

Set a private `AI_EMBEDDING_KEY` then `docker compose up -d --build`. `/health` is ready only after loading the model. POST `/v1/embeddings` accepts one string, fixed model, authenticated Bearer key. No user text is logged or sent to external inference. Long passages are split into 350-character chunks and averaged before cosine retrieval. This is not a reranker.

For a host AI service, use `AI_EMBEDDING_URL=http://127.0.0.1:9981/v1`. For containers, connect both services to the same private Docker network and use `http://ai-embedding:9981/v1`; localhost inside another container is not this service. Set `AI_EMBEDDING_MODEL=BAAI/bge-small-zh-v1.5` and the matching key in the AI service. Do not expose this port publicly. Configure restart/readiness/model cache before enabling production retrieval.

Migrate `scripts/db/20260930_ai_knowledge_memory.sql` against `askxuan_ai` first. Re-save existing entries after enabling/changing models to rebuild their vectors. No network/model service means the application can run explicitly in keyword-only mode by omitting both URL and MODEL. A configured but failing service returns an error instead of claiming semantic retrieval.

Local smoke: 512 dimensions, related Chinese paraphrase 0.6175 vs unrelated 0.2789, unauthorized POST rejected with 401. Production resource limits and recall quality still require environment-specific acceptance.
