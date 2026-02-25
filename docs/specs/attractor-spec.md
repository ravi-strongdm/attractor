# Attractor Spec Reference

This file is a stub pointing to the canonical specification.

**Canonical source**: https://github.com/strongdm/attractor

## Summary

Attractor is a DOT-graph-based agentic pipeline runner. Pipelines are described
as Graphviz DOT files where nodes have a `type` attribute selecting a handler and
edges may have `label` conditions.

The engine traverses the graph from the `start` node, executing each node's handler,
evaluating edge conditions against the pipeline context, and checkpointing state
after every node for resumability.

Key concepts:
- **Node types**: `start`, `exit`, `set`, `codergen`, `wait.human`, `fan_out`, `fan_in`
- **Pipeline context**: key-value store threaded through all nodes
- **Edge conditions**: Go template expressions (`{{ .var }} == "value"`)
- **Checkpoints**: JSON snapshots saved after each node for `resume`

See `CLAUDE.md` for the developer quick-reference.
