# third_party (not checked into git)

Downloaded binaries the local embedding provider (`internal/llm/local_embed.go`)
loads at runtime. `internal/llm/local_embed_test.go` skips gracefully if these
are missing.

## ONNX Runtime shared library (~28 MB)

```sh
curl -sSL -o /tmp/ort.tgz https://github.com/microsoft/onnxruntime/releases/download/v1.29.0/onnxruntime-linux-x64-1.29.0.tgz
mkdir -p cma/third_party/onnxruntime/lib
tar xzf /tmp/ort.tgz -C /tmp
cp /tmp/onnxruntime-linux-x64-1.29.0/lib/libonnxruntime.so.1.29.0 cma/third_party/onnxruntime/lib/
ln -s libonnxruntime.so.1.29.0 cma/third_party/onnxruntime/lib/libonnxruntime.so
```

## all-MiniLM-L6-v2, quint8-quantized ONNX export (~22 MB) + vocab (~230 KB)

384-dim sentence embeddings. **Not** the production Qdrant collection's
1536-dim -- use a separate collection, don't reconfigure production to match.

```sh
mkdir -p cma/third_party/models/all-MiniLM-L6-v2
cd cma/third_party/models/all-MiniLM-L6-v2
curl -sSL -o model_quint8_avx2.onnx https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/onnx/model_quint8_avx2.onnx
curl -sSL -o vocab.txt https://huggingface.co/sentence-transformers/all-MiniLM-L6-v2/resolve/main/vocab.txt
```

Total: ~50 MB.
