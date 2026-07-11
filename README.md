# bacnet-cli

A script-friendly BACnet/IP command-line client for discovery, inspection, polling and carefully gated property writes.

The command structure follows the sibling industrial CLIs:

- configuration through `init-config` and `validate-config`
- global flags before or after the command
- table, text, JSON and CSV snapshots
- text, JSON Lines and CSV streams
- writes are dry-run by default and require `--yes`
- stable exit-code meanings across the CLI family

## Features

- BACnet/IP device discovery using Who-Is and I-Am
- connection diagnostics
- single-property reads
- device object-list inspection
- polling-based property watches
- property writes with BACnet priority and null relinquish support
- BACnet router discovery
- YAML profiles
- Bash and Zsh completions

This first release intentionally omits COV subscriptions, events and ReadPropertyMultiple. The underlying Go implementation does not yet provide those areas with the same confidence as Who-Is, ReadProperty and WriteProperty.

## Install

```bash
go install github.com/DishanRajapaksha/bacnet-cli@latest
```

The current BACnet dependency uses CGO, so a working C compiler is required when building from source.

## Quick start

```bash
go build -o bacnet-cli .
./bacnet-cli init-config
./bacnet-cli validate-config --profile local
./bacnet-cli discover --interface en0
```

On Linux, the interface is commonly `eth0`, `ens18` or a similar predictable-network name. When neither `--interface` nor `--local-ip` is supplied, the CLI selects the first active non-loopback IPv4 interface.

## Commands

### Discover devices

```bash
bacnet-cli discover --interface en0
bacnet-cli discover --low 1000 --high 1999 --format json
```

### Test communication

```bash
bacnet-cli test-connection --device-id 1234
```

Without `--device-id`, any I-Am response counts as success.

### Read a property

```bash
bacnet-cli read \
  --device-id 1234 \
  --object analog-input:1 \
  --property present-value
```

A direct address avoids the preliminary Who-Is:

```bash
bacnet-cli read \
  --device-id 1234 \
  --device-address 192.0.2.20 \
  --object analog-input:1 \
  --property present-value
```

Object types and properties may also be numeric:

```bash
bacnet-cli read --device-id 1234 --object 0:1 --property 85
```

### List objects

```bash
bacnet-cli objects --device-id 1234
bacnet-cli objects --device-id 1234 --format csv
```

The command reads the device object list, then attempts to retrieve each object's name and description. Large or poorly behaved devices may expose weaknesses in the upstream library's beta ReadPropertyMultiple support.

### Poll a property

```bash
bacnet-cli watch \
  --device-id 1234 \
  --object analog-input:1 \
  --property present-value \
  --interval 2s \
  --format jsonl
```

Use `--count 10` for a finite run. A count of zero continues until interrupted.

### Write a property

Writes are dry-run by default:

```bash
bacnet-cli write \
  --device-id 1234 \
  --object analog-output:1 \
  --property present-value \
  --type float32 \
  --value 21.5
```

Transmit only after reviewing the plan:

```bash
bacnet-cli write \
  --device-id 1234 \
  --object analog-output:1 \
  --property present-value \
  --type float32 \
  --value 21.5 \
  --priority 16 \
  --yes
```

Relinquish a priority by writing BACnet null:

```bash
bacnet-cli write \
  --device-id 1234 \
  --object analog-output:1 \
  --property present-value \
  --null \
  --priority 16 \
  --yes
```

Supported write types are `string`, `bool`, `uint`, `int`, `float32` and `float64`.

## Configuration

```yaml
connection:
  port: 47808
  subnet_cidr: 24
  timeout: 5s
output:
  format: table
default_profile: local
profiles:
  local:
    connection:
      interface: en0
      port: 47808
      subnet_cidr: 24
      timeout: 5s
  linux:
    connection:
      interface: eth0
      port: 47808
      subnet_cidr: 24
      timeout: 5s
```

Use `local_ip` instead of `interface` when binding by address is more convenient. Do not set both.

## Output contract

Snapshot commands support:

```text
table, text, json, csv
```

Streaming commands support:

```text
text, jsonl, csv
```

## Exit codes

| Code | Meaning |
|---:|---|
| 0 | Success |
| 1 | General error |
| 2 | Usage or configuration error |
| 3 | Transport or connection error |
| 4 | BACnet request or protocol error |
| 7 | Write or control rejected |
| 8 | Timeout |
| 9 | Output or formatting error |

## Safety

BACnet writes can alter physical equipment. The CLI therefore refuses to transmit a write unless `--yes` is present. Use a test network, confirm object identifiers and priorities, and keep a known route back to the original value. Buildings are famously poor at accepting pull requests.

## Library

The implementation currently uses `github.com/NubeDev/bacnet`, pinned to a known commit. It provides Who-Is/I-Am, ReadProperty, WriteProperty, object discovery and router discovery. Its COV, event and several bulk-service areas remain incomplete, so the CLI does not pretend otherwise.
