# Copyright (c) 2023-2026 Microbus LLC and various contributors
# 
# Licensed under the Apache License, Version 2.0 (the "License");
# you may not use this file except in compliance with the License.
# You may obtain a copy of the License at
# 
# 	http://www.apache.org/licenses/LICENSE-2.0
# 
# Unless required by applicable law or agreed to in writing, software
# distributed under the License is distributed on an "AS IS" BASIS,
# WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
# See the License for the specific language governing permissions and
# limitations under the License.

import logging
import sys
import time

from sentence_transformers import SentenceTransformer, util

# Log to stderr so messages reach the framework's stderrRing buffer. stdout is reserved for
# worker.py's JSON frame protocol with Go; writing to it would corrupt that stream.
logging.basicConfig(
    level=logging.INFO,
    format="%(asctime)s.%(msecs)03d %(levelname)s %(message)s",
    datefmt="%H:%M:%S",
    stream=sys.stderr,
)
_log = logging.getLogger("embedder")

_log.info("loading sentence-transformers model all-MiniLM-L6-v2 ...")
_t0 = time.monotonic()
# Load the model once when Define exec's this file. The model is ~80MB on disk and a few hundred
# MB in RAM; cached in ~/.cache/torch/sentence_transformers on first download. Subsequent calls
# run inference in milliseconds on CPU.
_model = SentenceTransformer("sentence-transformers/all-MiniLM-L6-v2")
_log.info(
    "model loaded in %.2fs (embedding dim = %d)",
    time.monotonic() - _t0,
    _model.get_sentence_embedding_dimension(),
)


def embed(args):  # MARKER: Embed
    """Embed returns the sentence-embedding vector for the input text."""
    text = args.get("text", "")
    _log.info("embed: text=%r (%d chars)", text[:60], len(text))
    t0 = time.monotonic()
    vector = _model.encode(text, normalize_embeddings=True)
    elapsed = time.monotonic() - t0
    _log.info("embed: returned %d-dim vector in %.3fs", len(vector), elapsed)
    return {"vector": vector.tolist()}


def similarity(args):  # MARKER: Similarity
    """Similarity returns the cosine similarity between the embeddings of strings a and b."""
    a = args.get("a", "")
    b = args.get("b", "")
    _log.info("similarity: a=%r b=%r", a[:40], b[:40])
    t0 = time.monotonic()
    va = _model.encode(a, normalize_embeddings=True)
    vb = _model.encode(b, normalize_embeddings=True)
    # Vectors are L2-normalized, so the dot product equals cosine similarity.
    score = float(util.cos_sim(va, vb).item())
    elapsed = time.monotonic() - t0
    _log.info("similarity: score=%.4f computed in %.3fs", score, elapsed)
    return {"score": score}
