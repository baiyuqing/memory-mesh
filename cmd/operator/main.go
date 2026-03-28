// Command operator starts the ottoplus Kubernetes operator.
//
// It watches Cluster CRDs (ottoplus.io/v1alpha1) and reconciles them
// by expanding the spec into a Composition, topologically sorting the
// blocks, and reconciling each block in dependency order.
//
// Phase 1 registers three blocks: storage.local-pv, datastore.postgresql,
// and gateway.pgbouncer.
package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"

	"github.com/baiyuqing/ottoplus/src/core/block"
	"github.com/baiyuqing/ottoplus/src/core/compiler"
	blocks "github.com/baiyuqing/ottoplus/src/operator/blocks"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/datastore/postgresql"
	"github.com/baiyuqing/ottoplus/src/operator/blocks/gateway/pgbouncer"
	localpv "github.com/baiyuqing/ottoplus/src/operator/blocks/storage/local-pv"

	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	ctrllog "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	runtimereconcile "sigs.k8s.io/controller-runtime/pkg/reconcile"
)

var clusterGVK = schema.GroupVersionKind{
	Group:   "ottoplus.io",
	Version: "v1alpha1",
	Kind:    "Cluster",
}

func main() {
	metricsAddr := flag.String("metrics-addr", ":9090", "Metrics listen address.")
	flag.Parse()

	ctrllog.SetLogger(zap.New(zap.UseDevMode(true)))
	logger := ctrllog.Log.WithName("operator")

	// Build registries.
	domainRegistry := block.NewRegistry()
	runtimeRegistry := blocks.NewRuntimeRegistry()

	for _, rt := range []blocks.BlockRuntime{
		&localpv.Block{},
		&postgresql.Block{},
		&pgbouncer.Block{},
	} {
		if err := domainRegistry.Register(rt); err != nil {
			logger.Error(err, "register block")
			os.Exit(1)
		}
		runtimeRegistry.Register(rt)
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), ctrl.Options{
		Metrics: metricsserver.Options{BindAddress: *metricsAddr},
	})
	if err != nil {
		logger.Error(err, "create manager")
		os.Exit(1)
	}

	clusterReconciler := &ClusterReconciler{
		client:          mgr.GetClient(),
		domainRegistry:  domainRegistry,
		runtimeRegistry: runtimeRegistry,
	}

	// Use unstructured watch so we don't need generated types.
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(clusterGVK)

	if err := ctrl.NewControllerManagedBy(mgr).
		For(u).
		Complete(clusterReconciler); err != nil {
		logger.Error(err, "create controller")
		os.Exit(1)
	}

	log.Println("ottoplus operator starting")
	if err := mgr.Start(ctrl.SetupSignalHandler()); err != nil {
		logger.Error(err, "run manager")
		os.Exit(1)
	}
}

// ClusterReconciler watches Cluster CRDs and reconciles them.
type ClusterReconciler struct {
	client          client.Client
	domainRegistry  *block.Registry
	runtimeRegistry *blocks.RuntimeRegistry
}

func (r *ClusterReconciler) Reconcile(ctx context.Context, req runtimereconcile.Request) (runtimereconcile.Result, error) {
	logger := ctrllog.FromContext(ctx).WithValues("cluster", req.NamespacedName)

	// Fetch the Cluster CR as unstructured.
	obj := &unstructured.Unstructured{}
	obj.SetGroupVersionKind(clusterGVK)
	if err := r.client.Get(ctx, req.NamespacedName, obj); err != nil {
		return runtimereconcile.Result{}, client.IgnoreNotFound(err)
	}

	// Parse spec into ClusterSpec.
	specRaw, found, err := unstructured.NestedMap(obj.Object, "spec")
	if err != nil || !found {
		logger.Error(err, "read spec from CR")
		return runtimereconcile.Result{}, err
	}
	specJSON, err := json.Marshal(specRaw)
	if err != nil {
		return runtimereconcile.Result{}, fmt.Errorf("marshal spec: %w", err)
	}
	var spec compiler.ClusterSpec
	if err := json.Unmarshal(specJSON, &spec); err != nil {
		return runtimereconcile.Result{}, fmt.Errorf("unmarshal spec: %w", err)
	}

	// Run the unified compilation pipeline.
	compiled, errs := compiler.Compile(spec, r.domainRegistry)
	if compiled == nil || len(errs) > 0 {
		logger.Error(errs[0], "compile composition", "allErrors", errs)
		return runtimereconcile.Result{}, errs[0]
	}

	comp := compiled.Composition
	sorted := compiled.Sorted

	// Reconcile each block in dependency order.
	outputs := make(map[string]map[string]string) // blockName -> portName -> value
	var blockStatuses []block.BlockStatus

	for _, ref := range sorted {
		rt, ok := r.runtimeRegistry.Get(ref.Kind)
		if !ok {
			return runtimereconcile.Result{}, fmt.Errorf("no runtime for block kind %q", ref.Kind)
		}

		// Resolve inputs from upstream outputs.
		resolvedInputs := make(map[string]string)
		for _, w := range comp.Wires {
			if w.ToBlock == ref.Name {
				if upstreamOutputs, ok := outputs[w.FromBlock]; ok {
					resolvedInputs[w.ToPort] = upstreamOutputs[w.FromPort]
				}
			}
		}

		result, err := rt.Reconcile(ctx, r.client, blocks.ReconcileRequest{
			ClusterName:      req.Name,
			ClusterNamespace: req.Namespace,
			BlockRef:         ref,
			ResolvedInputs:   resolvedInputs,
		})
		if err != nil {
			logger.Error(err, "reconcile block", "block", ref.Name, "kind", ref.Kind)
			blockStatuses = append(blockStatuses, block.BlockStatus{
				Kind: ref.Kind, Name: ref.Name, Phase: block.PhaseFailed, Message: err.Error(),
			})
			continue
		}

		outputs[ref.Name] = result.Outputs
		blockStatuses = append(blockStatuses, block.BlockStatus{
			Kind: ref.Kind, Name: ref.Name, Phase: result.Phase, Message: result.Message,
		})
	}

	// Update status.
	overallPhase := "Running"
	for _, bs := range blockStatuses {
		if bs.Phase == block.PhaseFailed {
			overallPhase = "Failed"
			break
		}
		if bs.Phase != block.PhaseReady {
			overallPhase = "Provisioning"
		}
	}

	// Find the DSN endpoint from the last dsn-producing block.
	endpoint := ""
	for i := len(sorted) - 1; i >= 0; i-- {
		if outs, ok := outputs[sorted[i].Name]; ok {
			if dsn, ok := outs["dsn"]; ok {
				endpoint = dsn
				break
			}
		}
	}

	statusMap := map[string]interface{}{
		"phase":    overallPhase,
		"endpoint": endpoint,
		"message":  fmt.Sprintf("reconciled %d blocks", len(sorted)),
	}
	statusBlocks := make([]interface{}, 0, len(blockStatuses))
	for _, bs := range blockStatuses {
		statusBlocks = append(statusBlocks, map[string]interface{}{
			"kind":    bs.Kind,
			"name":    bs.Name,
			"phase":   string(bs.Phase),
			"message": bs.Message,
		})
	}
	statusMap["blocks"] = statusBlocks

	if err := unstructured.SetNestedField(obj.Object, statusMap, "status"); err != nil {
		return runtimereconcile.Result{}, fmt.Errorf("set status: %w", err)
	}
	if err := r.client.Status().Update(ctx, obj); err != nil {
		logger.Error(err, "update cluster status")
		return runtimereconcile.Result{}, err
	}

	logger.Info("reconciliation complete", "phase", overallPhase, "blocks", len(sorted))
	return runtimereconcile.Result{}, nil
}
