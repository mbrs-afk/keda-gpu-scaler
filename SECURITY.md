# Security Policy

## Reporting a Vulnerability

Don't open a public GitHub issue for security bugs. Report them privately:

1. [GitHub private vulnerability reporting](https://github.com/pmady/keda-gpu-scaler/security/advisories/new)
2. Or email **pavan4devops@gmail.com**

Include: what the bug is, how to reproduce it, and the impact. I'll acknowledge within 48 hours and coordinate a fix + disclosure timeline.

## Supported Versions

| Version | Supported |
|---------|-----------|
| latest  | ✅        |

## Deployment Notes

- Run the DaemonSet with least-privilege RBAC — it only needs to read GPU metrics and serve gRPC
- Use network policies to restrict which pods can reach the scaler's gRPC port
- The scaler runs with the NVIDIA container runtime; don't mount `/dev` directly unless you have to
