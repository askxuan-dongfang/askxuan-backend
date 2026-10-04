# Private rerank service

Python 3.12 / FastEmbed 0.8.1 share the existing embedding image runtime.
BAAI/bge-reranker-base (MIT), official revision `2cfc18c9415c912f9d8155881c133215df768a70`.
Download `onnx/model.onnx`, `config.json`, `tokenizer.json`, `tokenizer_config.json`,
`special_tokens_map.json`, `sentencepiece.bpe.model` from that revision of
https://huggingface.co/BAAI/bge-reranker-base into `/data/models/bge-reranker-base`
in the rerank volume (UID 10001). `MODEL_PATH` selects this offline directory.
No user document or credentials are used during download. Record SHA256 checksums
when transferring the model; do not point at a moving branch or unofficial mirror.

`POST /v1/rerank`: Bearer AI_EMBEDDING_KEY, JSON query/documents. At most 40
candidates, 4000 question characters, 16000 characters per document, 256 KiB body.
ONNX truncates to the model sequence limit. One CPU request at a time, batch size 1,
2 threads; no document logging, no public ports. Scores are sigmoid(logit), not a
calibrated probability that a claim is true. Context-dependent latency must be tested.

Build the embedding image first, then `docker compose -p askxuan-knowledge build rerank`.
The API supports a failure response so Harness can either reject retrieval or visibly
fall back to hybrid ranking according to its published policy.
