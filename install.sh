#!/usr/bin/env bash
set -euo pipefail

die() { printf 'Install failed: %s\n' "$*" >&2; exit 1; }

command -v go >/dev/null 2>&1 || die 'Go is required. Install Go, then run this script again.'
: "${HOME:?HOME must be set}"

work_dir=$(mktemp -d)
trap 'rm -rf "$work_dir"' EXIT

# A checkout installs its own source; a downloaded or piped script fetches main.
source_dir=''
if [[ -n ${BASH_SOURCE[0]:-} ]]; then
  script_dir=$(cd -- "$(dirname -- "${BASH_SOURCE[0]}")" && pwd)
  if [[ -f "$script_dir/go.mod" && -f "$script_dir/cmd/ccfg/main.go" ]]; then
    source_dir=$script_dir
  fi
fi
if [[ -z "$source_dir" ]]; then
  command -v git >/dev/null 2>&1 || die 'Git is required to download the source.'
  git clone --quiet --depth 1 --branch main \
    https://github.com/jorden-parker/claude-config-cli.git "$work_dir/source"
  source_dir=$work_dir/source
fi

printf 'Building claude-config-cli and ccfg…\n'
(
  cd "$source_dir"
  go build -o "$work_dir/claude-config-cli" .
  go build -o "$work_dir/ccfg" ./cmd/ccfg
)

install_dir="$HOME/.local/bin"
mkdir -p "$install_dir"
install -m 755 "$work_dir/claude-config-cli" "$install_dir/claude-config-cli"
install -m 755 "$work_dir/ccfg" "$install_dir/ccfg"

add_path() {
  local config=$1 line=$2
  mkdir -p "$(dirname -- "$config")"
  if ! grep -Fqx -- "$line" "$config" 2>/dev/null; then
    printf '\n# Added by claude-config-cli installer\n%s\n' "$line" >> "$config"
  fi
  printf 'PATH configured in %s\n' "$config"
}

path_line='case ":$PATH:" in *":$HOME/.local/bin:"*) ;; *) export PATH="$HOME/.local/bin:$PATH" ;; esac'
user_shell=${SHELL:-sh}
case "${user_shell##*/}" in
  zsh)
    add_path "${ZDOTDIR:-$HOME}/.zshrc" "$path_line"
    ;;
  bash)
    add_path "$HOME/.bashrc" "$path_line"
    # Bash reads only the first existing login profile.
    profile="$HOME/.profile"
    if [[ -f "$HOME/.bash_profile" ]]; then
      profile="$HOME/.bash_profile"
    elif [[ -f "$HOME/.bash_login" ]]; then
      profile="$HOME/.bash_login"
    fi
    add_path "$profile" "$path_line"
    ;;
  fish)
    add_path "${XDG_CONFIG_HOME:-$HOME/.config}/fish/conf.d/claude-config-cli.fish" \
      'fish_add_path "$HOME/.local/bin"'
    ;;
  *)
    add_path "$HOME/.profile" "$path_line"
    printf 'For other shells, add ~/.local/bin to PATH in your shell configuration.\n'
    ;;
esac

printf '\nInstalled both commands in %s\n' "$install_dir"
printf 'Open a new terminal, then run: ccfg --help\n'
printf 'Or run now: "%s/ccfg" --help\n' "$install_dir"
