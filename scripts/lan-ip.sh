#!/usr/bin/env bash
set -euo pipefail

is_private_ipv4() {
  local ip="${1:-}"
  local first second rest

  IFS=. read -r first second rest <<<"${ip}"
  [[ "${first:-}" == "10" ]] && return 0
  [[ "${first:-}" == "192" && "${second:-}" == "168" ]] && return 0
  [[ "${first:-}" == "172" && "${second:-}" =~ ^[0-9]+$ ]] \
    && (( second >= 16 && second <= 31 )) && return 0
  return 1
}

emit_if_private() {
  local ip="${1:-}"
  if is_private_ipv4 "${ip}"; then
    printf '%s\n' "${ip}"
    return 0
  fi
  return 1
}

# Prefer the address of the interface carrying the default route.
if command -v route >/dev/null 2>&1 && command -v ipconfig >/dev/null 2>&1; then
  default_interface="$(route -n get default 2>/dev/null | awk '/interface:/{print $2; exit}' || true)"
  if [[ -n "${default_interface}" ]]; then
    default_ip="$(ipconfig getifaddr "${default_interface}" 2>/dev/null || true)"
    emit_if_private "${default_ip}" && exit 0
  fi
fi

# macOS commonly uses en0/en1; checking these also works when route lookup is
# restricted by the current environment.
if command -v ipconfig >/dev/null 2>&1; then
  for interface in en0 en1 en2 en3; do
    interface_ip="$(ipconfig getifaddr "${interface}" 2>/dev/null || true)"
    emit_if_private "${interface_ip}" && exit 0
  done
fi

# Linux normally exposes all host addresses through hostname -I.
if command -v hostname >/dev/null 2>&1; then
  for candidate in $(hostname -I 2>/dev/null || true); do
    emit_if_private "${candidate}" && exit 0
  done
fi

# Final portable fallback for Unix-like systems.
if command -v ifconfig >/dev/null 2>&1; then
  while IFS= read -r candidate; do
    emit_if_private "${candidate}" && exit 0
  done < <(ifconfig 2>/dev/null | awk '/inet / {print $2}')
fi

exit 0
