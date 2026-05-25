# operator ms

## Summary

Multisign actions: helpers to decode/encode multisign API data, append transactions to the multisign service, request broadcasts, fetch transactions, and sign pending multisign transactions.

## Usage

`operator ms [command]`

## Subcommands

- `decode [Transaction]` — decode a transaction JSON and display its raw form.
- `encode [Transaction]` — encode a transaction from multisign API format into the operator's encoded form.
- `append [Transaction]` — append transaction data into the multisign API (accepts raw TX JSON or already-encoded multisign API data).
- `broadcast [Transaction]` — ask the multisign API to broadcast/execute the transaction on-chain (calls the multisign service broadcast endpoint).
- `by-hash [Transaction]` — fetch a transaction from the multisign API by its hash.
- `by-address [Address]` — list transactions for a given address from the multisign API.
- `sign [txHash]` — sign a multisign transaction and post the signature to the multisign API.

## Flags

- `--multisign-api` — multisign API URL (default: `https://multisign.mainnet.klever.org`). This flag is persistent on the `ms` command.
- `--yes`, `-y` — skip the confirmation prompt (useful for scripts / non-interactive use).

## Important note about `sign`

The `operator ms sign` command fetches a pending multisign transaction, signs it with the operator's private key, and posts the signed data (the signature) back to the multisign service. It does NOT broadcast or execute the transaction on-chain itself. "Posting the signature" means submitting the operator's signature to the multisign API so the multisign service can collect signatures and, separately, perform any broadcast when configured or requested.

## Examples

--------
- Sign a specific transaction by hash (posts signature to multisign API):
```
operator ms sign <txHash>
```

- Interactively select a pending transaction to sign:
```
operator ms sign
```

- Append a transaction (or encoded multisign data) to the multisign service:
```
operator ms append '<transaction-or-encoded-json>'
```

- Request the multisign service to broadcast a transaction by hash (this triggers the multisign service to attempt on-chain broadcast):
```
operator ms broadcast <txHash>
```

See other operator docs for `operator sign` (local signing/broadcast) and general CLI usage.
