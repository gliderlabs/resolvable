#!/bin/bash
set -eo pipefail

NAME="dnsmasq"
VERSION="2.72"
FULLNAME="$NAME-$VERSION"
ARCHIVE="$FULLNAME.tar.gz"
SHA256="635f1b47417d17cf32e45cfcfd0213ac39fd09918479a25373ba9b2ce4adc05d"

if [ ! -e "$FULLNAME/src/$NAME" ]; then
  wget "http://www.thekelleys.org.uk/dnsmasq/$ARCHIVE"
  echo "$SHA256  $ARCHIVE" | shasum -c
  tar xzf "$ARCHIVE"
  make -C "$FULLNAME"
fi

install -m 755 "$FULLNAME/src/$NAME" ~/bin