---
name: superllm-deploy
description: Deploy, upgrade, verify, and roll back SuperLLM on Linux systemd servers, including SSH preflight, GitHub Release installation, PostgreSQL and Redis configuration, Sub2API readonly identity integration, Nginx or BaoTa reverse proxy setup, TLS, health checks, login smoke tests, logs, and rollback. Use when the user asks to deploy SuperLLM to a new server, upgrade an existing server, configure a domain or reverse proxy, verify production, or provide server deployment information.
---

# SuperLLM Deploy

## Overview

Deploy SuperLLM from official Linux GitHub Release assets and verify the complete production path. Treat server addresses, domains, credentials, ports, and database topology as deployment-specific facts supplied by the user; never inherit them from an older project or server.

## Required Reference

Read [references/deploy-runbook.md](references/deploy-runbook.md) completely before running deployment commands. Use its fact table, input checklist, command templates, verification gates, and rollback procedure.

## Operating Rules

- Default to the systemd binary installation. Do not use local Docker.
- Preserve a co-located `/opt/sub2api/sub2api` binary and `sub2api.service`; they are the authoritative identity source, not a legacy SuperLLM installation.
- Support only Linux `amd64` and `arm64` Release assets.
- Keep the SuperLLM primary PostgreSQL database independent and named `superllm` unless the user explicitly supplies a different dedicated database.
- Connect user identity and linked Sub2API data through `SUB2API_READONLY_DATABASE_URL` or `sub2api.readonly_database_url`; never create or fall back to a separate SuperLLM user system.
- Permit login only for Sub2API users whose role is administrator and whose status is active.
- Treat Sub2API Redis access as optional and readonly. Share the Sub2API TOTP encryption key only when existing administrators use TOTP.
- Do not print, repeat, commit, or persist passwords, tokens, private keys, database URLs with credentials, or TOTP keys in logs or reports. Redact values when discussing configuration.
- Do not place secrets in shell history, process arguments, generated Skill files, or Git-tracked files. Prefer an interactive secret prompt or an existing root-readable server-side secret/config file.
- Perform read-only discovery first. Before package installation, database creation or grants, firewall changes, service replacement/restart, Nginx changes, destructive cleanup, rollback, Git commit, or push, show the exact impact and obtain explicit confirmation.
- Do not deploy Railway or publish a release unless the user explicitly asks for that separate action.
- Preserve unrelated services, databases, reverse-proxy hosts, and user changes.

## Workflow

### 1. Collect Deployment Facts

Use the input checklist in the runbook. Ask only for facts that cannot be safely discovered. It is acceptable to receive an SSH alias instead of a raw key path. Never ask the user to paste a private key.

Do not begin mutation until these facts are known: SSH target, OS and architecture, domain, local listen address, reverse-proxy type, TLS method, dedicated SuperLLM database connection, Redis connection, Sub2API readonly database connection, target version, and backup expectation.

### 2. Run Read-Only Preflight

Verify SSH access, supported Linux architecture, systemd, required commands, available disk and memory, port ownership, existing SuperLLM layout, PostgreSQL/Redis reachability, reverse-proxy layout, DNS resolution, and current service health. Redact connection strings and secret-bearing environment values.

Report conflicts before changing anything. In particular, do not take over a port already owned by another service.

### 3. Present the Mutation Plan

State the selected version, downloaded asset, installation path, service name, listen address, database target, reverse-proxy file, backup paths, expected downtime, and rollback command. Request explicit confirmation using the repository's dangerous-operation confirmation format.

### 4. Back Up and Install

Back up `/etc/superllm`, the current binary, the systemd unit, the reverse-proxy host config, and the SuperLLM PostgreSQL database when they exist. A binary backup does not replace a database backup.

Install with the official one-click installer or `sudo superllm upgrade -v <version>`. Never bypass the published SHA-256 checksum. Keep the service bound to `127.0.0.1` when it is behind a local reverse proxy.

### 5. Configure Integrations

Configure the dedicated SuperLLM primary database, application Redis, required Sub2API readonly PostgreSQL URL, optional Sub2API readonly Redis, stable JWT secret, and shared TOTP key when needed. Ensure files containing secrets are owned by root or the `superllm` service user and are not world-readable.

Validate the readonly database account can `SELECT` the required Sub2API tables but cannot write or perform DDL. Do not copy the Sub2API database into SuperLLM's primary database.

### 6. Configure Reverse Proxy and TLS

Point the dedicated domain to the local SuperLLM listener. Preserve forwarding headers, WebSocket upgrades, streaming responses, and long-running request timeouts. Validate Nginx or BaoTa configuration before reload. Confirm DNS and certificate validity rather than assuming propagation.

### 7. Verify End to End

Require all of these gates:

1. `superllm.service` is active and enabled.
2. Only the intended local address and port are listening.
3. Local `/health` returns HTTP 200.
4. Public `/health` and `/api/v1/settings/public` return HTTP 200 through TLS.
5. An active Sub2API administrator can log in; an inactive or non-admin user is denied when test accounts are available.
6. The frontend loads without blocking console or network errors.
7. Recent service and reverse-proxy logs contain no startup, database migration, Redis, authentication-source, or proxy errors.

Never send real credentials in a logged command. Use an interactive browser session or a temporary root-readable request file and remove it after the smoke test.

### 8. Report or Roll Back

Report the installed version, service state, local listener, public URL, database names with credentials redacted, verification results, backup locations, and remaining risks.

If a verification gate fails, stop further rollout. Collect logs, identify the root cause, and either repair within the approved scope or request confirmation to roll back. Application rollback does not reverse database migrations; use the prepared database backup when migration compatibility requires it.

## Completion Criteria

The deployment is complete only when systemd, local health, public TLS health, public settings, Sub2API administrator login, frontend loading, and logs have all been checked. Installation success alone is not completion.
