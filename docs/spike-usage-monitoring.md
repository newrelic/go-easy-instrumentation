# Spike: Usage Monitoring for go-easy-instrumentation

**Status:** implemented as option A (staging endpoint only). The spike text below is kept as
written. Where it differs from the shipped code, the code and the outcome notes here win.

## Outcome

What shipped, and where it differs from the spike:

- **Option A, not B.** Runs reuse `/usage` and land as `NRDiagRun` / `NRDiagTask`.
  `configProduct = go-easy-instrumentation` marks our rows, so every query must filter on it.
- **Staging endpoint only.** The repo has no build tooling for an `-ldflags` switch, since
  distribution is `go install`. `productionEndpoint` is defined but unused. The endpoint can be
  overridden for testing with `NEW_RELIC_USAGE_ENDPOINT`.
- **Opt-out names.** The flag is `--usage-opt-out`, matching nrdiag. The env var is
  `NEW_RELIC_USAGE_REPORTING_ENABLED=false`. The spike's `--no-usage-data` and
  `NEW_RELIC_DISABLE_USAGE_REPORTING` do not exist. There is no first-run notice.
- **Synchronous send, 3s timeout, no retry.** A detached goroutine never sends, because the
  process exits first. The send runs after the completion message, so a slow endpoint only delays
  process exit. `Send` takes `RunData` with no context.
- **The payload is tuned to four questions:** how many runs, how often and why they fail, which
  libraries applications use, and where instrumentation has gaps. Flag usage, diff size and the
  duplicate `interactive` attribute were dropped as not answering any of them.
- **Libraries are reported as `Go/Library/<name>` results,** matched from the application's
  direct imports against a fixed list in `internal/usage/libraries.go`. Status is `Success` when
  instrumented, `Warning` when supported but not instrumented, `Info` when no integration
  exists, and `None` when the outcome cannot be observed. Unlisted import paths are never sent.
- **Load failures carry `loadErrorKinds`,** the go/packages error kinds seen, so a toolchain
  problem can be told apart from code that does not build.
- **Not sent:** comment counts, files changed, and a failure category beyond the failed step.
  Cancelled runs report a single `Go/Pipeline/Cancelled` result.
- **`Go/Integration/newrelic` carries little signal.** The agent-injection step adds the base
  agent import on almost every run, so it means "something was instrumented", not a specific
  integration.
- **CI runs of this repo do not report.** The end-to-end workflow sets the opt-out env var. The
  tool does not auto-disable when `CI` is set, because CI usage by others is worth measuring.

**Drafted with:** Claude Code, from the linked nrdiag and help-agnostic-service source. Endpoint
behavior and schema claims come from reading that source, not model recall. Effort estimates and
the option-B recommendation are mine.

## What

At the end of every `go-easy-instrumentation` run, send one anonymous event describing what the
run did — over the same pipe [`newrelic-diagnostics-cli`](https://github.com/newrelic/newrelic-diagnostics-cli)
has used for years.

A new `internal/usage` package (~150 LOC, no new `go.mod` deps) POSTs a small JSON blob to an
already-public New Relic endpoint. That service records it as a custom event using the Go agent,
so **this repo needs no agent dependency, no license key, and no credentials of any kind**. The
call is fire-and-forget with a 5s timeout, and there is a flag and an env var to turn it off.

Nothing about the user's code leaves their machine — no source, no identifiers, no file paths, no
module path, no hostname, no username.

## Why

Right now we ship a tool with ten integrations and no idea which ones anyone uses. That's the main
thing: `nrgin`, `nrgrpc`, `nrpq`, `nrpgx5`, `nrslog`, `nrlogrus`, `nrnethttp`, `nrecho-v3`,
`nrecho-v4`, `nrgochi` all cost maintenance, and some are probably dead weight we could stop
carrying.

Four other things we'd learn cheaply along the way:

- **Where the pipeline breaks on real code.** A failed run in the wild is completely silent today.
- **Whether runs actually produce a usable diff**, or whether we mostly bail out with "can't
  instrument this" comments. That comment count is the real quality metric for the tool.
- **Whether anyone runs this in CI** — decides if the TUI or the plain-text path gets investment.
- **Which releases are in use**, so we know when old behavior can go.

The precedent is settled. nrdiag does exactly this, and the ingress route is already external and
unauthenticated, so there's very little new surface to argue about.

## How

### The existing path

nrdiag's [`SendUsageData`](https://github.com/newrelic/newrelic-diagnostics-cli/blob/main/usage/usage.go#L105)
POSTs JSON with four headers: `Content-Type`, `Usage-Protocol`, `Run-Id`, and `Insert-Key`. That
last one is not a real key — it's a SHA512 of the run ID with every-other character reversed
([`genInsertKey`](https://github.com/newrelic/newrelic-diagnostics-cli/blob/main/usage/usage.go#L86)),
and the service never validates it.

On the receiving side, [help-agnostic-service](https://source.datanerd.us/support-tools/help-agnostic-service)
unmarshals the body and calls
[`RecordCustomEvent`](https://source.datanerd.us/support-tools/help-agnostic-service/blob/master/internal/core/adapter/repository/helpers.go#L30) —
one `NRDiagRun` per run plus one `NRDiagTask` per task. Events land in the Support Tools
Engineering account (10013014, staging).

The route is already open to the world
([`ingressdomains.yml#L99`](https://source.datanerd.us/support-tools/help-agnostic-service/blob/master/grandcentral/ingressdomains.yml#L99):
`auth: unauthenticated`, 10s timeout, CORS on):

```
POST https://diagnostics.service.newrelic.com/usage
```

### Two things in the way

**The schema is nrdiag-shaped.** [`entity.Payload`](https://source.datanerd.us/support-tools/help-agnostic-service/blob/master/internal/core/domain/entity/usage.go)
expects nrdiag's exact fields, event types are hardcoded to `NRDiagRun`/`NRDiagTask`, version
writes to `nrdiagVersion`, and each result `Identifier` gets split on `/` into exactly three parts
in [`GetTaskResultsSet`](https://source.datanerd.us/support-tools/help-agnostic-service/blob/master/internal/core/adapter/repository/usage.go) —
a two-part identifier misindexes.

**We have no account to attribute runs to.** go-easy-instrumentation never touches a license key
or account ID (grep for `license`/`apiKey` across `cmd/`, `parser/`, `internal/` finds nothing), so
`licenseKeys` and `rpmApps` are always empty and `nrAccountId`/`nrAppId` never get set. Every event
is anonymous. For an OSS tool that's fine — arguably the point.

### Three ways to send

**A — reuse `/usage` as-is** (~1 day, CLI only). Map our data into nrdiag's shape: GEI version into
`nrdiagVersion`, flags into `configuration`, each integration as a result with a three-part
identifier like `Go/Integration/nrgin`. No service or GrandCentral change at all. Downside: our
rows sit inside `NRDiagRun`/`NRDiagTask`, so every query needs a marker filter.

**B — new route and event types** (~1 day CLI + ~2 days service + GC review). Add
`/usage/go-easy-instrumentation` writing `GoEasyInstrumentationRun` and `...Detection`, copying the
existing unauthenticated route block. Clean schema, own dashboards. **Recommended**, with A as the
fallback if the coordination isn't worth it for v1.

**C — post straight to the Insights custom-event API** (~0.5 day). Needs an ingest key baked into a
public OSS binary. Rejected.

### The CLI change

```
internal/usage/
  usage.go      // payload structs, Send(ctx, RunData) — fire and forget
  runid.go      // random run ID + genInsertKey, same algorithm as nrdiag
  endpoint.go   // endpoint const, overridable via -ldflags and NR_GEI_USAGE_ENDPOINT
```

It hooks in at the end of [`cmd/instrument.go`](../cmd/instrument.go) — in both
`instrumentPackages` (text/CI) and the `runTUIMode` goroutine, after `WriteDiff`. It has to fire on
the error path too, or we lose exactly the runs we most want to see.

Non-negotiable: own goroutine, 5s timeout, `recover()` around it. A usage failure must never change
the exit code or delay the diff. Errors go to `debug` output only.

What we'd send: tool version, a random run ID, Go version / OS / arch, mode (`tui`/`text`/
`interactive`), which flags were set, packages loaded, **one row per integration applied**, info and
warning comment counts (needs counters in `internal/comment`), files changed and diff size, run
duration, and on failure the pipeline step name plus the error type. Hash the module path if we
ever need an opaque "same project" key.

Opt-out is mandatory for an OSS CLI, both forms: a `--no-usage-data` persistent flag and
`NEW_RELIC_DISABLE_USAGE_REPORTING=1` so CI can kill it globally. Plus a README section and a
one-line stderr notice on first run.

### Proving it works — half a day

`curl` an nrdiag-shaped fixture at the live endpoint with an obviously fake
`nrdiagVersion: "gei-spike"`, then check `SELECT * FROM NRDiagRun WHERE nrdiagVersion = 'gei-spike'`
in account 10013014 staging. That confirms all three unknowns at once: reachable off-VPN, no auth
needed, schema accepted. Then commit to A or B.

## Open questions

- Staging Support Tools account, or a production account with its own dashboard? The
  [nrdiag usage docs](https://source.datanerd.us/support-tools/help-agnostic-service/blob/master/docs/usage.md)
  point at staging only.
- Who owns the service change and the GrandCentral ingress review for option B?
- Per-integration success/failure, or is "detected" enough for v1?
- Opt-out (assumed here, matches nrdiag) or opt-in? Opt-in shrinks the numbers a lot, and most of
  the value with them.
- Do we want nrdiag's survey-link response, where the service returns a prompt and URI for the CLI
  to print? Nearly free if we reuse `/usage`.

## Effort

Validation spike 0.5 day · CLI package + hooks + opt-out + tests 1–1.5 days · service route and
event types 2 days plus GC review (option B only) · dashboard and README 0.5 day.

## References

**newrelic-diagnostics-cli** (public)
- [`usage/usage.go`](https://github.com/newrelic/newrelic-diagnostics-cli/blob/main/usage/usage.go) — client payload, headers, `genInsertKey`

**help-agnostic-service** (`source.datanerd.us`, requires GHE access)
- [`handler/usage.go`](https://source.datanerd.us/support-tools/help-agnostic-service/blob/master/internal/core/adapter/handler/usage.go) — route handler
- [`repository/usage.go`](https://source.datanerd.us/support-tools/help-agnostic-service/blob/master/internal/core/adapter/repository/usage.go) — attribute mapping
- [`repository/helpers.go#L30`](https://source.datanerd.us/support-tools/help-agnostic-service/blob/master/internal/core/adapter/repository/helpers.go#L30) — `RecordCustomEvent`
- [`usecase/usage.go`](https://source.datanerd.us/support-tools/help-agnostic-service/blob/master/internal/core/application/usecase/usage.go) — license key lookup, account resolution
- [`entity/usage.go`](https://source.datanerd.us/support-tools/help-agnostic-service/blob/master/internal/core/domain/entity/usage.go) — payload schema
- [`grandcentral/ingressdomains.yml#L99`](https://source.datanerd.us/support-tools/help-agnostic-service/blob/master/grandcentral/ingressdomains.yml#L99) — unauthenticated `/usage` route
- [`docs/usage.md`](https://source.datanerd.us/support-tools/help-agnostic-service/blob/master/docs/usage.md) — existing attribute list + dashboards
