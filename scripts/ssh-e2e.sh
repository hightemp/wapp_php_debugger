#!/bin/sh
# Runs the viewer against a real OpenSSH server in Docker (TestAgainstRealOpenSSH).
# A throwaway key is made for the run; the user's ~/.ssh and ssh-agent are not used.
set -eu

image=${SSH_E2E_IMAGE:-lscr.io/linuxserver/openssh-server:10.3_p1-r1-ls239}
work=$(mktemp -d)
name="wapp-debugger-sshd-$$"

cleanup() {
	docker rm -f "$name" >/dev/null 2>&1 || true
	rm -rf "$work"
}
trap cleanup EXIT INT TERM

# Registries rate-limit now and then: a few tries before giving up
pulled=0
for attempt in 1 2 3 4; do
	if docker pull -q "$image" >/dev/null; then
		pulled=1
		break
	fi
	echo "docker pull failed (attempt $attempt), retrying" >&2
	sleep $((attempt * 10))
done
if [ "$pulled" -ne 1 ]; then
	echo "cannot pull $image" >&2
	exit 1
fi

ssh-keygen -q -t ed25519 -N '' -C wapp-debugger-e2e -f "$work/id"
docker run -d --name "$name" \
	-e PUBLIC_KEY="$(cat "$work/id.pub")" -e USER_NAME=dev \
	-e PUID="$(id -u)" -e PGID="$(id -g)" \
	-p 127.0.0.1::2222 "$image" >/dev/null
port=$(docker port "$name" 2222/tcp | head -n 1 | sed 's/.*://')

# sshd is up once it shows its host keys
tries=0
until ssh-keyscan -p "$port" -T 2 127.0.0.1 >/dev/null 2>&1; do
	tries=$((tries + 1))
	if [ "$tries" -ge 60 ]; then
		echo "sshd did not start" >&2
		docker logs "$name" >&2
		exit 1
	fi
	sleep 1
done

cd "$(dirname "$0")/../server"
WAPP_E2E_SSH_ADDR="127.0.0.1:$port" WAPP_E2E_SSH_USER=dev WAPP_E2E_SSH_KEY="$work/id" \
	go test ./internal/app -run '^TestAgainstRealOpenSSH$' -count=1 -v
