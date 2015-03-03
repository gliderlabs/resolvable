#!/bin/bash
readonly COMMENT="# added by docker-resolver"

add-resolv-conf() {
	declare ip="$1" filename="$2"

	declare line="nameserver $ip $COMMENT"

	(echo "$line" ; cat "$filename") > "$filename.tmp"
	cat "$filename.tmp" > "$filename"
	rm "$filename.tmp"
}

clean-resolv-conf() {
	declare filename="$1"
	sed "/$COMMENT\$/d" "$filename" > "$filename.tmp"
	cat "$filename.tmp" > "$filename"
	rm "$filename.tmp"
}

main() {
	set -eo pipefail; [[ "$TRACE" ]] && set -x

	declare resolv_conf="${1:-/tmp/resolv.conf}"
	declare hostip="$(hostname -i | cut -f1)"
	# declare hostip="1.2.3.4"

	clean-resolv-conf "$resolv_conf"
	add-resolv-conf "$hostip" "$resolv_conf"

	trap 'kill -INT $child 2>/dev/null' SIGINT
	trap 'kill -TERM $child 2>/dev/null' SIGTERM
	trap 'clean-resolv-conf "$resolv_conf"' EXIT

	/bin/docker-resolver &

	child=$!
	wait "$child"
}

main "$@"
