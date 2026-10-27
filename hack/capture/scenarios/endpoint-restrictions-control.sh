# shellcheck shell=bash
# Control for endpoint-restrictions.sh: the same read-only routes outside the allow-list, called
# with the admin's API key on an instance without restrictions. They succeed here, so the 403s
# of the restricted capture come from the restriction. The destructive routes (models/sync,
# models/delete/all) are not repeated. Sourced by capture.sh; see hack/capture/lib.sh.

capture_scenario outside-the-list
capture_call session-user GET /api/v1/auths/ >/dev/null
capture_call list-users GET /api/v1/users/ >/dev/null
capture_call model-list-all GET /api/v1/models/all >/dev/null
capture_call openai-config GET /openai/config >/dev/null
capture_call files GET /api/v1/files/ >/dev/null
