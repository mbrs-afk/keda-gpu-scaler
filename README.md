# KEDA GPU Scaler

**Scale Kubernetes GPU workloads from real hardware metrics. No DCGM. No PromQL. Optional Prometheus metrics built in.**

[![CI](https://github.com/pmady/kedagpuscaler/actions/workflows/ci.yaml/badge.svg)](https://github.com/pmady/kedagpuscaler/actions/workflows/ci.yaml)
[![Go Report Card](https://goreportcard.com/badge/github.com/pmady/kedagpuscaler)](https://goreportcard.com/report/github.com/pmady/kedagpuscaler)
[![License](https://img.shields.io/badge/LicenseApache%202.0blue.svg)](LICENSE)

A [KEDA External Scaler](https://keda.sh/docs/latest/concepts/externalscalers/) that reads NVIDIA GPU metrics directly from NVML Cbindings and autoscales your vLLM, Triton, and custom inference deployments  including scaletozero.

## Why This Exists

Kubernetes HPA watches CPU and memory. It can't see GPU utilization. Your vLLM pod shows 8% CPU while the GPU is at 100%.

The usual fix is dcgmexporter → Prometheus → KEDA, but that's 5 components and 1530s of latency.

This project reads GPU metrics directly from NVML and serves them to KEDA over gRPC. 2 components, 24 second latency.

### Why Not a Native KEDA Scaler?

Putting GPU support inside KEDA core doesn't work:

1. **CGO Constraint**: NVIDIA's Go bindings ([`gonvml`](https://github.com/NVIDIA/gonvml)) require `CGO_ENABLED=1`. KEDA builds with `CGO_ENABLED=0`.
2. **NodeLevel Hardware Access**: The KEDA operator runs as a central pod. NVML requires local GPU device access via `libnvidiaml.so`, which only a **DaemonSet on GPU nodes** can provide.
3. **Independent Release Cycle**: Ship GPU scaling improvements without waiting for KEDA release cycles.

This design is documented in [KEDA issue #7538](https://github.com/kedacore/keda/issues/7538).



## Architecture

<p align="center">
  <img src="docs/images/architecture.png" alt="kedagpuscaler architecture" width="100%"/>
</p>

1. **DaemonSet**  Runs on nodes labeled with `nvidia.com/gpu.present: "true"`.
2. **NVML Bindings**  Directly reads Streaming Multiprocessor (SM) utilization and Frame Buffer Memory via `gonvml` Cbindings.
3. **gRPC Interface**  Implements `externalscaler.ExternalScalerServer` (`IsActive`, `StreamIsActive`, `GetMetricSpec`, `GetMetrics`) to natively integrate with the central KEDA operator.
4. **ScaledObject Trigger**  Kubernetes deployments scale up/down (including to zero) based on GPU thresholds defined in the ScaledObject.



## GPU Metrics

| Metric | Description | Unit |
||||
| `gpu_utilization` | GPU compute (SM) utilization | % (0100) |
| `memory_utilization` | GPU memory controller utilization | % (0100) |
| `memory_used_mib` | GPU VRAM used | MiB |
| `memory_used_percent` | GPU VRAM used as percentage of total | % (0100) |
| `temperature` | GPU die temperature | Celsius |
| `power_draw` | GPU power consumption | Watts |
| `pcie_tx_kbps` | PCIe transmit throughput (CPU→GPU) | KB/s |
| `pcie_rx_kbps` | PCIe receive throughput (GPU→CPU) | KB/s |
| `nvlink_tx_mbps` | NVLink transmit throughput (GPU→GPU) | MB/s |
| `nvlink_rx_mbps` | NVLink receive throughput (GPU→GPU) | MB/s |



## Prebuilt Scaling Profiles

Instead of configuring raw metric thresholds, use a profile optimized for your workload:

| Profile | Primary Metric | Target | Activation | Use Case |
||||||
| `vllminference` | Memory % | 80 | 5 | vLLM / LLM serving with scaletozero |
| `tritoninference` | GPU Util | 75 | 10 | NVIDIA Triton Inference Server |
| `training` | GPU Util | 90 | 0 | Training jobs (no scaletozero) |
| `batch` | Memory % | 70 | 1 | Batch inference with aggressive scaledown |
| `distributedtraining` | NVLink TX | 800 | 100 | Dataparallel training on NVLink systems |



## Prerequisites

 A Kubernetes cluster (e.g., **OKE**, GKE, EKS, AKS) with **NVIDIA GPU worker nodes**
 [KEDA v2.10+](https://keda.sh/docs/latest/deploy/) installed in the cluster
 NVIDIA GPU drivers and [Device Plugin](https://github.com/NVIDIA/k8sdeviceplugin) installed



## Quick Start

### 1. Deploy the Scaler

Deploy the DaemonSet and gRPC service into your cluster. (Ensure KEDA is already installed.)

```bash
kubectl apply f deploy/manifests.yaml
```

This deploys a DaemonSet that runs on every GPU node in your cluster, plus a ClusterIP Service for KEDA to discover it.

Or use Helm:

```bash
helm install kedagpuscaler deploy/helm/kedagpuscaler \
  namespace keda \
  set nodeSelector."nvidia\.com/gpu\.present"=true
```

### 2. Attach to your AI Workload

Create a ScaledObject pointing to the external scaler service:

```yaml
apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: vllminferencescaler
  namespace: aiworkloads
spec:
  scaleTargetRef:
    name: vllmdeepseekdeployment
  minReplicaCount: 1
  maxReplicaCount: 50
  triggers:
     type: external
      metadata:
        scalerAddress: "kedagpuscaler.keda.svc.cluster.local:6000"
        targetGpuUtilization: "80"
```

Or use a prebuilt profile:

```yaml
triggers:
   type: external
    metadata:
      scalerAddress: "kedagpuscaler.keda.svc.cluster.local:6000"
      profile: "vllminference"
```

### 3. Custom Configuration

Override any profile default or use raw GPU metrics directly:

```yaml
triggers:
   type: external
    metadata:
      scalerAddress: "kedagpuscaler.keda.svc.cluster.local:6000"
      metricType: "gpu_utilization"
      targetValue: "85"
      activationThreshold: "10"
      gpuIndex: "0"              # specific GPU index, or omit for all
      aggregation: "max"         # max, min, avg, sum across GPUs
```

See `deploy/examples/` for readytouse ScaledObject manifests.



## Configuration Reference

| Parameter | Description | Default |
||||
| `profile` | Prebuilt scaling profile name | (none) |
| `metricType` | GPU metric to scale on | `gpu_utilization` |
| `targetValue` | Target metric value for scaling | `80` |
| `targetGpuUtilization` | Shorthand for GPU utilization target | (none) |
| `targetMemoryUtilization` | Shorthand for VRAM utilization target | (none) |
| `activationThreshold` | Value below which scaletozero activates | `0` |
| `gpuIndex` | Specific GPU index to monitor | `1` (all GPUs) |
| `aggregation` | MultiGPU aggregation: `max`, `min`, `avg`, `sum` | `max` |
| `pollIntervalSeconds` | Metric polling interval | `10` |



## Prometheus Metrics (Optional)

The scaler exposes an optional Prometheuscompatible `/metrics` endpoint for monitoring the scaler itself and GPU fleet health. **This is independent of the KEDA scaling path**  scaling works identically with or without it.

### Enable/Disable

```bash
# Enabled by default on port 9090
metricsport=9090

# Disable entirely (zero overhead)
metricsport=0
```

Helm:
```yaml
metrics:
  enabled: true   # set to false to disable
  port: 9090
```

### Exposed Metrics

| Metric | Type | Description |
||||
| `keda_gpu_scaler_gpu_utilization_percent` | Gauge | GPU compute utilization (per GPU) |
| `keda_gpu_scaler_gpu_memory_used_bytes` | Gauge | GPU memory in use (per GPU) |
| `keda_gpu_scaler_gpu_memory_total_bytes` | Gauge | Total GPU memory (per GPU) |
| `keda_gpu_scaler_gpu_temperature_celsius` | Gauge | GPU temperature (per GPU) |
| `keda_gpu_scaler_gpu_power_draw_watts` | Gauge | GPU power draw (per GPU) |
| `keda_gpu_scaler_collections_total` | Counter | Total NVML collection calls |
| `keda_gpu_scaler_collection_errors_total` | Counter | Failed NVML collection calls |
| `keda_gpu_scaler_collection_duration_seconds` | Histogram | NVML collection latency |
| `keda_gpu_scaler_scaler_requests_total` | Counter | gRPC requests by method |
| `keda_gpu_scaler_scaler_request_errors_total` | Counter | gRPC errors by method |

All perGPU metrics are labeled with `gpu_index`, `gpu_uuid`, and `gpu_name`.

## Kubernetes Probes

The scaler exposes liveness and readiness endpoints on a dedicated probe port:

 `/healthz` returns `200` while the process is alive.
 `/readyz` returns `200` after NVML initializes and the first metrics collection succeeds.

```bash
probeport=8081
```

Helm:
```yaml
probes:
  enabled: true
  port: 8081
```



## Build it Yourself

This project requires `CGO_ENABLED=1` to compile the NVIDIA Cbindings.

> [!NOTE]
> The compiled binaries (`kedagpuscaler` and `gpumetrics`) dynamically link NVIDIA's NVML library and load `libnvidiaml.so` at runtime. They will **fail to start on any machine that does not have the NVIDIA driver installed** (which provides `libnvidiaml.so`)  for example, a laptop or CI runner with no NVIDIA GPU. You can still build, lint, and run the test suite without a GPU, since the tests use a mock collector (see [Can I run this without a GPU?](docs/FAQ.md#canirunthiswithoutagpufordevelopment)).

```bash
# Build KEDA scaler binary (requires CGO for NVML)
make build

# Build standalone GPU metrics CLI (no KEDA/gRPC needed)
make buildmetrics

# Build all binaries
make buildall

# Run unit tests
make test

# Run linter
make lint

# Generate protobuf Go code
make proto

# Build and push a release image
make dockerrelease VERSION=v0.1.0

# Deploy to cluster
make deploy
```

### Checking the Version

Both binaries accept a `version` flag (and a bare `version` argument) that
prints the version, Go version, and build date, then exits. Unlike normal
operation, this does **not** require a GPU or the NVIDIA driver:

```bash
kedagpuscaler version    # kedagpuscaler v0.5.0 (go1.26.4, built 20260625)
gpumetrics version        # gpumetrics v0.5.0 (go1.26.4, built 20260625)
```

`make build` stamps the version from `git describe` at link time; builds without
ldflags (e.g. `go run`) report `dev`.

### Standalone GPU Metrics CLI

Collect GPU metrics without Kubernetes  works on bare metal, SLURM jobs, Flux jobs, Kubernetes pods, and Singularity containers. The same binary and the same JSON schema work everywhere.

> [!IMPORTANT]
> `gpumetrics` requires `libnvidiaml.so` (installed with the NVIDIA driver) on the host. On a machine without an NVIDIA driver it exits immediately with `nvml init failed`.

```bash
gpumetrics                       # oneshot table output (env autodetected)
gpumetrics format json         # JSON for scripting
gpumetrics format csv          # CSV for analysis
gpumetrics interval 5s         # continuous collection
gpumetrics device 0 quiet    # single GPU, no logs
gpumetrics env slurm           # force environment (auto|k8s|slurm|flux|standalone)
gpumetrics version             # print version and exit (no GPU/NVML required)
```

The `env` flag autodetects the orchestrator by default. Detection priority: **SLURM → Flux → Kubernetes → standalone**.

Every environment emits the same unified JSON schema with an `environment` block so you can compare GPU performance across onprem and cloud with identical tooling:

```json
{
  "environment": { "orchestrator": "slurm", "node": "compute01", "job_id": "123", "task_rank": 0 },
  "collected_at": "20260617T10:00:00Z",
  "devices": [...]
}
```

**SLURM**  autodetected when `SLURM_JOB_ID` is set; collects only the GPUs assigned to your job step:

```bash
srun gres=gpu:2 gpumetrics format json
```

**Flux**  autodetected when `FLUX_JOB_ID` is set; collects only the GPUs in `CUDA_VISIBLE_DEVICES`:

```bash
flux run N1 g2 gpumetrics format json
```

See **[HPC & CrossEnvironment Metrics](docs/hpc.md)** for full usage, and **[CrossEnvironment Comparison Guide](docs/crossenvcomparison.md)** for comparing onprem vs cloud GPU runs.

Or build the Docker image directly:

```bash
docker build t yourregistry/kedagpuscaler:v0.1.0 .
docker push yourregistry/kedagpuscaler:v0.1.0
```



## How It Compares

| | kedagpuscaler | dcgmexporter + Prometheus | Custom Metrics API |
|||||
| **Components** | 1 DaemonSet (+ optional /metrics) | dcgmexporter + Prometheus + adapter | Custom metrics server |
| **Metric latency** | Subsecond (direct NVML) | 1530s (scrape interval) | Depends on implementation |
| **Scaletozero** | Yes (KEDA native) | Yes (with KEDA Prometheus scaler) | Manual |
| **Configuration** | 3line ScaledObject | PromQL query per metric | Custom code |
| **GPU metrics** | 10 hardware metrics | 50+ DCGM metrics | Whatever you build |
| **Dependencies** | KEDA, NVIDIA drivers | KEDA, Prometheus, dcgmexporter | Varies |
| **Failure domain** | Nodelocal | Centralized Prometheus | Varies |



## Documentation

 **[Design Document](docs/DESIGN.md)**  Architecture decisions, gRPC interface, scaling profiles, testing strategy
 **[Migration Guide](docs/MIGRATION.md)**  Replace dcgmexporter + Prometheus with kedagpuscaler
 **[HPC & CrossEnvironment Metrics](docs/hpc.md)**  SLURM, Flux, Kubernetes, and standalone GPU metrics
 **[CrossEnvironment Comparison](docs/crossenvcomparison.md)**  Compare GPU performance across onprem and cloud
 **[FAQ](docs/FAQ.md)**  Common questions about GPU scaling, MIG, multiGPU, scaletozero
 **[Changelog](CHANGELOG.md)**  Release history



## Related

 [CNCF Blog: GPU Autoscaling on Kubernetes with KEDA](https://www.cncf.io/blog/2026/05/27/gpuautoscalingonkuberneteswithkedabuildinganexternalscaler/)
 [KEDA issue #7538](https://github.com/kedacore/keda/issues/7538)  original discussion
 [CNCF TOC initiative #2188](https://github.com/cncf/toc/issues/2188)  whitepaper proposal



## Adopters

Using kedagpuscaler? Add your organization to [ADOPTERS.md](ADOPTERS.md).



## Roadmap

 AMD ROCm support
 MIG perinstance metrics
 vLLM queue depth scaling



## Contributors

Thanks to everyone who helps build kedagpuscaler.

<! readme: contributors start >
<table>
<tr>
    <td align="center"><a href="https://github.com/pmady"><img src="https://avatars.githubusercontent.com/u/15876315?v=4" width="80;" alt="pmady"/><br /><sub><b>Pavan Madduri</b></sub></a></td>
    <td align="center"><a href="https://github.com/venkata22a"><img src="https://avatars.githubusercontent.com/u/31258325?v=4" width="80;" alt="venkata22a"/><br /><sub><b>venkata22a</b></sub></a></td>
    <td align="center"><a href="https://github.com/penpal"><img src="https://avatars.githubusercontent.com/u/61139563?v=4" width="80;" alt="penpal"/><br /><sub><b>Manish Khadka</b></sub></a></td>
    <td align="center"><a href="https://github.com/ibobgunardi"><img src="https://avatars.githubusercontent.com/u/24878946?v=4" width="80;" alt="ibobgunardi"/><br /><sub><b>Bobi Gunardi</b></sub></a></td>
    <td align="center"><a href="https://github.com/KaustAbhinand"><img src="https://avatars.githubusercontent.com/u/154255646?v=4" width="80;" alt="KaustAbhinand"/><br /><sub><b>Kaustubh Abhinand</b></sub></a></td>
    <td align="center"><a href="https://github.com/AtharvAC"><img src="https://avatars.githubusercontent.com/u/235652593?v=4" width="80;" alt="AtharvAC"/><br /><sub><b>Atharv</b></sub></a></td>
</tr>
</table>
<! readme: contributors end >

See [CONTRIBUTORS.md](CONTRIBUTORS.md) for detailed contributions.

## Contributing

Contributions welcome  GPU autoscaling use cases, vendor support (AMD ROCm, Intel), or docs improvements. See [CONTRIBUTING.md](CONTRIBUTING.md).

## License

Apache License 2.0. See [LICENSE](LICENSE) for details.
