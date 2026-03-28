// Package reconciler implements the top-level composition reconciler
// that orchestrates block reconciliation for Cluster CRs.
//
// The compilation pipeline (expand, normalize, auto-wire, validate,
// topo-sort) is handled by the core/compiler package. This package
// focuses on runtime reconciliation of individual blocks.
package reconciler
