# Eval datasets — durable location, provenance, licences

Referenced by `cma/eval/PREREGISTRATION.md` §4.1. The data files are **not** committed
(18 MB of third-party corpora); the sha256 values below pin their identity exactly.

**Canonical directory: `~/data/memora-eval/`** — on the real filesystem, NOT `/tmp`.
`/tmp` on the development box is `tmpfs` (RAM-backed) and a reboot deletes anything stored there.
`~/data/memora-eval/SHA256SUMS` carries the same digests; verify with `sha256sum -c SHA256SUMS`.

The harness MUST verify both dataset digests at startup and `t.Fatal` on mismatch
(pre-registration §5.2, void condition V8).

| File | Bytes | sha256 | Source URL | Licence |
|---|---|---|---|---|
| `lme_oracle.json` | 15388478 | `821a2034d219ab45846873dd14c14f12cfe7776e73527a483f9dac095d38620c` | https://huggingface.co/datasets/xiaowu0162/longmemeval-cleaned (public, keyless) | MIT |
| `locomo10.json` | 2805274 | `79fa87e90f04081343b8c8debecb80a9a6842b76a7aa537dc9fdf651ea698ff4` | https://raw.githubusercontent.com/snap-research/locomo/main/data/locomo10.json | **CC BY-NC 4.0** |
| `LICENSE-LoCoMo.txt` | 19347 | `41003d4a74749c0220e33dd415042164b5a1093ed401f36277234f772d22d3d0` | https://raw.githubusercontent.com/snap-research/locomo/main/LICENSE.txt | (the licence text itself) |

## LoCoMo — attribution and use restrictions

LoCoMo is **CC BY-NC 4.0** (`Attribution-NonCommercial 4.0 International`; the GitHub API reports no
SPDX id, so the licence is discoverable only from `LICENSE.txt` itself). Registered gates, from
pre-registration §4.1 — **no LoCoMo number may be published until all four hold**:

1. `LICENSE-LoCoMo.txt` is present in `~/data/memora-eval/`.
2. The CC BY-NC 4.0 attribution line appears in the write-up.
3. Use is **research only — never any product claim**.
4. The sha256 of the **stripped** subset is published *first*.

## Mandatory LoCoMo loader rule — enforced in code, not prose

The loader MUST `delete` `observation`, `session_summary`, and `event_summary` from every
conversation object at parse time. Those fields are GPT-written gists tagged with **the same
`dia_id`s the gold evidence points to**; reading any of them — to build units, to seed clustering,
to define grouping, or even to pick a threshold — is a guaranteed, meaningless win.
