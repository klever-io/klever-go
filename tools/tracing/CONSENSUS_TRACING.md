# Consensus Tracing Guide

This document describes the distributed tracing implementation for KleverGo's BLS consensus mechanism, providing insights into how to monitor and analyze consensus operations.

## Overview

The consensus tracing system provides hierarchical, distributed tracing across the entire consensus lifecycle, from slot initialization through block finalization. The tracing is implemented using OpenZipkin format and can be visualized in Zipkin UI or any compatible tracing backend.

## Architecture

### Trace Hierarchy

The consensus tracing follows a hierarchical structure:

```
consensus.slot.{slot_number}                    [Slot Level - Root Span]
├── consensus.subslot.StartSlot                 [Subslot Level]
│   └── consensus.startSlot.resetState
├── consensus.subslot.Block                     [Subslot Level]
│   ├── consensus.block.doBlockJob
│   ├── consensus.block.createHeader
│   ├── consensus.block.createBlock
│   └── consensus.block.sendBlock
├── consensus.subslot.Signature                 [Subslot Level]
│   ├── consensus.signature.doSignatureJob
│   ├── consensus.signature.createSignatureShare
│   ├── consensus.signature.broadcastSignature
│   └── consensus.signature.waitAllSignatures
└── consensus.subslot.EndSlot                   [Subslot Level]
    ├── consensus.endSlot.checkSignatures
    ├── consensus.endSlot.aggregateSignatures
    ├── consensus.endSlot.commitBlock
    └── consensus.endSlot.broadcastBlock
```

## Implementation Details

### 1. Slot-Level Tracing (chronology.go)

The root trace span is created when a new slot begins:

```go
// In chronology.go - updateSlot()
if oldSlotIndex != chr.slotManager.Index() {
    // Start slot-level tracing
    if tracing.IsEnabled() {
        tracer := tracing.GetConfiguredTracer()
        taskName := fmt.Sprintf("consensus.slot.%d", chr.slotManager.Index())
        tracer.StartWithTags(taskName, map[string]string{
            "slot.index":     fmt.Sprintf("%d", chr.slotManager.Index()),
            "slot.timestamp": fmt.Sprintf("%d", chr.slotManager.Timestamp().Unix()),
        })
    }
}
```

The slot trace is closed when the slot ends (in subslotEndSlot.go):

```go
defer func() {
    taskName := fmt.Sprintf("consensus.slot.%d", sr.SlotManager().Index())
    tracer := tracing.GetConfiguredTracer()
    tracer.Stop(taskName)
}()
```

### 2. Subslot-Level Tracing

Each subslot (StartSlot, Block, Signature, EndSlot) creates a child span under the active slot trace:

```go
// In subslot.go - DoWork() - Using the simplified API
defer tracing.StartSpan(
    "consensus.subslot."+sr.name,
    "subslot.name", sr.name,
    "slot.index", strconv.FormatInt(slotManager.Index(), 10),
)()
```

### 3. Operation-Level Tracing

Within each subslot, specific operations are traced:

#### StartSlot Operations
- `consensus.startSlot.resetState` - Resetting consensus state for new slot

#### Block Operations
- `consensus.block.createHeader` - Creating block header
- `consensus.block.createBlock` - Building block with transactions
- `consensus.block.sendBlock` - Broadcasting block to validators

#### Signature Operations
- `consensus.signature.createSignatureShare` - Creating BLS signature share
- `consensus.signature.broadcastSignature` - Broadcasting signature
- `consensus.signature.waitAllSignatures` - Waiting for signature collection

#### EndSlot Operations
- `consensus.endSlot.checkSignatures` - Validating collected signatures
- `consensus.endSlot.aggregateSignatures` - Aggregating BLS signatures
- `consensus.endSlot.commitBlock` - Committing block to blockchain
- `consensus.endSlot.broadcastBlock` - Broadcasting finalized block

### 4. Transaction Processing Tracing

Transaction preprocessing includes detailed tracing:

```go
// In preprocess/transactions.go - Using the simplified API
defer tracing.StartSpan(
    "process.block.preprocess.processBlockTxs",
    "block.nonce", strconv.FormatUint(blk.GetNonce(), 10),
    "num.txs", strconv.Itoa(len(blk.TxHashes)),
)()

// For specific operations within the function
func computeGasProvided() {
    defer tracing.StartSpan("process.transactions.computeGasProvided")()
    // ... computation logic
}
```

## Configuration

### Environment Variables

Enable consensus tracing using environment variables:

```bash
# Enable tracing
export KLEVER_TRACING_ENABLED=true

# Configure Zipkin server
export KLEVER_TRACING_SERVER_URL=http://localhost:9411

# Set unique service name for multi-instance deployments
export KLEVER_TRACING_SERVICE_NAME=validator-node-1

# Configure batching
export KLEVER_TRACING_BATCH_SIZE=100
export KLEVER_TRACING_PUSH_INTERVAL=5s

# Save traces on exit (useful for debugging)
export KLEVER_TRACING_SAVE_ON_EXIT=true
export KLEVER_TRACING_SAVE_PATH=./traces
```

### Docker Compose Example

```yaml
version: '3.8'

services:
  zipkin:
    image: openzipkin/zipkin
    ports:
      - "9411:9411"
    environment:
      - STORAGE_TYPE=mem
      - MEM_MAX_SPANS=1000000

  validator-1:
    image: kleverapp/klever-go:latest
    environment:
      - KLEVER_TRACING_ENABLED=true
      - KLEVER_TRACING_SERVER_URL=http://zipkin:9411
      - KLEVER_TRACING_SERVICE_NAME=validator-1
      - INSTANCE_ID=validator-1
    depends_on:
      - zipkin

  validator-2:
    image: kleverapp/klever-go:latest
    environment:
      - KLEVER_TRACING_ENABLED=true
      - KLEVER_TRACING_SERVER_URL=http://zipkin:9411
      - KLEVER_TRACING_SERVICE_NAME=validator-2
      - INSTANCE_ID=validator-2
    depends_on:
      - zipkin
```

## Tags and Metadata

The tracing system automatically extracts and adds contextual tags:

### Automatic Tags (via Tag Extractors)

1. **Consensus Tags**:
   - `component`: "consensus"
   - `consensus.phase`: "start" | "block" | "signature" | "end"
   - `operation.type`: "create" | "broadcast" | "commit" | "aggregate"
   - `subslot.name`: Name of the current subslot

2. **Node Tags**:
   - `slot.index`: Current slot number
   - `slot.timestamp`: Slot start timestamp
   - `node.role`: "leader" | "validator"
   - `block.nonce`: Block nonce (when applicable)

3. **Network Tags**:
   - `network.direction`: "inbound" | "outbound"
   - IP address in endpoint information

### Custom Tags

You can add custom tags programmatically:

```go
tracer := tracing.GetConfiguredTracer()
tracer.AddTag("validator.pubkey", hex.EncodeToString(pubKey))
tracer.AddTag("signatures.count", fmt.Sprintf("%d", len(signatures)))
```

### Annotations

Add time-stamped events within a span:

```go
tracer.AddAnnotation("Starting signature aggregation")
tracer.AddAnnotation(fmt.Sprintf("Aggregated %d signatures", count))
```

## Analyzing Traces

### 1. Slot Timing Analysis

View the complete slot lifecycle:
- Filter by service name and slot index
- Analyze time distribution across subslots
- Identify bottlenecks in consensus phases

### 2. Leader vs Validator Behavior

Compare traces between leader and validator nodes:
- Leaders show block creation and signature aggregation
- Validators show signature creation and block validation
- Network latency between nodes

### 3. Performance Metrics

Key metrics to monitor:
- **Slot Duration**: Total time from slot start to end
- **Block Creation Time**: Time to create and fill block with transactions
- **Signature Collection Time**: Time to collect required signatures
- **Commit Time**: Time to finalize and commit block

### 4. Common Trace Patterns

#### Successful Consensus Round
```
slot.123 [3.5s]
├── StartSlot [50ms]
├── Block [1.2s] (leader only)
├── Signature [1.5s]
└── EndSlot [750ms]
```

#### Extended Consensus (timeout)
```
slot.124 [10s]
├── StartSlot [50ms]
├── Block [1.2s]
├── Signature [7s] (timeout waiting for signatures)
└── EndSlot [extended]
```

## Troubleshooting

### Missing Traces

1. Verify tracing is enabled:
   ```bash
   echo $KLEVER_TRACING_ENABLED  # Should be "true"
   ```

2. Check Zipkin connectivity:
   ```bash
   curl http://localhost:9411/api/v2/services
   ```

3. Verify service name uniqueness for multi-instance setups

### Performance Impact

- Tracing adds minimal overhead (~1-2% CPU)
- Network bandwidth: ~1KB per trace
- Memory: ~100MB for 100,000 cached spans

### Debug Mode

For detailed debugging, save traces locally:

```bash
export KLEVER_TRACING_SAVE_ON_EXIT=true
export KLEVER_TRACING_SAVE_PATH=./debug-traces
```

Traces will be saved as JSON files on graceful shutdown.

## Best Practices

1. **Production Deployment**:
   - Use sampling (future feature) for high-traffic nodes
   - Configure appropriate batch sizes (100-500)
   - Use persistent storage for Zipkin in production

2. **Development**:
   - Enable full tracing for all operations
   - Use local Zipkin instance
   - Save traces for offline analysis

3. **Multi-Instance**:
   - Always set unique service names
   - Use INSTANCE_ID or NODE_NAME environment variables
   - Include node role in service name (validator-1, observer-2)

4. **Analysis**:
   - Create dashboards for key metrics
   - Set up alerts for anomalous slot times
   - Regular review of trace patterns

## API Reference

### Tracer Methods

```go
// Get configured tracer instance
tracer := tracing.GetConfiguredTracer()

// Start a span with tags
tracer.StartWithTags("operation.name", map[string]string{
    "key": "value",
})

// Stop a span
tracer.Stop("operation.name")

// Add tags to active span
tracer.AddTag("key", "value")

// Add annotation to active span
tracer.AddAnnotation("Event occurred")

// Check if tracing is enabled
if tracing.IsEnabled() {
    // Tracing code
}
```

## Examples

### Example 1: Tracing Custom Consensus Operation

```go
func (c *CustomConsensus) ProcessBlock(block *Block) error {
    tracer := tracing.GetConfiguredTracer()
    
    tracer.Start("consensus.custom.processBlock")
    defer tracer.Stop("consensus.custom.processBlock")
    
    tracer.AddTag("block.height", fmt.Sprintf("%d", block.Height))
    tracer.AddTag("block.txCount", fmt.Sprintf("%d", len(block.Transactions)))
    
    tracer.AddAnnotation("Starting block validation")
    
    // Validation logic
    tracer.Start("consensus.custom.validateBlock")
    err := c.validateBlock(block)
    tracer.Stop("consensus.custom.validateBlock")
    
    if err != nil {
        tracer.AddTag("error", err.Error())
        return err
    }
    
    tracer.AddAnnotation("Block validated successfully")
    
    // Processing logic
    tracer.Start("consensus.custom.applyBlock")
    err = c.applyBlock(block)
    tracer.Stop("consensus.custom.applyBlock")
    
    return err
}
```

### Example 2: Analyzing Slow Consensus

```go
// Use the TraceAnalyzer to identify slow operations
analyzer := tracing.NewTraceAnalyzer()
analyzer.LoadFromFile("traces_20240829_120000.json")

// Get slowest operations
fmt.Println(analyzer.GetSlowOperations(10))

// Get consensus statistics
fmt.Println(analyzer.GetConsensusStats())

// View trace tree for specific slot
fmt.Println(analyzer.GetTraceTree("trace-id-123"))
```

## Future Enhancements

- [ ] Sampling strategies for production environments
- [ ] Metrics integration (Prometheus export)
- [ ] Trace correlation with logs
- [ ] Custom span processors
- [ ] Trace-based testing utilities
- [ ] Real-time anomaly detection
- [ ] Consensus visualization dashboard

## Support

For issues or questions about consensus tracing:
1. Check trace logs in `./traces` directory
2. Verify Zipkin UI at http://localhost:9411
3. Review this documentation
4. Submit issues to the KleverGo repository