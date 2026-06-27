#!/usr/bin/env bash
# deploy_nas.sh — Build BlazeAI, deploy and install on a remote server.
# Usage: ./deploy_nas.sh user@host
set -euo pipefail

[ $# -eq 1 ] || { printf 'Usage: %s user@host\n' "$0" >&2; exit 1; }
remote="$1"

printf 'Building linux/amd64 binary...\n'
CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -trimpath -ldflags="-s -w" -o /tmp/blazeai-bin .

printf 'Packaging self-contained installer...\n'
b64=$(base64 -w0 /tmp/blazeai-bin)
mkdir -p /tmp/blazeai_deploy
cat > /tmp/blazeai_deploy/install.sh << 'SCRIPT'
#!/usr/bin/env bash
set -euo pipefail
fail() { printf '\033[31mblazeai: %s\033[0m\n' "$*" >&2; exit 1; }
target_dir="${HOME}/.local/bin"
target_bin="${target_dir}/blazeai"
profile_file="${HOME}/.profile"
temp_bin="$(mktemp)"
trap 'rm -f "$temp_bin"' EXIT
script_self="$(CDPATH= cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)/$(basename -- "${BASH_SOURCE[0]}")"
awk 'found{print} /^# __BLAZEAI_BINARY__$/{found=1}' "$script_self" | base64 -d > "$temp_bin"
[ -s "$temp_bin" ] || fail "embedded binary missing or corrupt"
chmod 0755 "$temp_bin"
mkdir -p -- "$target_dir"
rm -f -- "$target_bin"
cp -- "$temp_bin" "$target_bin"
chmod 0755 "$target_bin"
case ":${PATH}:" in
  *":${target_dir}:"*) ;;
  *)
    mkdir -p -- "$(dirname -- "$profile_file")"
    if [ ! -f "$profile_file" ] || ! grep -Fq '# BlazeAI PATH' "$profile_file"; then
      {
        printf '\n# BlazeAI PATH\n'
        printf 'if [ -d "$HOME/.local/bin" ]; then\n'
        printf '  PATH="$HOME/.local/bin:$PATH"\n'
        printf 'fi\n'
        printf 'export PATH\n'
      } >> "$profile_file"
    fi
    ;;
esac
printf '\033[1;32mBlazeAI installed to %s\033[0m\n' "$target_bin"
printf 'Open a new shell or run: source %s\n' "$profile_file"
exit 0
# __BLAZEAI_BINARY__
SCRIPT
echo "$b64" >> /tmp/blazeai_deploy/install.sh
chmod 0755 /tmp/blazeai_deploy/install.sh

printf 'Deploying to %s...\n' "$remote"
scp /tmp/blazeai_deploy/install.sh "${remote}:~/blazeai_installer/install.sh"

printf 'Installing on remote...\n'
ssh "$remote" "cd ~/blazeai_installer && ./install.sh"

printf 'Done.\n'
