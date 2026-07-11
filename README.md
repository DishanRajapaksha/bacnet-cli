# bacnet-cli

A script-friendly BACnet/IP command-line client for discovery, inspection, polling and carefully gated property writes.

The command structure follows the sibling industrial CLIs:

- configuration through `init-config` and `validate-config`
- global flags before or after the command
- table, text, JSON and CSV snapshots
- text, JSON Lines and CSV streams
- writes are dry-run by default and require `--yes`
- stable exit-code meanings across the CLI family
- reusable named devices and points for operator-facing workflows

## Features

- BACnet/IP device discovery using Who-Is and I-Am
- connection diagnostics
- single-property reads and writes
- configured named devices and points
- named point reads, writes and polling
- device identity inspection
- device object-list inspection
- BACnet object-type and property catalogues
- BACnet priority and null relinquish support
- BACnet router discovery
- YAML profiles
- Bash and Zsh completions

COV subscriptions, events and general ReadPropertyMultiple commands remain deliberately excluded. The underlying Go implementation does not yet provide those areas with the same confidence as Who-Is, ReadProperty and WriteProperty.

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
./bacnet-cli devices
./bacnet-cli points
```

On Linux, the interface is commonly `eth0`, `ens18` or a similar predictable-network name. When neither `--interface` nor `--local-ip` is supplied, the CLI selects the first active non-loopback IPv4 interface.

## Discovery and diagnostics

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

### Identify a device

Use a configured device name:

```bash
bacnet-cli identify ahu
bacnet-cli identify ahu --format json
```

Or supply a target directly:

```bash
bacnet-cli identify \
  --device-id 1234 \
  --device-address 192.0.2.20
```

The command reads standard device-object properties such as object name, vendor, model, firmware, application version, protocol revision, database revision and segmentation. Unsupported optional properties are reported per field instead of aborting the entire command.

## Direct property commands

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

## Named devices and points

Named devices keep BACnet addressing details in configuration. Named points then reference a device, object and property.

```yaml
connection:
  port: 47808
  subnet_cidr: 24
  timeout: 5s
output:
  format: table

devices:
  - name: ahu
    device_id: 1234
    address: 192.0.2.20
    port: 47808
    max_apdu: 1476
    segmentation: 3

  - name: routed_vav
    device_id: 2001
    network: 10
    mstp_mac: 7
    max_apdu: 480
    segmentation: 3

points:
  - name: supply_air_temperature
    device: ahu
    object: analog-input:1
    property: present-value
    unit: °C

  - name: cooling_setpoint
    device: ahu
    object: analog-value:1
    property: present-value
    type: float32
    unit: °C
    writable: true
    priority: 16

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

An omitted device address causes the CLI to discover the device by instance number. Use an explicit address when predictable unicast traffic matters. Routed MS/TP targets use `network` and `mstp_mac`.

### List configured devices and points

```bash
bacnet-cli devices
bacnet-cli devices --format json
bacnet-cli points
bacnet-cli points --format csv
```

### Read a named point

```bash
bacnet-cli read-point supply_air_temperature
bacnet-cli --profile local --format json read-point supply_air_temperature
```

### Watch a named point

```bash
bacnet-cli watch-point supply_air_temperature \
  --interval 2s \
  --duration 30s \
  --format jsonl
```

`--count` and `--duration` can be used to bound polling. Zero means no limit.

### Write a named point

Named point writes require `writable: true` and a configured `type`. They remain dry-run by default:

```bash
bacnet-cli write-point cooling_setpoint --value 21.5
```

Send after reviewing the plan:

```bash
bacnet-cli write-point cooling_setpoint --value 21.5 --yes
```

Override the configured priority when necessary:

```bash
bacnet-cli write-point cooling_setpoint --value 22.0 --priority 8 --yes
```

Relinquish the selected priority:

```bash
bacnet-cli write-point cooling_setpoint --null --yes
```

## Local catalogues

List accepted object names, aliases and numeric identifiers:

```bash
bacnet-cli object-types
bacnet-cli object-types --format json
```

List accepted property names and numeric identifiers:

```bash
bacnet-cli properties
bacnet-cli properties --format csv
```

These commands are local and do not open a BACnet socket.

## Configuration rules

- Device names and point names must be unique.
- Points must reference a configured device.
- Objects use `TYPE:INSTANCE`, for example `analog-input:1` or `0:1`.
- The default point property is `present-value`.
- The default array index is BACnet `ARRAY_ALL`.
- Writable points require a write type.
- The default write priority is 16.
- The default device port is 47808.
- The default maximum APDU is 1476.
- The default segmentation value is 3, meaning no segmentation.
- Use `local_ip` instead of `interface` when binding by address is more convenient. Do not set both.

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

BACnet writes can alter physical equipment. The CLI refuses to transmit a write unless `--yes` is present, and named points must also be marked writable. Use a test network, confirm object identifiers and priorities, and keep a known route back to the original value. Buildings are famously poor at accepting pull requests.

## Library

The implementation currently uses `github.com/NubeDev/bacnet`, pinned to a known commit. It provides Who-Is/I-Am, ReadProperty, WriteProperty, object discovery and router discovery. Its COV, event and several bulk-service areas remain incomplete, so the CLI does not pretend otherwise.
