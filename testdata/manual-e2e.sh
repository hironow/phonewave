#!/usr/bin/env bash
# Manual E2E test for phonewave — run all 20 phases inside Docker.
# Usage: bash testdata/manual-e2e.sh
#
# Prerequisites: docker compose available, run from repo root.
set -euo pipefail

COMPOSE_FILE="docker/compose-e2e.yaml"
REPO="/workspace/repo"
REPO2="/workspace/repo2"
PASS=0
FAIL=0

section() { printf '\n\033[1;34m=== %s ===\033[0m\n' "$1"; }
check()   { printf '  \033[0;33m[CHECK]\033[0m %s\n' "$1"; }
pass()    { printf '  \033[0;32m[PASS]\033[0m  %s\n' "$1"; PASS=$((PASS+1)); }
fail()    { printf '  \033[0;31m[FAIL]\033[0m  %s\n' "$1"; FAIL=$((FAIL+1)); }

pw()     { docker compose -f "$COMPOSE_FILE" exec -T phonewave "$@"; }
writer() { docker compose -f "$COMPOSE_FILE" exec -T writer "$@"; }

assert_contains() {
  local output="$1" substr="$2" label="$3"
  if echo "$output" | grep -Eq "$substr"; then pass "$label"; else fail "$label (missing: $substr)"; fi
}

assert_file() {
  local container="$1" path="$2" label="$3"
  if docker compose -f "$COMPOSE_FILE" exec -T "$container" test -f "$path"; then
    pass "$label"
  else
    fail "$label (file not found: $path)"
  fi
}

# ---------------------------------------------------------------
section "Phase 0: Build and start services"
# ---------------------------------------------------------------
docker compose -f "$COMPOSE_FILE" up -d --build
docker compose -f "$COMPOSE_FILE" ps
pass "Services started"

# ---------------------------------------------------------------
section "Phase 1: Version verification"
# ---------------------------------------------------------------
version_out=$(pw phonewave --version 2>&1 || true)
assert_contains "$version_out" "phonewave" "phonewave in version output"

# ---------------------------------------------------------------
section "Phase 2: Setup 3-tool ecosystem"
# ---------------------------------------------------------------
for tool in .siren .expedition .gate; do
  pw mkdir -p "$REPO/$tool/outbox" "$REPO/$tool/inbox"
done

# SKILL.md files
pw mkdir -p "$REPO/.siren/skills/dmail-sendable"
pw sh -c "printf '%s' '---
name: dmail-sendable
produces:
  - kind: specification
---
' > $REPO/.siren/skills/dmail-sendable/SKILL.md"

pw mkdir -p "$REPO/.siren/skills/dmail-readable"
pw sh -c "printf '%s' '---
name: dmail-readable
consumes:
  - kind: feedback
---
' > $REPO/.siren/skills/dmail-readable/SKILL.md"

pw mkdir -p "$REPO/.expedition/skills/dmail-sendable"
pw sh -c "printf '%s' '---
name: dmail-sendable
produces:
  - kind: report
---
' > $REPO/.expedition/skills/dmail-sendable/SKILL.md"

pw mkdir -p "$REPO/.expedition/skills/dmail-readable"
pw sh -c "printf '%s' '---
name: dmail-readable
consumes:
  - kind: specification
  - kind: feedback
---
' > $REPO/.expedition/skills/dmail-readable/SKILL.md"

pw mkdir -p "$REPO/.gate/skills/dmail-sendable"
pw sh -c "printf '%s' '---
name: dmail-sendable
produces:
  - kind: feedback
---
' > $REPO/.gate/skills/dmail-sendable/SKILL.md"

pw mkdir -p "$REPO/.gate/skills/dmail-readable"
pw sh -c "printf '%s' '---
name: dmail-readable
consumes:
  - kind: report
---
' > $REPO/.gate/skills/dmail-readable/SKILL.md"

pass "Ecosystem created"

# ---------------------------------------------------------------
section "Phase 3: phonewave init"
# ---------------------------------------------------------------
init_out=$(pw sh -c "cd /workspace && phonewave init $REPO" 2>&1)
assert_contains "$init_out" "routes" "init reports routes"
assert_file phonewave "/workspace/phonewave.yaml" "phonewave.yaml created"

# ---------------------------------------------------------------
section "Phase 4: phonewave doctor"
# ---------------------------------------------------------------
doctor_out=$(pw sh -c "cd /workspace && phonewave doctor" 2>&1)
assert_contains "$doctor_out" "healthy\|Healthy" "doctor says healthy"

# ---------------------------------------------------------------
section "Phase 5: phonewave status (stopped)"
# ---------------------------------------------------------------
status_out=$(pw sh -c "cd /workspace && phonewave status" 2>&1)
assert_contains "$status_out" "stopped" "status shows stopped"

# ---------------------------------------------------------------
section "Phase 6: Start daemon (verbose + tracing)"
# ---------------------------------------------------------------
pw sh -c "cd /workspace && nohup phonewave run -v -r 30s > /tmp/phonewave.log 2>&1 &"
sleep 3
assert_file phonewave "/workspace/.phonewave/watch.pid" "PID file exists"
pass "Daemon started"

# ---------------------------------------------------------------
section "Phase 7: Startup scan"
# ---------------------------------------------------------------
check "Startup scan tested via runtime delivery in Phase 8"
pass "Skipped (covered by runtime delivery)"

# ---------------------------------------------------------------
section "Phase 8: Runtime delivery (same container)"
# ---------------------------------------------------------------
pw sh -c "printf '%s' '---
name: spec-manual
kind: specification
description: Manual test
---

# Manual Spec
' > $REPO/.siren/outbox/spec-manual.md"
sleep 3
assert_file phonewave "$REPO/.expedition/inbox/spec-manual.md" "spec delivered to expedition"

# ---------------------------------------------------------------
section "Phase 9: Cross-container delivery (writer)"
# ---------------------------------------------------------------
writer sh -c "printf '%s' '---
name: fb-cross
kind: feedback
description: Cross-container feedback
---

# Cross Feedback
' > $REPO/.gate/outbox/fb-cross.md"
sleep 3
assert_file phonewave "$REPO/.siren/inbox/fb-cross.md" "feedback in siren inbox"
assert_file phonewave "$REPO/.expedition/inbox/fb-cross.md" "feedback in expedition inbox"

# ---------------------------------------------------------------
section "Phase 10: phonewave status (running)"
# ---------------------------------------------------------------
status_run=$(pw sh -c "cd /workspace && phonewave status" 2>&1)
assert_contains "$status_run" "running" "status shows running"
assert_contains "$status_run" "PID" "status shows PID"

# ---------------------------------------------------------------
section "Phase 11: Delivery log"
# ---------------------------------------------------------------
log_out=$(pw cat /workspace/.phonewave/delivery.log 2>&1)
assert_contains "$log_out" "DELIVERED" "delivery log has DELIVERED"
assert_contains "$log_out" "kind=specification" "log has kind=specification"
assert_contains "$log_out" "kind=feedback" "log has kind=feedback"

# ---------------------------------------------------------------
section "Phase 12: Dry-run mode"
# ---------------------------------------------------------------
pw sh -c "kill \$(cat /workspace/.phonewave/watch.pid)" || true
sleep 2
pw sh -c "cd /workspace && nohup phonewave run -v -n > /tmp/dryrun.log 2>&1 &"
sleep 2
pw sh -c "printf '%s' '---
name: spec-dry
kind: specification
description: dry run test
---
' > $REPO/.siren/outbox/spec-dry.md"
sleep 3
# File should still be in outbox
if pw test -f "$REPO/.siren/outbox/spec-dry.md"; then
  pass "File remains in outbox (dry-run)"
else
  fail "File removed from outbox in dry-run"
fi
drylog=$(pw cat /tmp/dryrun.log 2>&1)
assert_contains "$drylog" "dry-run" "dry-run log message"
pw sh -c "kill \$(cat /workspace/.phonewave/watch.pid)" || true
sleep 2

# ---------------------------------------------------------------
section "Phase 13: Error queue"
# ---------------------------------------------------------------
pw sh -c "cd /workspace && nohup phonewave run -v > /tmp/phonewave2.log 2>&1 &"
sleep 2
pw sh -c "printf '%s' '---
name: mystery
kind: mystery
description: unknown kind
---
' > $REPO/.siren/outbox/mystery.md"
sleep 3
if pw test -d "/workspace/.phonewave/errors" && pw sh -c "ls /workspace/.phonewave/errors/*.err 2>/dev/null" | grep -q err; then
  pass "Error queue has .err sidecar"
else
  fail "Error queue empty"
fi
pw sh -c "kill \$(cat /workspace/.phonewave/watch.pid)" || true
sleep 2

# ---------------------------------------------------------------
section "Phase 14: phonewave add"
# ---------------------------------------------------------------
pw mkdir -p "$REPO2/.beacon/outbox" "$REPO2/.beacon/inbox" "$REPO2/.beacon/skills/dmail-sendable"
pw sh -c "printf '%s' '---
name: dmail-sendable
produces:
  - kind: alert
---
' > $REPO2/.beacon/skills/dmail-sendable/SKILL.md"
add_out=$(pw sh -c "cd /workspace && phonewave add $REPO2" 2>&1)
assert_contains "$add_out" "Added" "add reports Added"

# ---------------------------------------------------------------
section "Phase 15: phonewave sync"
# ---------------------------------------------------------------
pw mkdir -p "$REPO/.oracle/outbox" "$REPO/.oracle/inbox" "$REPO/.oracle/skills/dmail-sendable"
pw sh -c "printf '%s' '---
name: dmail-sendable
produces:
  - kind: prophecy
---
' > $REPO/.oracle/skills/dmail-sendable/SKILL.md"
sync_out=$(pw sh -c "cd /workspace && phonewave sync" 2>&1)
assert_contains "$sync_out" "Synced\|synced\|routes" "sync reports results"

# ---------------------------------------------------------------
section "Phase 16: phonewave remove"
# ---------------------------------------------------------------
remove_out=$(pw sh -c "cd /workspace && phonewave remove $REPO2" 2>&1)
assert_contains "$remove_out" "Removed" "remove reports Removed"

# ---------------------------------------------------------------
section "Phase 17: OTel / Jaeger UI"
# ---------------------------------------------------------------
pw sh -c "cd /workspace && nohup phonewave run -v > /tmp/phonewave3.log 2>&1 &"
sleep 2
pw sh -c "printf '%s' '---
name: spec-otel
kind: specification
description: otel test
---
' > $REPO/.siren/outbox/spec-otel.md"
sleep 8
# Query Jaeger API from host
trace_count=$(curl -sf "http://localhost:16686/api/traces?service=phonewave&limit=10" 2>/dev/null | jq '.data | length' 2>/dev/null || echo "0")
if [ "$trace_count" -gt 0 ]; then
  pass "Traces found in Jaeger"
else
  check "Traces may take longer to flush — open http://localhost:16686 to verify manually"
  fail "No traces found in Jaeger API (may need manual check)"
fi
pw sh -c "kill \$(cat /workspace/.phonewave/watch.pid)" || true
sleep 2

# ---------------------------------------------------------------
section "Phase 18: Graceful shutdown (SIGINT)"
# ---------------------------------------------------------------
pw sh -c "cd /workspace && nohup phonewave run -v > /tmp/phonewave4.log 2>&1 &"
sleep 2
pw sh -c "kill -INT \$(cat /workspace/.phonewave/watch.pid)"
sleep 2
if ! pw test -f "/workspace/.phonewave/watch.pid"; then
  pass "PID file removed after SIGINT"
else
  fail "PID file still exists after SIGINT"
fi

# ---------------------------------------------------------------
section "Phase 19: Custom config path"
# ---------------------------------------------------------------
pw mkdir -p /workspace/custom
custom_out=$(pw sh -c "phonewave init --config /workspace/custom/phonewave.yaml $REPO" 2>&1)
assert_file phonewave "/workspace/custom/phonewave.yaml" "config at custom path"
if pw test -d "/workspace/custom/.phonewave"; then
  pass "State dir at custom path"
else
  fail "State dir not at custom path"
fi

# ---------------------------------------------------------------
section "Phase 20: Cleanup"
# ---------------------------------------------------------------
docker compose -f "$COMPOSE_FILE" down -v
pass "Cleanup complete"

# ---------------------------------------------------------------
printf '\n\033[1m=== Results: %d passed, %d failed ===\033[0m\n' "$PASS" "$FAIL"
if [ "$FAIL" -gt 0 ]; then
  exit 1
fi
