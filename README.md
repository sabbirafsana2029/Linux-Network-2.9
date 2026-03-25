# Linux Network Simulation in Go

This project creates two network namespaces connected via bridges and a central router, all implemented using pure Go.

## 📊 Topology Details
- **Namespace 1 (ns1)**: 10.0.1.10/24
- **Namespace 2 (ns2)**: 10.0.2.10/24
- **Router Namespace (router-ns)**: Gateway for both subnets.

## 🚀 How to Run
1. Open Terminal in this folder.
2. Setup the network:
   ```bash
   make setup