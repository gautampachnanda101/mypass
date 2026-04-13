package commands

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/spf13/cobra"
)

func cmdK3d() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "k3d",
		Short: "Helpers for k3d / Kubernetes External Secrets integration",
	}
	cmd.AddCommand(cmdK3dSetup(), cmdK3dToken(), cmdK3dStatus())
	return cmd
}

// cmdK3dSetup installs External Secrets Operator and creates the vaultx SecretStore.
func cmdK3dSetup() *cobra.Command {
	var namespace string
	var clusterWide bool
	var port int

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Install ESO and configure the vaultx SecretStore in your k3d cluster",
		Long: `Runs the following steps:
  1. helm install external-secrets (if not already installed)
  2. kubectl create secret vaultx-token (from ~/.vaultx/daemon.token)
  3. kubectl apply SecretStore (or ClusterSecretStore with --cluster-wide)

Requires: helm, kubectl, and a running k3d cluster.
The vaultx daemon must be running (vaultx serve --port <port>).`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			// 1. Install ESO via helm (idempotent).
			fmt.Fprintln(os.Stderr, "→ Installing External Secrets Operator...")
			if err := runCmd("helm", "repo", "add", "external-secrets",
				"https://charts.external-secrets.io"); err != nil {
				return fmt.Errorf("helm repo add: %w", err)
			}
			if err := runCmd("helm", "upgrade", "--install", "external-secrets",
				"external-secrets/external-secrets",
				"-n", "external-secrets", "--create-namespace",
				"--wait"); err != nil {
				return fmt.Errorf("helm install ESO: %w", err)
			}

			// 2. Create / update the token secret.
			tokenPath := filepath.Join(homeDir(), ".vaultx", "daemon.token")
			if _, err := os.Stat(tokenPath); err != nil {
				return fmt.Errorf("daemon token not found at %s — run: vaultx serve", tokenPath)
			}
			tokenNS := "external-secrets"
			if !clusterWide {
				tokenNS = namespace
			}
			fmt.Fprintln(os.Stderr, "→ Creating vaultx-token secret...")
			// Delete existing (ignore error) then create fresh.
			_ = runCmd("kubectl", "delete", "secret", "vaultx-token",
				"-n", tokenNS, "--ignore-not-found")
			if err := runCmd("kubectl", "create", "secret", "generic", "vaultx-token",
				"-n", tokenNS,
				"--from-file=token="+tokenPath); err != nil {
				return fmt.Errorf("create token secret: %w", err)
			}

			// 3. Apply the SecretStore manifest.
			fmt.Fprintln(os.Stderr, "→ Applying SecretStore...")
			manifestDir := "k3d"
			manifest := filepath.Join(manifestDir, "secretstore.yaml")
			if clusterWide {
				manifest = filepath.Join(manifestDir, "clusterwide-secretstore.yaml")
			}

			if _, err := os.Stat(manifest); err != nil {
				// Fall back to inline manifest if k3d/ dir isn't present.
				manifest, err = writeInlineManifest(clusterWide, port, tokenNS)
				if err != nil {
					return err
				}
				defer os.Remove(manifest)
			}

			if err := runCmd("kubectl", "apply", "-f", manifest, "-n", namespace); err != nil {
				return fmt.Errorf("apply SecretStore: %w", err)
			}

			fmt.Fprintf(os.Stderr, "\n✓ vaultx SecretStore ready in namespace %q\n", namespace)
			fmt.Fprintln(os.Stderr, "  Apply secrets:  kubectl apply -f k3d/externalsecret-example.yaml")
			return nil
		},
	}

	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Kubernetes namespace")
	cmd.Flags().BoolVar(&clusterWide, "cluster-wide", false, "Install a ClusterSecretStore (all namespaces)")
	cmd.Flags().IntVarP(&port, "port", "p", 7474, "vaultx daemon port")
	return cmd
}

// cmdK3dToken refreshes the vaultx-token secret from the current daemon token.
func cmdK3dToken() *cobra.Command {
	var namespace string

	cmd := &cobra.Command{
		Use:   "token",
		Short: "Refresh the vaultx-token Kubernetes secret from ~/.vaultx/daemon.token",
		RunE: func(cmd *cobra.Command, _ []string) error {
			tokenPath := filepath.Join(homeDir(), ".vaultx", "daemon.token")
			if _, err := os.Stat(tokenPath); err != nil {
				return fmt.Errorf("daemon token not found — run: vaultx serve")
			}
			_ = runCmd("kubectl", "delete", "secret", "vaultx-token",
				"-n", namespace, "--ignore-not-found")
			if err := runCmd("kubectl", "create", "secret", "generic", "vaultx-token",
				"-n", namespace,
				"--from-file=token="+tokenPath); err != nil {
				return fmt.Errorf("refresh token secret: %w", err)
			}
			fmt.Fprintf(os.Stderr, "✓ vaultx-token refreshed in namespace %q\n", namespace)
			return nil
		},
	}
	cmd.Flags().StringVarP(&namespace, "namespace", "n", "default", "Kubernetes namespace")
	return cmd
}

// cmdK3dStatus checks whether ESO and the vaultx SecretStore are healthy.
func cmdK3dStatus() *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Check ESO and vaultx SecretStore status in the cluster",
		RunE: func(cmd *cobra.Command, _ []string) error {
			fmt.Fprintln(os.Stderr, "ESO pods:")
			_ = runCmd("kubectl", "get", "pods", "-n", "external-secrets")
			fmt.Fprintln(os.Stderr, "\nSecretStores:")
			_ = runCmd("kubectl", "get", "secretstore,clustersecretstore", "-A")
			fmt.Fprintln(os.Stderr, "\nExternalSecrets:")
			_ = runCmd("kubectl", "get", "externalsecret", "-A")
			return nil
		},
	}
}

// writeInlineManifest writes a temporary SecretStore YAML when the k3d/ dir
// isn't present (e.g. installed via go install).
func writeInlineManifest(clusterWide bool, port int, tokenNS string) (string, error) {
	var tmpl string
	if clusterWide {
		tmpl = fmt.Sprintf(clusterStoreTemplate, port, tokenNS)
	} else {
		tmpl = fmt.Sprintf(storeTemplate, port)
	}
	f, err := os.CreateTemp("", "vaultx-secretstore-*.yaml")
	if err != nil {
		return "", err
	}
	defer f.Close()
	_, err = f.WriteString(tmpl)
	return f.Name(), err
}

const storeTemplate = `apiVersion: external-secrets.io/v1beta1
kind: SecretStore
metadata:
  name: vaultx
spec:
  provider:
    webhook:
      url: "http://host.k3d.internal:%d/externalsecrets/{{ .remoteRef.key }}"
      headers:
        X-Vaultx-Token:
          secretKeyRef:
            name: vaultx-token
            key: token
      result:
        jsonPath: "$.value"
      timeout: 10s
`

const clusterStoreTemplate = `apiVersion: external-secrets.io/v1beta1
kind: ClusterSecretStore
metadata:
  name: vaultx
spec:
  provider:
    webhook:
      url: "http://host.k3d.internal:%d/externalsecrets/{{ .remoteRef.key }}"
      headers:
        X-Vaultx-Token:
          secretKeyRef:
            name: vaultx-token
            namespace: %s
            key: token
      result:
        jsonPath: "$.value"
      timeout: 10s
`

func runCmd(name string, args ...string) error {
	c := exec.Command(name, args...)
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func homeDir() string {
	h, _ := os.UserHomeDir()
	return h
}
