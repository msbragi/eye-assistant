# GemmaLink: Eye Assistant

**GemmaLink** is a local-first AI tool that turns your smartphone into a vision sensor for Gemma 4. It enables a "Zero-Footprint" multimodal experience, allowing you to discuss what your phone sees with a high-performance AI running entirely on your PC.

Designed for privacy and performance, it bypasses the cloud entirely using a high-concurrency Go backend.

## Key Features
*   **100% Local:** No cloud, no tunnels, no external dependencies.
*   **Smart Discovery:** Automatically selects the best binary for your CPU/GPU (Vulkan/AVX2).
*   **Multimodal:** Real-time visual analysis using Gemma 4.
*   **Architectural Simplicity:** Single-binary distribution (Go + Sidecar pattern).

## Plug-and-Play Experience
GemmaLink is engineered for users of all technical levels. Unlike most local AI setups, it requires **zero configuration**:
* **No Python/Environment Setup:** A single self-contained executable handles everything.
* **Auto-Provisioning:** The app automatically finds, downloads, and configures the best AI engine for your specific hardware.
* **Instant Mobile Link:** Just scan the generated QR code. The only "technical" step is a one-time confirmation in your mobile browser to trust the local secure connection.

## Project Architecture
For a deep dive into the system design, components, and data flow, see the [Architecture Documentation](docs/architecture.md).

## Build Instructions
The project uses a structured build system located in the `scripts` directory.

### Prerequisites
*   Go 1.21+
*   Wails CLI
*   Task or standard Bash environment

### Building for Production
Pass the target OS and version tag to the build script:

```bash
# General syntax: ./scripts/build.sh [OS] [Version]

# Build for Windows
./scripts/build.sh win 1.0.0

# Build for Linux
./scripts/build.sh linux 1.0.0

# Build all supported platforms
./scripts/build.sh all 1.0.0

```
Development Build

To load assets directly from the filesystem for rapid iteration:
```bash

./scripts/build-dev.sh

```
---

## Releases & Downloads

* **Latest Binary (Windows/Linux):** [Link Placeholder]
* **Source Code (zip/tar.gz):** [Link Placeholder]

## Disclaimer

This project is part of the **Gemma 4 Challenge**. It is intended for local deployment to demonstrate the capabilities of multimodal models in a private network environment.

## Footprint note

### Design Philosophy
GemmaLink: Eye Assistant is not intended to compete with cloud-scale vision services like Google Lens. Instead, it serves a different paradigm: **Sovereign Local Intelligence**. 

While cloud services offer global-scale knowledge, GemmaLink provides:
* **Absolute Privacy:** Your camera feed never touches a server outside your home.
* **Latency Control:** Fast, local-loop processing independent of your internet bandwidth.
* **Extensibility:** A focused tool for personal automation and private context analysis.