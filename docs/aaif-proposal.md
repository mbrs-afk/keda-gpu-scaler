# AAIF Project Proposal — keda-gpu-scaler

Draft proposal for the Agentic AI Foundation. Submit via: https://github.com/aaif/project-proposals/issues/new

**Target submission:** Late June 2026 (after HPSF submission)

---

## Project Name

keda-gpu-scaler

## Project Description

keda-gpu-scaler enables GPU-aware autoscaling for AI inference workloads on Kubernetes. It reads NVIDIA GPU metrics directly via NVML and exposes them to KEDA, letting you scale LLM serving (vLLM, Triton), batch inference, and training jobs based on actual GPU utilization — not CPU or memory.

The problem it solves: standard Kubernetes HPA can't see GPU load. A vLLM pod serving 200 concurrent requests shows 8% CPU while the GPU is saturated. The existing workaround (dcgm-exporter → Prometheus → KEDA) adds 15-30 seconds of latency and 5 components. This project reduces that to 2-4 seconds with 2 components.

Pre-built scaling profiles for common AI workloads:
- **vllm-inference** — Scale on VRAM pressure (LLMs pre-allocate KV cache)
- **triton-inference** — Scale on SM utilization (multi-model serving)
- **batch** — Aggressive scale-down for offline inference
- **training** — High utilization target, no scale-to-zero

## How does this project align with the AAIF mission?

AI inference infrastructure is the foundation that agentic systems run on. When an agent makes 50 tool calls in a conversation, each hitting an LLM endpoint, the inference backend needs to scale dynamically. Fixed replica counts either waste GPU resources or create latency spikes.

This project provides the autoscaling primitive that AI platforms need. It's not an agent framework itself, but it's infrastructure that agent deployments depend on.

Specific alignment:
- **GPU efficiency** — Scale inference pods based on actual GPU load, not guesswork
- **Scale-to-zero** — Activation thresholds enable spinning down idle models
- **Multi-model serving** — Aggregation modes handle nodes with multiple models/GPUs
- **Production-ready** — Already running in production on A100 clusters

## Project Website

https://github.com/pmady/keda-gpu-scaler

Documentation: https://pmady.github.io/keda-gpu-scaler

## Open Source License

Apache-2.0: https://github.com/pmady/keda-gpu-scaler/blob/main/LICENSE

## Code of Conduct

https://github.com/pmady/keda-gpu-scaler/blob/main/CODE_OF_CONDUCT.md

## Governance

https://github.com/pmady/keda-gpu-scaler/blob/main/GOVERNANCE.md

Single-maintainer model. Decisions happen in GitHub Issues and PR review.

## Source Control

GitHub: https://github.com/pmady/keda-gpu-scaler

## Issue Tracking

GitHub Issues: https://github.com/pmady/keda-gpu-scaler/issues

## External Dependencies

| Dependency | License | Purpose |
|---|---|---|
| github.com/NVIDIA/go-nvml | Apache-2.0 | GPU metrics via NVML |
| google.golang.org/grpc | Apache-2.0 | KEDA external scaler interface |
| go.uber.org/zap | MIT | Logging |
| github.com/prometheus/client_golang | Apache-2.0 | Optional metrics endpoint |

## Release Methodology

Automated via GitHub Actions:
- Semver tags trigger releases
- Multi-arch Docker images (amd64, arm64) pushed to GHCR
- Binaries attached to GitHub Releases with checksums

## Software Quality

- CI: Build, test, lint on every PR (amd64 + arm64)
- Security: OpenSSF Scorecard, CodeQL, Dependabot
- Testing: Table-driven Go tests, mock GPU collector, race detector
- Code review: All changes via PR, DCO sign-off required

## Project Leadership

- **Pavan Madduri** ([@pmady](https://github.com/pmady)) — Creator, maintainer

## Commit Access

- Pavan Madduri (@pmady) — maintainer

## Decision-Making Process

Features proposed as GitHub Issues, discussed, implemented via PRs. Maintainer approval required to merge.

## Project Maturity

Early production. v0.4.0 released, running on A100 clusters. Small but growing contributor base.

## Communication Channels

- GitHub Issues and Discussions
- GitHub Pull Requests

## Social Media

N/A — project communications on GitHub

## Financial Sponsorships

None. Volunteer-maintained.

## Infrastructure Needs

Currently using GitHub Actions free tier, GHCR, GitHub Pages. No immediate needs. GPU-enabled CI runners would be nice eventually for integration testing.

---

## Notes for TC Presentation

**Angle:** Infrastructure for AI inference autoscaling, not an agent framework. The pitch is "this is what your vLLM/Triton backends need to scale properly."

**Demo:** Show KEDA scaling a vLLM deployment based on GPU memory pressure. 30 seconds.

**Differentiator:** No Prometheus pipeline, sub-5-second metric latency, pre-built profiles for LLM serving.
