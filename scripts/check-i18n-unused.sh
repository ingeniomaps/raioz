#!/usr/bin/env bash
# Fails when a locale catalog carries a key no Go source references.
#
# Why this exists: nothing else notices. A key whose caller is deleted
# stays in every catalog forever, and translators keep translating it. By
# v0.15.0 the catalogs had grown 232 such keys — a quarter of the file —
# including help text for commands that no longer exist.
#
# A key counts as used when the source contains its full name as a literal,
# OR a literal prefix of it ending in a dot. The prefix rule is what covers
# dynamic keys: `i18n.T("lang.name." + lang)` leaves only "lang.name." in
# the source, and lang.name.en / lang.name.es must not read as orphans.
#
# That rule is deliberately loose — a stray "up." literal would shield the
# whole family. It is the trade that keeps the gate from failing on correct
# code, and the alternative (parsing every call site) buys little: the one
# dynamic construction in the tree today is the lang list.
set -euo pipefail

cd "$(dirname "$0")/.."
echo "Checking for i18n keys nothing references..."

python3 - <<'PY'
import json, subprocess, sys, pathlib

catalog = pathlib.Path("internal/i18n/locales/en.json")
keys = json.loads(catalog.read_text())
src = subprocess.run(
    ["grep", "-rhoE", r'"[a-zA-Z0-9_]+(\.[a-zA-Z0-9_]*)+"', "--include=*.go", "internal/", "cmd/"],
    capture_output=True, text=True, check=False).stdout
used = {line.strip('"') for line in src.splitlines()}

prefixes = {u for u in used if u.endswith(".")}
orphans = sorted(
    k for k in keys
    if k not in used and not any(k.startswith(p) for p in prefixes)
)
if orphans:
    print(f"❌ {len(orphans)} key(s) in {catalog} that no source references:")
    for k in orphans:
        print(f"   {k}")
    print()
    print("   Delete them from every locale, or wire the caller that was meant to use them.")
    sys.exit(1)
print(f"✅ i18n keys: all {len(keys)} are referenced")
PY
