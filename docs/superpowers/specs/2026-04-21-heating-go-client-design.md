# Heating Go Client Design

## Summary

Build a Go-based heater test client as a reusable library plus a thin CLI wrapper.
The first version should connect to the Garmin websocket, replay the required startup traffic, keep the session alive, power on the Alde heater if needed, infer the current target temperature when possible, and set a new target temperature safely.

This design intentionally treats the Garmin and Alde combination as a high-latency, eventually consistent system.
The goal is not a full heater protocol model on day one.
The goal is a safe, inspectable control path that can evolve into an interface service with a more stable API later.

## Goals

- Replace the current heater-test direction with a Go implementation.
- Provide a reusable library package with a small CLI wrapper.
- Support these first-pass actions:
  - connect and bootstrap the websocket session
  - ensure heater power is on
  - get the current target temperature when it can be inferred
  - set a new target temperature directly if future decoding allows it, otherwise by paced 0.5 C increments
- Support verbose tracing that echoes relevant heating frames during an interaction and for a short window afterwards.
- Preserve room for future expansion into a long-running service that exposes a saner heater API.

## Non-Goals

- Full support for hot water controls in the first pass.
- Full support for gas and electric fuel-source controls in the first pass.
- A complete reverse-engineered heater protocol before implementation starts.
- A production-ready multi-client service in the first pass.

## Constraints And Assumptions

- The Garmin websocket requires bootstrap traffic before useful interaction.
- The session requires periodic heartbeat traffic to remain alive.
- Heater startup has meaningful latency, including a period where the Garmin UI becomes non-interactive.
- Some heater state appears to round-trip through the Alde unit rather than being purely local UI state.
- The target temperature may not be explicitly encoded in currently captured messages.
- We expect to refine message decoding as new HAR or NDJSON captures become available.

## Capture Findings Incorporated Into This Design

The `Heating.har` capture from 2026-04-21 provides several concrete heater-control observations:

- heater power-on is sent as `type=17 cmd=0 data=[101,0,3]`
- heater power-off is sent as `type=17 cmd=0 data=[101,0,5]`
- temperature-up appears as a press and release pair on signal `107`:
  - press: `type=17 cmd=1 data=[107,0,1]`
  - release: `type=17 cmd=1 data=[107,0,0]`
- temperature-down appears as a press and release pair on signal `108`:
  - press: `type=17 cmd=1 data=[108,0,1]`
  - release: `type=17 cmd=1 data=[108,0,0]`

The same capture also strongly suggests that target temperature is observable through a report rather than only inferred from button counts:

- signal `105` with `type=16 cmd=5 size=8` changes after each temperature step
- the `signal 105` payload returns to its original value when the target temperature returns to `8 C`
- the sequence observed across the known temperature changes is:
  - `8.0 C`: `[105,0,128,22,12,74,4,0]`
  - `8.5 C`: `[105,0,0,22,0,76,4,0]`
  - `9.0 C`: `[105,0,0,22,244,77,4,0]`
  - `9.5 C`: `[105,0,0,22,232,79,4,0]`
  - `10.0 C`: `[105,0,0,22,230,81,4,0]`

This does not yet prove the exact field encoding, but it is enough to justify treating `signal 105` as the primary candidate for target-temperature decoding in the first Go implementation.

Additional captures taken later the same day strengthen that conclusion:

- `Heating 13C-20C.har` shows the same `signal 105` payload changing monotonically from `13.0 C` through `20.0 C`
- `Load with Heating on at 20C.har` shows a stable loaded state at `20.0 C` with the same `signal 105` family

Across the current captures, the best working decode is:

- define `raw = data[4] + 256*data[5]`
- target temperature increases by roughly `500` raw units per `0.5 C`
- there is a small discontinuity of about `+10` raw units at `10.0 C`, `20.0 C`, and likely each ten-degree boundary

This is good enough for a first-pass implementation that rounds to the nearest valid `0.5 C` setpoint, but the library should still treat it as a capture-derived decoder rather than a fully proven protocol constant.

## Recommended Approach

Use a hybrid session client.

The library should execute commands conservatively, but it should be structured around an internal derived state model.
That gives us a safe first implementation without forcing us to fully decode all heater traffic up front, and it leaves a clear path toward a future interface service.

## Architecture

### Go module layout

Use a Go module with:

- a library package for websocket session management and heater control
- a thin CLI entrypoint that maps subcommands and flags onto library calls
- fixture-driven tests that replay captures into the state interpreter and command logic

Illustrative package split:

- `internal/ws` or `pkg/ws`: websocket transport, bootstrap replay, heartbeat, reconnect policy
- `internal/protocol` or `pkg/protocol`: frame envelope parsing and normalization
- `internal/heating` or `pkg/heating`: heater state interpreter and command sequencing
- `cmd/heatingctl`: CLI wrapper

The exact package names can follow normal Go repo conventions, but the boundaries should remain the same.

### Core types

The library should define a small set of explicit types:

- `SessionConfig`: websocket URL, origin, headers, cookies, heartbeat interval, timeouts, verbose settings
- `Session`: owns the live websocket connection, bootstrap, heartbeat, receive loop, and frame subscriptions
- `Frame`: normalized Garmin websocket frame with direction, timestamp, envelope fields, and signal metadata when known
- `HeaterState`: best-effort derived heater state
- `HeaterClient`: high-level operations built on a `Session`

`HeaterState` should start small and evidence-aware:

- `PowerKnown`
- `PowerOn`
- `TargetTempKnown`
- `TargetTempC`
- `ReadyKnown`
- `Ready`
- `LastUpdated`
- evidence fields describing whether a value came from explicit decoding, inferred correlation, or a fallback heuristic

## Data Flow

The session should ingest all websocket frames and normalize them into a common frame type.
Those frames then feed two consumers:

- a heater interpreter that derives best-effort state from heater-group traffic
- an optional verbose tracer that prints relevant frames around an operation

The heater interpreter should not assume every incoming frame is authoritative.
Instead, it should attach confidence or evidence to state changes and only allow command logic to depend on values that are known enough for safe use.

The command layer should expose a simple API even when the underlying evidence is partial:

- `EnsureOn(ctx)`
- `GetTargetTemp(ctx) (float64, error)`
- `SetTargetTemp(ctx, target float64) error`

Internally, these methods may use different strategies depending on what the interpreter has learned.

## Command Behavior

### EnsureOn

`EnsureOn` should:

1. inspect the current derived state
2. return immediately if the heater is already known to be on
3. send the heater-on action if the heater is known to be off or unknown
4. enter a readiness wait loop before allowing further commands

The readiness wait loop exists because the Garmin UI and Alde system appear to have startup latency.
Readiness may be inferred from one or more signals:

- heater power-state transitions on signal `101`
- follow-up heater status fan-out after power-on, such as fuel and hot-water status frames
- target-temperature related activity stabilizing, especially `signal 105`
- future captures that reveal an explicit ready or enabled signal

If readiness cannot be established within a bounded timeout, the operation should fail with a useful error.

### GetTargetTemp

`GetTargetTemp` should return the current target temperature only when the library has enough evidence to do so safely.

The first version should support two evidence paths:

- decoding from `signal 105` using the current capture-derived raw-to-temperature mapping
- inferred target state if message patterns make the current value reliable enough

If the temperature cannot be determined confidently, the method should return an error rather than a guess.

### SetTargetTemp

`SetTargetTemp` should support two internal strategies behind one public API:

- direct set if future protocol work reveals a direct target-temperature write
- stepwise adjustment in 0.5 C increments from the current known target temperature

For stepwise adjustment, the library should:

1. require a known current target temperature
2. calculate the required number of 0.5 C steps
3. send one increment or decrement at a time
4. wait after each step for the expected round-trip evidence or timeout
5. stop early if the state diverges or confirmation fails

If the current target temperature is not known, the method should fail safely instead of blindly walking toward a guessed value.

The first implementation should explicitly model the observed Garmin button semantics as press and release pairs for signals `107` and `108`, rather than assuming a single fire-and-forget write.

Because live behavior can coalesce or skip intermediate target-temperature reports, stepwise adjustment should follow the last observed decoded target rather than requiring every intermediate half-degree report to appear in the websocket stream.

## Verbose Mode

The CLI and library should support a verbose mode similar in spirit to the current Python filtering workflow.

Verbose mode should:

- print relevant heating frames during connection and command execution
- always show matching sent heater control frames
- show likely related receive frames during the interaction
- highlight candidate target-temperature reports, especially `signal 105`
- continue tracing relevant heating traffic for a short configurable window after the command finishes

The output should help answer questions like:

- what was sent
- what immediate responses came back
- whether the heater appeared to become ready
- whether the target temperature appeared to change

The default trace window should be a few seconds after the interaction, with a flag to override it.
Verbose output should be grouped by operation so live testing remains readable.

## Error Handling

All live operations should use bounded retries and clear timeouts.

Failure cases should include:

- websocket connection failure
- bootstrap failure
- heartbeat failure
- heater not becoming ready in time
- target temperature still unknown after observation
- increment or decrement confirmation missing
- state contradiction, such as responses that do not match the expected step direction

Errors should preserve enough context for debugging, including operation name, timeout stage, and the most recent relevant frame evidence when available.

## CLI Shape

The CLI should remain thin and focus on manual testing.

Initial commands:

- `heatingctl ensure-on`
- `heatingctl get-target-temp`
- `heatingctl set-target-temp --value 21.0`

Useful first-pass flags:

- `--ws-url`
- `--origin`
- `--header`
- `--cookie`
- `--bootstrap-from-har`
- `--heartbeat-interval`
- `--timeout`
- `--verbose`
- `--trace-window`

The CLI should print human-readable results by default and enough detail in verbose mode to correlate command behavior with live frames.

## Testing Strategy

Most behavior should be tested without a live van connection.

Tests should use replay fixtures built from HAR or NDJSON captures to cover:

- frame normalization
- heater-group signal filtering
- derived state transitions
- command sequencing decisions
- timeout and retry behavior
- verbose trace window behavior

The websocket transport itself can be covered by thinner integration tests or local fakes.
Live testing against the Garmin unit should remain a manual verification step for now.

## Extensibility

This design should make later work incremental rather than disruptive.

Likely next additions:

- hot water on and off support
- gas and electric source controls
- explicit preference controls for gas versus electricity
- a long-running service layer with a stable internal API
- better heater decoding from new captures

The service layer should be able to reuse the same `Session`, `Frame`, `HeaterState`, and `HeaterClient` concepts rather than replacing them.

## Open Questions

- Which `signal 101` receive value should be treated as authoritative for off, powering up, ready, and on states?
- Which combination of post-power-on frames most reliably indicates heater readiness after startup?
- Is the current `signal 105` decoder exact, or is there still a small hidden transform around ten-degree boundaries?
- Are hot water and fuel-source actions toggles, separate idempotent commands, or state writes?

These questions do not block the first pass as long as the implementation stays conservative and evidence-driven.
