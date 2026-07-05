# configurator.core

## Design Rationale

The configurator is the source of runtime configuration for every microservice in the application. It loads
`config.yaml` from its working directory and serves each microservice only that microservice's own values over the
bus. (`config.local.yaml` is a git-ignored overlay used only in local development to keep secrets out of source
control; a production deployment carries the secrets in a single `config.yaml` delivered through the secrets
pipeline.) Operational deployment guidance (run several isolated instances, because it sits on every microservice's
boot path and its `config.yaml` aggregates secrets across microservices) lives in the production deployment guide;
this file captures the rationale a framework agent needs when touching the configurator or the config-fetch path.

### The config fetch is a hard boot dependency by design - do not add a fallback

Every microservice that defines a config property fetches its values from the configurator during `Startup`, before
`OnStartup` runs, and fails startup if the fetch fails. This looks like a single point of failure, and it is a
deliberate one. A microservice that cannot reach the configurator cannot distinguish "no override exists for me"
from "an override exists that I failed to fetch," so falling back to compiled-in defaults would start it on
silently-wrong configuration. In production the config file is not on the microservice's host anyway, and must not
be: the production `config.yaml` aggregates every microservice's secrets, and the configurator is the access-control
boundary that hands each caller only its own slice. So the fetch must block, and an unreachable configurator must fail
startup. Do not "fix" this by reading a local file or using defaults when the configurator is unreachable; the
reliability answer is operational (run the configurator highly available), not a code fallback.

### The configurator must not define any config properties of its own

`refreshConfig` runs before the configurator's own `Values` subscription is activated, so if the configurator
defined a config property it would try to fetch config from itself before it can answer, and deadlock its own
startup. It currently defines none, and it must stay that way - adding a `DefineConfig` to the configurator is a
self-inflicted boot deadlock. `TestConfigurator_NoConfigsOfItsOwn` guards this: it starts the service under a
renamed hostname next to a stand-in `configurator.core` and fails if startup fetches config from the stand-in,
which only happens when the service declares a config of its own.

### Configuration distribution is disabled in TESTING

`refreshConfig` skips the call to `configurator.core` when `deployment == TESTING`, using YAML defaults plus values
set via `SetConfig`. Tests override config with `SetConfig` / `ResetConfig` directly; outside TESTING those setters
error. This is why a test app does not need the configurator present.

### Distribution is push-triggered-pull, with no file watch

The configurator loads the config files once at its `OnStartup`, then multicasts `config-refresh` to tell every
microservice to pull its values from `:888/values`. There is no file-system watch: an edit to `config.yaml` on disk
takes effect only when the configurator is restarted (which re-triggers the refresh) or on the periodic refresh
below. The `PeriodicRefresh` ticker (20 minutes) is the only autonomous propagation path, so a config change made
without a configurator restart can take up to that interval to reach every microservice.

### Replicas reconcile by timestamp

Configurator replicas gossip their loaded repositories to each other via `SyncRepo` and pick a winner by comparing
the wall-clock time at which each loaded its files. There is no generation counter or content hash, so the
reconciliation assumes replica clocks are loosely synchronized: a replica whose clock runs fast while holding stale
config can win over a freshly-deployed replica during a rollout. Keep this in mind when changing `SyncRepo` - the
arbitration is time-based, not version-based.
