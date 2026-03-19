# P2P API Shared Platform

## Overview

P2P API sharing system built on top of CLIProxyAPI.

Current implementation status as of 2026-03-19:

- The P2P module is no longer a standalone draft; it is wired into the main service lifecycle.
- Shared traffic is isolated from the default proxy traffic through a dedicated request pool.
- Verified contributed providers are synced into the runtime auth/model registry automatically.
- Shared usage is persisted and used for contribution / consumption ratio enforcement.

## Completed

- Rebuilt `internal/p2p/models.go`
- Rebuilt `internal/p2p/store.go`
- Rebuilt `internal/p2p/verifier.go`
- Rebuilt `internal/p2p/handlers.go`
- Rebuilt `internal/p2p/frontend.go`
- Rebuilt `internal/p2p/init.go`
- Added `internal/p2p/access.go` for shared-key authentication
- Added `internal/p2p/module.go` for runtime integration, provider sync, and route registration
- Added `internal/p2p/usage.go` for usage persistence plugin
- Integrated P2P into `sdk/cliproxy/service.go`
- Added dedicated shared API namespaces: `/p2p/v1` and `/p2p/v1beta`
- Added request-pool metadata propagation in request execution
- Added request-pool-aware auth filtering in conductor and scheduler
- Added runtime model reconstruction for dynamically synced P2P providers
- Added hourly 1.2x suspension guard
- Added platform overview endpoint and updated P2P frontend dashboard
- Added request-pool selection tests

## Current behavior

- `/p2p/register` registers a contributed upstream credential and issues a `sk-p2p-*` shared key
- Provider verification runs in the background before the provider joins the runtime pool
- Shared OpenAI-compatible traffic uses `/p2p/v1/*`
- Shared Gemini-compatible traffic uses `/p2p/v1beta/*`
- Shared requests carry `request_pool=p2p` metadata so they do not consume normal runtime credentials
- Usage records update:
  contributed tokens for provider owner
  consumed tokens and request count for shared-key user
- Over-limit accounts are suspended by hourly enforcement when consumed > contributed * 1.2

## TODO

1. Move bootstrap settings from env-only wiring into first-class `config.yaml` fields
2. Add scheduled provider re-verification / health checks after initial registration
3. Add admin-protected monitoring, audit, and manual recovery operations
4. Add end-to-end tests against a real Postgres-backed environment
5. Add startup validation and documentation for deployments without Go toolchain ambiguity

## Database

- `p2p_users`
- `p2p_providers`
- `p2p_usage_records`
- `p2p_daily_stats`

## Runtime bootstrap

Required:

- `P2P_PG_DSN` or `P2P_POSTGRES_DSN`

Optional:

- `P2P_REQUEST_POOL` default `p2p`
- `P2P_SYNC_INTERVAL` default `1m`
- `P2P_ENFORCEMENT_INTERVAL` default `1h`
- `P2P_SUSPEND_RATIO` default `1.2`
