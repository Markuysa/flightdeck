#!/usr/bin/env bash
# Setup script for Claude Code cloud sessions.
#
# Point your cloud environment at this file: claude.ai/code → environment settings →
# Setup script. It runs once per environment and the result is cached, so it does not
# re-run on every session.
#
# A cloud session is a fresh VM with your repository cloned and nothing installed beyond
# the base image (Node, Python, Go, Rust, Docker, Postgres, Redis and friends are already
# there). This script installs *your project's* dependencies.
#
# Network access is restricted by default to a package-registry allowlist. If a step here
# needs another host, add it under the environment's Allowed domains.

set -euo pipefail

echo "==> Installing dependencies"

if [ -f pnpm-lock.yaml ]; then
  corepack enable && pnpm install --frozen-lockfile
elif [ -f yarn.lock ]; then
  corepack enable && yarn install --frozen-lockfile
elif [ -f package-lock.json ]; then
  npm ci
elif [ -f package.json ]; then
  npm install
fi

if [ -f poetry.lock ]; then
  poetry install --no-interaction
elif [ -f requirements.txt ]; then
  pip install -r requirements.txt
fi

if [ -f go.mod ]; then
  go mod download
fi

if [ -f Cargo.toml ]; then
  cargo fetch
fi

# --- Project-specific steps -------------------------------------------------
# Add anything else the agent needs before it can run your tests: code generation,
# a database schema, seed data. Keep it idempotent — it may run more than once.
#
# Examples:
#   npx prisma generate
#   pg_isready -q || (pg_ctlcluster 16 main start && sleep 2)
#   npm run db:migrate

echo "==> Setup complete"
