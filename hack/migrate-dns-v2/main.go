// hack/migrate-dns-v2/main.go
// Usage: go run ./hack/migrate-dns-v2 --kubeconfig <path> [--dry-run]
//
// For each DNS CR:
//  1. Read the annotation sreportal.io/v1alpha1-groups (set by conversion webhook).
//  2. Create a DNSRecord origin=manual per non-empty group.
//  3. Remove the annotation after every non-empty group succeeded.
package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/tools/clientcmd"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1alpha1 "github.com/golgoth31/sreportal/api/v1alpha1"
	v1alpha2 "github.com/golgoth31/sreportal/api/v1alpha2"
)

const annotationV1Alpha1Groups = "sreportal.io/v1alpha1-groups"

func main() {
	kubeconfig := flag.String("kubeconfig", "", "path to kubeconfig")
	dryRun := flag.Bool("dry-run", false, "validate via server (DryRunAll) without persisting")
	flag.Parse()

	cfg, err := clientcmd.BuildConfigFromFlags("", *kubeconfig)
	if err != nil {
		fmt.Fprintf(os.Stderr, "kubeconfig: %v\n", err)
		os.Exit(1)
	}

	scheme := newScheme()
	c, err := client.New(cfg, client.Options{Scheme: scheme})
	if err != nil {
		fmt.Fprintf(os.Stderr, "client: %v\n", err)
		os.Exit(1)
	}

	ctx := context.Background()
	sum, err := Migrate(ctx, c, *dryRun)
	fmt.Printf("summary: processed=%d created=%d alreadyExist=%d skipped=%d failures=%d\n",
		sum.DNSProcessed, sum.Created, sum.AlreadyExist, sum.Skipped, sum.Failures)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(2)
	}
	if sum.Failures > 0 {
		os.Exit(2)
	}
}

func newScheme() *runtime.Scheme {
	s := runtime.NewScheme()
	_ = v1alpha1.AddToScheme(s)
	_ = v1alpha2.AddToScheme(s)
	return s
}
