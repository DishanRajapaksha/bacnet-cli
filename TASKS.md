# BACnet CLI Tasks

## Phase 1: Scaffold

- [x] Add the Go module, Makefile, entry point and repository layout.
- [x] Add config, output, CLI and BACnet client packages.

## Phase 2: BACnet/IP foundation

- [x] Implement interface and local-address binding.
- [x] Implement Who-Is and I-Am discovery.
- [x] Implement direct ReadProperty and WriteProperty commands.
- [x] Add routed BACnet network and MS/TP target fields.

## Phase 3: CLI contracts

- [x] Add YAML defaults, profiles, validation and CLI overrides.
- [x] Add table, text, JSON, JSON Lines and CSV output.
- [x] Align exit codes with the sibling industrial CLIs.
- [x] Keep writes dry-run by default and require `--yes`.
- [x] Add Bash and Zsh completions.

## Phase 4: Inspection

- [x] Add device object-list inspection.
- [x] Add device identity inspection.
- [x] Add local object-type and property catalogues.
- [x] Add BACnet router discovery.

## Phase 5: Named devices and points

- [x] Add reusable named device definitions.
- [x] Add named point definitions with object, property, unit and write metadata.
- [x] Add profile merging and cross-reference validation.
- [x] Add `devices`, `points`, `read-point`, `write-point` and `watch-point`.

## Phase 6: Multi-point operations

- [x] Add `read-points` with repeated point and device selectors.
- [x] Add `watch-points` with count, duration and interval bounds.
- [x] Preserve successful samples when some point reads fail.
- [x] Add fail-fast mode and shared exit-code behaviour.
- [x] Use one BACnet session with sequential ReadProperty calls.

## Phase 7: Fleet inventory and generated configuration

- [x] Add `inventory` with discovery filters and identity enrichment.
- [x] Preserve discovered devices when identity reads partially fail.
- [x] Add `generate-config` with YAML output and deterministic device names.
- [x] Add collision handling, discovery-only generation and overwrite protection.
- [x] Keep inventory and generation within one BACnet session.

## Later phases

- [ ] Add optional object-to-point template generation after broader device testing.
- [ ] Add COV subscriptions when the client dependency provides dependable support.
- [ ] Add event notification inspection when the client dependency provides dependable support.
- [ ] Add ReadPropertyMultiple only after interoperability testing against real devices.
- [ ] Evaluate BACnet/SC support separately from BACnet/IP.
