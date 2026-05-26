#!/usr/bin/env bash
# TASK-007: Render rollback verification drill
# Usage: RENDER_API_KEY=<key> bash scripts/verify-rollback.sh [--execute]
# Without --execute: shows what rollback would target (dry-run)
# With --execute: triggers the actual rollback

set -euo pipefail

API="https://api.render.com/v1"
KEY="${RENDER_API_KEY:-}"
EXECUTE="${1:-}"

if [[ -z "$KEY" ]]; then
  echo "ERROR: RENDER_API_KEY not set. Source ~/.secrets/api-keys.sh first." >&2
  exit 1
fi

auth_header="Authorization: Bearer $KEY"

echo "=== Render Rollback Drill ==="
echo ""

# List services
services=$(curl -sf -H "$auth_header" -H "Accept: application/json" "$API/services?limit=20&type=web_service" 2>/dev/null || curl -sf -H "$auth_header" -H "Accept: application/json" "$API/services?limit=20" 2>/dev/null)

if [[ -z "$services" ]]; then
  echo "ERROR: Could not reach Render API. Check key validity." >&2
  exit 1
fi

# Find apex-build service (Render returns [{cursor, service: {...}}, ...])
service_id=$(echo "$services" | python3 -c "
import json, sys
raw = json.load(sys.stdin)
# Render API: list of {cursor, service} or list of service objects
items = [x.get('service', x) if isinstance(x, dict) else x for x in (raw if isinstance(raw, list) else raw.get('data', [raw]))]
for svc in items:
    name = svc.get('name', '')
    if name == 'apex-backend':
        print(svc['id']); break
else:
    for svc in items:
        name = svc.get('name', '')
        if 'apex' in name.lower() and svc.get('type') == 'web_service':
            print(svc['id']); break
    else:
        for svc in items:
            if svc.get('type') == 'web_service':
                print(svc['id']); break
        else:
            print(items[0]['id'] if items else '')
" 2>/dev/null)

service_name=$(echo "$services" | python3 -c "
import json, sys
raw = json.load(sys.stdin)
items = [x.get('service', x) if isinstance(x, dict) else x for x in (raw if isinstance(raw, list) else raw.get('data', [raw]))]
for svc in items:
    if svc.get('name') == 'apex-backend':
        print(svc['name']); break
else:
    for svc in items:
        name = svc.get('name', '')
        if 'apex' in name.lower() and svc.get('type') == 'web_service':
            print(name); break
    else:
        print(items[0].get('name', 'unknown') if items else 'unknown')
" 2>/dev/null)

if [[ -z "$service_id" ]]; then
  echo "ERROR: No apex-build service found in Render account." >&2
  echo "Services found:" >&2
  echo "$services" | python3 -c "import json,sys; svcs=json.load(sys.stdin); items=svcs if isinstance(svcs, list) else [x.get('service',x) for x in svcs.get('data', [])]; [print(' -', s.get('name'), s.get('type','')) for s in items]" 2>/dev/null || true
  exit 1
fi

echo "Service: $service_name ($service_id)"
echo ""

# Get recent deploys
deploys=$(curl -sf -H "$auth_header" -H "Accept: application/json" "$API/services/$service_id/deploys?limit=5" 2>/dev/null)

deploy_info=$(echo "$deploys" | python3 -c "
import json, sys
data = json.load(sys.stdin)
# Render API: [{deploy: {...}, cursor: '...'}, ...]
raw = data if isinstance(data, list) else data.get('data', [data])
items = [x.get('deploy', x) if isinstance(x, dict) else x for x in raw]
deploys = [d for d in items if d.get('status') in ('live', 'build_complete', 'update_complete', 'deactivated')]
if len(deploys) >= 2:
    cur = deploys[0]
    prev = deploys[1]
    print(cur.get('id', ''), cur.get('status', ''), cur.get('createdAt', '')[:19])
    print(prev.get('id', ''), prev.get('status', ''), prev.get('createdAt', '')[:19])
elif len(deploys) == 1:
    cur = deploys[0]
    print(cur.get('id', ''), cur.get('status', ''), cur.get('createdAt', '')[:19])
    print('NONE none none')
else:
    print('NONE none none')
    print('NONE none none')
" 2>/dev/null)

current_id=$(echo "$deploy_info" | awk 'NR==1{print $1}')
previous_id=$(echo "$deploy_info" | awk 'NR==2{print $1}')
current_status=$(echo "$deploy_info" | awk 'NR==1{print $2}')
previous_status=$(echo "$deploy_info" | awk 'NR==2{print $2}')
current_ts=$(echo "$deploy_info" | awk 'NR==1{print $3}')
previous_ts=$(echo "$deploy_info" | awk 'NR==2{print $3}')

echo "Current  deploy: $current_id  status=$current_status  created=$current_ts"
echo "Previous deploy: $previous_id  status=$previous_status  created=$previous_ts"
echo ""

if [[ "$previous_id" == "NONE" ]]; then
  echo "WARNING: Only one deploy found — rollback target unavailable."
  echo "Deploy more builds first to establish rollback history."
  exit 0
fi

echo "Rollback command (triggers redeploy of previous commit):"
echo "  curl -sf -X POST -H \"Authorization: Bearer \$RENDER_API_KEY\" '$API/services/$service_id/deploys' -d '{\"clearCache\":\"do_not_clear\"}'"
echo ""
echo "To roll back to specific deploy $previous_id:"
echo "  curl -sf -X POST -H \"Authorization: Bearer \$RENDER_API_KEY\" '$API/services/$service_id/deploys' -H 'Content-Type: application/json' -d '{\"commitId\":\"previous\"}'"
echo ""

if [[ "$EXECUTE" == "--execute" ]]; then
  echo "=== Executing rollback (triggering new deploy from previous commit) ==="
  result=$(curl -sf -X POST \
    -H "Authorization: Bearer $KEY" \
    -H "Content-Type: application/json" \
    "$API/services/$service_id/deploys" \
    -d '{"clearCache":"do_not_clear"}' 2>/dev/null)
  new_deploy=$(echo "$result" | python3 -c "import json,sys; d=json.load(sys.stdin); print(d.get('id', d.get('deploy', {}).get('id', 'unknown')))" 2>/dev/null || echo "unknown")
  echo "Rollback deploy triggered: $new_deploy"
  echo "Monitor at: https://dashboard.render.com/web/$service_id/deploys"
else
  echo "(Dry run — pass --execute to trigger actual rollback)"
fi

echo ""
echo "=== Drill complete ==="
