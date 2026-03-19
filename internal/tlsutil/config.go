// Package tlsutil provides shared TLS configuration building from Kubernetes secrets.
package tlsutil

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	sreportalv1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
)

// BuildTLSConfig constructs a *tls.Config from a RemoteTLSConfig and the referenced secrets.
func BuildTLSConfig(ctx context.Context, reader client.Reader, namespace string, tlsCfg *sreportalv1alpha1.RemoteTLSConfig) (*tls.Config, error) {
	config := &tls.Config{
		MinVersion: tls.VersionTLS12,
	}

	if tlsCfg.InsecureSkipVerify {
		config.InsecureSkipVerify = true //nolint:gosec // user-requested insecure mode for self-signed certs
	}

	if tlsCfg.CASecretRef != nil {
		secret, err := getSecret(ctx, reader, namespace, tlsCfg.CASecretRef.Name)
		if err != nil {
			return nil, fmt.Errorf("get CA secret %q: %w", tlsCfg.CASecretRef.Name, err)
		}

		caCert, ok := secret.Data["ca.crt"]
		if !ok {
			return nil, fmt.Errorf("CA secret %q does not contain key \"ca.crt\"", tlsCfg.CASecretRef.Name)
		}

		pool := x509.NewCertPool()
		if !pool.AppendCertsFromPEM(caCert) {
			return nil, fmt.Errorf("CA secret %q contains invalid certificate data", tlsCfg.CASecretRef.Name)
		}

		config.RootCAs = pool
	}

	if tlsCfg.CertSecretRef != nil {
		secret, err := getSecret(ctx, reader, namespace, tlsCfg.CertSecretRef.Name)
		if err != nil {
			return nil, fmt.Errorf("get client cert secret %q: %w", tlsCfg.CertSecretRef.Name, err)
		}

		certPEM, ok := secret.Data["tls.crt"]
		if !ok {
			return nil, fmt.Errorf("client cert secret %q does not contain key \"tls.crt\"", tlsCfg.CertSecretRef.Name)
		}

		keyPEM, ok := secret.Data["tls.key"]
		if !ok {
			return nil, fmt.Errorf("client cert secret %q does not contain key \"tls.key\"", tlsCfg.CertSecretRef.Name)
		}

		cert, err := tls.X509KeyPair(certPEM, keyPEM)
		if err != nil {
			return nil, fmt.Errorf("parse client certificate from secret %q: %w", tlsCfg.CertSecretRef.Name, err)
		}

		config.Certificates = []tls.Certificate{cert}
	}

	return config, nil
}

// SecretVersions returns the resourceVersion of each Secret referenced by the TLS config.
// This is used as a cache invalidation key — if any version changes, the TLS config
// must be rebuilt. The reads go through the controller-runtime informer cache.
func SecretVersions(ctx context.Context, reader client.Reader, namespace string, tlsCfg *sreportalv1alpha1.RemoteTLSConfig) (map[string]string, error) {
	if tlsCfg == nil {
		return nil, nil
	}

	versions := make(map[string]string, 2)

	if tlsCfg.CASecretRef != nil {
		secret, err := getSecret(ctx, reader, namespace, tlsCfg.CASecretRef.Name)
		if err != nil {
			return nil, err
		}
		versions[tlsCfg.CASecretRef.Name] = secret.ResourceVersion
	}

	if tlsCfg.CertSecretRef != nil {
		secret, err := getSecret(ctx, reader, namespace, tlsCfg.CertSecretRef.Name)
		if err != nil {
			return nil, err
		}
		versions[tlsCfg.CertSecretRef.Name] = secret.ResourceVersion
	}

	return versions, nil
}

func getSecret(ctx context.Context, reader client.Reader, namespace, name string) (*corev1.Secret, error) {
	secret := &corev1.Secret{}
	if err := reader.Get(ctx, types.NamespacedName{Name: name, Namespace: namespace}, secret); err != nil {
		return nil, fmt.Errorf("get secret %s/%s: %w", namespace, name, err)
	}

	return secret, nil
}
