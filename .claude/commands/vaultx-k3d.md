Set up or troubleshoot vaultx integration with a k3d / Kubernetes cluster using the External Secrets Operator.

Walk through the full setup:

1. **Prerequisites check** — verify `kubectl`, `helm`, and `k3d` are installed; verify the vaultx daemon is running.

```bash
kubectl version --client
helm version
k3d version
vaultx version
curl -s http://localhost:7474/health
```

2. **Start the daemon** if not running:

```bash
vaultx serve --port 7474
```

3. **One-time cluster setup**:

```bash
# Namespace-scoped SecretStore (default namespace)
vaultx k3d setup

# Or cluster-wide ClusterSecretStore (all namespaces)
vaultx k3d setup --cluster-wide

# Custom namespace
vaultx k3d setup --namespace my-app --port 7474
```

4. **Declare secrets** with an ExternalSecret manifest — show an example based on secrets currently in the vault (`vaultx list`).

5. **Check status**:

```bash
vaultx k3d status
kubectl get externalsecret -A
kubectl describe externalsecret myapp-secrets -n default
```

6. **Refresh token** after a daemon restart:

```bash
vaultx k3d token
```

7. **Reference the k8s Secret in a Deployment**:

```yaml
env:
  - name: DB_PASSWORD
    valueFrom:
      secretKeyRef:
        name: myapp-secrets
        key: DB_PASSWORD
```

If there are errors, diagnose: check ESO pod logs, SecretStore Ready condition, and that the daemon token matches the k8s secret.
